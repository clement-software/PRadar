// Package analyse owns the single leased analysis worker: claiming due work,
// materialising the pull-request content, invoking the engine, validating the
// contract and completing or retrying. It declares the store, workspace and
// analyzer interfaces it consumes.
package analyse

import (
	"errors"

	"github.com/clement-software/PRadar/internal/pullrequest"
)

// Errors shared by the work store and the worker.
var (
	ErrNoWork    = errors.New("no analysis work is eligible")
	ErrLeaseLost = errors.New("analysis lease is no longer owned")
)

// Job is one claimed analysis work item.
type Job struct {
	ID              int64
	Identity        pullrequest.Identity
	Ref             pullrequest.Ref
	Generation      int64
	HeadSHA         string
	PreviousHeadSHA string
	// PreviousAnalysis is the latest successful analysis of this pull
	// request under another identity, materialised as prior evidence.
	PreviousAnalysis pullrequest.Analysis
	Title            string
	Body             string
	Author           string
	HTMLURL          string
	Revision         pullrequest.InputRevision
	Profile          pullrequest.Profile
	Attempt          int
	LeaseToken       string
	// PreviousOutcome explains why an earlier attempt ended: a technical
	// failure message or an expired lease. Empty on the first attempt.
	PreviousOutcome string
}
