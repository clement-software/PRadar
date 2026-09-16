package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/clement-software/PRadar/internal/app"
	"github.com/clement-software/PRadar/internal/pullrequest"
)

const cardColumns = `p.repository, p.number, p.title, p.author, p.state, p.html_url, p.updated_unix, p.activity_unix, p.unread, a.result_json`

func scanCard(row interface{ Scan(...any) error }) (app.Card, error) {
	var (
		card              app.Card
		updated, activity int64
		result            string
	)
	if err := row.Scan(&card.Ref.Repository, &card.Ref.Number, &card.Title, &card.Author, &card.State, &card.HTMLURL,
		&updated, &activity, &card.Unread, &result); err != nil {
		return app.Card{}, err
	}
	card.UpdatedAt, card.ActivityAt = fromUnix(updated), fromUnix(activity)
	if err := json.Unmarshal([]byte(result), &card.Analysis); err != nil {
		return app.Card{}, fmt.Errorf("decode analysis: %w", err)
	}
	return card, nil
}

// ListCards returns one carte per non-archived pull request with a published
// analysis, most recent activity first.
func (s *Store) ListCards(ctx context.Context) ([]app.Card, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT `+cardColumns+` FROM pull_requests p JOIN analyses a ON a.identity = p.published_identity
WHERE p.archived = 0 ORDER BY p.activity_unix DESC, p.pr_key`)
	if err != nil {
		return nil, fmt.Errorf("list cartes: %w", err)
	}
	defer rows.Close()
	cards := []app.Card{}
	for rows.Next() {
		card, err := scanCard(rows)
		if err != nil {
			return nil, fmt.Errorf("scan carte: %w", err)
		}
		cards = append(cards, card)
	}
	return cards, rows.Err()
}

// GetDetail returns the reading view: current carte when published, the
// ordered historique and lifecycle events.
func (s *Store) GetDetail(ctx context.Context, ref pullrequest.Ref) (app.Detail, error) {
	key := ref.Key()
	var (
		detail            app.Detail
		updated, activity int64
		published         string
	)
	err := s.db.QueryRowContext(ctx, `
SELECT repository, number, title, author, state, html_url, updated_unix, activity_unix, unread, archived, published_identity
FROM pull_requests WHERE pr_key = ?`, key).Scan(&detail.Card.Ref.Repository, &detail.Card.Ref.Number, &detail.Card.Title,
		&detail.Card.Author, &detail.Card.State, &detail.Card.HTMLURL, &updated, &activity, &detail.Card.Unread, &detail.Archived, &published)
	if errors.Is(err, sql.ErrNoRows) {
		return app.Detail{}, app.ErrNotFound
	}
	if err != nil {
		return app.Detail{}, fmt.Errorf("read pull request: %w", err)
	}
	detail.Card.UpdatedAt, detail.Card.ActivityAt = fromUnix(updated), fromUnix(activity)

	rows, err := s.db.QueryContext(ctx, `
SELECT identity, result_json, provenance_json, published, created_unix FROM analyses WHERE pr_key = ? ORDER BY id DESC`, key)
	if err != nil {
		return app.Detail{}, fmt.Errorf("list analyses: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var (
			entry              app.HistoryEntry
			result, provenance string
			created            int64
		)
		if err := rows.Scan(&entry.Identity, &result, &provenance, &entry.Published, &created); err != nil {
			return app.Detail{}, fmt.Errorf("scan analysis: %w", err)
		}
		if err := errors.Join(json.Unmarshal([]byte(result), &entry.Analysis), json.Unmarshal([]byte(provenance), &entry.Provenance)); err != nil {
			return app.Detail{}, fmt.Errorf("decode analysis: %w", err)
		}
		entry.CreatedAt = fromUnix(created)
		if string(entry.Identity) == published {
			detail.HasCard = true
			detail.Card.Analysis = entry.Analysis
			detail.Provenance = entry.Provenance
		}
		detail.History = append(detail.History, entry)
	}
	if err := rows.Err(); err != nil {
		return app.Detail{}, err
	}
	events, err := s.db.QueryContext(ctx, `SELECT at_unix, kind, detail FROM pr_events WHERE pr_key = ? ORDER BY id`, key)
	if err != nil {
		return app.Detail{}, fmt.Errorf("list events: %w", err)
	}
	defer events.Close()
	for events.Next() {
		var (
			ev app.Event
			at int64
		)
		if err := events.Scan(&at, &ev.Kind, &ev.Detail); err != nil {
			return app.Detail{}, fmt.Errorf("scan event: %w", err)
		}
		ev.At = fromUnix(at)
		detail.Events = append(detail.Events, ev)
	}
	return detail, events.Err()
}

// Status summarises work and abonnement health.
func (s *Store) Status(ctx context.Context) (app.Status, error) {
	var status app.Status
	now := s.now().Unix()
	err := s.db.QueryRowContext(ctx, `
SELECT
  COUNT(*) FILTER (WHERE status = 'queued'),
  COUNT(*) FILTER (WHERE status = 'running' AND lease_until_unix > ?),
  COUNT(*) FILTER (WHERE status = 'queued' AND attempts > 0),
  COUNT(*) FILTER (WHERE status = 'unavailable')
FROM analysis_jobs`, now).Scan(&status.Pending, &status.Running, &status.Retrying, &status.Unavailable)
	if err != nil {
		return app.Status{}, fmt.Errorf("count work: %w", err)
	}
	subscriptions, err := s.ListSubscriptions(ctx)
	if err != nil {
		return app.Status{}, err
	}
	status.Subscriptions = subscriptions
	for _, subscription := range subscriptions {
		if subscription.BlockedReason != "" {
			status.Blocked = append(status.Blocked, subscription)
		}
		if subscription.LastSyncAt.After(status.LastSyncAt) {
			status.LastSyncAt = subscription.LastSyncAt
		}
	}
	return status, nil
}

func (s *Store) setReadState(ctx context.Context, ref pullrequest.Ref, archived bool, kind string) error {
	key := ref.Key()
	now := s.now()
	return s.tx(ctx, func(tx *sql.Tx) error {
		n, err := rowsAffected(tx.ExecContext(ctx, `
UPDATE pull_requests SET unread = 0, archived = ? WHERE pr_key = ? AND archived = 0 AND published_identity <> ''`, archived, key))
		if err != nil {
			return fmt.Errorf("%s: %w", kind, err)
		}
		if n != 1 {
			return app.ErrNotVisible
		}
		return event(ctx, tx, key, now, kind, "")
	})
}

// MarkRead records that the visible carte was understood.
func (s *Store) MarkRead(ctx context.Context, ref pullrequest.Ref) error {
	return s.setReadState(ctx, ref, false, "read")
}

// Archive removes the carte from the active timeline while keeping history.
func (s *Store) Archive(ctx context.Context, ref pullrequest.Ref) error {
	return s.setReadState(ctx, ref, true, "archived")
}

var _ interface {
	app.CollectionStore
	app.WorkStore
	app.ReadModel
} = (*Store)(nil)
