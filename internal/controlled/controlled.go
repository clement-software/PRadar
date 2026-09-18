// Package controlled supplies the deterministic Forgejo and analyzer
// substitutes used by `pradar run --controlled` to demonstrate one analysis
// end to end without a live instance or model tokens.
package controlled

import (
	"context"
	"fmt"
	"time"

	"github.com/clement-software/PRadar/internal/analyse"
	"github.com/clement-software/PRadar/internal/collect"
	"github.com/clement-software/PRadar/internal/pullrequest"
)

// Repository is the fixture repository followed in controlled mode.
const Repository = "controlled/demo"

// Forge serves one fixed open pull request.
type Forge struct{ Now func() time.Time }

var ref = pullrequest.Ref{Repository: Repository, Number: 1}

func (f Forge) observation() pullrequest.Observation {
	return pullrequest.Observation{
		Ref: ref, Title: "Add retry with jittered backoff to the Forgejo client", Author: "controlled-bot",
		Body:  "Transient 429 and 5xx responses are retried three times with jittered exponential backoff.",
		State: pullrequest.StateOpen, HeadSHA: "c0ffee0000000000000000000000000000000001",
		HTMLURL: "https://forge.example/controlled/demo/pulls/1", UpdatedAt: f.Now(),
	}
}

// CheckRepository always succeeds.
func (Forge) CheckRepository(context.Context, string) error { return nil }

// ListOpenPullRequests lists the fixture pull request.
func (f Forge) ListOpenPullRequests(context.Context, string) ([]pullrequest.Observation, error) {
	return []pullrequest.Observation{f.observation()}, nil
}

// FetchPullRequest returns the fixture pull request.
func (f Forge) FetchPullRequest(_ context.Context, r pullrequest.Ref) (pullrequest.Observation, error) {
	if r != ref {
		return pullrequest.Observation{}, fmt.Errorf("%s: %w", r.Key(), collect.ErrNotFound)
	}
	return f.observation(), nil
}

// FetchDiff returns a small fixed diff.
func (Forge) FetchDiff(context.Context, pullrequest.Ref) ([]byte, error) {
	return []byte("diff --git a/client.go b/client.go\n+func retry(ctx context.Context, do func() error) error {\n"), nil
}

// Analyzer returns a deterministic pradar.analysis.v1 contract.
type Analyzer struct{}

// Analyse builds the controlled analysis for the claimed job.
func (Analyzer) Analyse(_ context.Context, request analyse.AnalysisRequest) (analyse.AnalysisResult, error) {
	job := request.Job
	return analyse.AnalysisResult{Analysis: pullrequest.Analysis{
		SchemaVersion: pullrequest.SchemaVersion, PullRequest: job.Ref.Key(), HeadSHA: job.HeadSHA, PreviousHeadSHA: job.PreviousHeadSHA,
		Status: pullrequest.AnalysisOK, Intent: "Make transient Forgejo failures recover automatically instead of blocking the abonnement.",
		Importance: pullrequest.ImportanceMedium, Risks: []string{"retry-storm"},
		Body:                "```mermaid\nsequenceDiagram\n  Client->>Forgejo: GET /pulls\n  Forgejo-->>Client: 503\n  Client->>Client: sleep(jitter)\n  Client->>Forgejo: GET /pulls\n  Forgejo-->>Client: 200\n```\n\n```text\nretry(do)\n  for attempt in 1..3\n    if do() succeeds: return\n    if not transient: return error\n    sleep(base << attempt + jitter)\n```",
		ChangeSincePrevious: "First analysed version.",
	}, Usage: map[string]any{"input_tokens": 0, "output_tokens": 0}}, nil
}
