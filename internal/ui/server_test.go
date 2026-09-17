package ui_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/clement-software/PRadar/internal/adapter/sqlite"
	"github.com/clement-software/PRadar/internal/app"
	"github.com/clement-software/PRadar/internal/controlled"
	"github.com/clement-software/PRadar/internal/evaluation"
	"github.com/clement-software/PRadar/internal/pullrequest"
	"github.com/clement-software/PRadar/internal/ui"
)

var profile = pullrequest.Profile{PromptVersion: "p1", SkillVersion: "s1", Engine: "controlled", Model: "m"}

type visualizer struct {
	t         *testing.T
	client    *http.Client
	base      string
	store     *sqlite.Store
	worker    *app.Worker
	coll      *app.Collector
	evaluator *app.Evaluator
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
	evaluator := &app.Evaluator{Store: store, Read: store, Profile: profile, Now: time.Now}
	server := &ui.Server{Timeline: &app.Timeline{Store: store, Profile: profile, Now: time.Now}, Collector: coll, Evaluator: evaluator, Log: log,
		ParseRepositoryURL: func(raw string) (string, string, error) { return controlled.Repository, raw, nil }}
	ctx, cancel := context.WithCancel(ctx)
	t.Cleanup(cancel)
	ready := make(chan string, 1)
	go func() { _ = server.Serve(ctx, "", func(url string) { ready <- url }) }()
	base := <-ready
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	return &visualizer{t: t, client: client, base: strings.TrimSuffix(base, "/"), store: store, worker: worker, coll: coll, evaluator: evaluator}
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

func hostilePR(t *testing.T, v *visualizer, number int64, head string, at time.Time) {
	t.Helper()
	ref := pullrequest.Ref{Repository: controlled.Repository, Number: number}
	if _, err := v.store.ObservePullRequest(t.Context(), app.ObservationRequest{
		Observation: pullrequest.Observation{Ref: ref, Title: `<script>alert("title")</script> PR ` + head, Body: "b", Author: "<b>mallory</b>",
			State: pullrequest.StateOpen, HeadSHA: head, HTMLURL: "javascript:alert(1)", UpdatedAt: at},
		Profile: profile, Schedule: true, NotBefore: at,
	}); err != nil {
		t.Fatal(err)
	}
	job, err := v.store.Claim(t.Context(), time.Now().Add(time.Minute), "w-"+head)
	if err != nil {
		t.Fatal(err)
	}
	analysis := pullrequest.Analysis{
		SchemaVersion: pullrequest.SchemaVersion, PullRequest: ref.Key(), HeadSHA: head, Status: pullrequest.AnalysisOK,
		Intent: "<img src=x onerror=alert(1)> intent " + head, Importance: pullrequest.ImportanceHigh, Risks: []string{"<svg onload=alert(1)>"},
		Body: "<iframe src=//evil></iframe>\n\n```mermaid\nflowchart LR\n A-->B\n```", ChangeSincePrevious: "",
	}
	if _, err := v.store.Complete(t.Context(), job, analysis, pullrequest.Provenance{Profile: profile}); err != nil {
		t.Fatal(err)
	}
}

func TestVisualizer_OrdersCartesAndKeepsHostileValuesInert(t *testing.T) {
	t.Parallel()
	v := start(t)
	if err := v.store.PutSubscription(t.Context(), app.Subscription{Repository: controlled.Repository, HTMLURL: "https://forge.example/controlled/demo", Active: true}); err != nil {
		t.Fatal(err)
	}
	hostilePR(t, v, 7, "older", time.Now().Add(-time.Hour))
	time.Sleep(1100 * time.Millisecond) // activity is recorded with second precision
	hostilePR(t, v, 8, "newer", time.Now())
	status, body := v.get("/")
	if status != http.StatusOK {
		t.Fatalf("status %d", status)
	}
	if strings.Index(body, "PR newer") > strings.Index(body, "PR older") {
		t.Error("cartes must be ordered by most recent activity first")
	}
	if strings.Count(body, `<li class="card`) != 2 {
		t.Error("one carte per pull request expected")
	}
	for _, forbidden := range []string{"<script>", "<img", "<svg", "<iframe", `href="javascript:`, "<b>mallory</b>"} {
		if strings.Contains(body, forbidden) {
			t.Errorf("timeline renders hostile value %q", forbidden)
		}
	}
	if !strings.Contains(body, "Lien Forgejo invalide") {
		t.Error("an unsafe Forgejo URL must be replaced by an inert notice")
	}
	_, detail := v.get("/pr/controlled/demo%238")
	for _, forbidden := range []string{"<script>", "<img", "<svg", "<iframe", `href="javascript:`} {
		if strings.Contains(detail, forbidden) {
			t.Errorf("detail renders hostile value %q", forbidden)
		}
	}
	if !strings.Contains(detail, `<pre class="mermaid">`) || !strings.Contains(detail, "&lt;img src=x") {
		t.Error("model Markdown must stay sanitised (escaped intent, inert Mermaid source) while Mermaid renders in strict mode client side")
	}
	if _, filtered := v.get("/?importance=low"); strings.Contains(filtered, `<li class="card`) {
		t.Error("importance filter must hide non-matching cartes")
	}
	if _, filtered := v.get("/?risk=" + url.QueryEscape("<svg onload=alert(1)>")); strings.Count(filtered, `<li class="card`) != 2 {
		t.Error("risk filter must match the recorded risk without mutating data")
	}
}

func TestVisualizer_AccessibilityAndThemes(t *testing.T) {
	t.Parallel()
	v := start(t)
	_, page := v.get("/")
	for _, want := range []string{`lang="fr"`, `class="skip"`, `id="main"`, `aria-label=`, `<label>`, `<h1`} {
		if !strings.Contains(page, want) {
			t.Errorf("page lacks %q", want)
		}
	}
	_, css := v.get("/assets/style.css")
	for _, want := range []string{"color-scheme: light dark", "prefers-color-scheme: dark", ":focus-visible", "outline: 3px solid"} { //nolint:misspell // CSS keywords
		if !strings.Contains(css, want) {
			t.Errorf("stylesheet lacks %q", want)
		}
	}
	_, js := v.get("/assets/app.js")
	if !strings.Contains(js, `securityLevel: "strict"`) {
		t.Error("Mermaid must run in strict mode")
	}
	for _, pair := range [][2]string{{"#17202a", "#f6f8fb"}, {"#4b5563", "#ffffff"}, {"#e6edf3", "#0f1419"}, {"#b3bcc7", "#161c24"}, {"#8ab4ff", "#161c24"}, {"#2f6fed", "#ffffff"}} {
		if ratio := contrast(pair[0], pair[1]); ratio < 4.5 {
			t.Errorf("contrast %s on %s = %.2f, below WCAG AA 4.5", pair[0], pair[1], ratio)
		}
		if !strings.Contains(css, pair[0]) || !strings.Contains(css, pair[1]) {
			t.Errorf("stylesheet no longer uses %s/%s; update the contrast check", pair[0], pair[1])
		}
	}
}

func contrast(fg, bg string) float64 {
	lum := func(hex string) float64 {
		var rgb [3]float64
		for i := range 3 {
			var c int
			_, _ = fmt.Sscanf(hex[1+2*i:3+2*i], "%02x", &c)
			v := float64(c) / 255
			if v <= 0.03928 {
				v /= 12.92
			} else {
				v = math.Pow((v+0.055)/1.055, 2.4)
			}
			rgb[i] = v
		}
		return 0.2126*rgb[0] + 0.7152*rgb[1] + 0.0722*rgb[2]
	}
	l1, l2 := lum(fg), lum(bg)
	if l1 < l2 {
		l1, l2 = l2, l1
	}
	return (l1 + 0.05) / (l2 + 0.05)
}

func TestVisualizer_EvaluationScorecardAndReport(t *testing.T) {
	t.Parallel()
	v := start(t)
	if status, body := v.get("/evaluation"); status != http.StatusOK || !strings.Contains(body, "Aucun corpus") {
		t.Fatalf("evaluation without corpus: %d %s", status, body)
	}
	if _, err := v.coll.Subscribe(t.Context(), app.SubscribeRequest{Repository: controlled.Repository, HTMLURL: "https://forge.example/controlled/demo"}); err != nil {
		t.Fatal(err)
	}
	if err := v.worker.RunOne(t.Context()); err != nil {
		t.Fatal(err)
	}
	head := controlled.Forge{Now: time.Now}
	fixture, _ := head.FetchPullRequest(t.Context(), pullrequest.Ref{Repository: controlled.Repository, Number: 1})
	var manifest evaluation.Manifest
	for i := range evaluation.CorpusSize {
		item := evaluation.Item{Repository: controlled.Repository, Number: int64(i + 1), HeadSHA: fixture.HeadSHA, Size: evaluation.SizeSmall,
			Authorship: evaluation.AuthorHuman, Category: evaluation.CategoryCode, Reason: "fixture"}
		if i%2 == 1 {
			item.Repository, item.Size, item.Authorship = "controlled/other", evaluation.SizeLarge, evaluation.AuthorAgent
		}
		item.Category = []evaluation.Category{evaluation.CategoryCode, evaluation.CategoryCI, evaluation.CategoryInfra}[i%3]
		manifest.Items = append(manifest.Items, item)
	}
	if _, err := v.evaluator.Freeze(t.Context(), manifest); err != nil {
		t.Fatal(err)
	}
	_, detail := v.get("/pr/controlled/demo%231")
	for _, want := range []string{"Démarrer le minuteur", `name="elapsed_ms"`, `name="critical_error"`, `name="useful"`, `name="review_needed"`} {
		if !strings.Contains(detail, want) {
			t.Errorf("scorecard lacks %q", want)
		}
	}
	form := url.Values{"elapsed_ms": {"42000"}, "intent": {"on"}, "structure": {"on"}, "risks": {"on"}, "review_needed": {"on"}, "useful": {"on"}}
	if status := v.post("/evaluation/score/controlled/demo%231", form); status != http.StatusSeeOther {
		t.Fatalf("score status %d", status)
	}
	if _, detail := v.get("/pr/controlled/demo%231"); !strings.Contains(detail, "Score enregistré") || !strings.Contains(detail, "42s, réussi") {
		t.Fatalf("detail after scoring lacks the recorded score:\n%s", detail)
	}
	status, body := v.get("/evaluation")
	if status != http.StatusOK || !strings.Contains(body, "Verdict : incomplete") || !strings.Contains(body, "1 réussies") || !strings.Contains(body, `href="/pr/controlled/demo%231"`) {
		t.Fatalf("evaluation page: %d\n%s", status, body)
	}
	_, report := v.get("/evaluation/report.json")
	if !strings.Contains(report, `"verdict": "incomplete"`) || !strings.Contains(report, `"scored": 1`) || strings.Contains(report, "Transient 429") {
		t.Fatalf("report export: %s", report)
	}
}
