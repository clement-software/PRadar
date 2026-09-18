package app_test

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"slices"
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

var (
	repo    = "acme/widgets"
	ref     = pullrequest.Ref{Repository: repo, Number: 42}
	profile = pullrequest.Profile{PromptVersion: "p1", SkillVersion: "s1", Engine: "fake", Model: "m1"}
)

type clock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *clock) Now() time.Time { c.mu.Lock(); defer c.mu.Unlock(); return c.now }
func (c *clock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

// fakeForge is the controlled Forgejo substitute.
type fakeForge struct {
	mu       sync.Mutex
	lists    int
	checkErr error
	listErr  error
	open     []pullrequest.Observation
	byRef    map[pullrequest.Ref]pullrequest.Observation
	diff     []byte
}

func (f *fakeForge) CheckRepository(context.Context, string) error { return f.checkErr }

// listed is how many times the collector asked the forge for open pull
// requests: one per complete reconciliation of one abonnement.
func (f *fakeForge) listed() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.lists
}

func (f *fakeForge) ListOpenPullRequests(context.Context, string) ([]pullrequest.Observation, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.lists++
	if f.listErr != nil {
		return nil, f.listErr
	}
	open := make([]pullrequest.Observation, 0, len(f.open))
	for _, observation := range f.open {
		if observation.State == pullrequest.StateOpen {
			open = append(open, observation)
		}
	}
	return open, nil
}

func (f *fakeForge) FetchPullRequest(_ context.Context, ref pullrequest.Ref) (pullrequest.Observation, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, observation := range f.open {
		if observation.Ref == ref {
			return observation, nil
		}
	}
	return pullrequest.Observation{}, fmt.Errorf("%s: %w", ref.Key(), collect.ErrNotFound)
}

func (f *fakeForge) FetchDiff(context.Context, pullrequest.Ref) ([]byte, error) { return f.diff, nil }

func (f *fakeForge) set(observations ...pullrequest.Observation) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.open = observations
}

// fakeAnalyzer produces a deterministic contract or a scripted failure.
type fakeAnalyzer struct {
	mu       sync.Mutex
	fail     error
	block    chan struct{}
	requests []analyse.AnalysisRequest
}

func (a *fakeAnalyzer) Analyse(ctx context.Context, request analyse.AnalysisRequest) (analyse.AnalysisResult, error) {
	a.mu.Lock()
	a.requests = append(a.requests, request)
	fail, block := a.fail, a.block
	a.mu.Unlock()
	if block != nil {
		select {
		case <-block:
		case <-ctx.Done():
			return analyse.AnalysisResult{}, ctx.Err()
		}
	}
	if fail != nil {
		return analyse.AnalysisResult{}, fail
	}
	job := request.Job
	return analyse.AnalysisResult{Analysis: pullrequest.Analysis{
		SchemaVersion: pullrequest.SchemaVersion, PullRequest: job.Ref.Key(), HeadSHA: job.HeadSHA, PreviousHeadSHA: job.PreviousHeadSHA,
		Status: pullrequest.AnalysisOK, Intent: "Controlled intent for " + job.HeadSHA, Importance: pullrequest.ImportanceMedium,
		Risks: []string{"contract"}, Body: "```mermaid\nflowchart LR\n  A --> B\n```", ChangeSincePrevious: "controlled change",
	}, Usage: map[string]any{"input_tokens": 1}}, nil
}

// fakeWorkspace records materialised and cleaned directories.
type fakeWorkspace struct {
	mu      sync.Mutex
	root    string
	cleaned []string
	files   [][]string
}

func (w *fakeWorkspace) Materialise(_ context.Context, name string, files map[string][]byte) (string, func() error, error) {
	dir := filepath.Join(w.root, name)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", nil, err
	}
	w.mu.Lock()
	w.files = append(w.files, slices.Sorted(maps.Keys(files)))
	w.mu.Unlock()
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), content, 0o600); err != nil {
			return "", nil, err
		}
	}
	return dir, func() error {
		w.mu.Lock()
		w.cleaned = append(w.cleaned, dir)
		w.mu.Unlock()
		return os.RemoveAll(dir)
	}, nil
}

