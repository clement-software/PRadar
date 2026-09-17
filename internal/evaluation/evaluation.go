// Package evaluation holds the corpus manifest, the per-item comprehension
// score and the deterministic threshold report that gates production work.
// It depends on no storage, transport or UI code.
package evaluation

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/clement-software/PRadar/internal/pullrequest"
)

// Gate constants from the PRD.
const (
	CorpusSize        = 20
	MinRepositories   = 2
	MaxRepositories   = 3
	RequiredUseful    = 16
	ComprehensionTime = time.Minute
)

// Size is the change-size axis every corpus must cover.
type Size string

// Authorship is the human-or-agent axis every corpus must cover.
type Authorship string

// Category is the code, CI or infrastructure axis every corpus must cover.
type Category string

// Accepted axis values.
const (
	SizeSmall Size = "small"
	SizeLarge Size = "large"

	AuthorHuman Authorship = "human"
	AuthorAgent Authorship = "agent"

	CategoryCode  Category = "code"
	CategoryCI    Category = "ci"
	CategoryInfra Category = "infra"
)

// Item is one frozen pull request; it carries no token and no description.
type Item struct {
	Repository string     `json:"repository"`
	Number     int64      `json:"number"`
	HeadSHA    string     `json:"head_sha"`
	Size       Size       `json:"size"`
	Authorship Authorship `json:"authorship"`
	Category   Category   `json:"category"`
	Reason     string     `json:"reason"`
}

// Ref is the pull-request identity of the item.
func (i Item) Ref() pullrequest.Ref {
	return pullrequest.Ref{Repository: i.Repository, Number: i.Number}
}

// LargeChangeLines is the heuristic boundary between a small and a large change.
const LargeChangeLines = 200

// DraftItem proposes a manifest item from Forgejo metadata: size from the
// changed line count and authorship from bot-like logins. The category and
// the inclusion reason are the evaluator's judgement and stay empty.
func DraftItem(ref pullrequest.Ref, headSHA, author string, changedLines int) Item {
	item := Item{Repository: ref.Repository, Number: ref.Number, HeadSHA: headSHA, Size: SizeSmall, Authorship: AuthorHuman}
	if changedLines >= LargeChangeLines {
		item.Size = SizeLarge
	}
	lowered := strings.ToLower(author)
	for _, marker := range []string{"[bot]", "-bot", "bot-", "agent", "renovate", "dependabot", "claude", "codex", "copilot"} {
		if strings.Contains(lowered, marker) {
			item.Authorship = AuthorAgent
			break
		}
	}
	return item
}

// Manifest is the immutable corpus definition.
type Manifest struct {
	Items []Item `json:"items"`
}

// ParseManifest decodes a manifest file, rejecting unknown fields so a typo
// cannot silently drop an item's category or reason.
func ParseManifest(raw []byte) (Manifest, error) {
	var manifest Manifest
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return Manifest{}, fmt.Errorf("decode manifest: %w", err)
	}
	return manifest, nil
}

// Validate enforces the PRD breadth rules.
func (m Manifest) Validate() error {
	if len(m.Items) != CorpusSize {
		return fmt.Errorf("corpus must contain exactly %d pull requests, got %d", CorpusSize, len(m.Items))
	}
	repositories := map[string]bool{}
	seen := map[string]bool{}
	axes := map[string]bool{}
	for _, item := range m.Items {
		key := item.Ref().Key()
		if seen[key] {
			return fmt.Errorf("pull request %s appears twice", key)
		}
		seen[key] = true
		if item.Repository == "" || item.Number <= 0 || len(item.HeadSHA) < 7 || strings.TrimSpace(item.Reason) == "" {
			return fmt.Errorf("item %s needs a repository, number, frozen head SHA and inclusion reason", key)
		}
		if !slices.Contains([]Size{SizeSmall, SizeLarge}, item.Size) ||
			!slices.Contains([]Authorship{AuthorHuman, AuthorAgent}, item.Authorship) ||
			!slices.Contains([]Category{CategoryCode, CategoryCI, CategoryInfra}, item.Category) {
			return fmt.Errorf("item %s has an unknown size, authorship or category", key)
		}
		repositories[item.Repository] = true
		axes[string(item.Size)], axes[string(item.Authorship)], axes[string(item.Category)] = true, true, true
	}
	if len(repositories) < MinRepositories || len(repositories) > MaxRepositories {
		return fmt.Errorf("corpus must span %d to %d repositories, got %d", MinRepositories, MaxRepositories, len(repositories))
	}
	for _, required := range []string{"small", "large", "human", "agent", "code", "ci", "infra"} {
		if !axes[required] {
			return fmt.Errorf("corpus lacks a %s pull request", required)
		}
	}
	return nil
}

// ID is the content hash of the manifest: any change yields a new corpus.
func (m Manifest) ID() string {
	items := slices.Clone(m.Items)
	slices.SortFunc(items, func(a, b Item) int { return strings.Compare(a.Ref().Key(), b.Ref().Key()) })
	payload, _ := json.Marshal(items)
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:8])
}

