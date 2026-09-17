// Package pullrequest holds the PRadar domain policy: pull-request identity,
// input revisions, analysis identity, the pradar.analysis.v1 contract and the
// lifecycle decisions that turn a Forgejo observation into durable state.
//
// The package depends on no transport, storage or process API.
package pullrequest

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"time"
)

// SchemaVersion is the analysis contract version accepted by the demonstrator.
const SchemaVersion = "pradar.analysis.v1"

// State is the Forgejo lifecycle state of a pull request.
type State string

// Pull-request states known to PRadar.
const (
	StateOpen   State = "open"
	StateClosed State = "closed"
	StateMerged State = "merged"
)

// Ref identifies one pull request on the single configured Forgejo instance.
type Ref struct {
	Repository string // "owner/name"
	Number     int64
}

// Key is the durable identity used everywhere in PRadar ("owner/name#42").
func (r Ref) Key() string { return r.Repository + "#" + strconv.FormatInt(r.Number, 10) }

// ParseKey is the inverse of Ref.Key.
func ParseKey(key string) (Ref, error) {
	repository, number, ok := strings.Cut(key, "#")
	if !ok || repository == "" {
		return Ref{}, fmt.Errorf("invalid pull request key %q", key)
	}
	n, err := strconv.ParseInt(number, 10, 64)
	if err != nil || n <= 0 {
		return Ref{}, fmt.Errorf("invalid pull request key %q", key)
	}
	return Ref{Repository: repository, Number: n}, nil
}

// Observation is what one poll saw for one pull request.
type Observation struct {
	Ref       Ref
	Title     string
	Body      string
	Author    string
	State     State
	Draft     bool
	HeadSHA   string
	HTMLURL   string
	UpdatedAt time.Time
}

// InputRevision identifies every agreed analysis trigger: head, normalised
// title and description, and the reopen generation.
type InputRevision string

// ComputeInputRevision derives the deterministic revision of an observation.
func ComputeInputRevision(headSHA, title, body string, reopenGeneration int64) InputRevision {
	return InputRevision(digest(headSHA, normalise(title), normalise(body), strconv.FormatInt(reopenGeneration, 10)))
}

