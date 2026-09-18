package sqlite

import (
	"path/filepath"
	"testing"
	"time"
)

func fixedClock() func() time.Time {
	return func() time.Time { return time.Date(2026, 9, 18, 9, 0, 0, 0, time.UTC) }
}

// ladder returns the real first step plus the given test steps, so migration
// behaviour is proven without waiting for the next real schema change.
func ladder(extra ...migration) []migration {
	return append(append([]migration{}, migrations...), extra...)
}

func openAt(t *testing.T, path string, steps []migration) (*Store, MigrationReport) {
	t.Helper()
	db, err := openDB(path)
	if err != nil {
		t.Fatal(err)
	}
	report, err := migrate(t.Context(), db, path, fixedClock(), steps)
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	store := &Store{db: db, now: fixedClock(), migration: report}
	t.Cleanup(func() { _ = store.Close() })
	return store, report
}

func TestMigrate_AppliesPendingStepsInOrderAndBacksUpFirst(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "pradar.sqlite")
	first, initial := openAt(t, path, migrations)
	if _, err := first.db.ExecContext(t.Context(), `INSERT INTO subscriptions(repository, html_url, created_unix) VALUES ('acme/widgets', '', 0)`); err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}

	steps := ladder(
		migration{version: initial.ToVersion + 1, name: "add notes", stmts: `CREATE TABLE notes (note TEXT NOT NULL)`},
		migration{version: initial.ToVersion + 2, name: "note the first subscription", stmts: `INSERT INTO notes(note) SELECT repository FROM subscriptions`},
	)
	migrated, report := openAt(t, path, steps)
	if report.Created || report.FromVersion != initial.ToVersion || report.ToVersion != initial.ToVersion+2 || report.BackupPath == "" {
		t.Fatalf("report = %+v", report)
	}
	var note string
	if err := migrated.db.QueryRowContext(t.Context(), `SELECT note FROM notes`).Scan(&note); err != nil || note != "acme/widgets" {
		t.Fatalf("steps must run in order over existing data: %q %v", note, err)
	}
	var version int
	if err := migrated.db.QueryRowContext(t.Context(), `PRAGMA user_version`).Scan(&version); err != nil || version != report.ToVersion {
		t.Fatalf("recorded version = %d, want %d (%v)", version, report.ToVersion, err)
	}

	// The backup predates the migration and is a working database.
	backup, backupReport := openAt(t, report.BackupPath, migrations)
	if backupReport.Applied() {
		t.Fatal("the backup is already at its own version")
	}
	if _, err := backup.db.ExecContext(t.Context(), `SELECT 1 FROM notes`); err == nil {
		t.Fatal("the backup must predate the new table")
	}
	var repository string
	if err := backup.db.QueryRowContext(t.Context(), `SELECT repository FROM subscriptions`).Scan(&repository); err != nil || repository != "acme/widgets" {
		t.Fatalf("the backup must carry the data: %q %v", repository, err)
	}
}

func TestMigrate_FailureKeepsThePreviousVersionAndTheData(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "pradar.sqlite")
	first, initial := openAt(t, path, migrations)
	if _, err := first.db.ExecContext(t.Context(), `INSERT INTO subscriptions(repository, html_url, created_unix) VALUES ('acme/widgets', '', 0)`); err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}

	broken := ladder(
		migration{version: initial.ToVersion + 1, name: "add notes", stmts: `CREATE TABLE notes (note TEXT NOT NULL)`},
		migration{version: initial.ToVersion + 2, name: "deliberately invalid", stmts: `CREATE TABLE ??? (`},
	)
	db, err := openDB(path)
	if err != nil {
		t.Fatal(err)
	}
	report, err := migrate(t.Context(), db, path, fixedClock(), broken)
	if err == nil || report.ToVersion != initial.ToVersion+1 {
		t.Fatalf("a failing step must stop the ladder: %+v %v", report, err)
	}
	if report.BackupPath == "" {
		t.Fatal("the database must be backed up before migrating")
	}
	var version int
	if err := db.QueryRowContext(t.Context(), `PRAGMA user_version`).Scan(&version); err != nil || version != initial.ToVersion+1 {
		t.Fatalf("version after the failure = %d, want the last successful step %d (%v)", version, initial.ToVersion+1, err)
	}
	var repository string
	if err := db.QueryRowContext(t.Context(), `SELECT repository FROM subscriptions`).Scan(&repository); err != nil || repository != "acme/widgets" {
		t.Fatalf("data must survive a failed migration: %q %v", repository, err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	// Reopening with the same broken ladder fails again rather than mutating.
	if _, _, err := reopen(t, path, broken); err == nil {
		t.Fatal("a broken ladder must keep failing instead of silently continuing")
	}
}

func reopen(t *testing.T, path string, steps []migration) (*Store, MigrationReport, error) {
	t.Helper()
	db, err := openDB(path)
	if err != nil {
		return nil, MigrationReport{}, err
	}
	report, err := migrate(t.Context(), db, path, fixedClock(), steps)
	if err != nil {
		return nil, report, err
	}
	store := &Store{db: db, now: fixedClock(), migration: report}
	t.Cleanup(func() { _ = store.Close() })
	return store, report, nil
}
