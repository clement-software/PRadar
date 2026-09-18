package collect

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"github.com/clement-software/PRadar/internal/pullrequest"
)

// Forge is the read-only view of the configured Forgejo instance.
type Forge interface {
	CheckRepository(ctx context.Context, repository string) error
	// ListOpenPullRequests returns every open pull request, most recently
	// updated first, across all pages.
	ListOpenPullRequests(ctx context.Context, repository string) ([]pullrequest.Observation, error)
	FetchPullRequest(ctx context.Context, ref pullrequest.Ref) (pullrequest.Observation, error)
	FetchDiff(ctx context.Context, ref pullrequest.Ref) ([]byte, error)
}

// CollectionStore is the durable state owned by collection.
type CollectionStore interface {
	PutSubscription(ctx context.Context, subscription Subscription) error
	GetSubscription(ctx context.Context, repository string) (Subscription, error)
	ListSubscriptions(ctx context.Context) ([]Subscription, error)
	RecordSync(ctx context.Context, repository string, at time.Time, blockedReason string) error
	Unsubscribe(ctx context.Context, repository string) error
	DeleteRepositoryData(ctx context.Context, repository string) error
	ObservePullRequest(ctx context.Context, request ObservationRequest) (pullrequest.Decision, error)
	ListLocallyOpen(ctx context.Context, repository string) ([]pullrequest.Ref, error)
}

// Collector owns abonnements and the polling reconciliation.
type Collector struct {
	Forge    Forge
	Store    CollectionStore
	Profile  pullrequest.Profile
	Debounce time.Duration
	Now      func() time.Time
	Log      *slog.Logger
	// Interrupt cancels the analysis currently running for a repository; nil when no worker runs.
	Interrupt func(repository string)
	// Tick supplies polling ticks; nil means time.Tick.
	Tick func(interval time.Duration) <-chan time.Time
}

// SubscribeRequest creates or reactivates an abonnement.
type SubscribeRequest struct {
	Repository      string
	HTMLURL         string
	Import          ImportMode
	ExcludedAuthors []string
}

const defaultImportLimit = 10

// Subscribe checks repository access, activates the abonnement and imports the
// requested initial set. An inaccessible repository leaves a blocked abonnement.
func (c *Collector) Subscribe(ctx context.Context, request SubscribeRequest) (Subscription, error) {
	limit := defaultImportLimit
	switch request.Import {
	case ImportNone:
		limit = 0
	case ImportAll:
		limit = -1
	case ImportTen, "":
	default:
		return Subscription{}, fmt.Errorf("unknown import mode %q", request.Import)
	}
	subscription := Subscription{
		Repository:      request.Repository,
		HTMLURL:         request.HTMLURL,
		Active:          true,
		ExcludedAuthors: slices.Clone(request.ExcludedAuthors),
	}
	if err := c.Forge.CheckRepository(ctx, request.Repository); err != nil {
		subscription.Active = false
		subscription.BlockedReason = err.Error()
		if putErr := c.Store.PutSubscription(ctx, subscription); putErr != nil {
			return Subscription{}, putErr
		}
		c.Log.Warn("abonnement blocked", "repository", request.Repository, "reason", err.Error())
		return subscription, nil
	}
	if err := c.Store.PutSubscription(ctx, subscription); err != nil {
		return Subscription{}, err
	}
	if err := c.reconcile(ctx, subscription, limit); err != nil {
		return Subscription{}, err
	}
	return c.Store.GetSubscription(ctx, request.Repository)
}

// Unsubscribe stops collection, cancels outstanding work and keeps history.
func (c *Collector) Unsubscribe(ctx context.Context, repository string) error {
	if err := c.Store.Unsubscribe(ctx, repository); err != nil {
		return err
	}
	if c.Interrupt != nil {
		c.Interrupt(repository)
	}
	c.Log.Info("abonnement stopped", "repository", repository)
	return nil
}

