// Package sqlite is the demonstrator's local durability adapter: one WAL
// database holding abonnements, observed versions, leased analysis work,
// append-only analyses, the visible projection, user state and evaluation
// records. It implements the store interfaces declared by internal/app.
package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	_ "modernc.org/sqlite" // database/sql driver selected for the demonstrator only
)

// Store is the SQLite-backed implementation of the application stores.
type Store struct {
	db        *sql.DB
	now       func() time.Time
	migration MigrationReport
}

// Migration reports what opening the database did to it.
func (s *Store) Migration() MigrationReport { return s.migration }

// Open opens or creates the database at path, enables WAL and a busy timeout,
// validates the schema version and creates missing tables.
func Open(ctx context.Context, path string, now func() time.Time) (*Store, error) {
	if strings.ContainsAny(path, "?#") {
		return nil, errors.New("database path must not contain a query string or fragment")
	}
	db, err := openDB(path)
	if err != nil {
		return nil, err
	}
	store := &Store{db: db, now: now}
	report, err := migrate(ctx, db, path, now, migrations)
	if err != nil {
		return nil, errors.Join(err, db.Close())
	}
	store.migration = report
	return store, nil
}

// openDB opens the file in WAL mode with the demonstrator's pragmas.
func openDB(path string) (*sql.DB, error) {
	dsn := "file:" + url.PathEscape(path) +
		"?_txlock=immediate&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=synchronous(NORMAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	return db, nil
}

// Close releases the connection pool.
func (s *Store) Close() error { return s.db.Close() }

// DB exposes the pool for tests that inspect durable state.
func (s *Store) DB() *sql.DB { return s.db }

func (s *Store) tx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	if err := fn(tx); err != nil {
		return errors.Join(err, tx.Rollback())
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

func unix(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.Unix()
}

func fromUnix(v int64) time.Time {
	if v == 0 {
		return time.Time{}
	}
	return time.Unix(v, 0).UTC()
}

func event(ctx context.Context, tx *sql.Tx, key string, at time.Time, kind, detail string) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO pr_events(pr_key, at_unix, kind, detail) VALUES (?, ?, ?, ?)`, key, at.Unix(), kind, detail)
	if err != nil {
		return fmt.Errorf("record %s event: %w", kind, err)
	}
	return nil
}

func rowsAffected(result sql.Result, err error) (int64, error) {
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func marshal(v any) string {
	payload, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("marshal %T: %v", v, err)) // only plain structs reach here
	}
	return string(payload)
}
