package forgejo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/clement-software/PRadar/internal/app"
	"github.com/clement-software/PRadar/internal/pullrequest"
)

// Token is the read-only Forgejo credential. It never prints itself: logs,
// %v, %#v and JSON all see a redaction marker.
type Token string

// String hides the credential from %v and %s.
func (Token) String() string { return "[redacted]" }

// GoString hides the credential from %#v.
func (Token) GoString() string { return "forgejo.Token([redacted])" }

// LogValue hides the credential from slog.
func (Token) LogValue() slog.Value { return slog.StringValue("[redacted]") }

// MarshalJSON hides the credential from exports.
func (Token) MarshalJSON() ([]byte, error) { return []byte(`"[redacted]"`), nil }

// redact replaces the token wherever it would leak through an error message.
func (t Token) redact(s string) string {
	if t == "" {
		return s
	}
	return strings.ReplaceAll(s, string(t), "[redacted]")
}

// APIError is a non-successful Forgejo response.
type APIError struct {
	Status  int
	Message string
}

func (e *APIError) Error() string {
	switch e.Status {
	case http.StatusUnauthorized:
		return "forgejo authentication failed (401): check the read-only token"
	case http.StatusForbidden:
		return "forgejo refused access (403): the token cannot read this repository"
	case http.StatusNotFound:
		return "forgejo repository or pull request not found (404)"
	}
	if e.Message != "" {
		return fmt.Sprintf("forgejo returned HTTP %d: %s", e.Status, e.Message)
	}
	return fmt.Sprintf("forgejo returned HTTP %d", e.Status)
}

// Is lets callers match a missing repository or pull request with app.ErrNotFound.
func (e *APIError) Is(target error) bool {
	return target == app.ErrNotFound && e.Status == http.StatusNotFound
}

// Transient reports whether the failure may succeed on retry.
func (e *APIError) Transient() bool {
	return e.Status == http.StatusTooManyRequests || e.Status >= 500
}

// Client is the bounded, read-only Forgejo API client.
type Client struct {
	instance Instance
	token    Token
	http     *http.Client
	// MaxBody bounds JSON responses; MaxDiff bounds a materialised diff.
	MaxBody int64
	MaxDiff int64
	// PageSize is the pagination limit requested from Forgejo.
	PageSize int
	// Attempts and Backoff bound transient retries.
	Attempts int
	Backoff  func(attempt int) time.Duration
}

// NewClient builds a client whose HTTP layer times out, follows at most a few
// same-origin redirects and never forwards the token elsewhere.
func NewClient(instance Instance, token Token) *Client {
	origin := instance.Origin()
	return &Client{
		instance: instance,
		token:    token,
		http: &http.Client{
			Timeout: 30 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 3 {
					return errors.New("too many redirects")
				}
				if req.URL.Scheme+"://"+req.URL.Host != origin {
					return fmt.Errorf("redirect to %s://%s leaves the configured instance", req.URL.Scheme, req.URL.Host)
				}
				return nil
			},
		},
		MaxBody:  4 << 20,
		MaxDiff:  2 << 20,
		PageSize: 50,
		Attempts: 3,
		Backoff:  JitteredBackoff(500 * time.Millisecond),
	}
}

// JitteredBackoff returns base·2^(attempt-1) plus up to 50% jitter.
func JitteredBackoff(base time.Duration) func(attempt int) time.Duration {
	return func(attempt int) time.Duration {
		delay := base << (attempt - 1)
		return delay + time.Duration(rand.Int64N(int64(delay)/2+1)) //nolint:gosec // jitter, not security
	}
}

func (c *Client) endpoint(path string, query url.Values) string {
	u := *c.instance.base
	u.Path = strings.TrimSuffix(u.Path, "/") + "/api/v1/" + path
	u.RawQuery = query.Encode()
	return u.String()
}

// get performs one bounded GET with transient retries and returns the body.
func (c *Client) get(ctx context.Context, endpoint, accept string, limit int64) ([]byte, bool, error) {
	var lastErr error
	for attempt := 1; attempt <= max(c.Attempts, 1); attempt++ {
		if attempt > 1 {
			select {
			case <-ctx.Done():
				return nil, false, ctx.Err()
			case <-time.After(c.Backoff(attempt - 1)):
			}
		}
		body, truncated, err := c.once(ctx, endpoint, accept, limit)
		if err == nil {
			return body, truncated, nil
		}
		lastErr = err
		var apiErr *APIError
		if ctx.Err() != nil || (errors.As(err, &apiErr) && !apiErr.Transient()) {
			return nil, false, err
		}
	}
	return nil, false, lastErr
}

func (c *Client) once(ctx context.Context, endpoint, accept string, limit int64) ([]byte, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, false, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Accept", accept)
	req.Header.Set("Authorization", "token "+string(c.token))
	response, err := c.http.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, false, fmt.Errorf("forgejo request cancelled: %w", ctx.Err())
		}
		return nil, false, fmt.Errorf("forgejo request failed: %s", c.token.redact(err.Error()))
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, limit+1))
	if err != nil {
		return nil, false, fmt.Errorf("read forgejo response: %w", err)
	}
	truncated := int64(len(body)) > limit
	if truncated {
		body = body[:limit]
	}
	if response.StatusCode != http.StatusOK {
		return nil, false, &APIError{Status: response.StatusCode, Message: c.token.redact(apiMessage(body))}
	}
	return body, truncated, nil
}

func apiMessage(body []byte) string {
	var payload struct {
		Message string `json:"message"`
	}
	if json.Unmarshal(body, &payload) != nil || payload.Message == "" {
		return ""
	}
	if len(payload.Message) > 200 {
		return payload.Message[:200]
	}
	return payload.Message
}

