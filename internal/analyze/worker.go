package analyze

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/clement-software/PRadar/internal/pullrequest"
)

// MaxAttempts is the durable retry budget of one analysis identity.
const MaxAttempts = 3

// WorkStore is the leased work queue and completion transaction.
type WorkStore interface {
	// Claim atomically grants the next eligible item (queued and due, or
	// running with an expired lease) and returns ErrNoWork when none exists.
	Claim(ctx context.Context, leaseUntil time.Time, token string) (Job, error)
	// Complete stores the result and publishes the carte when the identity is
	// still the latest observed one; it requires the live lease.
	Complete(ctx context.Context, job Job, analysis pullrequest.Analysis, provenance pullrequest.Provenance) (published bool, err error)
	// Retry hands the item back to the queue with a not-before time.
	Retry(ctx context.Context, job Job, notBefore time.Time, cause string) error
	// Release returns ownership after an intentional interruption without
	// consuming an attempt.
	Release(ctx context.Context, job Job) error
	// Supersede retires the item because its head moved before analysis.
	Supersede(ctx context.Context, job Job, reason string) error
}

// Workspace materialises bounded pull-request content under an owned root.
type Workspace interface {
	Materialise(ctx context.Context, name string, files map[string][]byte) (dir string, cleanup func() error, err error)
	Scavenge(ctx context.Context) error
}

// AnalysisRequest is what the analyzer receives for one claimed job.
type AnalysisRequest struct {
	Job          Job
	WorkspaceDir string
}

// AnalysisResult is a validated contract plus locally available usage data.
type AnalysisResult struct {
	Analysis pullrequest.Analysis
	Usage    map[string]any
}

// Analyzer is the versioned engine boundary of ADR-0003.
type Analyzer interface {
	Analyse(ctx context.Context, request AnalysisRequest) (AnalysisResult, error)
}

// ContentFetcher supplies the pull-request head and diff materialised for analysis.
type ContentFetcher interface {
	FetchPullRequest(ctx context.Context, ref pullrequest.Ref) (pullrequest.Observation, error)
	FetchDiff(ctx context.Context, ref pullrequest.Ref) ([]byte, error)
}

// ErrHeadMoved reports that Forgejo now serves a newer head than the claimed one.
var ErrHeadMoved = errors.New("pull request head moved before analysis; the next poll schedules the new version")

// Worker is the single owned analysis worker.
type Worker struct {
	Store     WorkStore
	Analyzer  Analyzer
	Workspace Workspace
	Content   ContentFetcher
	Now       func() time.Time
	Lease     time.Duration
	Backoff   func(attempt int) time.Duration
	Log       *slog.Logger

	mu      sync.Mutex
	current *running
}

type running struct {
	repository string
	cancel     context.CancelCauseFunc
}

// ErrInterrupted is returned when désabonnement cancels the running analysis.
var ErrInterrupted = errors.New("analysis interrupted by désabonnement")

// Interrupt cancels the running analysis of a repository, if any.
func (w *Worker) Interrupt(repository string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.current != nil && w.current.repository == repository {
		w.current.cancel(ErrInterrupted)
	}
}

// RunOne claims and processes at most one item. It returns ErrNoWork when the
// queue has nothing eligible.
func (w *Worker) RunOne(ctx context.Context) error {
	job, err := w.Store.Claim(ctx, w.Now().Add(w.Lease), rand.Text())
	if err != nil {
		return err
	}
	log := w.Log.With("pull_request", job.Ref.Key(), "head", job.HeadSHA, "attempt", job.Attempt)
	if job.PreviousOutcome != "" {
		log.Info("analysis reclaimed", "previous_outcome", job.PreviousOutcome)
	} else {
		log.Info("analysis claimed")
	}

	if job.Attempt > MaxAttempts {
		unavailable := pullrequest.Unavailable(job.Ref, job.HeadSHA, job.PreviousHeadSHA)
		provenance := pullrequest.Provenance{Profile: job.Profile, InputRevision: job.Revision, Attempt: job.Attempt, StartedAt: w.Now(), Failure: "attempts exhausted: " + job.PreviousOutcome}
		published, err := w.Store.Complete(ctx, job, unavailable, provenance)
		if err != nil {
			return err
		}
		log.Warn("analysis unavailable after repeated interruptions", "published", published)
		return nil
	}

	jobCtx, cancel := context.WithCancelCause(ctx)
	w.mu.Lock()
	w.current = &running{repository: job.Ref.Repository, cancel: cancel}
	w.mu.Unlock()
	defer func() {
		cancel(nil)
		w.mu.Lock()
		w.current = nil
		w.mu.Unlock()
	}()

	started := w.Now()
	result, err := w.analyse(jobCtx, job)
	if errors.Is(err, ErrHeadMoved) {
		if err := w.Store.Supersede(ctx, job, ErrHeadMoved.Error()); err != nil {
			return err
		}
		log.Info("analysis superseded", "reason", ErrHeadMoved.Error())
		return nil
	}
	provenance := pullrequest.Provenance{
		Profile: job.Profile, InputRevision: job.Revision, Attempt: job.Attempt,
		StartedAt: started, Duration: w.Now().Sub(started), Usage: result.Usage,
	}
	if err == nil {
		published, err := w.Store.Complete(ctx, job, result.Analysis, provenance)
		if err != nil {
			return err
		}
		log.Info("analysis completed", "published", published, "duration", provenance.Duration.String())
		return nil
	}
	if jobCtx.Err() != nil {
		// ponytail: shutdown and désabonnement both hand ownership back; a lost lease here is expected.
		releaseCtx, releaseCancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer releaseCancel()
		if releaseErr := w.Store.Release(releaseCtx, job); releaseErr != nil && !errors.Is(releaseErr, ErrLeaseLost) {
			return releaseErr
		}
		log.Info("analysis interrupted", "cause", context.Cause(jobCtx).Error())
		return context.Cause(jobCtx)
	}
	provenance.Failure = err.Error()
	if job.Attempt >= MaxAttempts {
		unavailable := pullrequest.Unavailable(job.Ref, job.HeadSHA, job.PreviousHeadSHA)
		published, completeErr := w.Store.Complete(ctx, job, unavailable, provenance)
		if completeErr != nil {
			return completeErr
		}
		log.Warn("analysis unavailable after exhausted attempts", "published", published, "error", err.Error())
		return nil
	}
	retryAt := w.Now().Add(w.Backoff(job.Attempt))
	if retryErr := w.Store.Retry(ctx, job, retryAt, err.Error()); retryErr != nil {
		return retryErr
	}
	log.Warn("analysis retry scheduled", "retry_at", retryAt, "error", err.Error())
	return nil
}