// Answers are the four PRD comprehension questions.
type Answers struct {
	Intent       bool `json:"intent"`
	Structure    bool `json:"structure"`
	Risks        bool `json:"risks"`
	ReviewNeeded bool `json:"review_needed"`
}

// Versions are the non-profile inputs whose change invalidates a scored run.
type Versions struct {
	Contract     string `json:"contract"`
	Presentation string `json:"presentation"`
}

// Score is the evaluator's record for one item.
type Score struct {
	Identity         pullrequest.Identity `json:"identity"`
	HeadSHA          string               `json:"head_sha"`
	Profile          pullrequest.Profile  `json:"profile"`
	Versions         Versions             `json:"versions"`
	Elapsed          time.Duration        `json:"elapsed_ns"`
	Answers          Answers              `json:"answers"`
	Useful           bool                 `json:"useful"`
	CriticalError    bool                 `json:"critical_error"`
	Notes            string               `json:"notes"`
	AnalysisDuration time.Duration        `json:"analysis_duration_ns"`
	Usage            map[string]any       `json:"usage,omitempty"`
	RecordedAt       time.Time            `json:"recorded_at"`
}

// Passed reports whether this item counts toward the threshold.
func (s Score) Passed() bool {
	return s.Useful && !s.CriticalError && s.Elapsed > 0 && s.Elapsed < ComprehensionTime &&
		s.Answers.Intent && s.Answers.Structure && s.Answers.Risks && s.Answers.ReviewNeeded
}

// Validate rejects an incomplete score.
func (s Score) Validate(item Item) error {
	switch {
	case s.Identity == "":
		return errors.New("score needs the analysis identity")
	case s.HeadSHA != item.HeadSHA:
		return fmt.Errorf("scored head %s differs from frozen head %s", s.HeadSHA, item.HeadSHA)
	case s.Elapsed <= 0:
		return errors.New("comprehension time must be measured")
	}
	return nil
}

// Report is the deterministic outcome of one scored run.
type Report struct {
	CorpusID    string              `json:"corpus_id"`
	Profile     pullrequest.Profile `json:"profile"`
	Versions    Versions            `json:"versions"`
	Items       []ReportItem        `json:"items"`
	Scored      int                 `json:"scored"`
	Passed      int                 `json:"passed"`
	Failed      int                 `json:"failed"`
	Critical    int                 `json:"critical_errors"`
	Complete    bool                `json:"complete"`
	Verdict     string              `json:"verdict"`
	Invalid     []string            `json:"invalidated_by,omitempty"`
	GeneratedAt time.Time           `json:"generated_at"`
}

// ReportItem is one line of the report.
type ReportItem struct {
	PullRequest string               `json:"pull_request"`
	HeadSHA     string               `json:"head_sha"`
	Identity    pullrequest.Identity `json:"identity,omitempty"`
	Scored      bool                 `json:"scored"`
	Passed      bool                 `json:"passed"`
	Critical    bool                 `json:"critical_error"`
	Elapsed     time.Duration        `json:"elapsed_ns,omitempty"`
	Duration    time.Duration        `json:"analysis_duration_ns,omitempty"`
}

// Verdict values.
const (
	VerdictPass       = "pass"
	VerdictFail       = "fail"
	VerdictIncomplete = "incomplete"
)

// BuildReport aggregates scores against the manifest under the current
// profile. Scores recorded under another profile invalidate the aggregate:
// the report lists them and demands a complete rerun.
func BuildReport(manifest Manifest, scores map[string]Score, profile pullrequest.Profile, versions Versions, now time.Time) Report {
	report := Report{CorpusID: manifest.ID(), Profile: profile, Versions: versions, GeneratedAt: now.UTC()}
	items := slices.Clone(manifest.Items)
	slices.SortFunc(items, func(a, b Item) int { return strings.Compare(a.Ref().Key(), b.Ref().Key()) })
	for _, item := range items {
		key := item.Ref().Key()
		line := ReportItem{PullRequest: key, HeadSHA: item.HeadSHA}
		if score, ok := scores[key]; ok {
			if score.Profile != profile || score.Versions != versions || score.HeadSHA != item.HeadSHA {
				report.Invalid = append(report.Invalid, key)
			} else {
				line.Scored, line.Identity, line.Elapsed, line.Duration = true, score.Identity, score.Elapsed, score.AnalysisDuration
				line.Passed, line.Critical = score.Passed(), score.CriticalError
				report.Scored++
				if line.Passed {
					report.Passed++
				} else {
					report.Failed++
				}
				if line.Critical {
					report.Critical++
				}
			}
		}
		report.Items = append(report.Items, line)
	}
	report.Complete = report.Scored == len(items) && len(report.Invalid) == 0
	switch {
	case !report.Complete:
		report.Verdict = VerdictIncomplete
	case report.Passed >= RequiredUseful && report.Critical == 0:
		report.Verdict = VerdictPass
	default:
		report.Verdict = VerdictFail
	}
	return report
}
