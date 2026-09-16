package pullrequest_test

import (
	"testing"
	"time"

	"github.com/clement-software/PRadar/internal/pullrequest"
)

var (
	ref = pullrequest.Ref{Repository: "acme/widgets", Number: 42}
	t0  = time.Date(2026, 9, 16, 10, 0, 0, 0, time.UTC)
)

func observed(head string, state pullrequest.State, at time.Time) pullrequest.Observation {
	return pullrequest.Observation{Ref: ref, Title: "Title", Body: "Body", State: state, HeadSHA: head, UpdatedAt: at}
}

func stored(state pullrequest.State, at time.Time, generation int64, revision pullrequest.InputRevision) pullrequest.Stored {
	return pullrequest.Stored{Exists: true, State: state, UpdatedAt: at, ReopenGeneration: generation, Revision: revision}
}

func TestReconcile(t *testing.T) {
	t.Parallel()
	rev1 := pullrequest.ComputeInputRevision("sha-1", "Title", "Body", 0)
	rev1Reopened := pullrequest.ComputeInputRevision("sha-1", "Title", "Body", 1)
	titleChanged := observed("sha-1", pullrequest.StateOpen, t0.Add(time.Minute))
	titleChanged.Title = "New title"
	draft := observed("sha-1", pullrequest.StateOpen, t0)
	draft.Draft = true

	cases := []struct {
		name        string
		stored      pullrequest.Stored
		observation pullrequest.Observation
		want        pullrequest.DecisionKind
		generation  int64
	}{
		{"new open pull request schedules", pullrequest.Stored{}, observed("sha-1", pullrequest.StateOpen, t0), pullrequest.DecideSchedule, 0},
		{"new draft is ignored", pullrequest.Stored{}, draft, pullrequest.DecideIgnore, 0},
		{"unknown closed pull request is ignored", pullrequest.Stored{}, observed("sha-1", pullrequest.StateClosed, t0), pullrequest.DecideIgnore, 0},
		{"same revision is a no-op", stored(pullrequest.StateOpen, t0, 0, rev1), observed("sha-1", pullrequest.StateOpen, t0), pullrequest.DecideIgnore, 0},
		{"new head schedules", stored(pullrequest.StateOpen, t0, 0, rev1), observed("sha-2", pullrequest.StateOpen, t0.Add(time.Minute)), pullrequest.DecideSchedule, 0},
		{"title change schedules", stored(pullrequest.StateOpen, t0, 0, rev1), titleChanged, pullrequest.DecideSchedule, 0},
		{"older observation never regresses state", stored(pullrequest.StateOpen, t0, 0, rev1), observed("sha-0", pullrequest.StateOpen, t0.Add(-time.Minute)), pullrequest.DecideIgnore, 0},
		{"close updates state only", stored(pullrequest.StateOpen, t0, 0, rev1), observed("sha-1", pullrequest.StateClosed, t0.Add(time.Minute)), pullrequest.DecideUpdateState, 0},
		{"merge updates state only", stored(pullrequest.StateOpen, t0, 0, rev1), observed("sha-1", pullrequest.StateMerged, t0.Add(time.Minute)), pullrequest.DecideUpdateState, 0},
		{"repeated close is a no-op", stored(pullrequest.StateClosed, t0, 0, rev1), observed("sha-1", pullrequest.StateClosed, t0), pullrequest.DecideIgnore, 0},
		{"reopen increments the generation and schedules", stored(pullrequest.StateClosed, t0, 0, rev1), observed("sha-1", pullrequest.StateOpen, t0.Add(time.Minute)), pullrequest.DecideSchedule, 1},
		{"second reopen increments again", stored(pullrequest.StateMerged, t0, 1, rev1Reopened), observed("sha-1", pullrequest.StateOpen, t0.Add(time.Minute)), pullrequest.DecideSchedule, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := pullrequest.Reconcile(tc.stored, tc.observation)
			if got.Kind != tc.want || got.ReopenGeneration != tc.generation {
				t.Fatalf("Reconcile() = %+v, want kind %v generation %d", got, tc.want, tc.generation)
			}
			if tc.want == pullrequest.DecideSchedule && tc.generation == 1 && got.Revision != rev1Reopened {
				t.Fatalf("reopen revision = %s, want %s", got.Revision, rev1Reopened)
			}
		})
	}
}

