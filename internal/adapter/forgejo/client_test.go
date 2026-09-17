package forgejo_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/clement-software/PRadar/internal/adapter/forgejo"
	"github.com/clement-software/PRadar/internal/pullrequest"
)

const secret = "tok_super_secret_value"

type fakeForgejo struct {
	t        *testing.T
	mux      *http.ServeMux
	server   *httptest.Server
	requests atomic.Int32
}

func newFakeForgejo(t *testing.T) *fakeForgejo {
	t.Helper()
	f := &fakeForgejo{t: t, mux: http.NewServeMux()}
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.requests.Add(1)
		if r.Header.Get("Authorization") != "token "+secret {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = fmt.Fprintf(w, `{"message":"token %s rejected"}`, r.Header.Get("Authorization"))
			return
		}
		f.mux.ServeHTTP(w, r)
	}))
	t.Cleanup(f.server.Close)
	return f
}

func (f *fakeForgejo) client() *forgejo.Client {
	f.t.Helper()
	instance, err := forgejo.ParseInstance(f.server.URL, true)
	if err != nil {
		f.t.Fatal(err)
	}
	client := forgejo.NewClient(instance, secret)
	client.Backoff = func(int) time.Duration { return time.Millisecond }
	client.PageSize = 2
	return client
}

func pr(number int64, state string, updated time.Time) map[string]any {
	return map[string]any{
		"number": number, "title": fmt.Sprintf("PR %d", number), "body": "body", "state": state, "draft": false, "merged": false,
		"html_url": fmt.Sprintf("https://forge.test/acme/widgets/pulls/%d", number), "updated_at": updated.Format(time.RFC3339),
		"user": map[string]any{"login": "alice"}, "head": map[string]any{"sha": fmt.Sprintf("sha-%d", number)},
	}
}

func writeJSON(w http.ResponseWriter, v any) { _ = json.NewEncoder(w).Encode(v) }

func mustAtoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

func TestClient_AuthenticationAndURLMapping(t *testing.T) {
	t.Parallel()
	f := newFakeForgejo(t)
	f.mux.HandleFunc("GET /api/v1/repos/acme/widgets", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, map[string]any{"full_name": "acme/widgets"})
	})
	client := f.client()
	if err := client.CheckRepository(t.Context(), "acme/widgets"); err != nil {
		t.Fatalf("CheckRepository: %v", err)
	}
	for _, bad := range []string{"acme", "acme/../widgets", "acme/wid gets", ""} {
		if err := client.CheckRepository(t.Context(), bad); err == nil {
			t.Errorf("CheckRepository(%q) accepted", bad)
		}
	}
	var apiErr *forgejo.APIError
	err := client.CheckRepository(t.Context(), "acme/missing")
	if !errors.As(err, &apiErr) || apiErr.Status != http.StatusNotFound {
		t.Fatalf("missing repository = %v", err)
	}
	instance, _ := forgejo.ParseInstance(f.server.URL, true)
	wrong := forgejo.NewClient(instance, "wrong")
	err = wrong.CheckRepository(t.Context(), "acme/widgets")
	if !errors.As(err, &apiErr) || apiErr.Status != http.StatusUnauthorized || !strings.Contains(err.Error(), "401") {
		t.Fatalf("wrong token = %v", err)
	}
}

