package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/clement-software/PRadar/internal/app"
	"github.com/clement-software/PRadar/internal/pullrequest"
)

// PutSubscription inserts or reactivates an abonnement. The generation is
// never reset so late work from a previous activation stays ineligible.
func (s *Store) PutSubscription(ctx context.Context, subscription app.Subscription) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO subscriptions(repository, html_url, active, blocked_reason, excluded_authors, created_unix)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT(repository) DO UPDATE SET
  html_url = excluded.html_url,
  active = excluded.active,
  blocked_reason = excluded.blocked_reason,
  excluded_authors = excluded.excluded_authors`,
		subscription.Repository, subscription.HTMLURL, subscription.Active, subscription.BlockedReason,
		marshal(nonNil(subscription.ExcludedAuthors)), s.now().Unix())
	if err != nil {
		return fmt.Errorf("store abonnement: %w", err)
	}
	return nil
}

func nonNil(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

const subscriptionColumns = `repository, html_url, generation, active, blocked_reason, excluded_authors, last_sync_unix`

func scanSubscription(row interface{ Scan(...any) error }) (app.Subscription, error) {
	var (
		subscription app.Subscription
		authors      string
		lastSync     int64
	)
	if err := row.Scan(&subscription.Repository, &subscription.HTMLURL, &subscription.Generation, &subscription.Active,
		&subscription.BlockedReason, &authors, &lastSync); err != nil {
		return app.Subscription{}, err
	}
	if err := json.Unmarshal([]byte(authors), &subscription.ExcludedAuthors); err != nil {
		return app.Subscription{}, fmt.Errorf("decode excluded authors: %w", err)
	}
	subscription.LastSyncAt = fromUnix(lastSync)
	return subscription, nil
}

// GetSubscription returns one abonnement or app.ErrNotFound.
func (s *Store) GetSubscription(ctx context.Context, repository string) (app.Subscription, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+subscriptionColumns+` FROM subscriptions WHERE repository = ?`, repository)
	subscription, err := scanSubscription(row)
	if errors.Is(err, sql.ErrNoRows) {
		return app.Subscription{}, app.ErrNotFound
	}
	if err != nil {
		return app.Subscription{}, fmt.Errorf("read abonnement: %w", err)
	}
	return subscription, nil
}

// ListSubscriptions returns every abonnement, active or not, by repository.
func (s *Store) ListSubscriptions(ctx context.Context) ([]app.Subscription, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+subscriptionColumns+` FROM subscriptions ORDER BY repository`)
	if err != nil {
		return nil, fmt.Errorf("list abonnements: %w", err)
	}
	defer rows.Close()
	var subscriptions []app.Subscription
	for rows.Next() {
		subscription, err := scanSubscription(rows)
		if err != nil {
			return nil, fmt.Errorf("scan abonnement: %w", err)
		}
		subscriptions = append(subscriptions, subscription)
	}
	return subscriptions, rows.Err()
}

// RecordSync stores the last successful synchronisation or the blocking reason.
func (s *Store) RecordSync(ctx context.Context, repository string, at time.Time, blockedReason string) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE subscriptions
SET last_sync_unix = CASE WHEN ? > 0 THEN ? ELSE last_sync_unix END, blocked_reason = ?
WHERE repository = ?`, unix(at), unix(at), blockedReason, repository)
	if err != nil {
		return fmt.Errorf("record synchronisation: %w", err)
	}
	return nil
}

// Unsubscribe deactivates the abonnement, bumps its generation and cancels
// queued work while keeping every pull request, analysis and event.
func (s *Store) Unsubscribe(ctx context.Context, repository string) error {
	return s.tx(ctx, func(tx *sql.Tx) error {
		n, err := rowsAffected(tx.ExecContext(ctx, `UPDATE subscriptions SET active = 0, generation = generation + 1 WHERE repository = ?`, repository))
		if err != nil {
			return fmt.Errorf("deactivate abonnement: %w", err)
		}
		if n == 0 {
			return app.ErrNotFound
		}
		if _, err := tx.ExecContext(ctx, `
UPDATE analysis_jobs SET status = 'cancelled'
WHERE status = 'queued' AND pr_key IN (SELECT pr_key FROM pull_requests WHERE repository = ?)`, repository); err != nil {
			return fmt.Errorf("cancel queued work: %w", err)
		}
		return nil
	})
}

// DeleteRepositoryData removes the abonnement and everything attached to it.
func (s *Store) DeleteRepositoryData(ctx context.Context, repository string) error {
	n, err := rowsAffected(s.db.ExecContext(ctx, `DELETE FROM subscriptions WHERE repository = ?`, repository))
	if err != nil {
		return fmt.Errorf("delete repository data: %w", err)
	}
	if n == 0 {
		return app.ErrNotFound
	}
	return nil
}