func TestInputRevision_NormalisesWhitespaceOnly(t *testing.T) {
	t.Parallel()
	a := pullrequest.ComputeInputRevision("sha", "Title  ", "line\r\nnext  \n", 0)
	b := pullrequest.ComputeInputRevision("sha", "Title", "line\nnext", 0)
	c := pullrequest.ComputeInputRevision("sha", "Title", "line\nnext!", 0)
	if a != b {
		t.Fatal("trailing whitespace and line endings must not change the revision")
	}
	if a == c {
		t.Fatal("a description change must change the revision")
	}
}

func TestIdentity_ChangesWithProfile(t *testing.T) {
	t.Parallel()
	rev := pullrequest.ComputeInputRevision("sha", "t", "b", 0)
	base := pullrequest.Profile{PromptVersion: "p1", SkillVersion: "s1", Engine: "claude-cli", Model: "m"}
	changed := base
	changed.PromptVersion = "p2"
	if pullrequest.IdentityOf(ref, rev, base) == pullrequest.IdentityOf(ref, rev, changed) {
		t.Fatal("a changed prompt version must produce a new analysis identity")
	}
}

func TestAnalysis_Validate(t *testing.T) {
	t.Parallel()
	valid := pullrequest.Analysis{
		SchemaVersion: pullrequest.SchemaVersion, PullRequest: ref.Key(), HeadSHA: "sha-2", PreviousHeadSHA: "sha-1",
		Status: pullrequest.AnalysisOK, Intent: "Add retry", Importance: pullrequest.ImportanceLow, Risks: []string{},
		Body: "# Body", ChangeSincePrevious: "Adds a test",
	}
	if err := valid.Validate(ref, "sha-2", "sha-1"); err != nil {
		t.Fatalf("valid analysis rejected: %v", err)
	}
	mutate := func(f func(a *pullrequest.Analysis)) pullrequest.Analysis { a := valid; f(&a); return a }
	rejected := map[string]pullrequest.Analysis{
		"schema":         mutate(func(a *pullrequest.Analysis) { a.SchemaVersion = "pradar.analysis.v2" }),
		"pull request":   mutate(func(a *pullrequest.Analysis) { a.PullRequest = "acme/widgets#43" }),
		"head":           mutate(func(a *pullrequest.Analysis) { a.HeadSHA = "sha-3" }),
		"previous head":  mutate(func(a *pullrequest.Analysis) { a.PreviousHeadSHA = "" }),
		"status":         mutate(func(a *pullrequest.Analysis) { a.Status = "maybe" }),
		"intent":         mutate(func(a *pullrequest.Analysis) { a.Intent = " " }),
		"importance":     mutate(func(a *pullrequest.Analysis) { a.Importance = "huge" }),
		"nil risks":      mutate(func(a *pullrequest.Analysis) { a.Risks = nil }),
		"body":           mutate(func(a *pullrequest.Analysis) { a.Body = "" }),
		"missing change": mutate(func(a *pullrequest.Analysis) { a.ChangeSincePrevious = "" }),
	}
	for name, analysis := range rejected {
		if err := analysis.Validate(ref, "sha-2", "sha-1"); err == nil {
			t.Errorf("%s: invalid analysis accepted", name)
		}
	}
	if err := pullrequest.Unavailable(ref, "sha-2", "sha-1").Validate(ref, "sha-2", "sha-1"); err != nil {
		t.Fatalf("unavailable contract rejected: %v", err)
	}
}

func TestParseKey(t *testing.T) {
	t.Parallel()
	got, err := pullrequest.ParseKey(ref.Key())
	if err != nil || got != ref {
		t.Fatalf("ParseKey(%q) = %+v, %v", ref.Key(), got, err)
	}
	for _, bad := range []string{"", "acme/widgets", "#1", "acme/widgets#0", "acme/widgets#x"} {
		if _, err := pullrequest.ParseKey(bad); err == nil {
			t.Errorf("ParseKey(%q) accepted", bad)
		}
	}
}
