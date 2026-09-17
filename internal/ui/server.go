// Package ui is the disposable loopback visualizer used to evaluate the
// demonstrator: timeline, detail, status and the reading actions. It talks
// only to application use cases and makes no production toolkit decision.
package ui

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/clement-software/PRadar/internal/app"
	"github.com/clement-software/PRadar/internal/pullrequest"
)

//go:embed templates/*.html assets/*
var content embed.FS

// Server serves the visualizer on loopback only.
type Server struct {
	Timeline  *app.Timeline
	Collector *app.Collector
	Log       *slog.Logger
	// ParseRepositoryURL maps a Forgejo repository URL to "owner/name" and its canonical URL.
	ParseRepositoryURL func(raw string) (repository, htmlURL string, err error)

	templates *template.Template
}

func (s *Server) handler() http.Handler {
	s.templates = template.Must(template.New("").Funcs(template.FuncMap{
		"markdown": RenderMarkdown,
		"when":     func(t time.Time) string { return t.Local().Format("2006-01-02 15:04") },
		"duration": func(d time.Duration) string { return d.Round(time.Second).String() },
		"join":     strings.Join,
		"prPath":   prPath,
		"safeURL":  safeURL,
	}).ParseFS(content, "templates/*.html"))
	assets, _ := fs.Sub(content, "assets")

	mux := http.NewServeMux()
	mux.Handle("GET /assets/", http.StripPrefix("/assets/", http.FileServerFS(assets)))
	mux.HandleFunc("GET /{$}", s.timeline)
	mux.HandleFunc("GET /pr/{repository...}", s.detail)
	mux.HandleFunc("POST /pr/{repository...}", s.action)
	mux.HandleFunc("POST /subscriptions", s.subscribe)
	mux.HandleFunc("POST /subscriptions/{repository...}", s.subscriptionAction)
	return securityHeaders(mux)
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", "default-src 'none'; script-src 'self'; style-src 'self'; img-src 'self' data:; form-action 'self'; base-uri 'none'; frame-ancestors 'none'")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(w, r)
	})
}

// Serve binds 127.0.0.1 on an ephemeral port (or addr when given), reports the
// URL through ready and serves until ctx ends.
func (s *Server) Serve(ctx context.Context, addr string, ready func(url string)) error {
	if addr == "" {
		addr = "127.0.0.1:0"
	}
	host, _, err := net.SplitHostPort(addr)
	if err != nil || !net.ParseIP(host).IsLoopback() {
		return fmt.Errorf("visualizer address %q must be loopback", addr)
	}
	listener, err := (&net.ListenConfig{}).Listen(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	server := &http.Server{Handler: s.handler(), ReadHeaderTimeout: 5 * time.Second}
	if ready != nil {
		ready("http://" + listener.Addr().String() + "/")
	}
	stop := context.AfterFunc(ctx, func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	})
	defer stop()
	if err := server.Serve(listener); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

type page struct {
	Title       string
	Status      app.Status
	Cards       []app.Card
	Filter      app.Filter
	Detail      app.Detail
	Notice      string
	Problem     string
	Repos       []string
	Risks       []string
	States      []string
	Importances []string
	Ref         pullrequest.Ref
	Replayed    bool
}

func (s *Server) render(w http.ResponseWriter, name string, data page) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.templates.ExecuteTemplate(w, name, data); err != nil {
		s.Log.Error("render failed", "template", name, "error", err.Error())
	}
}

func (s *Server) fail(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, app.ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, app.ErrNotVisible), errors.Is(err, app.ErrReplayUnchanged):
		status = http.StatusConflict
	}
	http.Error(w, err.Error(), status)
}

func (s *Server) timeline(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filter := app.Filter{
		UnreadOnly: q.Get("unread") == "1", Repository: q.Get("repository"),
		State: pullrequest.State(q.Get("state")), Importance: pullrequest.Importance(q.Get("importance")), Risk: q.Get("risk"),
	}
	status, err := s.Timeline.Status(r.Context())
	if err != nil {
		s.fail(w, err)
		return
	}
	all, err := s.Timeline.Cards(r.Context(), app.Filter{})
	if err != nil {
		s.fail(w, err)
		return
	}
	data := page{Title: "Timeline", Status: status, Filter: filter, Notice: q.Get("notice"), Problem: q.Get("problem"),
		States: []string{"open", "closed", "merged"}, Importances: []string{"low", "medium", "high"}}
	seenRepo, seenRisk := map[string]bool{}, map[string]bool{}
	for _, card := range all {
		if !seenRepo[card.Ref.Repository] {
			seenRepo[card.Ref.Repository] = true
			data.Repos = append(data.Repos, card.Ref.Repository)
		}
		for _, risk := range card.Analysis.Risks {
			if !seenRisk[risk] {
				seenRisk[risk] = true
				data.Risks = append(data.Risks, risk)
			}
		}
	}
	data.Cards, err = s.Timeline.Cards(r.Context(), filter)
	if err != nil {
		s.fail(w, err)
		return
	}
	s.render(w, "timeline.html", data)
}

