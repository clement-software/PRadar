// Command pradar is the PRadar demonstrator: one process owning
// configuration, SQLite, the application use cases, the loopback visualizer,
// root cancellation and orderly exit.
package main

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/clement-software/PRadar/internal/adapter/claudecli"
	"github.com/clement-software/PRadar/internal/adapter/desktop"
	"github.com/clement-software/PRadar/internal/adapter/forgejo"
	"github.com/clement-software/PRadar/internal/adapter/keychain"
	"github.com/clement-software/PRadar/internal/adapter/sqlite"
	"github.com/clement-software/PRadar/internal/adapter/wake"
	"github.com/clement-software/PRadar/internal/adapter/workspace"
	"github.com/clement-software/PRadar/internal/analyse"
	"github.com/clement-software/PRadar/internal/collect"
	"github.com/clement-software/PRadar/internal/controlled"
	"github.com/clement-software/PRadar/internal/evaluation"
	"github.com/clement-software/PRadar/internal/pullrequest"
	"github.com/clement-software/PRadar/internal/timeline"
	"github.com/clement-software/PRadar/internal/ui"
)

// version is set at build time; see the app target of the Makefile.
var version = "dev"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "pradar:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: pradar run [flags] | pradar token set --instance <url> | pradar corpus ... | pradar smoke ... | pradar version")
	}
	switch args[0] {
	case "run":
		return runDemonstrator(args[1:])
	case "token":
		return runToken(args[1:])
	case "corpus":
		return runCorpus(args[1:])
	case "version":
		fmt.Println("pradar", version)
		return nil
	case "smoke":
		return runSmoke(args[1:])
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

// runCorpus prepares (candidates) or freezes (freeze) the evaluation corpus.
func runCorpus(args []string) error {
	if len(args) >= 1 && args[0] == "candidates" {
		return runCorpusCandidates(args[1:])
	}
	if len(args) < 2 || args[0] != "freeze" {
		return errors.New("usage: pradar corpus candidates --instance <url> <owner/name>... | pradar corpus freeze <manifest.json> [--data <dir>]")
	}
	fs := flag.NewFlagSet("corpus freeze", flag.ContinueOnError)
	dataDir := fs.String("data", defaultDataDir(), "directory holding the SQLite database")
	if err := fs.Parse(args[2:]); err != nil {
		return err
	}
	raw, err := os.ReadFile(args[1]) //nolint:gosec // the user names their own manifest file on the command line
	if err != nil {
		return fmt.Errorf("read manifest: %w", err)
	}
	manifest, err := evaluation.ParseManifest(raw)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := os.MkdirAll(*dataDir, 0o700); err != nil {
		return fmt.Errorf("create data directory: %w", err)
	}
	store, err := sqlite.Open(ctx, filepath.Join(*dataDir, "pradar.sqlite"), time.Now)
	if err != nil {
		return err
	}
	defer store.Close()
	evaluator := &timeline.Evaluator{Store: store, Read: store, Now: time.Now}
	id, err := evaluator.Freeze(ctx, manifest)
	if err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "corpus frozen with id", id)
	return nil
}

