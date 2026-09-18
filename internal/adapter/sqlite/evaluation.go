package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/clement-software/PRadar/internal/evaluation"
	"github.com/clement-software/PRadar/internal/timeline"
)

// FreezeCorpus stores the manifest immutably under its content id.
func (s *Store) FreezeCorpus(ctx context.Context, id string, manifest evaluation.Manifest) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO corpora(id, frozen_unix, manifest_json) VALUES (?, ?, ?) ON CONFLICT(id) DO NOTHING`,
		id, s.now().Unix(), marshal(manifest))
	if err != nil {
		return fmt.Errorf("freeze corpus: %w", err)
	}
	return nil
}

// CurrentCorpus returns the most recently frozen corpus.
func (s *Store) CurrentCorpus(ctx context.Context) (string, evaluation.Manifest, error) {
	var (
		id  string
		raw string
	)
	err := s.db.QueryRowContext(ctx, `SELECT id, manifest_json FROM corpora ORDER BY frozen_unix DESC, id DESC LIMIT 1`).Scan(&id, &raw)
	if errors.Is(err, sql.ErrNoRows) {
		return "", evaluation.Manifest{}, timeline.ErrNoCorpus
	}
	if err != nil {
		return "", evaluation.Manifest{}, fmt.Errorf("read corpus: %w", err)
	}
	var manifest evaluation.Manifest
	if err := json.Unmarshal([]byte(raw), &manifest); err != nil {
		return "", evaluation.Manifest{}, fmt.Errorf("decode corpus: %w", err)
	}
	return id, manifest, nil
}

// RecordScore stores or replaces the score of one corpus item.
func (s *Store) RecordScore(ctx context.Context, corpusID, key string, score evaluation.Score) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO scores(corpus_id, pr_key, score_json, recorded_unix) VALUES (?, ?, ?, ?)
ON CONFLICT(corpus_id, pr_key) DO UPDATE SET score_json = excluded.score_json, recorded_unix = excluded.recorded_unix`,
		corpusID, key, marshal(score), s.now().Unix())
	if err != nil {
		return fmt.Errorf("record score: %w", err)
	}
	return nil
}

// Scores returns every score of a corpus keyed by pull request.
func (s *Store) Scores(ctx context.Context, corpusID string) (map[string]evaluation.Score, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT pr_key, score_json FROM scores WHERE corpus_id = ?`, corpusID)
	if err != nil {
		return nil, fmt.Errorf("list scores: %w", err)
	}
	defer rows.Close()
	scores := map[string]evaluation.Score{}
	for rows.Next() {
		var key, raw string
		if err := rows.Scan(&key, &raw); err != nil {
			return nil, fmt.Errorf("scan score: %w", err)
		}
		var score evaluation.Score
		if err := json.Unmarshal([]byte(raw), &score); err != nil {
			return nil, fmt.Errorf("decode score: %w", err)
		}
		scores[key] = score
	}
	return scores, rows.Err()
}

var _ timeline.EvaluationStore = (*Store)(nil)
