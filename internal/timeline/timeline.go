package timeline

import (
	"context"
	"slices"
	"time"

	"github.com/clement-software/PRadar/internal/pullrequest"
)

// ReadModel is the timeline and detail query surface plus durable user state.
type ReadModel interface {
	ListCards(ctx context.Context) ([]Card, error)
	GetDetail(ctx context.Context, ref pullrequest.Ref) (Detail, error)
	Status(ctx context.Context) (Status, error)
	MarkRead(ctx context.Context, ref pullrequest.Ref) error
	Archive(ctx context.Context, ref pullrequest.Ref) error
	Replay(ctx context.Context, ref pullrequest.Ref, profile pullrequest.Profile, notBefore time.Time) error
}

// Reader exposes the reading use cases to the visualizer.
type Reader struct {
	Store   ReadModel
	Profile pullrequest.Profile
	Now     func() time.Time
}

// Cards returns one carte per non-archived pull request, most recent activity
// first, narrowed by the filter.
func (t *Reader) Cards(ctx context.Context, filter Filter) ([]Card, error) {
	cards, err := t.Store.ListCards(ctx)
	if err != nil {
		return nil, err
	}
	return slices.DeleteFunc(cards, func(card Card) bool { return !filter.Matches(card) }), nil
}

// Matches reports whether the carte satisfies every set criterion.
func (f Filter) Matches(card Card) bool {
	switch {
	case f.UnreadOnly && !card.Unread,
		f.Repository != "" && card.Ref.Repository != f.Repository,
		f.State != "" && card.State != f.State,
		f.Importance != "" && card.Analysis.Importance != f.Importance,
		f.Risk != "" && !slices.Contains(card.Analysis.Risks, f.Risk):
		return false
	}
	return true
}

// Detail returns the full reading view of one pull request.
func (t *Reader) Detail(ctx context.Context, ref pullrequest.Ref) (Detail, error) {
	return t.Store.GetDetail(ctx, ref)
}

// Status reports synchronisation and work health.
func (t *Reader) Status(ctx context.Context) (Status, error) { return t.Store.Status(ctx) }

// MarkRead records that the latest analysed version was understood.
func (t *Reader) MarkRead(ctx context.Context, ref pullrequest.Ref) error {
	return t.Store.MarkRead(ctx, ref)
}

// Archive removes the carte from the active timeline, keeping history.
func (t *Reader) Archive(ctx context.Context, ref pullrequest.Ref) error {
	return t.Store.Archive(ctx, ref)
}

// Replay schedules a new analysis of the latest version with the configured
// profile; an unchanged identity is rejected so earlier results stay distinct.
func (t *Reader) Replay(ctx context.Context, ref pullrequest.Ref) error {
	return t.Store.Replay(ctx, ref, t.Profile, t.Now())
}
