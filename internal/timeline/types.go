// Package timeline owns the reading experience and the evaluation: timeline
// and detail queries, read and archive, replay, corpus freezing and scoring.
// It declares the read-model and evaluation-store interfaces it consumes.
package timeline

import (
	"errors"
	"time"

	"github.com/clement-software/PRadar/internal/pullrequest"
)

// Errors returned by the read model and the reading use cases.
var (
	ErrNotFound        = errors.New("not found")
	ErrNotVisible      = errors.New("pull request has no visible carte")
	ErrReplayUnchanged = errors.New("replay requires a changed prompt, skill, engine or model")
)

// Card is the visible projection of one pull request in the timeline.
type Card struct {
	Ref       pullrequest.Ref
	Title     string
	Author    string
	State     pullrequest.State
	HTMLURL   string
	UpdatedAt time.Time
	Unread    bool
	Analysis  pullrequest.Analysis
}

// HistoryEntry is one analysis of the historique de pull request.
type HistoryEntry struct {
	Identity   pullrequest.Identity
	Analysis   pullrequest.Analysis
	Provenance pullrequest.Provenance
	Published  bool
	CreatedAt  time.Time
}

// Event is one lifecycle change of the historique de pull request.
type Event struct {
	At     time.Time
	Kind   string
	Detail string
}

// Detail is the full reading view of one pull request.
type Detail struct {
	Card       Card
	Archived   bool
	HasCard    bool
	Identity   pullrequest.Identity // identity of the published analysis when HasCard
	Provenance pullrequest.Provenance
	History    []HistoryEntry
	Events     []Event
}

// Filter narrows the timeline without mutating durable data.
type Filter struct {
	UnreadOnly bool
	Repository string
	State      pullrequest.State
	Importance pullrequest.Importance
	Risk       string
}

// RepositoryStatus is the reading view of one followed repository. The
// reading side reports collection health without owning abonnements.
type RepositoryStatus struct {
	Repository    string
	Active        bool
	BlockedReason string
	LastSyncAt    time.Time
	// AuthorisedEngine is the engine configuration the user allowed for this
	// repository, so the interface can offer to authorise the current one.
	AuthorisedEngine string
}

// UsageRecord is one analysis's local cost and duration measurement. It
// carries no pull-request body and no credential, and never leaves the
// machine unless the user exports it.
type UsageRecord struct {
	PullRequest string                     `json:"pull_request"`
	HeadSHA     string                     `json:"head_sha"`
	Identity    pullrequest.Identity       `json:"identity"`
	Status      pullrequest.AnalysisStatus `json:"status"`
	Profile     pullrequest.Profile        `json:"profile"`
	Attempt     int                        `json:"attempt"`
	StartedAt   time.Time                  `json:"started_at"`
	Duration    time.Duration              `json:"duration_ns"`
	Usage       map[string]any             `json:"usage,omitempty"`
	Failure     string                     `json:"failure,omitempty"`
}

// PendingVersion is a version waiting for its analysis: the anti-rebond has
// not elapsed, the worker has not reached it yet, or it is running now.
type PendingVersion struct {
	Ref      pullrequest.Ref
	Title    string
	Author   string
	HeadSHA  string
	HTMLURL  string
	DueAt    time.Time
	Attempt  int  // attempts already made; zero before the first
	Running  bool // an analysis is under way right now
	Retrying bool // an earlier attempt failed and another is scheduled
}

// Status summarises collection and analysis health for the interface.
type Status struct {
	LastSyncAt time.Time
	// NextAnalysisAt is when the earliest waiting version becomes eligible;
	// zero when nothing is waiting.
	NextAnalysisAt time.Time
	Pending        int
	Running        int
	Retrying       int
	Unavailable    int
	Blocked        []RepositoryStatus
	Repositories   []RepositoryStatus
}