// DeleteRepositoryData is the separate destructive action; confirmation is the
// caller's responsibility and désabonnement never triggers it.
func (c *Collector) DeleteRepositoryData(ctx context.Context, repository string, confirmed bool) error {
	if !confirmed {
		return errors.New("repository data deletion requires explicit confirmation")
	}
	if err := c.Store.DeleteRepositoryData(ctx, repository); err != nil {
		return err
	}
	if c.Interrupt != nil {
		c.Interrupt(repository)
	}
	c.Log.Warn("repository data deleted", "repository", repository)
	return nil
}

// ReconcileAll performs the complete synchronisation of every active abonnement.
// It is the same operation on launch, wake and every polling tick.
func (c *Collector) ReconcileAll(ctx context.Context) error {
	subscriptions, err := c.Store.ListSubscriptions(ctx)
	if err != nil {
		return err
	}
	var errs []error
	for _, subscription := range subscriptions {
		if !subscription.Active {
			continue
		}
		if err := c.reconcile(ctx, subscription, -1); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// reconcile observes every open pull request (scheduling at most limit new
// ones, -1 meaning all) and refreshes locally open pull requests that Forgejo
// no longer lists as open. Forge failures block the abonnement visibly.
func (c *Collector) reconcile(ctx context.Context, subscription Subscription, limit int) error {
	open, err := c.Forge.ListOpenPullRequests(ctx, subscription.Repository)
	if err != nil {
		return c.block(ctx, subscription.Repository, err)
	}
	notBefore := c.Now().Add(c.Debounce)
	seen := make(map[pullrequest.Ref]bool, len(open))
	scheduled := 0
	for _, observation := range open {
		seen[observation.Ref] = true
		if slices.Contains(subscription.ExcludedAuthors, observation.Author) || observation.Draft {
			continue
		}
		schedule := limit < 0 || scheduled < limit
		decision, err := c.Store.ObservePullRequest(ctx, ObservationRequest{
			Observation: observation, Profile: c.Profile, Schedule: schedule, NotBefore: notBefore,
		})
		if err != nil {
			return err
		}
		if decision.Kind == pullrequest.DecideSchedule && schedule {
			scheduled++
		}
	}
	locallyOpen, err := c.Store.ListLocallyOpen(ctx, subscription.Repository)
	if err != nil {
		return err
	}
	for _, ref := range locallyOpen {
		if seen[ref] {
			continue
		}
		observation, err := c.Forge.FetchPullRequest(ctx, ref)
		if errors.Is(err, ErrNotFound) {
			c.Log.Warn("pull request vanished from forgejo", "pull_request", ref.Key())
			continue
		}
		if err != nil {
			return c.block(ctx, subscription.Repository, err)
		}
		if _, err := c.Store.ObservePullRequest(ctx, ObservationRequest{
			Observation: observation, Profile: c.Profile, Schedule: true, NotBefore: notBefore,
		}); err != nil {
			return err
		}
	}
	c.Log.Info("abonnement synchronised", "repository", subscription.Repository, "open", len(open), "scheduled", scheduled)
	return c.Store.RecordSync(ctx, subscription.Repository, c.Now(), "")
}

// block records a visible, actionable reason; a blocked abonnement is an
// expected outcome, not an error of the reconciliation itself.
func (c *Collector) block(ctx context.Context, repository string, cause error) error {
	if ctx.Err() != nil {
		return cause
	}
	c.Log.Warn("abonnement blocked", "repository", repository, "reason", cause.Error())
	return c.Store.RecordSync(ctx, repository, time.Time{}, cause.Error())
}

// Poll reconciles immediately (launch) and then on every tick until ctx ends.
// Every tick is a complete reconciliation, so a wake after sleep needs no
// special handling: the first tick after waking catches up. Tick is
// time.Tick in production and a test-controlled channel otherwise.
func (c *Collector) Poll(ctx context.Context, interval time.Duration) {
	tick := c.Tick
	if tick == nil {
		tick = time.Tick
	}
	ticks := tick(interval)
	for {
		if err := c.ReconcileAll(ctx); err != nil && ctx.Err() == nil {
			c.Log.Error("reconciliation failed", "error", err.Error())
		}
		select {
		case <-ctx.Done():
			return
		case <-ticks:
		}
	}
}