func TestClient_PaginationDraftsAndStates(t *testing.T) {
	t.Parallel()
	f := newFakeForgejo(t)
	now := time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)
	draft := pr(3, "open", now)
	draft["draft"] = true
	bot := pr(4, "open", now)
	bot["user"] = map[string]any{"login": "renovate[bot]"}
	open := []map[string]any{pr(1, "open", now), pr(2, "open", now.Add(-time.Minute)), draft, bot, pr(5, "open", now.Add(-time.Hour))}
	f.mux.HandleFunc("GET /api/v1/repos/acme/widgets/pulls", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("state") != "open" || r.URL.Query().Get("sort") != "recentupdate" {
			t.Errorf("unexpected query %s", r.URL.RawQuery)
		}
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		limit := min(2, mustAtoi(r.URL.Query().Get("limit")))
		start := min((page-1)*limit, len(open))
		end := min(start+limit, len(open))
		writeJSON(w, open[start:end])
	})
	merged := pr(9, "closed", now)
	merged["merged"] = true
	f.mux.HandleFunc("GET /api/v1/repos/acme/widgets/pulls/9", func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, merged) })
	f.mux.HandleFunc("GET /api/v1/repos/acme/widgets/pulls/8", func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, pr(8, "closed", now)) })
	f.mux.HandleFunc("GET /api/v1/repos/acme/widgets/pulls/7", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"number": 7, "state": "open"`))
	})
	client := f.client()
	client.PageSize = 10 // the fake caps pages at two items, like an instance with a lower MAX_RESPONSE_ITEMS
	observations, err := client.ListOpenPullRequests(t.Context(), "acme/widgets")
	if err != nil {
		t.Fatal(err)
	}
	if len(observations) != 5 || observations[0].Ref.Number != 1 || observations[4].Ref.Number != 5 {
		t.Fatalf("observations = %+v", observations)
	}
	if !observations[2].Draft || observations[3].Author != "renovate[bot]" || observations[0].HeadSHA != "sha-1" || !observations[1].UpdatedAt.Equal(now.Add(-time.Minute)) {
		t.Fatalf("mapping lost draft, author, head or time: %+v", observations)
	}
	if got, err := client.FetchPullRequest(t.Context(), pullrequest.Ref{Repository: "acme/widgets", Number: 9}); err != nil || got.State != pullrequest.StateMerged {
		t.Fatalf("merged = %+v, %v", got, err)
	}
	if got, err := client.FetchPullRequest(t.Context(), pullrequest.Ref{Repository: "acme/widgets", Number: 8}); err != nil || got.State != pullrequest.StateClosed {
		t.Fatalf("closed = %+v, %v", got, err)
	}
	if _, err := client.FetchPullRequest(t.Context(), pullrequest.Ref{Repository: "acme/widgets", Number: 7}); err == nil {
		t.Fatal("malformed JSON accepted")
	}
}

func TestClient_RetriesTransientFailuresOnly(t *testing.T) {
	t.Parallel()
	f := newFakeForgejo(t)
	var calls atomic.Int32
	f.mux.HandleFunc("GET /api/v1/repos/acme/widgets", func(w http.ResponseWriter, _ *http.Request) {
		switch calls.Add(1) {
		case 1:
			w.WriteHeader(http.StatusTooManyRequests)
		case 2:
			w.WriteHeader(http.StatusBadGateway)
		default:
			writeJSON(w, map[string]any{"full_name": "acme/widgets"})
		}
	})
	f.mux.HandleFunc("GET /api/v1/repos/acme/forbidden", func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusForbidden)
	})
	client := f.client()
	if err := client.CheckRepository(t.Context(), "acme/widgets"); err != nil || calls.Load() != 3 {
		t.Fatalf("transient retries: err=%v calls=%d", err, calls.Load())
	}
	calls.Store(0)
	if err := client.CheckRepository(t.Context(), "acme/forbidden"); err == nil || calls.Load() != 1 {
		t.Fatalf("403 must not retry: err=%v calls=%d", err, calls.Load())
	}
	always := newFakeForgejo(t)
	always.mux.HandleFunc("GET /", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusServiceUnavailable) })
	var apiErr *forgejo.APIError
	if err := always.client().CheckRepository(t.Context(), "acme/widgets"); !errors.As(err, &apiErr) || apiErr.Status != 503 || always.requests.Load() != 3 {
		t.Fatalf("exhausted retries = %v after %d requests", err, always.requests.Load())
	}
}

func TestClient_CancellationBoundedBodiesAndRedirects(t *testing.T) {
	t.Parallel()
	f := newFakeForgejo(t)
	release := make(chan struct{})
	f.mux.HandleFunc("GET /api/v1/repos/acme/slow", func(_ http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	})
	f.mux.HandleFunc("GET /api/v1/repos/acme/huge", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(bytes.Repeat([]byte("x"), 5<<20))
	})
	f.mux.HandleFunc("GET /api/v1/repos/acme/widgets/pulls/1.diff", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write(bytes.Repeat([]byte("d"), 3<<20))
	})
	other := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		t.Errorf("credential forwarded to another origin: %q", r.Header.Get("Authorization"))
	}))
	t.Cleanup(other.Close)
	f.mux.HandleFunc("GET /api/v1/repos/acme/elsewhere", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, other.URL+"/api/v1/repos/acme/elsewhere", http.StatusFound)
	})
	f.mux.HandleFunc("GET /api/v1/repos/acme/moved", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/api/v1/repos/acme/target", http.StatusFound)
	})
	f.mux.HandleFunc("GET /api/v1/repos/acme/target", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, map[string]any{"full_name": "acme/target"})
	})
	client := f.client()
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()
	if err := client.CheckRepository(ctx, "acme/slow"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("cancellation = %v", err)
	}
	close(release)
	if err := client.CheckRepository(t.Context(), "acme/huge"); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("oversized body = %v", err)
	}
	diff, err := client.FetchDiff(t.Context(), pullrequest.Ref{Repository: "acme/widgets", Number: 1})
	if err != nil || int64(len(diff)) > client.MaxDiff+200 || !bytes.Contains(diff, []byte("truncated by PRadar")) {
		t.Fatalf("diff bound: len=%d err=%v", len(diff), err)
	}
	if err := client.CheckRepository(t.Context(), "acme/elsewhere"); err == nil || !strings.Contains(err.Error(), "leaves the configured instance") {
		t.Fatalf("cross-origin redirect = %v", err)
	}
	if err := client.CheckRepository(t.Context(), "acme/moved"); err == nil || !strings.Contains(err.Error(), `"acme/target"`) {
		t.Fatalf("same-origin redirect must be followed and then checked: %v", err)
	}
}

func TestLogs_RedactForgejoToken(t *testing.T) {
	t.Parallel()
	var buffer bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buffer, nil))
	token := forgejo.Token(secret)
	log.Info("configured", "token", token, "printed", fmt.Sprint(token), "verbose", fmt.Sprintf("%#v %v %s", token, token, token))
	encoded, _ := json.Marshal(struct{ Token forgejo.Token }{token})
	if strings.Contains(buffer.String(), secret) || strings.Contains(string(encoded), secret) {
		t.Fatalf("token leaked: %s %s", buffer.String(), encoded)
	}

	f := newFakeForgejo(t)
	instance, _ := forgejo.ParseInstance(f.server.URL, true)
	err := forgejo.NewClient(instance, "wrong-"+secret).CheckRepository(t.Context(), "acme/widgets")
	if err == nil || strings.Contains(err.Error(), secret) {
		t.Fatalf("error message must not echo the credential: %v", err)
	}
}
