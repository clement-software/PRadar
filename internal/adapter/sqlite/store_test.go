package sqlite_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/clement-software/PRadar/internal/adapter/sqlite"
	"github.com/clement-software/PRadar/internal/analyse"
	"github.com/clement-software/PRadar/internal/collect"
	"github.com/clement-software/PRadar/internal/pullrequest"
	"github.com/clement-software/PRadar/internal/timeline"
)

type clock struct {
	mu  sync.Mutex
	now time.Time
}

func newClock() *clock { return &clock{now: time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)} }

func (c *clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *clock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

var (
	ref     = pullrequest.Ref{Repository: "acme/widgets", Number: 42}
	profile = pullrequest.Profile{PromptVersion: "p1", SkillVersion: "s1", Engine: "test", Model: "m1"}
)

type harness struct {
	t     *testing.T
	ctx   context.Context
	store *sqlite.Store
	clock *clock
	path  string
}

func open(t *testing.T) *harness {
	t.Helper()
	h := &harness{t: t, ctx: t.Context(), clock: newClock(), path: t.TempDir() + "/pradar.sqlite"}
	h.reopen()
	if err := h.store.PutSubscription(h.ctx, collect.Subscription{Repository: ref.Repository, HTMLURL: "https://forge.test/acme/widgets", Active: true}); err != nil {
		t.Fatal(err)
	}
	return h
}

func (h *harness) reopen() {
	h.t.Helper()
	if h.store != nil {
		if err := h.store.Close(); err != nil {
			h.t.Fatal(err)
		}
	}
	store, err := sqlite.Open(h.ctx, h.path, h.clock.Now)
	if err != nil {
		h.t.Fatal(err)
	}
	h.store = store
	h.t.Cleanup(func() { _ = store.Close() })
}

func (h *harness) observe(head string, schedule bool) pullrequest.Decision {
	h.t.Helper()
	decision, err := h.store.ObservePullRequest(h.ctx, collect.ObservationRequest{
		Observation: pullrequest.Observation{Ref: ref, Title: "Title", Body: "Body", Author: "alice", State: pullrequest.StateOpen,
			HeadSHA: head, HTMLURL: "https://forge.test/acme/widgets/pulls/42", UpdatedAt: h.clock.Now()},
		Profile: profile, Schedule: schedule, NotBefore: h.clock.Now().Add(10 * time.Minute),
	})
	if err != nil {
		h.t.Fatalf("ObservePullRequest(%s): %v", head, err)
	}
	return decision
}

func (h *harness) observeState(state pullrequest.State) {
	h.t.Helper()
	h.clock.Advance(time.Second)
	if _, err := h.store.ObservePullRequest(h.ctx, collect.ObservationRequest{
		Observation: pullrequest.Observation{Ref: ref, Title: "Title", Body: "Body", State: state, HeadSHA: "x", UpdatedAt: h.clock.Now()},
		Profile:     profile, Schedule: true, NotBefore: h.clock.Now(),
	}); err != nil {
		h.t.Fatal(err)
	}
}

func (h *harness) claim(token string) analyse.Job {
	h.t.Helper()
	job, err := h.store.Claim(h.ctx, h.clock.Now().Add(time.Minute), token)
	if err != nil {
		h.t.Fatalf("Claim(%s): %v", token, err)
	}
	return job
}

func (h *harness) noWork() {
	h.t.Helper()
	if _, err := h.store.Claim(h.ctx, h.clock.Now().Add(time.Minute), "probe"); !errors.Is(err, analyse.ErrNoWork) {
		h.t.Fatalf("Claim() = %v, want ErrNoWork", err)
	}
}

func okAnalysis(job analyse.Job) pullrequest.Analysis {
	return pullrequest.Analysis{
		SchemaVersion: pullrequest.SchemaVersion, PullRequest: job.Ref.Key(), HeadSHA: job.HeadSHA, PreviousHeadSHA: job.PreviousHeadSHA,
		Status: pullrequest.AnalysisOK, Intent: "intent", Importance: pullrequest.ImportanceLow, Risks: []string{"security"},
		Body: "body", ChangeSincePrevious: "change",
	}
}

func (h *harness) complete(job analyse.Job) bool {
	h.t.Helper()
	published, err := h.store.Complete(h.ctx, job, okAnalysis(job), pullrequest.Provenance{Profile: job.Profile, Attempt: job.Attempt})
	if err != nil {
		h.t.Fatalf("Complete: %v", err)
	}
	return published
}

func (h *harness) detail() timeline.Detail {
	h.t.Helper()
	detail, err := h.store.GetDetail(h.ctx, ref)
	if err != nil {
		h.t.Fatal(err)
	}
	return detail
}

func (h *harness) cards() []timeline.Card {
	h.t.Helper()
	cards, err := h.store.ListCards(h.ctx)
	if err != nil {
		h.t.Fatal(err)
	}
	return cards
}

func (h *harness) status() timeline.Status {
	h.t.Helper()
	status, err := h.store.Status(h.ctx)
	if err != nil {
		h.t.Fatal(err)
	}
	return status
}

func TestObserve_DuplicateIdentityIsIdempotent(t *testing.T) {
	t.Parallel()
	h := open(t)
	if d := h.observe("sha-1", true); d.Kind != pullrequest.DecideSchedule {
		t.Fatalf("first observation = %v", d.Kind)
	}
	if d := h.observe("sha-1", true); d.Kind != pullrequest.DecideIgnore {
		t.Fatalf("duplicate observation = %v, want ignore", d.Kind)
	}
	if got := h.status().Pending; got != 1 {
		t.Fatalf("pending = %d, want 1", got)
	}
	if got := len(h.detail().Events); got != 1 {
		t.Fatalf("events = %d, want 1 (no duplicate history)", got)
	}
}

func TestObserve_LaterCandidateReplacesDeadlineAndSurvivesRestart(t *testing.T) {
	t.Parallel()
	h := open(t)
	h.observe("sha-1", true)
	h.clock.Advance(5 * time.Minute)
	h.observe("sha-2", true)
	h.clock.Advance(6 * time.Minute) // sha-1 deadline passed, sha-2 still debouncing
	h.reopen()
	h.noWork()
	if got := h.status().Pending; got != 1 {
		t.Fatalf("pending after restart = %d, want 1", got)
	}
	h.clock.Advance(5 * time.Minute)
	job := h.claim("w")
	if job.HeadSHA != "sha-2" {
		t.Fatalf("claimed head = %s, want the latest candidate sha-2", job.HeadSHA)
	}
	h.noWork()
}

func TestObserve_UnscheduledVersionIsRecordedButNotAnalysed(t *testing.T) {
	t.Parallel()
	h := open(t)
	h.observe("sha-1", false)
	h.clock.Advance(time.Hour)
	h.noWork()
	h.clock.Advance(time.Second)
	if d := h.observe("sha-2", true); d.Kind != pullrequest.DecideSchedule {
		t.Fatalf("new head after seen version = %v, want schedule", d.Kind)
	}
}

func TestClaimAnalysis_GrantsSingleLease(t *testing.T) {
	t.Parallel()
	h := open(t)
	h.observe("sha-1", true)
	h.clock.Advance(11 * time.Minute)
	const workers = 8
	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		wins int
	)
	for i := range workers {
		wg.Go(func() {
			_, err := h.store.Claim(h.ctx, h.clock.Now().Add(time.Minute), "worker-"+string(rune('a'+i)))
			switch {
			case err == nil:
				mu.Lock()
				wins++
				mu.Unlock()
			case !errors.Is(err, analyse.ErrNoWork):
				t.Errorf("Claim: %v", err)
			}
		})
	}
	wg.Wait()
	if wins != 1 {
		t.Fatalf("winners = %d, want exactly 1", wins)
	}
}

