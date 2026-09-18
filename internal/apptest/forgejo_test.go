package app_test

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/clement-software/PRadar/internal/adapter/forgejo"
	"github.com/clement-software/PRadar/internal/adapter/sqlite"
	"github.com/clement-software/PRadar/internal/collect"
)

// TestCollector_ImportsThroughRealForgejoClient drives the collector through
// the HTTP adapter against a paginated fake instance: no live request is made.
func TestCollector_ImportsThroughRealForgejoClient(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 17, 9, 0, 0, 0, time.UTC)
	var open []map[string]any
	for i := range 23 {
		item := map[string]any{
			"number": i + 1, "title": fmt.Sprintf("PR %d", i+1), "body": "b", "state": "open", "draft": i == 3, "merged": false,
			"html_url": "https://forge.test/acme/widgets/pulls/" + strconv.Itoa(i+1), "updated_at": now.Add(-time.Duration(i) * time.Minute).Format(time.RFC3339),
			"user": map[string]any{"login": "alice"}, "head": map[string]any{"sha": "sha-" + strconv.Itoa(i+1)},
		}
		if i == 5 {
			item["user"] = map[string]any{"login": "agent[bot]"}
		}
		open = append(open, item)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "token secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/api/v1/repos/acme/widgets":
			_ = json.NewEncoder(w).Encode(map[string]any{"full_name": "acme/widgets"})
		case "/api/v1/repos/acme/widgets/pulls":
			page, _ := strconv.Atoi(r.URL.Query().Get("page"))
			limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
			start := min((page-1)*limit, len(open))
			_ = json.NewEncoder(w).Encode(open[start:min(start+limit, len(open))])
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	instance, err := forgejo.ParseInstance(server.URL, true)
	if err != nil {
		t.Fatal(err)
	}
	client := forgejo.NewClient(instance, "secret")
	client.PageSize = 10
	store, err := sqlite.Open(t.Context(), t.TempDir()+"/pradar.sqlite", func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	collector := &collect.Collector{Forge: client, Store: store, Profile: profile, Debounce: 10 * time.Minute,
		Now: func() time.Time { return now }, Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	subscription, err := collector.Subscribe(t.Context(), collect.SubscribeRequest{Repository: "acme/widgets", HTMLURL: server.URL + "/acme/widgets", ExcludedAuthors: []string{"agent[bot]"}})
	if err != nil || !subscription.Active {
		t.Fatalf("Subscribe = %+v, %v", subscription, err)
	}
	status, err := store.Status(t.Context())
	if err != nil || status.Pending != 10 {
		t.Fatalf("default import must schedule ten: %+v %v", status, err)
	}
	refs, err := store.ListLocallyOpen(t.Context(), "acme/widgets")
	if err != nil || len(refs) != 21 {
		t.Fatalf("all open non-draft non-excluded pull requests must be recorded across pages: %d %v", len(refs), err)
	}
}
