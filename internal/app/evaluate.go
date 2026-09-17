package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/clement-software/PRadar/internal/evaluation"
	"github.com/clement-software/PRadar/internal/pullrequest"
)

// ErrNoCorpus is returned before any corpus has been frozen.
var ErrNoCorpus = errors.New("no corpus has been frozen")

// EvaluationStore persists frozen corpora and scores.
type EvaluationStore interface {
	// FreezeCorpus stores the manifest under its content id; freezing the
	// same manifest twice is a no-op.
	FreezeCorpus(ctx context.Context, id string, manifest evaluation.Manifest) error
	// CurrentCorpus returns the most recently frozen corpus or ErrNoCorpus.
	CurrentCorpus(ctx context.Context) (id string, manifest evaluation.Manifest, err error)
	RecordScore(ctx context.Context, corpusID, key string, score evaluation.Score) error
	Scores(ctx context.Context, corpusID string) (map[string]evaluation.Score, error)
}

// Evaluator owns corpus freezing, scoring and the report.
type Evaluator struct {
	Store   EvaluationStore
	Read    ReadModel
	Profile pullrequest.Profile
	Now     func() time.Time
}

// Freeze validates and stores a manifest, returning its corpus id.
func (e *Evaluator) Freeze(ctx context.Context, manifest evaluation.Manifest) (string, error) {
	if err := manifest.Validate(); err != nil {
		return "", err
	}
	id := manifest.ID()
	if err := e.Store.FreezeCorpus(ctx, id, manifest); err != nil {
		return "", err
	}
	return id, nil
}

// Scorecard is the evaluator's input for one item.
type Scorecard struct {
	Elapsed       time.Duration
	Answers       evaluation.Answers
	Useful        bool
	CriticalError bool
	Notes         string
}

// Score records the assessment of the item's currently published analysis.
// The analysis must match the frozen head, otherwise the corpus changed.
func (e *Evaluator) Score(ctx context.Context, ref pullrequest.Ref, card Scorecard) error {
	id, manifest, err := e.Store.CurrentCorpus(ctx)
	if err != nil {
		return err
	}
	item, ok := findItem(manifest, ref)
	if !ok {
		return fmt.Errorf("%s is not part of corpus %s", ref.Key(), id)
	}
	detail, err := e.Read.GetDetail(ctx, ref)
	if err != nil {
		return err
	}
	if !detail.HasCard {
		return fmt.Errorf("%s has no published analysis to score", ref.Key())
	}
	var identity pullrequest.Identity
	for _, entry := range detail.History {
		if entry.Published {
			identity = entry.Identity
		}
	}
	score := evaluation.Score{
		Identity: identity, HeadSHA: detail.Card.Analysis.HeadSHA, Profile: detail.Provenance.Profile,
		Elapsed: card.Elapsed, Answers: card.Answers, Useful: card.Useful, CriticalError: card.CriticalError, Notes: card.Notes,
		AnalysisDuration: detail.Provenance.Duration, Usage: detail.Provenance.Usage, RecordedAt: e.Now().UTC(),
	}
	if err := score.Validate(item); err != nil {
		return err
	}
	return e.Store.RecordScore(ctx, id, ref.Key(), score)
}

// Progress is the evaluation page model.
type Progress struct {
	CorpusID string
	Manifest evaluation.Manifest
	Scores   map[string]evaluation.Score
	Report   evaluation.Report
}

// Report builds the deterministic report of the current corpus.
func (e *Evaluator) Report(ctx context.Context) (Progress, error) {
	id, manifest, err := e.Store.CurrentCorpus(ctx)
	if err != nil {
		return Progress{}, err
	}
	scores, err := e.Store.Scores(ctx, id)
	if err != nil {
		return Progress{}, err
	}
	return Progress{CorpusID: id, Manifest: manifest, Scores: scores, Report: evaluation.BuildReport(manifest, scores, e.Profile, e.Now())}, nil
}

// Item returns the corpus item of a pull request when it is part of the current corpus.
func (e *Evaluator) Item(ctx context.Context, ref pullrequest.Ref) (evaluation.Item, bool) {
	_, manifest, err := e.Store.CurrentCorpus(ctx)
	if err != nil {
		return evaluation.Item{}, false
	}
	return findItem(manifest, ref)
}

func findItem(manifest evaluation.Manifest, ref pullrequest.Ref) (evaluation.Item, bool) {
	for _, item := range manifest.Items {
		if item.Ref() == ref {
			return item, true
		}
	}
	return evaluation.Item{}, false
}