func TestComplete_RejectsLostLease(t *testing.T) {
	t.Parallel()
	h := open(t)
	h.observe("sha-1", true)
	h.clock.Advance(11 * time.Minute)
	late := h.claim("late")
	h.clock.Advance(2 * time.Minute) // lease expired
	recovered := h.claim("recovered")
	if recovered.Attempt != 2 {
		t.Fatalf("recovered attempt = %d, want 2", recovered.Attempt)
	}
	if _, err := h.store.Complete(h.ctx, late, okAnalysis(late), pullrequest.Provenance{}); !errors.Is(err, analyse.ErrLeaseLost) {
		t.Fatalf("late Complete = %v, want ErrLeaseLost", err)
	}
	if err := h.store.Retry(h.ctx, late, h.clock.Now(), "x"); !errors.Is(err, analyse.ErrLeaseLost) {
		t.Fatalf("late Retry = %v, want ErrLeaseLost", err)
	}
	if !h.complete(recovered) {
		t.Fatal("recovered worker must publish")
	}
	if got := len(h.cards()); got != 1 {
		t.Fatalf("cards = %d, want 1", got)
	}
}

func TestCompleteAnalysis_CommitsResultAndProjectionAtomically(t *testing.T) {
	t.Parallel()
	h := open(t)
	h.observe("sha-1", true)
	h.clock.Advance(11 * time.Minute)
	job := h.claim("w")
	if !h.complete(job) {
		t.Fatal("latest identity must publish")
	}
	detail := h.detail()
	if !detail.HasCard || detail.Card.Analysis.HeadSHA != "sha-1" || len(detail.History) != 1 || !detail.History[0].Published {
		t.Fatalf("detail = %+v", detail)
	}
	var count int
	if err := h.store.DB().QueryRowContext(h.ctx, `
SELECT COUNT(*) FROM pull_requests p JOIN analyses a ON a.identity = p.published_identity WHERE p.pr_key = ?`, ref.Key()).Scan(&count); err != nil || count != 1 {
		t.Fatalf("projection joined to result = %d, %v", count, err)
	}
	h.noWork()
}