// runCorpusCandidates prints a draft manifest of the most recent pull requests
// of the given repositories; the evaluator trims it to twenty items and fills
// the category and reason before freezing it.
func runCorpusCandidates(args []string) error {
	fs := flag.NewFlagSet("corpus candidates", flag.ContinueOnError)
	raw := fs.String("instance", os.Getenv("PRADAR_FORGEJO_INSTANCE"), "Forgejo instance URL (https)")
	limit := fs.Int("limit", 15, "pull requests listed per repository, most recently updated first")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() == 0 {
		return errors.New("usage: pradar corpus candidates --instance <url> <owner/name> [<owner/name>]")
	}
	instance, err := forgejo.ParseInstance(*raw, false)
	if err != nil {
		return fmt.Errorf("--instance: %w", err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	token, err := (keychain.Store{}).Lookup(ctx, instance.Host())
	if err != nil {
		return err
	}
	client := forgejo.NewClient(instance, token)
	var manifest evaluation.Manifest
	for _, repository := range fs.Args() {
		candidates, err := client.ListRecentPullRequests(ctx, repository, *limit)
		if err != nil {
			return fmt.Errorf("%s: %w", repository, err)
		}
		for _, candidate := range candidates {
			item := evaluation.DraftItem(candidate.Ref, candidate.HeadSHA, candidate.Author, candidate.ChangedLines)
			item.Reason = fmt.Sprintf("TODO — %s · %s · %s · %d lines in %d files · %s",
				candidate.Title, candidate.Author, candidate.State, candidate.ChangedLines, candidate.ChangedFiles, candidate.HTMLURL)
			manifest.Items = append(manifest.Items, item)
		}
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(manifest)
}

// runSmoke validates the live boundaries once before a scored run. With
// --controlled-forge it exercises only the Claude CLI against the fixture pull
// request, which needs no Forgejo instance or token.
func runSmoke(args []string) error {
	fs := flag.NewFlagSet("smoke", flag.ContinueOnError)
	raw := fs.String("instance", os.Getenv("PRADAR_FORGEJO_INSTANCE"), "Forgejo instance URL (https)")
	pull := fs.String("pull-request", "", "pull request to analyse, as owner/name#number")
	controlledForge := fs.Bool("controlled-forge", false, "use the fixture pull request instead of a Forgejo instance")
	model := fs.String("model", os.Getenv("PRADAR_CLAUDE_MODEL"), "the Claude model to validate")
	language := fs.String("analysis-language", cmp.Or(os.Getenv("PRADAR_ANALYSIS_LANGUAGE"), string(claudecli.French)), "language the analysis is written in: fr or en")
	claude := fs.String("claude", "claude", "Claude CLI executable")
	timeout := fs.Duration("analysis-timeout", 10*time.Minute, "maximum duration of the Claude invocation")
	maxTurns := fs.Int("max-turns", 12, "maximum agentic turns")
	maxBudget := fs.Float64("max-budget-usd", 1, "maximum estimated spend of the smoke analysis")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *model == "" {
		return errors.New("--model is required")
	}
	if !claudecli.Language(*language).Valid() {
		return fmt.Errorf("--analysis-language: %q is not a language PRadar writes; use fr or en", *language)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	log := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))

	var (
		forge collect.Forge
		ref   pullrequest.Ref
	)
	if *controlledForge {
		forge = controlled.Forge{Now: time.Now}
		ref = pullrequest.Ref{Repository: controlled.Repository, Number: 1}
	} else {
		instance, err := forgejo.ParseInstance(*raw, false)
		if err != nil {
			return fmt.Errorf("--instance: %w", err)
		}
		if ref, err = pullrequest.ParseKey(*pull); err != nil {
			return fmt.Errorf("--pull-request: %w", err)
		}
		token, err := (keychain.Store{}).Lookup(ctx, instance.Host())
		if err != nil {
			return fmt.Errorf("keychain: %w", err)
		}
		fmt.Fprintln(os.Stderr, "ok   keychain token lookup for", instance.Host())
		forge = forgejo.NewClient(instance, token)
	}

	claudePath, err := exec.LookPath(*claude)
	if err != nil {
		return fmt.Errorf("claude CLI not found: %w", err)
	}
	scratch, err := os.MkdirTemp("", "pradar-smoke-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(scratch)
	root, err := workspace.New(filepath.Join(scratch, "workspaces"), 8<<20)
	if err != nil {
		return err
	}
	smoke := &analyse.Smoke{
		Forge: forge, Workspace: root, Now: time.Now, Log: log,
		Analyzer: &claudecli.Analyzer{Executable: claudePath, Model: *model, Language: claudecli.Language(*language), Timeout: *timeout,
			MaxOutput: 16 << 20, MaxTurns: *maxTurns, MaxBudgetUSD: *maxBudget, Env: claudecli.MinimalEnv()},
		Profile: pullrequest.Profile{PromptVersion: claudecli.PromptVersion(claudecli.Language(*language)), SkillVersion: claudecli.SkillVersion, Engine: claudecli.Engine, Model: *model},
		Render:  func(markdown string) string { return string(ui.RenderMarkdown(markdown)) },
	}
	report := smoke.Run(ctx, ref)
	for _, step := range report.Steps {
		status := "ok  "
		if step.Err != nil {
			status = "FAIL"
		}
		fmt.Fprintf(os.Stderr, "%s %s (%s)\n", status, step.Name, step.Duration.Round(time.Millisecond))
		if step.Err != nil {
			fmt.Fprintln(os.Stderr, "     ", step.Err)
		}
	}
	if !report.Passed() {
		return errors.New("smoke run failed")
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(struct {
		Analysis pullrequest.Analysis `json:"analysis"`
		Usage    map[string]any       `json:"usage"`
	}{report.Analysis, report.Usage})
}

// reportMigration tells the operator what opening the database did, and above
// all where the backup went when one was taken.
func reportMigration(log *slog.Logger, report sqlite.MigrationReport) {
	switch {
	case report.Created:
		log.Info("database created", "schema_version", report.ToVersion)
	case report.Applied():
		log.Warn("database migrated", "from_version", report.FromVersion, "to_version", report.ToVersion, "backup", report.BackupPath)
		fmt.Fprintf(os.Stderr, "database migrated from version %d to %d; a copy of the previous database is at %s\n",
			report.FromVersion, report.ToVersion, report.BackupPath)
	default:
		log.Info("database opened", "schema_version", report.ToVersion)
	}
}

// waitFor waits for the owned goroutines, but never for longer than grace: a
// stuck engine must not keep the application alive after its window closed.
func waitFor(wg *sync.WaitGroup, grace time.Duration) bool {
	done := make(chan struct{})
	go func() { defer close(done); wg.Wait() }()
	select {
	case <-done:
		return true
	case <-time.After(grace):
		return false
	}
}

func defaultDataDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "Application Support", "PRadar-demonstrator")
}

// runToken reads a read-only Forgejo token from standard input and stores it
// in the macOS Keychain under the instance host.
func runToken(args []string) error {
	if len(args) == 0 || args[0] != "set" {
		return errors.New("usage: pradar token set --instance <url> < token")
	}
	fs := flag.NewFlagSet("token set", flag.ContinueOnError)
	raw := fs.String("instance", os.Getenv("PRADAR_FORGEJO_INSTANCE"), "Forgejo instance URL (https)")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	instance, err := forgejo.ParseInstance(*raw, false)
	if err != nil {
		return err
	}
	token, err := io.ReadAll(io.LimitReader(os.Stdin, 4<<10))
	if err != nil {
		return fmt.Errorf("read token: %w", err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := (keychain.Store{}).Save(ctx, instance.Host(), forgejo.Token(strings.TrimSpace(string(token)))); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "token stored in the Keychain for", instance.Host())
	return nil
}

type config struct {
	dataDir       string
	listen        string
	controlled    bool
	pollInterval  time.Duration
	debounce      time.Duration
	lease         time.Duration
	shutdownGrace time.Duration
	instance      string
	window        bool
	windowWidth   int
	windowHeight  int
	model         string
	language      string
	claude        string
	analysisTime  time.Duration
	maxTurns      int
	maxBudgetUSD  float64
}

func runDemonstrator(args []string) error {
	var cfg config
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.StringVar(&cfg.dataDir, "data", defaultDataDir(), "directory holding the SQLite database and temporary workspaces")
	fs.StringVar(&cfg.listen, "listen", "127.0.0.1:0", "loopback address of the visualizer")
	fs.BoolVar(&cfg.controlled, "controlled", false, "use the controlled Forgejo and analyzer substitutes")
	fs.DurationVar(&cfg.pollInterval, "poll", 5*time.Minute, "Forgejo polling interval")
	fs.DurationVar(&cfg.debounce, "debounce", 10*time.Minute, "anti-rebond window per pull request")
	fs.DurationVar(&cfg.lease, "lease", 15*time.Minute, "analysis lease duration")
	fs.DurationVar(&cfg.shutdownGrace, "shutdown-grace", 10*time.Second, "how long owned work may take to stop after the window closes")
	fs.StringVar(&cfg.instance, "instance", os.Getenv("PRADAR_FORGEJO_INSTANCE"), "Forgejo instance URL (https)")
	fs.BoolVar(&cfg.window, "window", runtime.GOOS == "darwin", "show the interface in a native window instead of printing its address")
	fs.IntVar(&cfg.windowWidth, "window-width", 1180, "window width in points")
	fs.IntVar(&cfg.windowHeight, "window-height", 860, "window height in points")
	fs.StringVar(&cfg.model, "model", os.Getenv("PRADAR_CLAUDE_MODEL"), "the single Claude model used for every analysis")
	fs.StringVar(&cfg.language, "analysis-language", cmp.Or(os.Getenv("PRADAR_ANALYSIS_LANGUAGE"), string(claudecli.French)), "language the analyses are written in: fr or en")
	fs.StringVar(&cfg.claude, "claude", "claude", "Claude CLI executable")
	fs.DurationVar(&cfg.analysisTime, "analysis-timeout", 10*time.Minute, "maximum duration of one Claude invocation")
	fs.IntVar(&cfg.maxTurns, "max-turns", 12, "maximum agentic turns per Claude invocation")
	fs.Float64Var(&cfg.maxBudgetUSD, "max-budget-usd", 2, "maximum estimated spend per Claude invocation (0 disables)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	log := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	now := time.Now

	// External boundaries: the controlled substitutes, or the real instance with its Keychain token.
	var (
		forge    collect.Forge
		analyzer analyse.Analyzer
		profile  pullrequest.Profile
		instance forgejo.Instance
	)
	if cfg.controlled {
		cfg.debounce = 0 // ponytail: the controlled demo should show a carte within seconds, not ten minutes
		instance, _ = forgejo.ParseInstance("https://forge.example", false)
		forge, analyzer = controlled.Forge{Now: now}, controlled.Analyzer{}
		profile = pullrequest.Profile{PromptVersion: "controlled-1", SkillVersion: "controlled-1", Engine: "controlled", Model: "deterministic"}
	} else {
		var err error
		if instance, err = forgejo.ParseInstance(cfg.instance, false); err != nil {
			return fmt.Errorf("--instance: %w", err)
		}
		token, err := (keychain.Store{}).Lookup(ctx, instance.Host())
		if err != nil {
			return err
		}
		forge = forgejo.NewClient(instance, token)
		if cfg.model == "" {
			return errors.New("--model is required in live mode")
		}
		claudePath, err := exec.LookPath(cfg.claude)
		if err != nil {
			return fmt.Errorf("claude CLI not found: %w", err)
		}
		language := claudecli.Language(cfg.language)
		if !language.Valid() {
			return fmt.Errorf("--analysis-language: %q is not a language PRadar writes; use fr or en", cfg.language)
		}
		analyzer = &claudecli.Analyzer{Executable: claudePath, Model: cfg.model, Language: language, Timeout: cfg.analysisTime,
			MaxOutput: 16 << 20, MaxTurns: cfg.maxTurns, MaxBudgetUSD: cfg.maxBudgetUSD, Env: claudecli.MinimalEnv()}
		profile = pullrequest.Profile{PromptVersion: claudecli.PromptVersion(language), SkillVersion: claudecli.SkillVersion, Engine: claudecli.Engine, Model: cfg.model}
	}

	if err := os.MkdirAll(cfg.dataDir, 0o700); err != nil {
		return fmt.Errorf("create data directory: %w", err)
	}
	store, err := sqlite.Open(ctx, filepath.Join(cfg.dataDir, "pradar.sqlite"), now)
	if err != nil {
		return err
	}
	defer store.Close()
	reportMigration(log, store.Migration())
	root, err := workspace.New(filepath.Join(cfg.dataDir, "workspaces"), 8<<20)
	if err != nil {
		return err
	}
	if err := root.Scavenge(ctx); err != nil {
		return fmt.Errorf("scavenge workspaces: %w", err)
	}

	waker := &wake.Detector{}
	collector := &collect.Collector{Forge: forge, Store: store, Profile: profile, Debounce: cfg.debounce, Now: now, Log: log,
		Wakes: waker.Wakes()}
	worker := &analyse.Worker{Store: store, Analyzer: analyzer, Workspace: root, Content: forge, Now: now, Lease: cfg.lease,
		Backoff: analyse.ExponentialBackoff(time.Minute), Log: log}
	collector.Interrupt = worker.Interrupt
	reader := &timeline.Reader{Store: store, Profile: profile, Now: now}
	evaluator := &timeline.Evaluator{Store: store, Read: store, Profile: profile, Presentation: ui.PresentationVersion, Now: now}
	if cfg.controlled {
		if _, err := collector.Subscribe(ctx, collect.SubscribeRequest{Repository: controlled.Repository,
			HTMLURL: "https://forge.example/" + controlled.Repository, Import: collect.ImportTen, AuthoriseEngine: true}); err != nil {
			return err
		}
	}

	server := &ui.Server{Reader: reader, Collector: collector, Evaluator: evaluator, Log: log,
		ParseRepositoryURL: instance.ParseRepositoryURL, Version: version, AnalysisLanguage: cfg.language}
	if cfg.window {
		// A window must never navigate away from the owned origin, so the
		// interface hands external links to the browser through the server.
		server.OpenExternal = desktop.OpenInBrowser
	}

	// Everything the application owns runs in goroutines; the main goroutine
	// belongs to the window, because macOS requires it.
	serveCtx, stopServing := context.WithCancel(ctx)
	defer stopServing()
	var (
		wg       sync.WaitGroup
		serveErr error
		address  = make(chan string, 1)
	)
	wg.Go(func() { waker.Run(serveCtx) })
	wg.Go(func() { collector.Poll(serveCtx, cfg.pollInterval) })
	wg.Go(func() { worker.Run(serveCtx, 2*time.Second) })
	wg.Go(func() {
		serveErr = server.Serve(serveCtx, cfg.listen, func(url string) {
			log.Info("interface ready", "url", url)
			address <- url
		})
	})

	var url string
	select {
	case url = <-address:
	case <-ctx.Done():
		stopServing()
		wg.Wait()
		return serveErr
	}

	if cfg.window {
		if err := desktop.Show(ctx, desktop.Window{URL: url, Title: "PRadar", Width: cfg.windowWidth, Height: cfg.windowHeight}); err != nil {
			log.Error("native window unavailable", "error", err.Error())
			fmt.Fprintln(os.Stderr, "the interface is reachable at", url)
			<-ctx.Done()
		}
	} else {
		fmt.Println(url)
		<-ctx.Done()
	}

	// Closing the window stops collection and analysis, cancels the running
	// invocation and removes temporary content, within a bounded grace period.
	stop()
	stopServing()
	if !waitFor(&wg, cfg.shutdownGrace) {
		log.Warn("owned work did not stop in time", "grace", cfg.shutdownGrace.String())
	}
	scavengeCtx, cancelScavenge := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancelScavenge()
	if err := root.Scavenge(scavengeCtx); err != nil {
		log.Error("workspace cleanup failed", "error", err.Error())
	}
	return serveErr
}