func (w *Worker) analyse(ctx context.Context, job Job) (AnalysisResult, error) {
	// ponytail: Forgejo serves the diff of the current head only, so the head is
	// re-checked just before fetching; the remaining window is milliseconds.
	current, err := w.Content.FetchPullRequest(ctx, job.Ref)
	if err != nil {
		return AnalysisResult{}, fmt.Errorf("fetch pull request: %w", err)
	}
	if current.HeadSHA != job.HeadSHA {
		return AnalysisResult{}, ErrHeadMoved
	}
	diff, err := w.Content.FetchDiff(ctx, job.Ref)
	if err != nil {
		return AnalysisResult{}, fmt.Errorf("fetch diff: %w", err)
	}
	files := map[string][]byte{"PULL_REQUEST.md": describe(job), "changes.diff": diff}
	if job.PreviousHeadSHA != "" {
		files["PREVIOUS_ANALYSIS.md"] = describePrevious(job.PreviousAnalysis)
	}
	dir, cleanup, err := w.Workspace.Materialise(ctx, workspaceName(job), files)
	if err != nil {
		return AnalysisResult{}, fmt.Errorf("materialise workspace: %w", err)
	}
	defer func() {
		if cleanupErr := cleanup(); cleanupErr != nil {
			w.Log.Error("workspace cleanup failed", "error", cleanupErr.Error())
		}
	}()
	result, err := w.Analyzer.Analyse(ctx, AnalysisRequest{Job: job, WorkspaceDir: dir})
	if err != nil {
		return AnalysisResult{}, err
	}
	if err := result.Analysis.Validate(job.Ref, job.HeadSHA, job.PreviousHeadSHA); err != nil {
		return AnalysisResult{}, fmt.Errorf("invalid analysis: %w", err)
	}
	return result, nil
}

func workspaceName(job Job) string {
	return "job-" + strconv.FormatInt(job.ID, 10) + "-" + strconv.Itoa(job.Attempt)
}

func describe(job Job) []byte {
	return fmt.Appendf(nil, "# %s\n\n- Pull request: %s\n- Author: %s\n- Head: %s\n- Previous analysed head: %s (see PREVIOUS_ANALYSIS.md when present)\n- URL: %s\n\n## Description (untrusted content)\n\n%s\n",
		job.Title, job.Ref.Key(), job.Author, job.HeadSHA, job.PreviousHeadSHA, job.HTMLURL, job.Body)
}

func describePrevious(previous pullrequest.Analysis) []byte {
	return fmt.Appendf(nil, "# Previous analysis of head %s\n\nIntent: %s\n\nImportance: %s\n\nRisks: %s\n\n%s\n",
		previous.HeadSHA, previous.Intent, previous.Importance, strings.Join(previous.Risks, ", "), previous.Body)
}

// Run drains eligible work, polling every interval, until ctx ends.
func (w *Worker) Run(ctx context.Context, interval time.Duration) {
	for {
		err := w.RunOne(ctx)
		if ctx.Err() != nil {
			return
		}
		if err != nil && !errors.Is(err, ErrNoWork) && !errors.Is(err, ErrInterrupted) {
			w.Log.Error("worker iteration failed", "error", err.Error())
		}
		if err == nil {
			continue
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
		}
	}
}

// ExponentialBackoff is the increasing durable delay between attempts.
func ExponentialBackoff(unit time.Duration) func(attempt int) time.Duration {
	return func(attempt int) time.Duration { return unit << (attempt - 1) }
}