// safeURL keeps only absolute http(s) links; anything else renders as an
// inert empty href so a hostile Forgejo or model value cannot run code.
func safeURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Host == "" {
		return ""
	}
	return parsed.String()
}

// prPath is the detail path of a pull request; '#' must be escaped so it is
// not taken as a fragment.
func prPath(ref pullrequest.Ref) string {
	return "/pr/" + ref.Repository + "%23" + strconv.FormatInt(ref.Number, 10)
}

func refFromPath(r *http.Request) (pullrequest.Ref, error) {
	return pullrequest.ParseKey(r.PathValue("repository"))
}

func (s *Server) detail(w http.ResponseWriter, r *http.Request) {
	ref, err := refFromPath(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	detail, err := s.Timeline.Detail(r.Context(), ref)
	if err != nil {
		s.fail(w, err)
		return
	}
	status, err := s.Timeline.Status(r.Context())
	if err != nil {
		s.fail(w, err)
		return
	}
	s.render(w, "detail.html", page{Title: detail.Card.Title, Detail: detail, Status: status, Ref: ref,
		Notice: r.URL.Query().Get("notice"), Problem: r.URL.Query().Get("problem")})
}

func (s *Server) action(w http.ResponseWriter, r *http.Request) {
	ref, err := refFromPath(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	var notice string
	switch r.FormValue("action") {
	case "read":
		err, notice = s.Timeline.MarkRead(r.Context(), ref), "Marquée comme lue"
	case "archive":
		err, notice = s.Timeline.Archive(r.Context(), ref), "Archivée"
	case "replay":
		err, notice = s.Timeline.Replay(r.Context(), ref), "Rejeu planifié"
	default:
		http.Error(w, "unknown action", http.StatusBadRequest)
		return
	}
	target := prPath(ref)
	if r.FormValue("action") == "archive" {
		target = "/"
	}
	redirect(w, r, target, notice, err)
}

func redirect(w http.ResponseWriter, r *http.Request, target, notice string, err error) {
	values := url.Values{}
	if err != nil {
		values.Set("problem", err.Error())
	} else if notice != "" {
		values.Set("notice", notice)
	}
	location := url.URL{Path: target, RawQuery: values.Encode()}
	http.Redirect(w, r, location.String(), http.StatusSeeOther)
}

func (s *Server) subscribe(w http.ResponseWriter, r *http.Request) {
	repository, htmlURL, err := s.ParseRepositoryURL(r.FormValue("url"))
	if err != nil {
		redirect(w, r, "/", "", err)
		return
	}
	var excluded []string
	for author := range strings.SplitSeq(r.FormValue("excluded_authors"), ",") {
		if author = strings.TrimSpace(author); author != "" {
			excluded = append(excluded, author)
		}
	}
	subscription, err := s.Collector.Subscribe(r.Context(), app.SubscribeRequest{
		Repository: repository, HTMLURL: htmlURL, Import: app.ImportMode(r.FormValue("import")), ExcludedAuthors: excluded,
	})
	notice := "Abonnement actif : " + repository
	if err == nil && !subscription.Active {
		err = errors.New("abonnement bloqué : " + subscription.BlockedReason)
	}
	redirect(w, r, "/", notice, err)
}

func (s *Server) subscriptionAction(w http.ResponseWriter, r *http.Request) {
	repository := r.PathValue("repository")
	var (
		err    error
		notice string
	)
	switch r.FormValue("action") {
	case "unsubscribe":
		err, notice = s.Collector.Unsubscribe(r.Context(), repository), "Désabonnement effectué, historique conservé"
	case "delete":
		err, notice = s.Collector.DeleteRepositoryData(r.Context(), repository, r.FormValue("confirm") == repository), "Données du dépôt supprimées"
	default:
		http.Error(w, "unknown action", http.StatusBadRequest)
		return
	}
	redirect(w, r, "/", notice, err)
}
