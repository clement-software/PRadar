package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/clement-software/PRadar/internal/app"
	"github.com/clement-software/PRadar/internal/pullrequest"
)

// Claim grants the next eligible item to the caller in one atomic mutation.
func (s *Store) Claim(ctx context.Context, leaseUntil time.Time, token string) (app.Job, error) {
	if token == "" {
		return app.Job{}, errors.New("lease token is required")
	}
	now := s.now().Unix()
	var job app.Job
	err := s.tx(ctx, func(tx *sql.Tx) error {
		var (
			key     string
			profile string
		)
		err := tx.QueryRowContext(ctx, `
UPDATE analysis_jobs
SET status = 'running', attempts = attempts + 1, lease_until_unix = ?, lease_token = ?
WHERE id = (
  SELECT id FROM analysis_jobs
  WHERE (status = 'queued' AND available_unix <= ?) OR (status = 'running' AND lease_until_unix <= ?)
  ORDER BY available_unix, id LIMIT 1
)
RETURNING id, identity, pr_key, generation, head_sha, input_revision, profile_json, attempts`,
			leaseUntil.Unix(), token, now, now).
			Scan(&job.ID, &job.Identity, &key, &job.Generation, &job.HeadSHA, &job.Revision, &profile, &job.Attempt)
		if errors.Is(err, sql.ErrNoRows) {
			return app.ErrNoWork
		}
		if err != nil {
			return fmt.Errorf("claim analysis work: %w", err)
		}
		job.LeaseToken = token
		if err := json.Unmarshal([]byte(profile), &job.Profile); err != nil {
			return fmt.Errorf("decode analysis profile: %w", err)
		}
		if err := tx.QueryRowContext(ctx, `SELECT repository, number, title, body, author, html_url FROM pull_requests WHERE pr_key = ?`, key).
			Scan(&job.Ref.Repository, &job.Ref.Number, &job.Title, &job.Body, &job.Author, &job.HTMLURL); err != nil {
			return fmt.Errorf("read claimed pull request: %w", err)
		}
		err = tx.QueryRowContext(ctx, `
SELECT head_sha FROM analyses WHERE pr_key = ? AND status = 'ok' AND head_sha <> ? ORDER BY id DESC LIMIT 1`, key, job.HeadSHA).
			Scan(&job.PreviousHeadSHA)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("read previous analysed head: %w", err)
		}
		return nil
	})
	if err != nil {
		return app.Job{}, err
	}
	return job, nil
}

// Complete records the result and, when the identity is still the latest
// observed one of an active abonnement generation, publishes the carte.
func (s *Store) Complete(ctx context.Context, job app.Job, analysis pullrequest.Analysis, provenance pullrequest.Provenance) (bool, error) {
	key := job.Ref.Key()
	now := s.now()
	published := false
	err := s.tx(ctx, func(tx *sql.Tx) error {
		status := "done"
		if analysis.Status == pullrequest.AnalysisUnavailable {
			status = "unavailable"
		}
		n, err := rowsAffected(tx.ExecContext(ctx, `
UPDATE analysis_jobs SET status = ?, lease_until_unix = NULL, lease_token = NULL
WHERE id = ? AND status = 'running' AND lease_token = ?`, status, job.ID, job.LeaseToken))
		if err != nil {
			return fmt.Errorf("complete analysis work: %w", err)
		}
		if n != 1 {
			return app.ErrLeaseLost
		}
		n, err = rowsAffected(tx.ExecContext(ctx, `
UPDATE pull_requests SET published_identity = ?, unread = 1, archived = 0, activity_unix = ?
WHERE pr_key = ? AND observed_identity = ?
  AND EXISTS (SELECT 1 FROM subscriptions s WHERE s.repository = pull_requests.repository AND s.active = 1 AND s.generation = ?)`,
			string(job.Identity), now.Unix(), key, string(job.Identity), job.Generation))
		if err != nil {
			return fmt.Errorf("publish analysis: %w", err)
		}
		published = n == 1
		if _, err := tx.ExecContext(ctx, `
INSERT INTO analyses(identity, pr_key, head_sha, status, result_json, provenance_json, published, created_unix)
VALUES (?, ?, ?, ?, ?, ?, ?, ?) ON CONFLICT(identity) DO NOTHING`,
			string(job.Identity), key, job.HeadSHA, string(analysis.Status), marshal(analysis), marshal(provenance), published, now.Unix()); err != nil {
			return fmt.Errorf("store analysis: %w", err)
		}
		kind := "analysed"
		switch {
		case analysis.Status == pullrequest.AnalysisUnavailable:
			kind = "unavailable"
		case !published:
			kind = "stale"
		}
		return event(ctx, tx, key, now, kind, "head "+job.HeadSHA+" attempt "+strconv.Itoa(job.Attempt))
	})
	return published, err
}

