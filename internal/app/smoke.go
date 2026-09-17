package app

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/clement-software/PRadar/internal/pullrequest"
)

// SmokeStep is one checked stage of the live smoke run.
type SmokeStep struct {
	Name     string
	Duration time.Duration
	Err      error
}

// SmokeReport is the outcome of one live smoke run; it carries no pull-request body.
type SmokeReport struct {
	Steps    []SmokeStep
	Analysis pullrequest.Analysis
	Usage    map[string]any
}

// Passed reports whether every step succeeded.
func (r SmokeReport) Passed() bool {
	for _, step := range r.Steps {
		if step.Err != nil {
			return false
		}
	}
	return len(r.Steps) > 0
}

// Smoke validates the live boundaries once before a scored run: repository
// access, pull-request content, one real analysis through the configured
// engine, contract validation, workspace cleanup and rendering. It writes
// nothing to the database and is never part of the automated test suite.
type Smoke struct {
	Forge     Forge
	Workspace Workspace
	Analyzer  Analyzer
	Profile   pullrequest.Profile
	// Render turns analysis Markdown into the visualizer's HTML.
	Render func(markdown string) string
	Now    func() time.Time
	Log    *slog.Logger
}

// Run executes the steps in order and stops at the first failure.
func (s *Smoke) Run(ctx context.Context, ref pullrequest.Ref) SmokeReport {
	var report SmokeReport
	step := func(name string, fn func() error) bool {
		started := s.Now()
		err := fn()
		report.Steps = append(report.Steps, SmokeStep{Name: name, Duration: s.Now().Sub(started), Err: err})
		return err == nil
	}

	var observation pullrequest.Observation
	if !step("forgejo repository access", func() error { return s.Forge.CheckRepository(ctx, ref.Repository) }) {
		return report
	}
	if !step("pull request metadata", func() (err error) {
		observation, err = s.Forge.FetchPullRequest(ctx, ref)
		return err
	}) {
		return report
	}

	workspace := &recordingWorkspace{Workspace: s.Workspace}
	worker := &Worker{Content: s.Forge, Workspace: workspace, Analyzer: s.Analyzer, Log: s.Log}
	revision := pullrequest.ComputeInputRevision(observation.HeadSHA, observation.Title, observation.Body, 0)
	job := Job{
		Identity: pullrequest.IdentityOf(ref, revision, s.Profile), Ref: ref, HeadSHA: observation.HeadSHA,
		Title: observation.Title, Body: observation.Body, Author: observation.Author, HTMLURL: observation.HTMLURL,
		Revision: revision, Profile: s.Profile, Attempt: 1,
	}
	if !step("analysis with "+s.Profile.Engine+"/"+s.Profile.Model+" and contract validation", func() error {
		result, err := worker.analyse(ctx, job)
		report.Analysis, report.Usage = result.Analysis, result.Usage
		return err
	}) {
		return report
	}
	if !step("workspace cleanup", workspace.verifyRemoved) {
		return report
	}
	step("rendering", func() error {
		html := s.Render(report.Analysis.Body)
		switch {
		case strings.TrimSpace(html) == "":
			return errors.New("analysis body rendered to nothing")
		case strings.Contains(report.Analysis.Body, "```mermaid") && !strings.Contains(html, `<pre class="mermaid">`):
			return errors.New("a Mermaid fence did not render as a strict-mode Mermaid block")
		case strings.Contains(strings.ToLower(html), "<script"):
			return errors.New("rendered analysis contains a script element")
		}
		return nil
	})
	return report
}

// recordingWorkspace remembers every materialised directory so the smoke run
// can prove each one was removed after the analysis.
type recordingWorkspace struct {
	Workspace
	mu   sync.Mutex
	dirs []string
}

func (w *recordingWorkspace) Materialise(ctx context.Context, name string, files map[string][]byte) (string, func() error, error) {
	dir, cleanup, err := w.Workspace.Materialise(ctx, name, files)
	if err == nil {
		w.mu.Lock()
		w.dirs = append(w.dirs, dir)
		w.mu.Unlock()
	}
	return dir, cleanup, err
}

func (w *recordingWorkspace) verifyRemoved() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.dirs) == 0 {
		return errors.New("no workspace was materialised")
	}
	for _, dir := range w.dirs {
		if _, err := os.Stat(dir); !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("workspace %s still exists after the analysis", dir)
		}
	}
	return nil
}