// ListLocallyOpen returns the pull requests still recorded as open.
func (s *Store) ListLocallyOpen(ctx context.Context, repository string) ([]pullrequest.Ref, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT repository, number FROM pull_requests WHERE repository = ? AND state = 'open' ORDER BY number`, repository)
	if err != nil {
		return nil, fmt.Errorf("list open pull requests: %w", err)
	}
	defer rows.Close()
	var refs []pullrequest.Ref
	for rows.Next() {
		var ref pullrequest.Ref
		if err := rows.Scan(&ref.Repository, &ref.Number); err != nil {
			return nil, fmt.Errorf("scan pull request: %w", err)
		}
		refs = append(refs, ref)
	}
	return refs, rows.Err()
}

// ObservePullRequest applies the lifecycle decision and its scheduling effect
// in one transaction. Repeating an observation is a successful no-op.
func (s *Store) ObservePullRequest(ctx context.Context, request app.ObservationRequest) (pullrequest.Decision, error) {
	observation := request.Observation
	key := observation.Ref.Key()
	now := s.now()
	var decision pullrequest.Decision
	err := s.tx(ctx, func(tx *sql.Tx) error {
		var (
			stored  pullrequest.Stored
			updated int64
		)
		err := tx.QueryRowContext(ctx, `SELECT state, updated_unix, reopen_generation, input_revision FROM pull_requests WHERE pr_key = ?`, key).
			Scan(&stored.State, &updated, &stored.ReopenGeneration, &stored.Revision)
		switch {
		case errors.Is(err, sql.ErrNoRows):
		case err != nil:
			return fmt.Errorf("read pull request: %w", err)
		default:
			stored.Exists = true
			stored.UpdatedAt = fromUnix(updated)
		}
		decision = pullrequest.Reconcile(stored, observation)
		switch decision.Kind {
		case pullrequest.DecideIgnore:
			return nil
		case pullrequest.DecideUpdateState:
			if _, err := tx.ExecContext(ctx, `
UPDATE pull_requests SET state = ?, title = ?, html_url = ?, updated_unix = ?, activity_unix = ? WHERE pr_key = ?`,
				observation.State, observation.Title, observation.HTMLURL, observation.UpdatedAt.Unix(), now.Unix(), key); err != nil {
				return fmt.Errorf("update pull request state: %w", err)
			}
			if _, err := tx.ExecContext(ctx, `UPDATE analysis_jobs SET status = 'cancelled' WHERE pr_key = ? AND status = 'queued'`, key); err != nil {
				return fmt.Errorf("cancel queued work: %w", err)
			}
			return event(ctx, tx, key, now, "state", string(observation.State))
		case pullrequest.DecideSchedule:
			return s.schedule(ctx, tx, request, decision, now)
		}
		return fmt.Errorf("unknown decision %d", decision.Kind)
	})
	return decision, err
}

func (s *Store) schedule(ctx context.Context, tx *sql.Tx, request app.ObservationRequest, decision pullrequest.Decision, now time.Time) error {
	observation := request.Observation
	key := observation.Ref.Key()
	identity := pullrequest.IdentityOf(observation.Ref, decision.Revision, request.Profile)
	if _, err := tx.ExecContext(ctx, `
INSERT INTO pull_requests(pr_key, repository, number, title, body, author, state, html_url, head_sha, updated_unix, activity_unix,
  reopen_generation, input_revision, observed_identity, unread)
VALUES (?, ?, ?, ?, ?, ?, 'open', ?, ?, ?, ?, ?, ?, ?, 1)
ON CONFLICT(pr_key) DO UPDATE SET
  title = excluded.title, body = excluded.body, author = excluded.author, state = 'open', html_url = excluded.html_url,
  head_sha = excluded.head_sha, updated_unix = excluded.updated_unix, activity_unix = excluded.activity_unix,
  reopen_generation = excluded.reopen_generation, input_revision = excluded.input_revision,
  observed_identity = excluded.observed_identity, unread = 1`,
		key, observation.Ref.Repository, observation.Ref.Number, observation.Title, observation.Body, observation.Author,
		observation.HTMLURL, observation.HeadSHA, observation.UpdatedAt.Unix(), now.Unix(),
		decision.ReopenGeneration, string(decision.Revision), string(identity)); err != nil {
		return fmt.Errorf("store observed version: %w", err)
	}
	if !request.Schedule {
		return event(ctx, tx, key, now, "observed", "version recorded without analysis")
	}
	var generation int64
	if err := tx.QueryRowContext(ctx, `SELECT generation FROM subscriptions WHERE repository = ? AND active = 1`, observation.Ref.Repository).Scan(&generation); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("repository %s has no active abonnement", observation.Ref.Repository)
		}
		return fmt.Errorf("read abonnement generation: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `UPDATE analysis_jobs SET status = 'superseded' WHERE pr_key = ? AND status = 'queued' AND identity <> ?`, key, string(identity)); err != nil {
		return fmt.Errorf("supersede pending candidate: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO analysis_jobs(identity, pr_key, generation, head_sha, input_revision, profile_json, status, available_unix, created_unix)
VALUES (?, ?, ?, ?, ?, ?, 'queued', ?, ?)
ON CONFLICT(identity) DO UPDATE SET status = 'queued', generation = excluded.generation, available_unix = excluded.available_unix
  WHERE analysis_jobs.status IN ('superseded', 'cancelled')`,
		string(identity), key, generation, observation.HeadSHA, string(decision.Revision), marshal(request.Profile),
		request.NotBefore.Unix(), now.Unix()); err != nil {
		return fmt.Errorf("schedule analysis: %w", err)
	}
	return event(ctx, tx, key, now, "observed", "version "+observation.HeadSHA+" scheduled after anti-rebond")
}