func TestCompleteAnalysis_DoesNotPublishStaleIdentity(t *testing.T) {
	t.Parallel()
	h := open(t)
	h.observe("sha-1", true)
	h.clock.Advance(11 * time.Minute)
	job := h.claim("w")
	h.clock.Advance(time.Second)
	h.observe("sha-2", true) // arrives while sha-1 is analysed
	if h.complete(job) {
		t.Fatal("stale sha-1 must not publish")
	}
	detail := h.detail()
	if detail.HasCard || len(detail.History) != 1 || detail.History[0].Published {
		t.Fatalf("stale result must be in history only: %+v", detail)
	}
	if len(h.cards()) != 0 {
		t.Fatal("no carte before the latest version is analysed")
	}
	h.clock.Advance(11 * time.Minute)
	latest := h.claim("w2")
	if latest.PreviousHeadSHA != "sha-1" {
		t.Fatalf("previous head = %q, want sha-1", latest.PreviousHeadSHA)
	}
	if !h.complete(latest) {
		t.Fatal("latest must publish")
	}
	if cards := h.cards(); len(cards) != 1 || cards[0].Analysis.HeadSHA != "sha-2" {
		t.Fatalf("cards = %+v", cards)
	}
}

func TestArchivedPR_ReappearsAfterLatestAnalysis(t *testing.T) {
	t.Parallel()
	h := open(t)
	h.observe("sha-1", true)
	h.clock.Advance(11 * time.Minute)
	h.complete(h.claim("w"))
	if err := h.store.MarkRead(h.ctx, ref); err != nil {
		t.Fatal(err)
	}
	if cards := h.cards(); cards[0].Unread {
		t.Fatal("card must be read")
	}
	if err := h.store.Archive(h.ctx, ref); err != nil {
		t.Fatal(err)
	}
	if len(h.cards()) != 0 {
		t.Fatal("archived carte must leave the timeline")
	}
	h.clock.Advance(time.Second)
	h.observe("sha-2", true)
	if d := h.detail(); !d.Archived || !d.Card.Unread {
		t.Fatalf("new version must be unread but still archived: archived=%v unread=%v", d.Archived, d.Card.Unread)
	}
	if len(h.cards()) != 0 {
		t.Fatal("archived carte must not reappear while the new version is pending")
	}
	h.clock.Advance(11 * time.Minute)
	h.complete(h.claim("w2"))
	cards := h.cards()
	if len(cards) != 1 || !cards[0].Unread || cards[0].Analysis.HeadSHA != "sha-2" {
		t.Fatalf("reappeared cards = %+v", cards)
	}
	if err := h.store.MarkRead(h.ctx, pullrequest.Ref{Repository: "acme/widgets", Number: 99}); !errors.Is(err, timeline.ErrNotVisible) {
		t.Fatalf("MarkRead(unknown) = %v", err)
	}
}

func TestCloseAndMerge_UpdateStateWithoutScheduling(t *testing.T) {
	t.Parallel()
	h := open(t)
	h.observe("sha-1", true)
	h.clock.Advance(11 * time.Minute)
	h.complete(h.claim("w"))
	h.observeState(pullrequest.StateMerged)
	cards := h.cards()
	if len(cards) != 1 || cards[0].State != pullrequest.StateMerged {
		t.Fatalf("merged card must stay visible with its new state: %+v", cards)
	}
	h.noWork()
	if s := h.status(); s.Pending != 0 {
		t.Fatalf("pending = %d", s.Pending)
	}
}

