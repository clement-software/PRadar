package evaluation_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/clement-software/PRadar/internal/evaluation"
	"github.com/clement-software/PRadar/internal/pullrequest"
)

var (
	profile  = pullrequest.Profile{PromptVersion: "p1", SkillVersion: "s1", Engine: "claude-cli", Model: "m"}
	versions = evaluation.Versions{Contract: "pradar.analysis.v1", Presentation: "v1"}
)

func manifest() evaluation.Manifest {
	var m evaluation.Manifest
	sizes := []evaluation.Size{evaluation.SizeSmall, evaluation.SizeLarge}
	authors := []evaluation.Authorship{evaluation.AuthorHuman, evaluation.AuthorAgent}
	categories := []evaluation.Category{evaluation.CategoryCode, evaluation.CategoryCI, evaluation.CategoryInfra}
	for i := range evaluation.CorpusSize {
		repo := "acme/widgets"
		if i%2 == 1 {
			repo = "acme/gadgets"
		}
		m.Items = append(m.Items, evaluation.Item{
			Repository: repo, Number: int64(i + 1), HeadSHA: strings.Repeat("a", 39) + string(rune('a'+i)),
			Size: sizes[i%2], Authorship: authors[i%2], Category: categories[i%3], Reason: "breadth",
		})
	}
	return m
}

func score(item evaluation.Item, elapsed time.Duration, useful, critical bool) evaluation.Score {
	return evaluation.Score{
		Identity: pullrequest.Identity("id-" + item.Ref().Key()), HeadSHA: item.HeadSHA, Profile: profile, Versions: versions, Elapsed: elapsed,
		Answers: evaluation.Answers{Intent: true, Structure: true, Risks: true, ReviewNeeded: true}, Useful: useful, CriticalError: critical,
	}
}

func TestManifest_Validate(t *testing.T) {
	t.Parallel()
	valid := manifest()
	if err := valid.Validate(); err != nil {
		t.Fatal(err)
	}
	mutate := func(f func(m *evaluation.Manifest)) evaluation.Manifest { m := manifest(); f(&m); return m }
	rejected := map[string]evaluation.Manifest{
		"nineteen items": mutate(func(m *evaluation.Manifest) { m.Items = m.Items[:19] }),
		"duplicate":      mutate(func(m *evaluation.Manifest) { m.Items[1] = m.Items[0] }),
		"one repository": mutate(func(m *evaluation.Manifest) {
			for i := range m.Items {
				m.Items[i].Repository = "acme/widgets"
			}
		}),
		"four repos": mutate(func(m *evaluation.Manifest) { m.Items[0].Repository, m.Items[2].Repository = "a/b", "c/d" }),
		"no large change": mutate(func(m *evaluation.Manifest) {
			for i := range m.Items {
				m.Items[i].Size = evaluation.SizeSmall
			}
		}),
		"no agent author": mutate(func(m *evaluation.Manifest) {
			for i := range m.Items {
				m.Items[i].Authorship = evaluation.AuthorHuman
			}
		}),
		"no infra": mutate(func(m *evaluation.Manifest) {
			for i := range m.Items {
				m.Items[i].Category = evaluation.CategoryCode
			}
		}),
		"missing reason":   mutate(func(m *evaluation.Manifest) { m.Items[3].Reason = " " }),
		"short head":       mutate(func(m *evaluation.Manifest) { m.Items[3].HeadSHA = "abc" }),
		"unknown category": mutate(func(m *evaluation.Manifest) { m.Items[3].Category = "docs" }),
	}
	for name, m := range rejected {
		if err := m.Validate(); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
	if valid.ID() == rejected["short head"].ID() || valid.ID() != manifest().ID() {
		t.Fatal("corpus id must be a deterministic content hash")
	}
}

func TestBuildReport_ThresholdEdgesAndInvalidation(t *testing.T) {
	t.Parallel()
	m := manifest()
	now := time.Date(2026, 9, 17, 10, 0, 0, 0, time.UTC)
	scores := func(passing int, critical bool, slow bool) map[string]evaluation.Score {
		out := map[string]evaluation.Score{}
		for i, item := range m.Items {
			elapsed := 40 * time.Second
			if slow && i == 0 {
				elapsed = 61 * time.Second
			}
			out[item.Ref().Key()] = score(item, elapsed, i < passing, critical && i == 0)
		}
		return out
	}
	cases := map[string]struct {
		scores map[string]evaluation.Score
		want   string
		passed int
	}{
		"sixteen useful passes": {scores(16, false, false), evaluation.VerdictPass, 16},
		"fifteen useful fails":  {scores(15, false, false), evaluation.VerdictFail, 15},
		"critical error fails":  {scores(20, true, false), evaluation.VerdictFail, 19},
		"over one minute fails": {scores(16, false, true), evaluation.VerdictFail, 15},
		"all twenty passes":     {scores(20, false, false), evaluation.VerdictPass, 20},
	}
	for name, tc := range cases {
		report := evaluation.BuildReport(m, tc.scores, profile, versions, now)
		if report.Verdict != tc.want || report.Passed != tc.passed || !report.Complete || len(report.Items) != 20 {
			t.Errorf("%s: verdict=%s passed=%d complete=%v", name, report.Verdict, report.Passed, report.Complete)
		}
	}
	incomplete := scores(20, false, false)
	delete(incomplete, m.Items[5].Ref().Key())
	if report := evaluation.BuildReport(m, incomplete, profile, versions, now); report.Verdict != evaluation.VerdictIncomplete || report.Scored != 19 {
		t.Fatalf("incomplete run = %s scored %d", report.Verdict, report.Scored)
	}
	changed := profile
	changed.PromptVersion = "p2"
	if report := evaluation.BuildReport(m, scores(20, false, false), changed, versions, now); report.Verdict != evaluation.VerdictIncomplete || len(report.Invalid) != 20 {
		t.Fatalf("a changed profile must invalidate every earlier score: %s %d", report.Verdict, len(report.Invalid))
	}
	if report := evaluation.BuildReport(m, scores(20, false, false), profile, evaluation.Versions{Contract: "pradar.analysis.v1", Presentation: "v2"}, now); report.Verdict != evaluation.VerdictIncomplete || len(report.Invalid) != 20 {
		t.Fatalf("a changed presentation must invalidate every earlier score: %s %d", report.Verdict, len(report.Invalid))
	}
	first, _ := json.Marshal(evaluation.BuildReport(m, scores(16, false, false), profile, versions, now))
	second, _ := json.Marshal(evaluation.BuildReport(m, scores(16, false, false), profile, versions, now))
	if string(first) != string(second) {
		t.Fatal("report generation must be deterministic")
	}
	if strings.Contains(string(first), "body") {
		t.Fatal("report must not carry pull-request bodies")
	}
	if err := score(m.Items[0], 0, true, false).Validate(m.Items[0]); err == nil {
		t.Fatal("unmeasured comprehension time accepted")
	}
	wrongHead := score(m.Items[0], time.Second, true, false)
	wrongHead.HeadSHA = "other"
	if err := wrongHead.Validate(m.Items[0]); err == nil {
		t.Fatal("score for another head accepted")
	}
}