func (w *fakeWorkspace) Scavenge(context.Context) error { return nil }

type fixture struct {
	logs      *syncBuffer
	t         *testing.T
	ctx       context.Context
	clock     *clock
	store     *sqlite.Store
	forge     *fakeForge
	analyzer  *fakeAnalyzer
	workspace *fakeWorkspace
	collector *collect.Collector
	worker    *analyse.Worker
	timeline  *timeline.Reader
	path      string
}

func observation(number int64, head string, updated time.Time) pullrequest.Observation {
	return pullrequest.Observation{
		Ref: pullrequest.Ref{Repository: repo, Number: number}, Title: fmt.Sprintf("PR %d", number), Body: "body", Author: "alice",
		State: pullrequest.StateOpen, HeadSHA: head, HTMLURL: fmt.Sprintf("https://forge.test/%s/pulls/%d", repo, number), UpdatedAt: updated,
	}
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{t: t, ctx: t.Context(), clock: &clock{now: time.Date(2026, 9, 16, 12, 0, 0, 0, time.UTC)}, path: t.TempDir() + "/pradar.sqlite"}
	f.forge = &fakeForge{diff: []byte("diff --git a/x b/x\n"), byRef: map[pullrequest.Ref]pullrequest.Observation{}}
	f.analyzer = &fakeAnalyzer{}
	f.workspace = &fakeWorkspace{root: t.TempDir()}
	f.forge.set(observation(42, "sha-1", f.clock.Now()))
	f.wire()
	return f
}

func (f *fixture) wire() {
	f.t.Helper()
	if f.store != nil {
		_ = f.store.Close()
	}
	store, err := sqlite.Open(f.ctx, f.path, f.clock.Now)
	if err != nil {
		f.t.Fatal(err)
	}
	f.t.Cleanup(func() { _ = store.Close() })
	f.store = store
	if f.logs == nil {
		f.logs = &syncBuffer{}
	}
	log := slog.New(slog.NewJSONHandler(f.logs, nil))
	f.collector = &collect.Collector{Forge: f.forge, Store: store, Profile: profile, Debounce: 10 * time.Minute, Now: f.clock.Now, Log: log}
	f.worker = &analyse.Worker{Store: store, Analyzer: f.analyzer, Workspace: f.workspace, Content: f.forge, Now: f.clock.Now,
		Lease: time.Minute, Backoff: analyse.ExponentialBackoff(time.Minute), Log: log}
	f.collector.Interrupt = f.worker.Interrupt
	f.timeline = &timeline.Reader{Store: store, Profile: profile, Now: f.clock.Now}
}

func (f *fixture) subscribe(mode collect.ImportMode, excluded ...string) collect.Subscription {
	f.t.Helper()
	subscription, err := f.collector.Subscribe(f.ctx, collect.SubscribeRequest{Repository: repo, HTMLURL: "https://forge.test/" + repo,
		Import: mode, ExcludedAuthors: excluded, AuthoriseEngine: true})
	if err != nil {
		f.t.Fatal(err)
	}
	return subscription
}

func (f *fixture) runOne() error {
	f.t.Helper()
	return f.worker.RunOne(f.ctx)
}

func (f *fixture) drain() {
	f.t.Helper()
	f.clock.Advance(11 * time.Minute)
	if err := f.runOne(); err != nil {
		f.t.Fatalf("RunOne: %v", err)
	}
}

func (f *fixture) cards(filter timeline.Filter) []timeline.Card {
	f.t.Helper()
	cards, err := f.timeline.Cards(f.ctx, filter)
	if err != nil {
		f.t.Fatal(err)
	}
	return cards
}

func (f *fixture) status() timeline.Status {
	f.t.Helper()
	status, err := f.timeline.Status(f.ctx)
	if err != nil {
		f.t.Fatal(err)
	}
	return status
}