func TestClose_CancelsPendingCandidate(t *testing.T) {
	t.Parallel()
	h := open(t)
	h.observe("sha-1", true)
	h.observeState(pullrequest.StateClosed)
	h.clock.Advance(time.Hour)
	h.noWork()
	if s := h.status(); s.Pending != 0 {
		t.Fatalf("pending = %d, want 0 after close", s.Pending)
	}
	h.observeState(pullrequest.StateOpen) // reopen schedules with a new generation
	h.clock.Advance(time.Hour)
	if job := h.claim("w"); job.HeadSHA != "x" {
		t.Fatalf("reopened head = %s", job.HeadSHA)
	}
}

func TestUnsubscribe_InvalidatesOutstandingWork(t *testing.T) {
	t.Parallel()
	h := open(t)
	h.observe("sha-1", true)
	h.clock.Advance(11 * time.Minute)
	running := h.claim("w")
	other := pullrequest.Ref{Repository: "acme/widgets", Number: 43}
	if _, err := h.store.ObservePullRequest(h.ctx, collect.ObservationRequest{
		Observation: pullrequest.Observation{Ref: other, State: pullrequest.StateOpen, HeadSHA: "sha-9", UpdatedAt: h.clock.Now()},
		Profile:     profile, Schedule: true, NotBefore: h.clock.Now(),
	}); err != nil {
		t.Fatal(err)
	}
	if err := h.store.Unsubscribe(h.ctx, ref.Repository); err != nil {
		t.Fatal(err)
	}
	h.noWork() // queued sha-9 is cancelled
	if h.complete(running) {
		t.Fatal("late completion after désabonnement must not publish")
	}
	detail := h.detail()
	if len(detail.History) != 1 || detail.HasCard {
		t.Fatalf("history retained, carte absent: %+v", detail)
	}
	if err := h.store.Release(h.ctx, running); !errors.Is(err, analyse.ErrLeaseLost) {
		t.Fatalf("Release after completion = %v", err)
	}
	if err := h.store.DeleteRepositoryData(h.ctx, ref.Repository); err != nil {
		t.Fatal(err)
	}
	if _, err := h.store.GetDetail(h.ctx, ref); !errors.Is(err, collect.ErrNotFound) {
		t.Fatalf("detail after deletion = %v", err)
	}
}

func TestRelease_HandsBackWithoutConsumingAttempt(t *testing.T) {
	t.Parallel()
	h := open(t)
	h.observe("sha-1", true)
	h.clock.Advance(11 * time.Minute)
	job := h.claim("w")
	if err := h.store.Release(h.ctx, job); err != nil {
		t.Fatal(err)
	}
	if again := h.claim("w2"); again.Attempt != 1 {
		t.Fatalf("attempt after release = %d, want 1", again.Attempt)
	}
}

func TestReplay_RequiresChangedIdentityAndKeepsHistory(t *testing.T) {
	t.Parallel()
	h := open(t)
	h.observe("sha-1", true)
	h.clock.Advance(11 * time.Minute)
	h.complete(h.claim("w"))
	if err := h.store.Replay(h.ctx, ref, profile, h.clock.Now()); !errors.Is(err, timeline.ErrReplayUnchanged) {
		t.Fatalf("Replay(same profile) = %v, want ErrReplayUnchanged", err)
	}
	changed := profile
	changed.PromptVersion = "p2"
	if err := h.store.Replay(h.ctx, ref, changed, h.clock.Now()); err != nil {
		t.Fatal(err)
	}
	job := h.claim("w2")
	if job.Profile != changed || job.PreviousHeadSHA != "sha-1" || job.PreviousAnalysis.Intent != "intent" {
		t.Fatalf("a replay must carry the same-head previous analysis as evidence: %+v", job)
	}
	if !h.complete(job) {
		t.Fatal("replay of the latest version must publish")
	}
	if d := h.detail(); len(d.History) != 2 || d.Provenance.Profile != changed {
		t.Fatalf("history = %d, provenance profile = %+v", len(d.History), d.Provenance.Profile)
	}
}

func TestOpen_RefusesUnknownSchemaVersion(t *testing.T) {
	t.Parallel()
	h := open(t)
	if _, err := h.store.DB().ExecContext(h.ctx, "PRAGMA user_version = 99"); err != nil {
		t.Fatal(err)
	}
	if err := h.store.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := sqlite.Open(h.ctx, h.path, h.clock.Now); err == nil {
		t.Fatal("an unsupported schema version must be refused")
	}
}

