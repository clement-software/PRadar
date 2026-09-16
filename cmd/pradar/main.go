// Command pradar is the PRadar demonstrator: one process owning
// configuration, SQLite, the application use cases, the loopback visualizer,
// root cancellation and orderly exit.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/clement-software/PRadar/internal/adapter/forgejo"
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
		return errors.New("usage: pradar run [flags]")
	}
	switch args[0] {
	case "run":
		return runDemonstrator(args[1:])
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

type config struct {
	dataDir      string
	listen       string
	controlled   bool
	pollInterval time.Duration
	debounce     time.Duration
	lease        time.Duration
	instance     string
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
	if err := fs.Parse(args); err != nil {
		return err
	}
	if !cfg.controlled {
		return errors.New("live mode is not available yet; use --controlled")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	log := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	now := time.Now

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

	profile := pullrequest.Profile{PromptVersion: "controlled-1", SkillVersion: "controlled-1", Engine: "controlled", Model: "deterministic"}
	forge := controlled.Forge{Now: now}
	var analyzer app.Analyzer = controlled.Analyzer{}
	instance, err := forgejo.ParseInstance("https://forge.example", false)
	if err != nil {
		return err
	}

	if cfg.controlled {
		cfg.debounce = 0 // ponytail: the controlled demo should show a carte within seconds, not ten minutes
	}
	collector := &app.Collector{Forge: forge, Store: store, Profile: profile, Debounce: cfg.debounce, Now: now, Log: log}
	worker := &app.Worker{Store: store, Analyzer: analyzer, Workspace: root, Diffs: forge, Now: now, Lease: cfg.lease,
		Backoff: app.ExponentialBackoff(time.Minute), Log: log}
	collector.Interrupt = worker.Interrupt
	timeline := &app.Timeline{Store: store, Profile: profile, Now: now}
	if _, err := collector.Subscribe(ctx, app.SubscribeRequest{Repository: controlled.Repository, HTMLURL: "https://forge.example/" + controlled.Repository, Import: app.ImportTen}); err != nil {
		return err
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
