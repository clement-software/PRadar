package app_test

import (
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/clement-software/PRadar/internal/app"
)

func smoke(t *testing.T, forge *fakeForge, analyzer app.Analyzer, render func(string) string) app.SmokeReport {
	t.Helper()
	s := &app.Smoke{
		Forge: forge, Workspace: &fakeWorkspace{root: t.TempDir()}, Analyzer: analyzer, Profile: profile,
		Render: render, Now: time.Now, Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	return s.Run(t.Context(), ref)
}

func TestSmoke_PassesEveryStepAndCleansTheWorkspace(t *testing.T) {
	t.Parallel()
	forge := &fakeForge{diff: []byte("diff")}
	forge.set(observation(42, "sha-1", time.Now()))
	report := smoke(t, forge, &fakeAnalyzer{}, func(md string) string { return `<pre class="mermaid">` + md + `</pre>` })
	if !report.Passed() || len(report.Steps) != 5 {
		t.Fatalf("report = %+v", report.Steps)
	}
	if report.Analysis.HeadSHA != "sha-1" || report.Usage["input_tokens"] == nil {
		t.Fatalf("analysis = %+v usage = %v", report.Analysis, report.Usage)
	}
}

func TestSmoke_StopsAtTheFirstFailingBoundary(t *testing.T) {
	t.Parallel()
	blocked := &fakeForge{checkErr: errors.New("forgejo authentication failed (401)")}
	if report := smoke(t, blocked, &fakeAnalyzer{}, strings.ToUpper); report.Passed() || len(report.Steps) != 1 {
		t.Fatalf("repository failure must stop the run: %+v", report.Steps)
	}

	forge := &fakeForge{diff: []byte("diff")}
	forge.set(observation(42, "sha-1", time.Now()))
	report := smoke(t, forge, &fakeAnalyzer{fail: errors.New("claude exited with an error")}, strings.ToUpper)
	if report.Passed() || len(report.Steps) != 3 || !strings.Contains(report.Steps[2].Err.Error(), "claude") {
		t.Fatalf("engine failure must be reported on the analysis step: %+v", report.Steps)
	}

	report = smoke(t, forge, &fakeAnalyzer{}, func(string) string { return "<p>no diagram</p>" })
	if report.Passed() || !strings.Contains(report.Steps[len(report.Steps)-1].Err.Error(), "Mermaid") {
		t.Fatalf("a Mermaid body rendered without its block must fail: %+v", report.Steps)
	}
}