func TestPersistenceSchema_HasNoCredentialColumn(t *testing.T) {
	t.Parallel()
	h := open(t)
	rows, err := h.store.DB().QueryContext(h.ctx, `SELECT m.name, p.name FROM sqlite_master m JOIN pragma_table_info(m.name) p WHERE m.type = 'table'`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var table, column string
		if err := rows.Scan(&table, &column); err != nil {
			t.Fatal(err)
		}
		for _, forbidden := range []string{"token", "secret", "password", "credential"} {
			if strings.Contains(strings.ToLower(column), forbidden) {
				if table == "analysis_jobs" && column == "lease_token" {
					continue // a random lease id, not a credential
				}
				t.Errorf("column %s.%s looks like a credential", table, column)
			}
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
}

func TestObserve_ConcurrentSchedulingCreatesOneWorkItem(t *testing.T) {
	t.Parallel()
	h := open(t)
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			if _, err := h.store.ObservePullRequest(h.ctx, collect.ObservationRequest{
				Observation: pullrequest.Observation{Ref: ref, Title: "Title", Body: "Body", State: pullrequest.StateOpen, HeadSHA: "sha-1", UpdatedAt: h.clock.Now()},
				Profile:     profile, Schedule: true, NotBefore: h.clock.Now().Add(10 * time.Minute),
			}); err != nil {
				t.Errorf("ObservePullRequest: %v", err)
			}
		})
	}
	wg.Wait()
	var jobs, events int
	if err := h.store.DB().QueryRowContext(h.ctx, `SELECT (SELECT COUNT(*) FROM analysis_jobs), (SELECT COUNT(*) FROM pr_events)`).Scan(&jobs, &events); err != nil {
		t.Fatal(err)
	}
	if jobs != 1 || events != 1 {
		t.Fatalf("jobs = %d, events = %d; want exactly one of each", jobs, events)
	}
	// Overdue candidate after a restart becomes eligible exactly once.
	h.clock.Advance(time.Hour)
	h.reopen()
	if s := h.status(); s.Pending != 1 {
		t.Fatalf("pending after restart = %d", s.Pending)
	}
	h.claim("w")
	h.noWork()
}

func TestRestart_PreservesUserStateGenerationAndHistory(t *testing.T) {
	t.Parallel()
	h := open(t)
	h.observe("sha-1", true)
	h.clock.Advance(11 * time.Minute)
	h.complete(h.claim("w"))
	if err := h.store.MarkRead(h.ctx, ref); err != nil {
		t.Fatal(err)
	}
	if err := h.store.Archive(h.ctx, ref); err != nil {
		t.Fatal(err)
	}
	if err := h.store.Unsubscribe(h.ctx, ref.Repository); err != nil {
		t.Fatal(err)
	}
	h.reopen()
	detail := h.detail()
	if !detail.Archived || detail.Card.Unread || len(detail.History) != 1 || !detail.HasCard {
		t.Fatalf("restart lost user state or history: %+v", detail)
	}
	subscription, err := h.store.GetSubscription(h.ctx, ref.Repository)
	if err != nil || subscription.Active || subscription.Generation != 2 {
		t.Fatalf("subscription after restart = %+v, %v", subscription, err)
	}
	if got := len(h.cards()); got != 0 {
		t.Fatalf("archived carte reappeared after restart: %d", got)
	}
}

func TestClaim_ReportsExpiredLeaseRecovery(t *testing.T) {
	t.Parallel()
	h := open(t)
	h.observe("sha-1", true)
	h.clock.Advance(11 * time.Minute)
	if first := h.claim("crashed"); first.PreviousOutcome != "" {
		t.Fatalf("first claim outcome = %q", first.PreviousOutcome)
	}
	h.clock.Advance(2 * time.Minute)
	recovered := h.claim("restarted")
	if recovered.Attempt != 2 || recovered.PreviousOutcome == "" {
		t.Fatalf("recovered claim = attempt %d outcome %q", recovered.Attempt, recovered.PreviousOutcome)
	}
}

func TestClaim_SkipsWorkOfStoppedAbonnementEvenAfterLeaseExpiry(t *testing.T) {
	t.Parallel()
	h := open(t)
	h.observe("sha-1", true)
	h.clock.Advance(11 * time.Minute)
	h.claim("crashed")
	if err := h.store.Unsubscribe(h.ctx, ref.Repository); err != nil {
		t.Fatal(err)
	}
	h.clock.Advance(time.Hour) // lease expired, abonnement stopped
	h.noWork()
}

