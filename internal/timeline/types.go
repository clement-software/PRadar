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
}

// Status summarises collection and analysis health for the interface.
type Status struct {
	LastSyncAt   time.Time
	Pending      int
	Running      int
	Retrying     int
	Unavailable  int
	Blocked      []RepositoryStatus
	Repositories []RepositoryStatus
}
