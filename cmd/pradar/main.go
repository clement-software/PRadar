// Command pradar is the PRadar demonstrator: one process owning
// configuration, SQLite, the application use cases, the loopback visualizer,
// root cancellation and orderly exit.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/clement-software/PRadar/internal/adapter/claudecli"
	"github.com/clement-software/PRadar/internal/adapter/forgejo"
	"github.com/clement-software/PRadar/internal/adapter/keychain"
	"github.com/clement-software/PRadar/internal/adapter/sqlite"
	"github.com/clement-software/PRadar/internal/adapter/workspace"
	"github.com/clement-software/PRadar/internal/app"
	"github.com/clement-software/PRadar/internal/controlled"
	"github.com/clement-software/PRadar/internal/pullrequest"
	"github.com/clement-software/PRadar/internal/ui"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "pradar:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return errors.New("usage: pradar run [flags] | pradar token set --instance <url>")
	}
	switch args[0] {
	case "run":
		return runDemonstrator(args[1:])
	case "token":
		return runToken(args[1:])
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
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
	dataDir      string
	listen       string
	controlled   bool
	pollInterval time.Duration
	debounce     time.Duration
	lease        time.Duration
	instance     string
	model        string
	claude       string
	analysisTime time.Duration
	maxTurns     int
	maxBudgetUSD float64
}

func runDemonstrator(args []string) error {
	home, _ := os.UserHomeDir()
	var cfg config
	fs := flag.NewFlagSet("run", flag.ContinueOnError)
	fs.StringVar(&cfg.dataDir, "data", filepath.Join(home, "Library", "Application Support", "PRadar-demonstrator"), "directory holding the SQLite database and temporary workspaces")
	fs.StringVar(&cfg.listen, "listen", "127.0.0.1:0", "loopback address of the visualizer")
	fs.BoolVar(&cfg.controlled, "controlled", false, "use the controlled Forgejo and analyzer substitutes")
	fs.DurationVar(&cfg.pollInterval, "poll", 5*time.Minute, "Forgejo polling interval")
	fs.DurationVar(&cfg.debounce, "debounce", 10*time.Minute, "anti-rebond window per pull request")
	fs.DurationVar(&cfg.lease, "lease", 15*time.Minute, "analysis lease duration")
	fs.StringVar(&cfg.instance, "instance", os.Getenv("PRADAR_FORGEJO_INSTANCE"), "Forgejo instance URL (https)")
	fs.StringVar(&cfg.model, "model", os.Getenv("PRADAR_CLAUDE_MODEL"), "the single Claude model used for every analysis")
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
		forge    app.Forge
		analyzer app.Analyzer
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
		pluginDir, err := claudecli.InstallPlugin(filepath.Join(cfg.dataDir, "plugins"))
		if err != nil {
			return err
		}
		analyzer = &claudecli.Analyzer{Executable: claudePath, PluginDir: pluginDir, Model: cfg.model, Timeout: cfg.analysisTime,
			MaxOutput: 4 << 20, MaxTurns: cfg.maxTurns, MaxBudgetUSD: cfg.maxBudgetUSD, Env: claudecli.MinimalEnv()}
		profile = pullrequest.Profile{PromptVersion: claudecli.PromptVersion, SkillVersion: claudecli.SkillVersion, Engine: claudecli.Engine, Model: cfg.model}
	}

	if err := os.MkdirAll(cfg.dataDir, 0o700); err != nil {
		return fmt.Errorf("create data directory: %w", err)
	}
	store, err := sqlite.Open(ctx, filepath.Join(cfg.dataDir, "pradar.sqlite"), now)
	if err != nil {
		return err
	}
	defer store.Close()
	root, err := workspace.New(filepath.Join(cfg.dataDir, "workspaces"), 8<<20)
	if err != nil {
		return err
	}
	if err := root.Scavenge(ctx); err != nil {
		return fmt.Errorf("scavenge workspaces: %w", err)
	}

	collector := &app.Collector{Forge: forge, Store: store, Profile: profile, Debounce: cfg.debounce, Now: now, Log: log}
	worker := &app.Worker{Store: store, Analyzer: analyzer, Workspace: root, Diffs: forge, Now: now, Lease: cfg.lease,
		Backoff: app.ExponentialBackoff(time.Minute), Log: log}
	collector.Interrupt = worker.Interrupt
	timeline := &app.Timeline{Store: store, Profile: profile, Now: now}
	if cfg.controlled {
		if _, err := collector.Subscribe(ctx, app.SubscribeRequest{Repository: controlled.Repository, HTMLURL: "https://forge.example/" + controlled.Repository, Import: app.ImportTen}); err != nil {
			return err
		}
	}

	server := &ui.Server{Timeline: timeline, Collector: collector, Log: log, ParseRepositoryURL: instance.ParseRepositoryURL}
	var wg sync.WaitGroup
	wg.Go(func() { collector.Poll(ctx, cfg.pollInterval) })
	wg.Go(func() { worker.Run(ctx, 2*time.Second) })
	err = server.Serve(ctx, cfg.listen, func(url string) {
		log.Info("visualizer ready", "url", url)
		fmt.Println(url)
	})
	stop()
	wg.Wait()
	return err
}