func TestComplete_RequiresUnexpiredLease(t *testing.T) {
	t.Parallel()
	h := open(t)
	h.observe("sha-1", true)
	h.clock.Advance(11 * time.Minute)
	job := h.claim("slow")
	h.clock.Advance(2 * time.Minute) // lease expired, nobody reclaimed yet
	if _, err := h.store.Complete(h.ctx, job, okAnalysis(job), pullrequest.Provenance{}); !errors.Is(err, analyse.ErrLeaseLost) {
		t.Fatalf("Complete after expiry = %v, want ErrLeaseLost", err)
	}
	if err := h.store.Retry(h.ctx, job, h.clock.Now(), "x"); !errors.Is(err, analyse.ErrLeaseLost) {
		t.Fatalf("Retry after expiry = %v, want ErrLeaseLost", err)
	}
	if len(h.cards()) != 0 {
		t.Fatal("an expired owner must not publish")
	}
	if recovered := h.claim("next"); recovered.Attempt != 2 {
		t.Fatalf("recovered attempt = %d", recovered.Attempt)
	}
}

func TestSupersede_RetiresClaimedWorkWithoutConsumingAttempt(t *testing.T) {
	t.Parallel()
	h := open(t)
	h.observe("sha-1", true)
	h.clock.Advance(11 * time.Minute)
	job := h.claim("w")
	if err := h.store.Supersede(h.ctx, job, "head moved"); err != nil {
		t.Fatal(err)
	}
	h.noWork()
	if s := h.status(); s.Pending != 0 || s.Retrying != 0 {
		t.Fatalf("status after supersede = %+v", s)
	}
	h.clock.Advance(time.Second)
	h.observe("sha-2", true)
	h.clock.Advance(11 * time.Minute)
	if next := h.claim("w2"); next.HeadSHA != "sha-2" || next.Attempt != 1 {
		t.Fatalf("next claim = %+v", next)
	}
}

func TestObserve_RevertedRevisionRepublishesItsAnalysis(t *testing.T) {
	t.Parallel()
	h := open(t)
	h.observe("sha-1", true)
	h.clock.Advance(11 * time.Minute)
	h.complete(h.claim("a"))
	h.clock.Advance(time.Second)
	retitled := h.observeTitled("sha-1", "Retitled")
	if retitled.Kind != pullrequest.DecideSchedule {
		t.Fatal("a title change must schedule")
	}
	h.clock.Advance(11 * time.Minute)
	h.complete(h.claim("b"))
	h.clock.Advance(time.Second)
	h.observe("sha-1", true) // title reverted: the first identity is the latest again
	h.noWork()
	if cards := h.cards(); len(cards) != 1 || cards[0].Title != "Title" {
		t.Fatalf("reverted revision must show its own analysis: %+v", cards)
	}
	if d := h.detail(); !d.HasCard || d.Card.Unread != true {
		t.Fatalf("republished carte must be current and unread: %+v", d)
	}
}

func (h *harness) observeTitled(head, title string) pullrequest.Decision {
	h.t.Helper()
	decision, err := h.store.ObservePullRequest(h.ctx, collect.ObservationRequest{
		Observation: pullrequest.Observation{Ref: ref, Title: title, Body: "Body", Author: "alice", State: pullrequest.StateOpen,
			HeadSHA: head, HTMLURL: "https://forge.test/acme/widgets/pulls/42", UpdatedAt: h.clock.Now()},
		Profile: profile, Schedule: true, NotBefore: h.clock.Now().Add(10 * time.Minute),
	})
	if err != nil {
		h.t.Fatal(err)
	}
	return decision
}

func TestReplay_SupersedesQueuedCandidate(t *testing.T) {
	t.Parallel()
	h := open(t)
	h.observe("sha-1", true)
	h.clock.Advance(11 * time.Minute)
	h.complete(h.claim("a"))
	h.clock.Advance(time.Second)
	h.observe("sha-2", true) // queued behind the anti-rebond
	changed := profile
	changed.PromptVersion = "p2"
	if err := h.store.Replay(h.ctx, ref, changed, h.clock.Now()); err != nil {
		t.Fatal(err)
	}
	if s := h.status(); s.Pending != 1 {
		t.Fatalf("replay must supersede the queued sibling: pending = %d", s.Pending)
	}
	if job := h.claim("r"); job.Profile != changed {
		t.Fatalf("claimed job = %+v", job)
	}
}
