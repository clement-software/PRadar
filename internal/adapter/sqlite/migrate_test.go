package sqlite_test

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/clement-software/PRadar/internal/adapter/sqlite"
)

func at(t *testing.T) func() time.Time {
	t.Helper()
	return func() time.Time { return time.Date(2026, 9, 18, 9, 0, 0, 0, time.UTC) }
}

func TestOpen_CreatesTheSchemaWithoutABackup(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "pradar.sqlite")
	store, err := sqlite.Open(t.Context(), path, at(t))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	report := store.Migration()
	if !report.Created || !report.Applied() || report.ToVersion == 0 || report.BackupPath != "" {
		t.Fatalf("creating a database = %+v, want a created ladder without a backup", report)
	}
	matches, _ := filepath.Glob(path + "*.backup")
	if len(matches) != 0 {
		t.Fatalf("a new database must not be backed up: %v", matches)
	}
}

func TestOpen_IsANoOpOnAnUpToDateDatabase(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "pradar.sqlite")
	first, err := sqlite.Open(t.Context(), path, at(t))
	if err != nil {
		t.Fatal(err)
	}
	version := first.Migration().ToVersion
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	second, err := sqlite.Open(t.Context(), path, at(t))
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	report := second.Migration()
	if report.Created || report.Applied() || report.FromVersion != version || report.BackupPath != "" {
		t.Fatalf("reopening an up-to-date database = %+v", report)
	}
}

// setVersion rewinds a database to another schema version.
func setVersion(t *testing.T, path string, version int) {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.ExecContext(t.Context(), "PRAGMA user_version = "+strconv.Itoa(version)); err != nil {
		t.Fatal(err)
	}
}

func TestOpen_RefusesANewerDatabaseAndKeepsIt(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "pradar.sqlite")
	store, err := sqlite.Open(t.Context(), path, at(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	setVersion(t, path, 9)

	_, err = sqlite.Open(t.Context(), path, at(t))
	if !errors.Is(err, sqlite.ErrSchemaTooNew) || !strings.Contains(err.Error(), "version 9") {
		t.Fatalf("Open on a newer database = %v, want ErrSchemaTooNew naming the version", err)
	}
	if _, statErr := os.Stat(path); statErr != nil {
		t.Fatal("the database must be preserved")
	}
	matches, _ := filepath.Glob(path + "*.backup")
	if len(matches) != 0 {
		t.Fatalf("a refused database must not be copied: %v", matches)
	}
}

func TestOpen_RefusesAnUnreadableDatabase(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "pradar.sqlite")
	if err := os.WriteFile(path, []byte("this is not a database"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := sqlite.Open(t.Context(), path, at(t))
	if !errors.Is(err, sqlite.ErrUnreadable) {
		t.Fatalf("Open on a corrupt file = %v, want ErrUnreadable", err)
	}
	content, readErr := os.ReadFile(path)
	if readErr != nil || string(content) != "this is not a database" {
		t.Fatal("the unreadable file must be preserved untouched")
	}
}

func TestCopyDatabase_WritesARestorableCopy(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "pradar.sqlite")
	store, err := sqlite.Open(t.Context(), path, at(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.PutSubscription(t.Context(), subscriptionFixture()); err != nil {
		t.Fatal(err)
	}
	copyPath := filepath.Join(dir, "backups", "pradar.sqlite")
	if err := store.CopyDatabase(t.Context(), copyPath); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	restored, err := sqlite.Open(t.Context(), copyPath, at(t))
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	if restored.Migration().Applied() {
		t.Fatal("a copy is already at the current version")
	}
	subscriptions, err := restored.ListSubscriptions(t.Context())
	if err != nil || len(subscriptions) != 1 {
		t.Fatalf("the copy must carry the data: %d %v", len(subscriptions), err)
	}
}