func TestControlledAnalysis_EndToEndAndRestart(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.subscribe(collect.ImportTen)
	if s := f.status(); s.Pending != 1 || s.LastSyncAt.IsZero() {
		t.Fatalf("status after subscribe = %+v", s)
	}
	if err := f.runOne(); !errors.Is(err, analyse.ErrNoWork) {
		t.Fatalf("work must wait for the anti-rebond: %v", err)
	}
	f.drain()
	cards := f.cards(timeline.Filter{})
	if len(cards) != 1 || cards[0].Analysis.Intent != "Controlled intent for sha-1" || !cards[0].Unread {
		t.Fatalf("cards = %+v", cards)
	}
	detail, err := f.timeline.Detail(f.ctx, ref)
	if err != nil || !detail.HasCard || detail.Provenance.Duration < 0 || detail.Provenance.Usage["input_tokens"] == nil {
		t.Fatalf("detail = %+v, %v", detail, err)
	}
	if len(f.workspace.cleaned) != 1 {
		t.Fatal("workspace must be cleaned after success")
	}
	f.wire() // restart against the same database
	if err := f.collector.ReconcileAll(f.ctx); err != nil {
		t.Fatal(err)
	}
	if err := f.runOne(); !errors.Is(err, analyse.ErrNoWork) {
		t.Fatalf("restart must not create duplicate work: %v", err)
	}
	detail, _ = f.timeline.Detail(f.ctx, ref)
	if len(f.cards(timeline.Filter{})) != 1 || len(detail.History) != 1 {
		t.Fatalf("restart duplicated visible state: cards=%d history=%d", len(f.cards(timeline.Filter{})), len(detail.History))
	}
}

func TestSubscribe_ImportModesDraftsAndExcludedAuthors(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	now := f.clock.Now()
	var many []pullrequest.Observation
	for i := range int64(12) {
		many = append(many, observation(100+i, fmt.Sprintf("sha-%d", i), now.Add(-time.Duration(i)*time.Minute)))
	}
	draft := observation(200, "draft", now)
	draft.Draft = true
	bot := observation(201, "bot", now)
	bot.Author = "renovate[bot]"
	f.forge.set(append(many, draft, bot)...)

	f.subscribe(collect.ImportTen, "renovate[bot]")
	if s := f.status(); s.Pending != 10 {
		t.Fatalf("pending after ten import = %d", s.Pending)
	}
	f.forge.set(append(many, draft, bot)...)
	if err := f.collector.ReconcileAll(f.ctx); err != nil {
		t.Fatal(err)
	}
	if s := f.status(); s.Pending != 10 {
		t.Fatalf("reconcile must not schedule seen versions: %d", s.Pending)
	}
	if _, err := f.store.GetDetail(f.ctx, draft.Ref); !errors.Is(err, collect.ErrNotFound) {
		t.Fatalf("draft must be ignored: %v", err)
	}
	if _, err := f.store.GetDetail(f.ctx, bot.Ref); !errors.Is(err, collect.ErrNotFound) {
		t.Fatalf("excluded author must be ignored: %v", err)
	}
	if err := f.collector.Unsubscribe(f.ctx, repo); err != nil {
		t.Fatal(err)
	}
	if err := f.store.DeleteRepositoryData(f.ctx, repo); err != nil {
		t.Fatal(err)
	}
	f.subscribe(collect.ImportAll, "renovate[bot]")
	if s := f.status(); s.Pending != 12 {
		t.Fatalf("pending after all import = %d", s.Pending)
	}
	if err := f.collector.Unsubscribe(f.ctx, repo); err != nil {
		t.Fatal(err)
	}
	if err := f.collector.DeleteRepositoryData(f.ctx, repo, false); err == nil {
		t.Fatal("deletion without confirmation must fail")
	}
	if err := f.collector.DeleteRepositoryData(f.ctx, repo, true); err != nil {
		t.Fatal(err)
	}
	f.subscribe(collect.ImportNone)
	if s := f.status(); s.Pending != 0 {
		t.Fatalf("pending after none import = %d", s.Pending)
	}
}

func TestSubscribe_InaccessibleRepositoryIsBlocked(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.forge.checkErr = errors.New("forgejo: 401 unauthorised")
	subscription := f.subscribe(collect.ImportTen)
	if subscription.Active || subscription.BlockedReason == "" {
		t.Fatalf("subscription = %+v", subscription)
	}
	if s := f.status(); len(s.Blocked) != 1 {
		t.Fatalf("blocked = %+v", s.Blocked)
	}
	f.forge.checkErr = nil
	f.forge.listErr = errors.New("forgejo: 503")
	f.subscribe(collect.ImportTen)
	if s := f.status(); len(s.Blocked) != 1 || s.Blocked[0].BlockedReason != "forgejo: 503" {
		t.Fatalf("listing failure must block visibly: %+v", s.Blocked)
	}
}

