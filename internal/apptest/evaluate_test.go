package app_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/clement-software/PRadar/internal/app"
	"github.com/clement-software/PRadar/internal/evaluation"
	"github.com/clement-software/PRadar/internal/pullrequest"
)

func corpusManifest(head string) evaluation.Manifest {
	var m evaluation.Manifest
	for i := range evaluation.CorpusSize {
		item := evaluation.Item{Repository: repo, Number: int64(100 + i), HeadSHA: "sha-frozen-" + strings.Repeat("x", 10),
			Size: evaluation.SizeSmall, Authorship: evaluation.AuthorHuman, Category: evaluation.CategoryCode, Reason: "breadth"}
		if i%2 == 1 {
			item.Repository, item.Size, item.Authorship = "acme/gadgets", evaluation.SizeLarge, evaluation.AuthorAgent
		}
		if i%3 == 1 {
			item.Category = evaluation.CategoryCI
		}
		if i%3 == 2 {
			item.Category = evaluation.CategoryInfra
		}
		m.Items = append(m.Items, item)
	}
	m.Items[0] = evaluation.Item{Repository: repo, Number: 42, HeadSHA: head, Size: evaluation.SizeSmall, Authorship: evaluation.AuthorHuman, Category: evaluation.CategoryCode, Reason: "the analysed fixture"}
	return m
}

func TestEvaluator_FreezeScoreAndReport(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.forge.set(observation(42, "sha-1111111", f.clock.Now()))
	f.subscribe(app.ImportTen)
	f.drain()
	evaluator := &app.Evaluator{Store: f.store, Read: f.store, Profile: profile, Now: f.clock.Now}
	if _, err := evaluator.Report(f.ctx); err == nil {
		t.Fatal("report before freezing must fail")
	}
	if _, err := evaluator.Freeze(f.ctx, evaluation.Manifest{}); err == nil {
		t.Fatal("empty manifest accepted")
	}
	id, err := evaluator.Freeze(f.ctx, corpusManifest("sha-1111111"))
	if err != nil {
		t.Fatal(err)
	}
	if again, err := evaluator.Freeze(f.ctx, corpusManifest("sha-1111111")); err != nil || again != id {
		t.Fatalf("refreezing the same manifest = %s, %v", again, err)
	}
	card := app.Scorecard{Elapsed: 45 * time.Second, Answers: evaluation.Answers{Intent: true, Structure: true, Risks: true, ReviewNeeded: true}, Useful: true}
	if err := evaluator.Score(f.ctx, pullrequest.Ref{Repository: repo, Number: 999}, card); err == nil {
		t.Fatal("scoring outside the corpus accepted")
	}
	if err := evaluator.Score(f.ctx, pullrequest.Ref{Repository: repo, Number: 100}, card); err == nil {
		t.Fatal("scoring an item without a published analysis accepted")
	}
	if err := evaluator.Score(f.ctx, ref, card); err != nil {
		t.Fatal(err)
	}
	progress, err := evaluator.Report(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	score := progress.Scores[ref.Key()]
	if !score.Passed() || score.Identity == "" || score.Usage["input_tokens"] == nil || score.Profile != profile {
		t.Fatalf("score = %+v", score)
	}
	if progress.Report.Verdict != evaluation.VerdictIncomplete || progress.Report.Scored != 1 || progress.Report.Passed != 1 {
		t.Fatalf("report = %+v", progress.Report)
	}
	exported, _ := json.Marshal(progress)
	if strings.Contains(string(exported), "\"body\"") || strings.Contains(string(exported), observation(42, "", time.Time{}).Body) {
		t.Fatal("evaluation export must not contain bodies or credentials")
	}
	// A new version of the fixture no longer matches the frozen head: the score is refused.
	f.forge.set(observation(42, "sha-2222222", f.clock.Now().Add(time.Minute)))
	if err := f.collector.ReconcileAll(f.ctx); err != nil {
		t.Fatal(err)
	}
	f.drain()
	if err := evaluator.Score(f.ctx, ref, card); err == nil || !strings.Contains(err.Error(), "frozen head") {
		t.Fatalf("score against a changed head = %v", err)
	}
	// A changed profile invalidates the earlier aggregate.
	evaluator.Profile.PromptVersion = "p2"
	progress, _ = evaluator.Report(f.ctx)
	if len(progress.Report.Invalid) != 1 || progress.Report.Scored != 0 {
		t.Fatalf("changed profile must invalidate: %+v", progress.Report)
	}
}
