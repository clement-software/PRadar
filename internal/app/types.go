// Package app is the former single use-case package. It now only re-exports
// the boundaries that own each concern, so callers can migrate one at a time.
//
// Deprecated: import internal/collect, internal/analyze or internal/timeline
// directly. This façade is removed once every caller has moved.
package app

import (
	"github.com/clement-software/PRadar/internal/analyze"
	"github.com/clement-software/PRadar/internal/collect"
	"github.com/clement-software/PRadar/internal/timeline"
)

// Collection boundary.
type (
	Collector          = collect.Collector
	Forge              = collect.Forge
	CollectionStore    = collect.CollectionStore
	Subscription       = collect.Subscription
	SubscribeRequest   = collect.SubscribeRequest
	ObservationRequest = collect.ObservationRequest
	ImportMode         = collect.ImportMode
)

// Import modes of the collection boundary.
const (
	ImportNone = collect.ImportNone
	ImportTen  = collect.ImportTen
	ImportAll  = collect.ImportAll
)

// Analysis boundary.
type (
	Worker          = analyze.Worker
	Job             = analyze.Job
	WorkStore       = analyze.WorkStore
	Workspace       = analyze.Workspace
	Analyzer        = analyze.Analyzer
	AnalysisRequest = analyze.AnalysisRequest
	AnalysisResult  = analyze.AnalysisResult
	ContentFetcher  = analyze.ContentFetcher
	Smoke           = analyze.Smoke
	SmokeReport     = analyze.SmokeReport
	SmokeStep       = analyze.SmokeStep
)

// MaxAttempts is the durable retry budget of one analysis identity.
const MaxAttempts = analyze.MaxAttempts

// ExponentialBackoff is the increasing durable delay between attempts.
var ExponentialBackoff = analyze.ExponentialBackoff

// Reading and evaluation boundary.
type (
	Timeline         = timeline.Reader
	ReadModel        = timeline.ReadModel
	Evaluator        = timeline.Evaluator
	EvaluationStore  = timeline.EvaluationStore
	Progress         = timeline.Progress
	Scorecard        = timeline.Scorecard
	Card             = timeline.Card
	Detail           = timeline.Detail
	HistoryEntry     = timeline.HistoryEntry
	Event            = timeline.Event
	Filter           = timeline.Filter
	Status           = timeline.Status
	RepositoryStatus = timeline.RepositoryStatus
)

// Errors of the three boundaries.
var (
	ErrNotFound        = collect.ErrNotFound
	ErrNoWork          = analyze.ErrNoWork
	ErrLeaseLost       = analyze.ErrLeaseLost
	ErrInterrupted     = analyze.ErrInterrupted
	ErrHeadMoved       = analyze.ErrHeadMoved
	ErrNotVisible      = timeline.ErrNotVisible
	ErrReplayUnchanged = timeline.ErrReplayUnchanged
	ErrNoCorpus        = timeline.ErrNoCorpus
)