func normalise(text string) string {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t")
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

// Profile is the analysis configuration that participates in the identity.
type Profile struct {
	PromptVersion string `json:"prompt_version"`
	SkillVersion  string `json:"skill_version"`
	Engine        string `json:"engine"`
	Model         string `json:"model"`
}

// Validate rejects a profile with a missing component.
func (p Profile) Validate() error {
	if p.PromptVersion == "" || p.SkillVersion == "" || p.Engine == "" || p.Model == "" {
		return errors.New("analysis profile requires prompt, skill, engine and model versions")
	}
	return nil
}

// Identity is the idempotency key of one analysis.
type Identity string

// IdentityOf combines the pull request, its input revision and the profile.
func IdentityOf(ref Ref, revision InputRevision, profile Profile) Identity {
	return Identity(digest(ref.Key(), string(revision), profile.PromptVersion, profile.SkillVersion, profile.Engine, profile.Model))
}

func digest(parts ...string) string {
	hash := sha256.New()
	for _, part := range parts {
		_, _ = io.WriteString(hash, strconv.Itoa(len(part)))
		_, _ = io.WriteString(hash, ":")
		_, _ = io.WriteString(hash, part)
	}
	return hex.EncodeToString(hash.Sum(nil))
}

// Importance is the structural scale of a change, never its personal relevance.
type Importance string

// Importance levels of pradar.analysis.v1.
const (
	ImportanceLow    Importance = "low"
	ImportanceMedium Importance = "medium"
	ImportanceHigh   Importance = "high"
)

// AnalysisStatus is the outcome recorded by an analysis.
type AnalysisStatus string

// Analysis statuses of pradar.analysis.v1.
const (
	AnalysisOK          AnalysisStatus = "ok"
	AnalysisUnavailable AnalysisStatus = "unavailable"
)

// Analysis is the pradar.analysis.v1 contract produced by the analyzer.
type Analysis struct {
	SchemaVersion       string         `json:"schema_version"`
	PullRequest         string         `json:"pull_request"`
	HeadSHA             string         `json:"head_sha"`
	PreviousHeadSHA     string         `json:"previous_head_sha,omitempty"`
	Status              AnalysisStatus `json:"status"`
	Intent              string         `json:"intent"`
	Importance          Importance     `json:"importance"`
	Risks               []string       `json:"risks"`
	Body                string         `json:"body"`
	ChangeSincePrevious string         `json:"change_since_previous"`
}

// Validate checks the contract against the claimed pull-request version.
func (a Analysis) Validate(ref Ref, headSHA, previousHeadSHA string) error {
	switch {
	case a.SchemaVersion != SchemaVersion:
		return fmt.Errorf("unexpected analysis schema %q", a.SchemaVersion)
	case a.PullRequest != ref.Key():
		return fmt.Errorf("analysis targets %q, claimed %q", a.PullRequest, ref.Key())
	case a.HeadSHA != headSHA:
		return fmt.Errorf("analysis head %q does not match claimed head %q", a.HeadSHA, headSHA)
	case a.PreviousHeadSHA != previousHeadSHA:
		return fmt.Errorf("analysis previous head %q does not match %q", a.PreviousHeadSHA, previousHeadSHA)
	case a.Status == AnalysisUnavailable:
		return nil
	case a.Status != AnalysisOK:
		return fmt.Errorf("unexpected analysis status %q", a.Status)
	case strings.TrimSpace(a.Intent) == "":
		return errors.New("analysis intent is required")
	case !slices.Contains([]Importance{ImportanceLow, ImportanceMedium, ImportanceHigh}, a.Importance):
		return fmt.Errorf("unexpected importance %q", a.Importance)
	case a.Risks == nil:
		return errors.New("analysis risks must be present (possibly empty)")
	case strings.TrimSpace(a.Body) == "":
		return errors.New("analysis body is required")
	case previousHeadSHA != "" && strings.TrimSpace(a.ChangeSincePrevious) == "":
		return errors.New("change since the previous analysed head is required")
	}
	return nil
}

// Unavailable builds the terminal contract recorded after exhausted attempts.
func Unavailable(ref Ref, headSHA, previousHeadSHA string) Analysis {
	return Analysis{
		SchemaVersion:   SchemaVersion,
		PullRequest:     ref.Key(),
		HeadSHA:         headSHA,
		PreviousHeadSHA: previousHeadSHA,
		Status:          AnalysisUnavailable,
		Risks:           []string{},
	}
}

// Provenance explains exactly which inputs produced an analysis.
type Provenance struct {
	Profile       Profile        `json:"profile"`
	InputRevision InputRevision  `json:"input_revision"`
	Attempt       int            `json:"attempt"`
	StartedAt     time.Time      `json:"started_at"`
	Duration      time.Duration  `json:"duration_ns"`
	Usage         map[string]any `json:"usage,omitempty"`
	Failure       string         `json:"failure,omitempty"`
}

// Stored is the durable state the lifecycle decision needs about a pull request.
type Stored struct {
	Exists           bool
	State            State
	UpdatedAt        time.Time
	ReopenGeneration int64
	Revision         InputRevision
}

// DecisionKind is the durable effect of one observation.
type DecisionKind int

// Decision kinds returned by Reconcile.
const (
	DecideIgnore      DecisionKind = iota // older, duplicate, draft or unknown non-open observation
	DecideUpdateState                     // close or merge: state changes, nothing is scheduled
	DecideSchedule                        // a new input revision becomes an analysis candidate
)

// Decision is the outcome of reconciling one observation with stored state.
type Decision struct {
	Kind             DecisionKind
	ReopenGeneration int64
	Revision         InputRevision
}

// Reconcile is the pure lifecycle policy of ADR-0004: opening, reopening and
// head/title/description changes schedule; close and merge only update state;
// older or repeated observations are ignored.
func Reconcile(stored Stored, observation Observation) Decision {
	if stored.Exists && observation.UpdatedAt.Before(stored.UpdatedAt) {
		return Decision{Kind: DecideIgnore, ReopenGeneration: stored.ReopenGeneration, Revision: stored.Revision}
	}
	if observation.State != StateOpen {
		if stored.Exists && stored.State != observation.State {
			return Decision{Kind: DecideUpdateState, ReopenGeneration: stored.ReopenGeneration, Revision: stored.Revision}
		}
		return Decision{Kind: DecideIgnore, ReopenGeneration: stored.ReopenGeneration, Revision: stored.Revision}
	}
	if observation.Draft {
		return Decision{Kind: DecideIgnore, ReopenGeneration: stored.ReopenGeneration, Revision: stored.Revision}
	}
	generation := stored.ReopenGeneration
	if stored.Exists && stored.State != StateOpen {
		generation++
	}
	revision := ComputeInputRevision(observation.HeadSHA, observation.Title, observation.Body, generation)
	if stored.Exists && stored.State == StateOpen && revision == stored.Revision {
		return Decision{Kind: DecideIgnore, ReopenGeneration: generation, Revision: revision}
	}
	return Decision{Kind: DecideSchedule, ReopenGeneration: generation, Revision: revision}
}
