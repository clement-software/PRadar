// Package collect owns abonnements, the polling reconciliation and the durable
// anti-rebond: it turns what Forgejo reports into durable observations and
// analysis candidates. It declares the forge and store interfaces it consumes.
package collect

import (
	"errors"
	"time"

	"github.com/clement-software/PRadar/internal/pullrequest"
)

// ErrNotFound is returned when a repository or pull request is unknown, both
// locally and on the forge.
var ErrNotFound = errors.New("not found")

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
	// AuthorisedEngine is the engine configuration the user allowed to read
	// this repository's content, as EngineFingerprint spells it. A
	// subscription whose fingerprint no longer matches cannot be active.
	AuthorisedEngine string
}

// EngineFingerprint identifies the engine configuration a user authorises for
// a repository. Changing the engine or the model requires a new authorisation.
func EngineFingerprint(profile pullrequest.Profile) string {
	return profile.Engine + "/" + profile.Model
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