func TestReconcile_CloseMergeAndReopen(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.subscribe(collect.ImportTen)
	f.drain()
	merged := observation(42, "sha-1", f.clock.Now().Add(time.Minute))
	merged.State = pullrequest.StateMerged
	f.forge.set(merged)
	if err := f.collector.ReconcileAll(f.ctx); err != nil {
		t.Fatal(err)
	}
	cards := f.cards(timeline.Filter{})
	if len(cards) != 1 || cards[0].State != pullrequest.StateMerged {
		t.Fatalf("merged carte = %+v", cards)
	}
	if err := f.runOne(); !errors.Is(err, analyse.ErrNoWork) {
		t.Fatalf("merge must not schedule: %v", err)
	}
	reopened := observation(42, "sha-1", f.clock.Now().Add(2*time.Minute))
	f.forge.set(reopened)
	if err := f.collector.ReconcileAll(f.ctx); err != nil {
		t.Fatal(err)
	}
	if s := f.status(); s.Pending != 1 {
		t.Fatalf("reopen must schedule a new revision: %+v", s)
	}
}

func TestAnalysis_ThirdFailurePublishesUnavailable(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.subscribe(collect.ImportTen)
	f.analyzer.fail = errors.New("engine exploded")
	f.clock.Advance(11 * time.Minute)
	for attempt := 1; attempt <= analyse.MaxAttempts; attempt++ {
		if err := f.runOne(); err != nil {
			t.Fatalf("attempt %d: %v", attempt, err)
		}
		if attempt < analyse.MaxAttempts {
			if err := f.runOne(); !errors.Is(err, analyse.ErrNoWork) {
				t.Fatalf("retry must wait for its increasing delay: %v", err)
			}
			if s := f.status(); s.Retrying != 1 {
				t.Fatalf("retrying = %d", s.Retrying)
			}
			f.clock.Advance(time.Duration(1<<(attempt-1)) * time.Minute)
		}
	}
	cards := f.cards(timeline.Filter{})
	if len(cards) != 1 || cards[0].Analysis.Status != pullrequest.AnalysisUnavailable || cards[0].HTMLURL == "" {
		t.Fatalf("unavailable carte = %+v", cards)
	}
	detail, _ := f.timeline.Detail(f.ctx, ref)
	if detail.Provenance.Failure != "engine exploded" || detail.Provenance.Attempt != 3 {
		t.Fatalf("provenance = %+v", detail.Provenance)
	}
	if len(f.workspace.cleaned) != 3 {
		t.Fatalf("workspace cleaned %d times, want 3", len(f.workspace.cleaned))
	}
	logs := f.logs.String()
	for _, want := range []string{"analysis claimed", "analysis retry scheduled", "analysis unavailable after exhausted attempts", "engine exploded"} {
		if !strings.Contains(logs, want) {
			t.Errorf("logs lack %q", want)
		}
	}
	if strings.Contains(logs, `"body"`) || strings.Contains(logs, observation(42, "", time.Time{}).Body) {
		t.Error("logs must not contain the pull-request description")
	}
	if err := f.timeline.Replay(f.ctx, ref); !errors.Is(err, timeline.ErrReplayUnchanged) {
		t.Fatalf("replay with unchanged profile = %v", err)
	}
	f.timeline.Profile.PromptVersion = "p2"
	f.analyzer.fail = nil
	if err := f.timeline.Replay(f.ctx, ref); err != nil {
		t.Fatal(err)
	}
	if err := f.runOne(); err != nil {
		t.Fatal(err)
	}
	cards = f.cards(timeline.Filter{})
	detail, _ = f.timeline.Detail(f.ctx, ref)
	if cards[0].Analysis.Status != pullrequest.AnalysisOK || len(detail.History) != 2 {
		t.Fatalf("replay must publish and keep the failure in history: %+v", detail.History)
	}
}

