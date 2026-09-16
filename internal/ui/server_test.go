package ui_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/clement-software/PRadar/internal/adapter/sqlite"
	"github.com/clement-software/PRadar/internal/app"
	"github.com/clement-software/PRadar/internal/controlled"
	"github.com/clement-software/PRadar/internal/pullrequest"
	"github.com/clement-software/PRadar/internal/ui"
)

var profile = pullrequest.Profile{PromptVersion: "p1", SkillVersion: "s1", Engine: "controlled", Model: "m"}

type visualizer struct {
	t      *testing.T
	client *http.Client
	base   string
	store  *sqlite.Store
	worker *app.Worker
	coll   *app.Collector
}

func start(t *testing.T) *visualizer {
	t.Helper()
	ctx := t.Context()
	store, err := sqlite.Open(ctx, t.TempDir()+"/pradar.sqlite", time.Now)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	forge := controlled.Forge{Now: time.Now}
	coll := &app.Collector{Forge: forge, Store: store, Profile: profile, Now: time.Now, Log: log}
	worker := &app.Worker{Store: store, Analyzer: controlled.Analyzer{}, Workspace: noWorkspace{}, Diffs: forge, Now: time.Now,
		Lease: time.Minute, Backoff: app.ExponentialBackoff(time.Minute), Log: log}
	server := &ui.Server{Timeline: &app.Timeline{Store: store, Profile: profile, Now: time.Now}, Collector: coll, Log: log,
		ParseRepositoryURL: func(raw string) (string, string, error) { return controlled.Repository, raw, nil }}
	ctx, cancel := context.WithCancel(ctx)
	t.Cleanup(cancel)
	ready := make(chan string, 1)
	go func() { _ = server.Serve(ctx, "", func(url string) { ready <- url }) }()
	base := <-ready
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	return &visualizer{t: t, client: client, base: strings.TrimSuffix(base, "/"), store: store, worker: worker, coll: coll}
}

type noWorkspace struct{}

func (noWorkspace) Materialise(_ context.Context, _ string, _ map[string][]byte) (string, func() error, error) {
	return "", func() error { return nil }, nil
}
func (noWorkspace) Scavenge(context.Context) error { return nil }

func (v *visualizer) get(path string) (int, string) {
	v.t.Helper()
	response, err := v.client.Get(v.base + path)
	if err != nil {
		v.t.Fatal(err)
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(response.Body)
	return response.StatusCode, string(body)
}

func (v *visualizer) post(path string, form url.Values) int {
	v.t.Helper()
	response, err := v.client.PostForm(v.base+path, form)
	if err != nil {
		v.t.Fatal(err)
	}
	defer response.Body.Close()
	return response.StatusCode
}

func TestVisualizer_ShowsCarteAndDetailAfterControlledAnalysis(t *testing.T) {
	t.Parallel()
	v := start(t)
	if !strings.HasPrefix(v.base, "http://127.0.0.1:") {
		t.Fatalf("visualizer must bind loopback, got %s", v.base)
	}
	if status, body := v.get("/"); status != http.StatusOK || !strings.Contains(body, "Aucune carte") {
		t.Fatalf("empty timeline: %d %s", status, body)
	}
	if _, err := v.coll.Subscribe(t.Context(), app.SubscribeRequest{Repository: controlled.Repository, HTMLURL: "https://forge.example/controlled/demo"}); err != nil {
		t.Fatal(err)
	}
	if err := v.worker.RunOne(t.Context()); err != nil {
		t.Fatal(err)
	}
	status, body := v.get("/")
	if status != http.StatusOK || !strings.Contains(body, `href="/pr/controlled/demo%231"`) || !strings.Contains(body, "non lue") {
		t.Fatalf("timeline after analysis: %d\n%s", status, body)
	}
	if strings.Count(body, `<li class="card`) != 1 {
		t.Fatal("exactly one carte expected")
	}
	status, body = v.get("/pr/controlled/demo%231")
	if status != http.StatusOK {
		t.Fatalf("detail status %d", status)
	}
	for _, want := range []string{`<pre class="mermaid">sequenceDiagram`, "Importance : medium", "retry-storm", "Provenance", "Historique de pull request", "https://forge.example/controlled/demo/pulls/1"} {
		if !strings.Contains(body, want) {
			t.Errorf("detail lacks %q", want)
		}
	}
	if status := v.post("/pr/controlled/demo%231", url.Values{"action": {"read"}}); status != http.StatusSeeOther {
		t.Fatalf("read action status %d", status)
	}
	if _, body := v.get("/?unread=1"); strings.Contains(body, `<li class="card`) {
		t.Fatal("read carte must leave the unread filter")
	}
	if status := v.post("/pr/controlled/demo%231", url.Values{"action": {"archive"}}); status != http.StatusSeeOther {
		t.Fatalf("archive action status %d", status)
	}
	if _, body := v.get("/"); strings.Contains(body, `<li class="card`) {
		t.Fatal("archived carte must leave the timeline")
	}
	if status, _ := v.get("/pr/nope"); status != http.StatusNotFound {
		t.Fatalf("unknown pull request status %d", status)
	}
}

func TestVisualizer_RefusesNonLoopbackAddress(t *testing.T) {
	t.Parallel()
	server := &ui.Server{Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	if err := server.Serve(t.Context(), "0.0.0.0:0", nil); err == nil {
		t.Fatal("non-loopback bind accepted")
	}
}
