package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/clement-software/PRadar/internal/analyse"
	"github.com/clement-software/PRadar/internal/collect"
	"github.com/clement-software/PRadar/internal/pullrequest"
	"github.com/clement-software/PRadar/internal/timeline"
)

const cardColumns = `p.repository, p.number, p.title, p.author, p.state, p.html_url, p.updated_unix, p.unread, a.result_json`

func scanCard(row interface{ Scan(...any) error }) (timeline.Card, error) {
	var (
		card    timeline.Card
		updated int64
		result  string
	)
	if err := row.Scan(&card.Ref.Repository, &card.Ref.Number, &card.Title, &card.Author, &card.State, &card.HTMLURL,
		&updated, &card.Unread, &result); err != nil {
		return timeline.Card{}, err
	}
	card.UpdatedAt = fromUnix(updated)
	if err := json.Unmarshal([]byte(result), &card.Analysis); err != nil {
		return timeline.Card{}, fmt.Errorf("decode analysis: %w", err)
	}
	return card, nil
}

// ListCards returns one carte per non-archived pull request with a published
// analysis, most recent activity first.
func (s *Store) ListCards(ctx context.Context) ([]timeline.Card, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT `+cardColumns+` FROM pull_requests p JOIN analyses a ON a.identity = p.published_identity
WHERE p.archived = 0 ORDER BY p.activity_unix DESC, p.pr_key`)
	if err != nil {
		return nil, fmt.Errorf("list cartes: %w", err)
	}
	defer rows.Close()
	cards := []timeline.Card{}
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
func (s *Store) GetDetail(ctx context.Context, ref pullrequest.Ref) (timeline.Detail, error) {
	key := ref.Key()
	var (
		detail    timeline.Detail
		updated   int64
		published string
	)
	err := s.db.QueryRowContext(ctx, `
SELECT repository, number, title, author, state, html_url, updated_unix, unread, archived, published_identity
FROM pull_requests WHERE pr_key = ?`, key).Scan(&detail.Card.Ref.Repository, &detail.Card.Ref.Number, &detail.Card.Title,
		&detail.Card.Author, &detail.Card.State, &detail.Card.HTMLURL, &updated, &detail.Card.Unread, &detail.Archived, &published)
	if errors.Is(err, sql.ErrNoRows) {
		return timeline.Detail{}, collect.ErrNotFound
	}
	if err != nil {
		return timeline.Detail{}, fmt.Errorf("read pull request: %w", err)
	}
	detail.Card.UpdatedAt = fromUnix(updated)

	rows, err := s.db.QueryContext(ctx, `
SELECT identity, result_json, provenance_json, published, created_unix FROM analyses WHERE pr_key = ? ORDER BY id DESC`, key)
	if err != nil {
		return timeline.Detail{}, fmt.Errorf("list analyses: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var (
			entry              timeline.HistoryEntry
			result, provenance string
			created            int64
		)
		if err := rows.Scan(&entry.Identity, &result, &provenance, &entry.Published, &created); err != nil {
			return timeline.Detail{}, fmt.Errorf("scan analysis: %w", err)
		}
		if err := errors.Join(json.Unmarshal([]byte(result), &entry.Analysis), json.Unmarshal([]byte(provenance), &entry.Provenance)); err != nil {
			return timeline.Detail{}, fmt.Errorf("decode analysis: %w", err)
		}
		entry.CreatedAt = fromUnix(created)
		if string(entry.Identity) == published {
			detail.HasCard = true
			detail.Identity = entry.Identity
			detail.Card.Analysis = entry.Analysis
			detail.Provenance = entry.Provenance
		}
		detail.History = append(detail.History, entry)
	}
	if err := rows.Err(); err != nil {
		return timeline.Detail{}, err
	}
	events, err := s.db.QueryContext(ctx, `SELECT at_unix, kind, detail FROM pr_events WHERE pr_key = ? ORDER BY id`, key)
	if err != nil {
		return timeline.Detail{}, fmt.Errorf("list events: %w", err)
	}
	defer events.Close()
	for events.Next() {
		var (
			ev timeline.Event
			at int64
		)
		if err := events.Scan(&at, &ev.Kind, &ev.Detail); err != nil {
			return timeline.Detail{}, fmt.Errorf("scan event: %w", err)
		}
		ev.At = fromUnix(at)
		detail.Events = append(detail.Events, ev)
	}
	return detail, events.Err()
}

// Status summarises work and abonnement health.
func (s *Store) Status(ctx context.Context) (timeline.Status, error) {
	var status timeline.Status
	now := s.now().Unix()
	err := s.db.QueryRowContext(ctx, `
SELECT
  COUNT(*) FILTER (WHERE status = 'queued'),
  COUNT(*) FILTER (WHERE status = 'running' AND lease_until_unix > ?),
  COUNT(*) FILTER (WHERE status = 'queued' AND attempts > 0),
  COUNT(*) FILTER (WHERE status = 'unavailable')
FROM analysis_jobs`, now).Scan(&status.Pending, &status.Running, &status.Retrying, &status.Unavailable)
	if err != nil {
		return timeline.Status{}, fmt.Errorf("count work: %w", err)
	}
	subscriptions, err := s.ListSubscriptions(ctx)
	if err != nil {
		return timeline.Status{}, err
	}
	for _, subscription := range subscriptions {
		repository := timeline.RepositoryStatus{
			Repository: subscription.Repository, Active: subscription.Active,
			BlockedReason: subscription.BlockedReason, LastSyncAt: subscription.LastSyncAt,
			AuthorisedEngine: subscription.AuthorisedEngine,
		}
		status.Repositories = append(status.Repositories, repository)
		if repository.BlockedReason != "" {
			status.Blocked = append(status.Blocked, repository)
		}
		if repository.LastSyncAt.After(status.LastSyncAt) {
			status.LastSyncAt = repository.LastSyncAt
		}
	}
	return status, nil
}

// UsageRecords lists every analysis measurement, oldest first.
func (s *Store) UsageRecords(ctx context.Context) ([]timeline.UsageRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT a.identity, a.pr_key, a.head_sha, a.status, a.provenance_json FROM analyses a ORDER BY a.id`)
	if err != nil {
		return nil, fmt.Errorf("list usage records: %w", err)
	}
	defer rows.Close()
	records := []timeline.UsageRecord{}
	for rows.Next() {
		var (
			record     timeline.UsageRecord
			provenance string
		)
		if err := rows.Scan(&record.Identity, &record.PullRequest, &record.HeadSHA, &record.Status, &provenance); err != nil {
			return nil, fmt.Errorf("scan usage record: %w", err)
		}
		var decoded pullrequest.Provenance
		if err := json.Unmarshal([]byte(provenance), &decoded); err != nil {
			return nil, fmt.Errorf("decode provenance: %w", err)
		}
		record.Profile, record.Attempt = decoded.Profile, decoded.Attempt
		record.StartedAt, record.Duration = decoded.StartedAt, decoded.Duration
		record.Usage, record.Failure = decoded.Usage, decoded.Failure
		records = append(records, record)
	}
	return records, rows.Err()
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
			return timeline.ErrNotVisible
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
	collect.CollectionStore
	analyse.WorkStore
	timeline.ReadModel
} = (*Store)(nil)
