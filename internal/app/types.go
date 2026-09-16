// Package app holds the PRadar demonstrator use cases: subscriptions,
// polling reconciliation, the single analysis worker, timeline reading and
// evaluation. Interfaces are declared next to the code that consumes them and
// implemented by adapters; nothing here imports SQL, HTTP, process or UI code.
package app

import (
	"errors"
	"time"

	"github.com/clement-software/PRadar/internal/pullrequest"
)

// Errors shared by the store implementations and the use cases.
var (
	ErrNoWork          = errors.New("no analysis work is eligible")
	ErrLeaseLost       = errors.New("analysis lease is no longer owned")
	ErrNotFound        = errors.New("not found")
	ErrReplayUnchanged = errors.New("replay requires a changed prompt, skill, engine or model")
	ErrNotVisible      = errors.New("pull request has no visible carte")
)

// ImportMode selects which currently open pull requests a new abonnement imports.
type ImportMode string

// Import modes offered when creating an abonnement.
const (
	ImportNone ImportMode = "none"
	ImportTen  ImportMode = "ten"
	ImportAll  ImportMode = "all"
)

// Subscription is one abonnement to a repository of the configured instance.
type Subscription struct {
	Repository      string // "owner/name"
	HTMLURL         string
	Generation      int64
	Active          bool
	BlockedReason   string
	ExcludedAuthors []string
	LastSyncAt      time.Time
}

// ObservationRequest is one durable observation plus its scheduling decision.
type ObservationRequest struct {
	Observation pullrequest.Observation
	Profile     pullrequest.Profile
	// Schedule is false when the pull request is only recorded as seen
	// (abonnement import mode "none" or beyond the import limit).
	Schedule  bool
	NotBefore time.Time // anti-rebond deadline of the candidate
}

// Job is one claimed analysis work item.
type Job struct {
	ID              int64
	Identity        pullrequest.Identity
	Ref             pullrequest.Ref
	Generation      int64
	HeadSHA         string
	PreviousHeadSHA string
	Title           string
	Body            string
	Author          string
	HTMLURL         string
	Revision        pullrequest.InputRevision
	Profile         pullrequest.Profile
	Attempt         int
	LeaseToken      string
}

// Card is the visible projection of one pull request in the timeline.
type Card struct {
	Ref        pullrequest.Ref
	Title      string
	Author     string
	State      pullrequest.State
	HTMLURL    string
	UpdatedAt  time.Time
	ActivityAt time.Time
	Unread     bool
	Analysis   pullrequest.Analysis
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

// Status summarises collection and analysis health for the visualizer.
type Status struct {
	LastSyncAt    time.Time
	Pending       int
	Running       int
	Retrying      int
	Unavailable   int
	Blocked       []Subscription
	Subscriptions []Subscription
}