func TestWorker_InvalidAnalysisIsATechnicalFailure(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.subscribe(collect.ImportTen)
	f.analyzer.fail = nil
	f.forge.set(observation(42, "sha-1", f.clock.Now()))
	f.clock.Advance(11 * time.Minute)
	invalid := &invalidAnalyzer{}
	f.worker.Analyzer = invalid
	if err := f.runOne(); err != nil {
		t.Fatal(err)
	}
	if s := f.status(); s.Retrying != 1 {
		t.Fatalf("mismatched head must schedule a retry: %+v", s)
	}
}

type invalidAnalyzer struct{}

func (invalidAnalyzer) Analyse(_ context.Context, request analyse.AnalysisRequest) (analyse.AnalysisResult, error) {
	analysis := pullrequest.Unavailable(request.Job.Ref, "other-sha", "")
	analysis.Status = pullrequest.AnalysisOK
	return analyse.AnalysisResult{Analysis: analysis}, nil
}

func TestWorker_InterruptionReleasesWithoutConsumingAttempt(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.subscribe(collect.ImportTen)
	f.clock.Advance(11 * time.Minute)
	f.analyzer.block = make(chan struct{})
	done := make(chan error, 1)
	go func() { done <- f.runOne() }()
	waitFor(t, func() bool { f.analyzer.mu.Lock(); defer f.analyzer.mu.Unlock(); return len(f.analyzer.requests) == 1 })
	if err := f.collector.Unsubscribe(f.ctx, repo); err != nil {
		t.Fatal(err)
	}
	if err := <-done; !errors.Is(err, analyse.ErrInterrupted) {
		t.Fatalf("interrupted run = %v", err)
	}
	if err := f.runOne(); !errors.Is(err, analyse.ErrNoWork) {
		t.Fatalf("cancelled abonnement must leave no work: %v", err)
	}
	if len(f.workspace.cleaned) != 1 {
		t.Fatal("workspace must be cleaned after cancellation")
	}

	// Shutdown of the root context hands the work back and keeps the attempt.
	f.subscribe(collect.ImportTen)
	f.forge.set(observation(42, "sha-2", f.clock.Now()))
	if err := f.collector.ReconcileAll(f.ctx); err != nil {
		t.Fatal(err)
	}
	f.clock.Advance(11 * time.Minute)
	ctx, cancel := context.WithCancel(f.ctx)
	go func() { done <- f.worker.RunOne(ctx) }()
	waitFor(t, func() bool { f.analyzer.mu.Lock(); defer f.analyzer.mu.Unlock(); return len(f.analyzer.requests) == 2 })
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("shutdown run = %v", err)
	}
	f.analyzer.block = nil
	if err := f.runOne(); err != nil {
		t.Fatal(err)
	}
	detail, _ := f.timeline.Detail(f.ctx, ref)
	if detail.Provenance.Attempt != 1 {
		t.Fatalf("attempt after shutdown release = %d, want 1", detail.Provenance.Attempt)
	}
}

