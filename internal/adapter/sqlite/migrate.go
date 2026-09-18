package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// migration is one ordered, irreversible step of the schema ladder. Version
// numbers are contiguous from 1 and never reused: a released version's
// statements are frozen, and a change is a new step.
type migration struct {
	version int
	name    string
	stmts   string
}

// migrations is the ladder applied at startup, in order.
var migrations = []migration{
	{version: 1, name: "initial schema", stmts: schema},
}

// latest is the version a ladder brings a database to.
func latest(ladder []migration) int { return ladder[len(ladder)-1].version }

// ErrSchemaTooNew reports a database written by a newer application.
var ErrSchemaTooNew = errors.New("database was created by a newer version of PRadar")

// ErrUnreadable reports a database that cannot be read, most often because it
// is corrupt. Mutation stops and the files are preserved.
var ErrUnreadable = errors.New("database cannot be read")

// MigrationReport says what Open did to the database, so the caller can tell
// the user where the backup went.
type MigrationReport struct {
	Created     bool
	FromVersion int
	ToVersion   int
	BackupPath  string
}

// Applied reports whether any migration ran.
func (r MigrationReport) Applied() bool { return r.FromVersion != r.ToVersion }

// migrate brings the database to schemaVersion, backing it up first when
// there is data to lose.
func migrate(ctx context.Context, db *sql.DB, path string, now func() time.Time, ladder []migration) (MigrationReport, error) {
	want := latest(ladder)
	if err := checkIntegrity(ctx, db); err != nil {
		return MigrationReport{}, err
	}
	var journal string
	if err := db.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&journal); err != nil {
		return MigrationReport{}, fmt.Errorf("read journal mode: %w", err)
	}
	if !strings.EqualFold(journal, "wal") {
		return MigrationReport{}, fmt.Errorf("sqlite journal mode is %q, want wal", journal)
	}
	var current int
	if err := db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&current); err != nil {
		return MigrationReport{}, fmt.Errorf("read schema version: %w", err)
	}
	report := MigrationReport{Created: current == 0, FromVersion: current, ToVersion: current}
	switch {
	case current > want:
		return report, fmt.Errorf("%w: it is at version %d and this build understands version %d", ErrSchemaTooNew, current, want)
	case current == want:
		return report, nil
	}
	// An existing database is copied before it is changed; a database being
	// created has nothing to lose.
	if !report.Created {
		backup, err := backupDatabase(ctx, db, path, current, now)
		if err != nil {
			return report, err
		}
		report.BackupPath = backup
	}
	for _, step := range ladder {
		if step.version <= current {
			continue
		}
		if err := applyMigration(ctx, db, step); err != nil {
			return report, err
		}
		report.ToVersion = step.version
	}
	return report, nil
}

// applyMigration runs one step and records its version in the same
// transaction, so a failure leaves the previous version in place.
func applyMigration(ctx context.Context, db *sql.DB, step migration) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration %d: %w", step.version, err)
	}
	if _, err := tx.ExecContext(ctx, step.stmts); err != nil {
		return errors.Join(fmt.Errorf("migration %d (%s): %w", step.version, step.name, err), tx.Rollback())
	}
	// PRAGMA user_version takes no parameter; the value is an integer literal
	// from the ladder, never user input.
	if _, err := tx.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", step.version)); err != nil {
		return errors.Join(fmt.Errorf("record schema version %d: %w", step.version, err), tx.Rollback())
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration %d: %w", step.version, err)
	}
	return nil
}

// checkIntegrity refuses to touch a database SQLite cannot read.
func checkIntegrity(ctx context.Context, db *sql.DB) error {
	var result string
	if err := db.QueryRowContext(ctx, "PRAGMA quick_check(1)").Scan(&result); err != nil {
		return fmt.Errorf("%w: %w", ErrUnreadable, err)
	}
	if !strings.EqualFold(result, "ok") {
		return fmt.Errorf("%w: integrity check reported %q", ErrUnreadable, result)
	}
	return nil
}

// backupDatabase writes a consistent copy next to the database, including the
// contents of the write-ahead log, and returns its path.
func backupDatabase(ctx context.Context, db *sql.DB, path string, version int, now func() time.Time) (string, error) {
	backup := fmt.Sprintf("%s.v%d.%s.backup", path, version, now().UTC().Format("20060102T150405Z"))
	// VACUUM INTO writes a single consistent file and needs no extra locking.
	if _, err := db.ExecContext(ctx, "VACUUM INTO ?", backup); err != nil {
		return "", fmt.Errorf("back up the database before migrating: %w", err)
	}
	if _, err := os.Stat(backup); err != nil {
		return "", fmt.Errorf("verify the backup: %w", err)
	}
	return backup, nil
}

// CopyDatabase writes a consistent copy of the open database to destination.
// It is the recovery procedure's building block and is used by tests.
func (s *Store) CopyDatabase(ctx context.Context, destination string) error {
	if err := os.MkdirAll(filepath.Dir(destination), 0o700); err != nil {
		return fmt.Errorf("create backup directory: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, "VACUUM INTO ?", destination); err != nil {
		return fmt.Errorf("copy the database: %w", err)
	}
	return nil
}