// Retry requeues the item after a technical failure; the attempt stays consumed.
func (s *Store) Retry(ctx context.Context, job app.Job, notBefore time.Time, cause string) error {
	n, err := rowsAffected(s.db.ExecContext(ctx, `
UPDATE analysis_jobs SET status = 'queued', available_unix = ?, lease_until_unix = NULL, lease_token = NULL, last_error = ?
WHERE id = ? AND status = 'running' AND lease_token = ?`, notBefore.Unix(), cause, job.ID, job.LeaseToken))
	if err != nil {
		return fmt.Errorf("schedule retry: %w", err)
	}
	if n != 1 {
		return app.ErrLeaseLost
	}
	return nil
}

// Release hands ownership back without consuming the attempt. Work whose
// abonnement stopped meanwhile is cancelled instead of requeued.
func (s *Store) Release(ctx context.Context, job app.Job) error {
	n, err := rowsAffected(s.db.ExecContext(ctx, `
UPDATE analysis_jobs SET
  status = CASE WHEN EXISTS (
    SELECT 1 FROM subscriptions s JOIN pull_requests p ON p.repository = s.repository
    WHERE p.pr_key = analysis_jobs.pr_key AND s.active = 1 AND s.generation = analysis_jobs.generation
  ) THEN 'queued' ELSE 'cancelled' END,
  attempts = attempts - 1, lease_until_unix = NULL, lease_token = NULL
WHERE id = ? AND status = 'running' AND lease_token = ?`, job.ID, job.LeaseToken))
	if err != nil {
		return fmt.Errorf("release analysis work: %w", err)
	}
	if n != 1 {
		return app.ErrLeaseLost
	}
	return nil
}

// Replay schedules the latest version again under a changed profile.
func (s *Store) Replay(ctx context.Context, ref pullrequest.Ref, profile pullrequest.Profile, notBefore time.Time) error {
	key := ref.Key()
	now := s.now()
	return s.tx(ctx, func(tx *sql.Tx) error {
		var (
			revision   pullrequest.InputRevision
			headSHA    string
			generation int64
		)
		err := tx.QueryRowContext(ctx, `
SELECT p.input_revision, p.head_sha, s.generation FROM pull_requests p
JOIN subscriptions s ON s.repository = p.repository AND s.active = 1 WHERE p.pr_key = ?`, key).Scan(&revision, &headSHA, &generation)
		if errors.Is(err, sql.ErrNoRows) {
			return app.ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("read pull request: %w", err)
		}
		identity := pullrequest.IdentityOf(ref, revision, profile)
		var existing int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM analysis_jobs WHERE identity = ? AND status NOT IN ('superseded', 'cancelled')`, string(identity)).Scan(&existing); err != nil {
			return fmt.Errorf("check replay identity: %w", err)
		}
		if existing > 0 {
			return app.ErrReplayUnchanged
		}
		if _, err := tx.ExecContext(ctx, `UPDATE pull_requests SET observed_identity = ? WHERE pr_key = ?`, string(identity), key); err != nil {
			return fmt.Errorf("record replay identity: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `
INSERT INTO analysis_jobs(identity, pr_key, generation, head_sha, input_revision, profile_json, status, available_unix, created_unix)
VALUES (?, ?, ?, ?, ?, ?, 'queued', ?, ?)
ON CONFLICT(identity) DO UPDATE SET status = 'queued', generation = excluded.generation, available_unix = excluded.available_unix`,
			string(identity), key, generation, headSHA, string(revision), marshal(profile), notBefore.Unix(), now.Unix()); err != nil {
			return fmt.Errorf("schedule replay: %w", err)
		}
		return event(ctx, tx, key, now, "replay", "requested with "+profile.Engine+"/"+profile.Model+" prompt "+profile.PromptVersion+" skill "+profile.SkillVersion)
	})
}