func (c *Client) getJSON(ctx context.Context, endpoint string, target any) error {
	body, truncated, err := c.get(ctx, endpoint, "application/json", c.MaxBody)
	if err != nil {
		return err
	}
	if truncated {
		return fmt.Errorf("forgejo response exceeds %d bytes", c.MaxBody)
	}
	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("decode forgejo response: %w", err)
	}
	return nil
}

func splitRepository(repository string) (owner, name string, err error) {
	owner, name, ok := strings.Cut(repository, "/")
	if !ok || !namePart.MatchString(owner) || !namePart.MatchString(name) {
		return "", "", fmt.Errorf("invalid repository %q", repository)
	}
	return owner, name, nil
}

// CheckRepository proves the token can read the repository.
func (c *Client) CheckRepository(ctx context.Context, repository string) error {
	owner, name, err := splitRepository(repository)
	if err != nil {
		return err
	}
	var payload struct {
		FullName string `json:"full_name"`
	}
	if err := c.getJSON(ctx, c.endpoint("repos/"+owner+"/"+name, nil), &payload); err != nil {
		return err
	}
	if !strings.EqualFold(payload.FullName, repository) {
		return fmt.Errorf("forgejo returned repository %q for %q", payload.FullName, repository)
	}
	return nil
}

type wirePullRequest struct {
	Number    int64     `json:"number"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	State     string    `json:"state"`
	Draft     bool      `json:"draft"`
	Merged    bool      `json:"merged"`
	HTMLURL   string    `json:"html_url"`
	UpdatedAt time.Time `json:"updated_at"`
	User      struct {
		Login string `json:"login"`
	} `json:"user"`
	Head struct {
		SHA string `json:"sha"`
	} `json:"head"`
}

func (w wirePullRequest) observation(repository string) (pullrequest.Observation, error) {
	if w.Number <= 0 || w.Head.SHA == "" || w.UpdatedAt.IsZero() {
		return pullrequest.Observation{}, fmt.Errorf("forgejo pull request %d lacks a number, head SHA or update time", w.Number)
	}
	state := pullrequest.State(w.State)
	switch {
	case w.Merged:
		state = pullrequest.StateMerged
	case state != pullrequest.StateOpen && state != pullrequest.StateClosed:
		return pullrequest.Observation{}, fmt.Errorf("forgejo pull request %d has unknown state %q", w.Number, w.State)
	}
	return pullrequest.Observation{
		Ref:   pullrequest.Ref{Repository: repository, Number: w.Number},
		Title: w.Title, Body: w.Body, Author: w.User.Login, State: state, Draft: w.Draft,
		HeadSHA: w.Head.SHA, HTMLURL: w.HTMLURL, UpdatedAt: w.UpdatedAt.UTC(),
	}, nil
}

// ListOpenPullRequests pages through every open pull request, most recently
// updated first, deterministically.
func (c *Client) ListOpenPullRequests(ctx context.Context, repository string) ([]pullrequest.Observation, error) {
	owner, name, err := splitRepository(repository)
	if err != nil {
		return nil, err
	}
	var observations []pullrequest.Observation
	const maxPages = 200
	for page := 1; page <= maxPages; page++ {
		query := url.Values{"state": {"open"}, "sort": {"recentupdate"}, "limit": {strconv.Itoa(c.PageSize)}, "page": {strconv.Itoa(page)}}
		var items []wirePullRequest
		if err := c.getJSON(ctx, c.endpoint("repos/"+owner+"/"+name+"/pulls", query), &items); err != nil {
			return nil, err
		}
		for _, item := range items {
			observation, err := item.observation(repository)
			if err != nil {
				return nil, err
			}
			observations = append(observations, observation)
		}
		// ponytail: an instance may cap the page below PageSize, so only an empty page ends the listing.
		if len(items) == 0 {
			return observations, nil
		}
	}
	return nil, fmt.Errorf("forgejo listing exceeded %d pages", maxPages)
}

// FetchPullRequest reads one pull request, including closed or merged ones.
func (c *Client) FetchPullRequest(ctx context.Context, ref pullrequest.Ref) (pullrequest.Observation, error) {
	owner, name, err := splitRepository(ref.Repository)
	if err != nil {
		return pullrequest.Observation{}, err
	}
	var item wirePullRequest
	if err := c.getJSON(ctx, c.endpoint("repos/"+owner+"/"+name+"/pulls/"+strconv.FormatInt(ref.Number, 10), nil), &item); err != nil {
		return pullrequest.Observation{}, err
	}
	if item.Number != ref.Number {
		return pullrequest.Observation{}, fmt.Errorf("forgejo returned pull request %d for %d", item.Number, ref.Number)
	}
	return item.observation(ref.Repository)
}

// FetchDiff reads the unified diff, truncated with a visible marker beyond MaxDiff.
func (c *Client) FetchDiff(ctx context.Context, ref pullrequest.Ref) ([]byte, error) {
	owner, name, err := splitRepository(ref.Repository)
	if err != nil {
		return nil, err
	}
	endpoint := c.endpoint("repos/"+owner+"/"+name+"/pulls/"+strconv.FormatInt(ref.Number, 10)+".diff", nil)
	body, truncated, err := c.get(ctx, endpoint, "text/plain", c.MaxDiff)
	if err != nil {
		return nil, err
	}
	if truncated {
		body = fmt.Appendf(body, "\n\n[diff truncated by PRadar after %d bytes]\n", c.MaxDiff)
	}
	return body, nil
}