type syncBuffer struct {
	mu  sync.Mutex
	buf strings.Builder
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func waitFor(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatal("condition not met in time")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestTimeline_Filters(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.subscribe(collect.ImportTen)
	f.drain()
	if err := f.timeline.MarkRead(f.ctx, ref); err != nil {
		t.Fatal(err)
	}
	cases := map[string]struct {
		filter timeline.Filter
		want   int
	}{
		"all":              {timeline.Filter{}, 1},
		"unread only":      {timeline.Filter{UnreadOnly: true}, 0},
		"repository":       {timeline.Filter{Repository: repo}, 1},
		"other repository": {timeline.Filter{Repository: "acme/other"}, 0},
		"state":            {timeline.Filter{State: pullrequest.StateOpen}, 1},
		"importance":       {timeline.Filter{Importance: pullrequest.ImportanceMedium}, 1},
		"other importance": {timeline.Filter{Importance: pullrequest.ImportanceHigh}, 0},
		"risk":             {timeline.Filter{Risk: "contract"}, 1},
		"other risk":       {timeline.Filter{Risk: "security"}, 0},
	}
	for name, tc := range cases {
		if got := len(f.cards(tc.filter)); got != tc.want {
			t.Errorf("%s: cards = %d, want %d", name, got, tc.want)
		}
	}
	if err := f.timeline.Archive(f.ctx, ref); err != nil {
		t.Fatal(err)
	}
	if len(f.cards(timeline.Filter{})) != 0 {
		t.Fatal("archived carte must be hidden")
	}
}

func TestPoll_ReconcilesOnLaunchAndEveryTick(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.subscribe(collect.ImportNone)
	ticks := make(chan time.Time)
	f.collector.Tick = func(time.Duration) <-chan time.Time { return ticks }
	ctx, cancel := context.WithCancel(f.ctx)
	done := make(chan struct{})
	go func() { defer close(done); f.collector.Poll(ctx, 5*time.Minute) }()
	waitFor(t, func() bool { return !f.status().LastSyncAt.IsZero() }) // launch reconciliation
	f.forge.set(observation(42, "sha-2", f.clock.Now().Add(time.Minute)))
	f.clock.Advance(5 * time.Minute)
	ticks <- f.clock.Now()
	waitFor(t, func() bool { return f.status().Pending == 1 })
	cancel()
	<-done
}

func TestReconcile_TitleBodyAndHeadChangesScheduleDistinctRevisions(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.subscribe(collect.ImportTen)
	f.drain()
	base := observation(42, "sha-1", f.clock.Now())
	changes := []func(o *pullrequest.Observation){
		func(o *pullrequest.Observation) { o.Title = "Retitled" },
		func(o *pullrequest.Observation) { o.Body = "rewritten description" },
		func(o *pullrequest.Observation) { o.HeadSHA = "sha-2" },
	}
	for i, change := range changes {
		f.clock.Advance(time.Minute)
		next := base
		next.UpdatedAt = f.clock.Now()
		change(&next)
		f.forge.set(next)
		if err := f.collector.ReconcileAll(f.ctx); err != nil {
			t.Fatal(err)
		}
		if s := f.status(); s.Pending != 1 {
			t.Fatalf("change %d: pending = %d, want a fresh candidate", i, s.Pending)
		}
		f.drain()
		if s := f.status(); s.Pending != 0 {
			t.Fatalf("change %d: pending after analysis = %d", i, s.Pending)
		}
		base = next
	}
	detail, _ := f.timeline.Detail(f.ctx, ref)
	if len(detail.History) != 4 {
		t.Fatalf("history = %d analyses, want 4 distinct revisions", len(detail.History))
	}
	// An older observation replayed by a stale poll never regresses state.
	stale := observation(42, "sha-1", f.clock.Now().Add(-time.Hour))
	f.forge.set(stale)
	if err := f.collector.ReconcileAll(f.ctx); err != nil {
		t.Fatal(err)
	}
	if cards := f.cards(timeline.Filter{}); cards[0].Analysis.HeadSHA != "sha-2" || f.status().Pending != 0 {
		t.Fatalf("stale observation changed durable state: %+v", cards)
	}
}

func TestWorker_SupersedesWhenHeadMovedBeforeFetch(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.subscribe(collect.ImportTen)
	f.clock.Advance(11 * time.Minute)
	f.forge.set(observation(42, "sha-2", f.clock.Now())) // commit lands after the claim window opened
	if err := f.runOne(); err != nil {
		t.Fatal(err)
	}
	if len(f.analyzer.requests) != 0 {
		t.Fatal("the engine must not analyse a diff that no longer matches the claimed head")
	}
	if s := f.status(); s.Pending != 0 || s.Retrying != 0 {
		t.Fatalf("superseded work must not stay pending or count as a retry: %+v", s)
	}
	if err := f.collector.ReconcileAll(f.ctx); err != nil {
		t.Fatal(err)
	}
	f.drain()
	cards := f.cards(timeline.Filter{})
	if len(cards) != 1 || cards[0].Analysis.HeadSHA != "sha-2" {
		t.Fatalf("the moved head must be analysed by the next poll: %+v", cards)
	}
	// The previous analysis is materialised as evidence for the next version.
	f.forge.set(observation(42, "sha-3", f.clock.Now().Add(time.Minute)))
	if err := f.collector.ReconcileAll(f.ctx); err != nil {
		t.Fatal(err)
	}
	f.drain()
	last := f.workspace.files[len(f.workspace.files)-1]
	if !slices.Contains(last, "PREVIOUS_ANALYSIS.md") || !slices.Contains(f.workspace.files[0], "changes.diff") || slices.Contains(f.workspace.files[0], "PREVIOUS_ANALYSIS.md") {
		t.Fatalf("workspace files = %v", f.workspace.files)
	}
}

func TestSubscribe_RejectsUnknownImportModeBeforePersisting(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	if _, err := f.collector.Subscribe(f.ctx, collect.SubscribeRequest{Repository: repo, Import: "everything", AuthoriseEngine: true}); err == nil {
		t.Fatal("unknown import mode accepted")
	}
	if _, err := f.store.GetSubscription(f.ctx, repo); !errors.Is(err, collect.ErrNotFound) {
		t.Fatalf("a rejected request must not persist an abonnement: %v", err)
	}
}

func TestDeleteRepositoryData_InterruptsRunningAnalysis(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.subscribe(collect.ImportTen)
	f.clock.Advance(11 * time.Minute)
	f.analyzer.block = make(chan struct{})
	done := make(chan error, 1)
	go func() { done <- f.runOne() }()
	waitFor(t, func() bool { f.analyzer.mu.Lock(); defer f.analyzer.mu.Unlock(); return len(f.analyzer.requests) == 1 })
	if err := f.collector.DeleteRepositoryData(f.ctx, repo, true); err != nil {
		t.Fatal(err)
	}
	if err := <-done; !errors.Is(err, analyse.ErrInterrupted) && !errors.Is(err, analyse.ErrLeaseLost) {
		t.Fatalf("deletion must cancel the running analysis: %v", err)
	}
	if len(f.workspace.cleaned) != 1 {
		t.Fatal("workspace must be cleaned after the interrupted analysis")
	}
}

func TestReconcile_VanishedPullRequestDoesNotBlockAbonnement(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.subscribe(collect.ImportTen)
	f.forge.set() // the pull request was deleted on Forgejo: listing is empty, fetching it is a 404
	if err := f.collector.ReconcileAll(f.ctx); err != nil {
		t.Fatal(err)
	}
	if s := f.status(); len(s.Blocked) != 0 || s.LastSyncAt.IsZero() {
		t.Fatalf("a vanished pull request must not block the abonnement: %+v", s)
	}
}

func TestWorker_RepeatedInterruptionsBecomeUnavailable(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.subscribe(collect.ImportTen)
	f.clock.Advance(11 * time.Minute)
	for range analyse.MaxAttempts {
		if _, err := f.store.Claim(f.ctx, f.clock.Now().Add(time.Minute), "crashed-worker"); err != nil {
			t.Fatal(err)
		}
		f.clock.Advance(2 * time.Minute) // the process died; the lease expired
	}
	if err := f.runOne(); err != nil {
		t.Fatal(err)
	}
	if len(f.analyzer.requests) != 0 {
		t.Fatal("no fourth invocation after three interrupted attempts")
	}
	cards := f.cards(timeline.Filter{})
	if len(cards) != 1 || cards[0].Analysis.Status != pullrequest.AnalysisUnavailable {
		t.Fatalf("exhausted attempts must publish analyse indisponible: %+v", cards)
	}
}

func TestSubscribe_RequiresTheUserToAuthoriseTheEngine(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	subscription, err := f.collector.Subscribe(f.ctx, collect.SubscribeRequest{Repository: repo, HTMLURL: "https://forge.test/" + repo, Import: collect.ImportTen})
	if err != nil {
		t.Fatal(err)
	}
	if subscription.Active || !strings.Contains(subscription.BlockedReason, "not authorised") {
		t.Fatalf("an unauthorised engine must leave the abonnement blocked: %+v", subscription)
	}
	if s := f.status(); len(s.Blocked) != 1 || s.Blocked[0].AuthorisedEngine != "" {
		t.Fatalf("status = %+v", s)
	}
	if err := f.collector.ReconcileAll(f.ctx); err != nil {
		t.Fatal(err)
	}
	if s := f.status(); s.Pending != 0 {
		t.Fatal("a blocked abonnement must not collect")
	}

	// The user authorises the configured engine: collection resumes without
	// recreating the abonnement.
	if err := f.collector.AuthoriseEngine(f.ctx, repo); err != nil {
		t.Fatal(err)
	}
	if err := f.collector.ReconcileAll(f.ctx); err != nil {
		t.Fatal(err)
	}
	status := f.status()
	if len(status.Blocked) != 0 || status.Pending != 1 || !status.Repositories[0].Active {
		t.Fatalf("status after authorisation = %+v", status)
	}
}

func TestReconcile_AChangedEngineBlocksUntilReauthorised(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.subscribe(collect.ImportTen)
	f.drain()

	// The operator switches model; the earlier authorisation no longer applies.
	changed := profile
	changed.Model = "another-model"
	f.collector.Profile = changed
	f.forge.set(observation(42, "sha-2", f.clock.Now().Add(time.Minute)))
	if err := f.collector.ReconcileAll(f.ctx); err != nil {
		t.Fatal(err)
	}
	status := f.status()
	if len(status.Blocked) != 1 || !strings.Contains(status.Blocked[0].BlockedReason, "another-model") || status.Pending != 0 {
		t.Fatalf("a changed engine must block collection: %+v", status)
	}
	if len(f.cards(timeline.Filter{})) != 1 {
		t.Fatal("history and the visible carte must survive the block")
	}
	if err := f.collector.AuthoriseEngine(f.ctx, repo); err != nil {
		t.Fatal(err)
	}
	if err := f.collector.ReconcileAll(f.ctx); err != nil {
		t.Fatal(err)
	}
	if s := f.status(); s.Pending != 1 || len(s.Blocked) != 0 {
		t.Fatalf("status after re-authorisation = %+v", s)
	}
}

func TestPoll_WakingReconcilesWithoutWaitingForATick(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.subscribe(collect.ImportNone)
	ticks := make(chan time.Time, 1)
	wakes := make(chan time.Time, 1)
	f.collector.Tick = func(time.Duration) <-chan time.Time { return ticks }
	f.collector.Wakes = wakes
	ctx, cancel := context.WithCancel(f.ctx)
	done := make(chan struct{})
	go func() { defer close(done); f.collector.Poll(ctx, time.Hour) }()

	launched := f.forge.listed()
	waitFor(t, func() bool { return f.forge.listed() > launched })

	// The next tick is an hour away, so only the resume can explain a second
	// reconciliation of the new head.
	f.forge.set(observation(42, "sha-2", f.clock.Now().Add(time.Minute)))
	f.clock.Advance(2 * time.Hour)
	wakes <- f.clock.Now()
	waitFor(t, func() bool { return f.status().Pending == 1 })
	cancel()
	<-done
}

func TestPoll_ASleepCostsOneCatchUp(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.subscribe(collect.ImportNone)
	ticks := make(chan time.Time, 1)
	wakes := make(chan time.Time, 1)
	f.collector.Tick = func(time.Duration) <-chan time.Time { return ticks }
	f.collector.Wakes = wakes

	// Subscribing already reconciled once; count from there.
	baseline := f.forge.listed()

	// What a sleeping machine leaves behind: the ticker fired while the process
	// was frozen, and the resume was detected. Both are already waiting when
	// the loop next looks, and they mean the same complete reconciliation.
	f.clock.Advance(2 * time.Hour)
	ticks <- f.clock.Now()
	wakes <- f.clock.Now()

	ctx, cancel := context.WithCancel(f.ctx)
	done := make(chan struct{})
	go func() { defer close(done); f.collector.Poll(ctx, time.Hour) }()
	waitFor(t, func() bool { return f.forge.listed() >= baseline+2 }) // launch, then the catch-up
	settle()
	if listed := f.forge.listed() - baseline; listed != 2 {
		t.Fatalf("polling reconciled %d times, want the launch plus one catch-up", listed)
	}
	cancel()
	<-done
}

// settle gives a racing goroutine time to do the thing a test asserts it will
// not do.
func settle() { time.Sleep(150 * time.Millisecond) }
