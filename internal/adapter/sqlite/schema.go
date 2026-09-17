package sqlite

// schemaVersion is stored in PRAGMA user_version. The demonstrator has no
// migration ladder: an older file is refused rather than silently rebuilt.
const schemaVersion = 1

const schema = `
CREATE TABLE IF NOT EXISTS subscriptions (
  repository       TEXT PRIMARY KEY,
  html_url         TEXT NOT NULL,
  generation       INTEGER NOT NULL DEFAULT 1,
  active           INTEGER NOT NULL DEFAULT 1,
  blocked_reason   TEXT NOT NULL DEFAULT '',
  excluded_authors TEXT NOT NULL DEFAULT '[]',
  last_sync_unix   INTEGER NOT NULL DEFAULT 0,
  created_unix     INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS pull_requests (
  pr_key             TEXT PRIMARY KEY,
  repository         TEXT NOT NULL REFERENCES subscriptions(repository) ON DELETE CASCADE,
  number             INTEGER NOT NULL,
  title              TEXT NOT NULL,
  body               TEXT NOT NULL,
  author             TEXT NOT NULL,
  state              TEXT NOT NULL,
  html_url           TEXT NOT NULL,
  head_sha           TEXT NOT NULL,
  updated_unix       INTEGER NOT NULL,
  activity_unix      INTEGER NOT NULL,
  reopen_generation  INTEGER NOT NULL DEFAULT 0,
  input_revision     TEXT NOT NULL,
  observed_identity  TEXT NOT NULL,
  published_identity TEXT NOT NULL DEFAULT '',
  unread             INTEGER NOT NULL DEFAULT 1,
  archived           INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS pull_requests_repository ON pull_requests(repository, state);

CREATE TABLE IF NOT EXISTS analysis_jobs (
  id               INTEGER PRIMARY KEY AUTOINCREMENT,
  identity         TEXT NOT NULL UNIQUE,
  pr_key           TEXT NOT NULL REFERENCES pull_requests(pr_key) ON DELETE CASCADE,
  generation       INTEGER NOT NULL,
  head_sha         TEXT NOT NULL,
  input_revision   TEXT NOT NULL,
  profile_json     TEXT NOT NULL,
  status           TEXT NOT NULL CHECK (status IN ('queued','running','done','unavailable','superseded','cancelled')),
  attempts         INTEGER NOT NULL DEFAULT 0,
  available_unix   INTEGER NOT NULL,
  lease_until_unix INTEGER,
  lease_token      TEXT,
  last_error       TEXT NOT NULL DEFAULT '',
  created_unix     INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS analysis_jobs_claimable ON analysis_jobs(status, available_unix, id);

CREATE TABLE IF NOT EXISTS analyses (
  id              INTEGER PRIMARY KEY AUTOINCREMENT,
  identity        TEXT NOT NULL UNIQUE,
  pr_key          TEXT NOT NULL REFERENCES pull_requests(pr_key) ON DELETE CASCADE,
  head_sha        TEXT NOT NULL,
  status          TEXT NOT NULL,
  result_json     TEXT NOT NULL,
  provenance_json TEXT NOT NULL,
  published       INTEGER NOT NULL,
  created_unix    INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS analyses_pr_key ON analyses(pr_key, id);

CREATE TABLE IF NOT EXISTS pr_events (
  id      INTEGER PRIMARY KEY AUTOINCREMENT,
  pr_key  TEXT NOT NULL REFERENCES pull_requests(pr_key) ON DELETE CASCADE,
  at_unix INTEGER NOT NULL,
  kind    TEXT NOT NULL,
  detail  TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS pr_events_pr_key ON pr_events(pr_key, id);

CREATE TABLE IF NOT EXISTS corpora (
  id            TEXT PRIMARY KEY,
  frozen_unix   INTEGER NOT NULL,
  manifest_json TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS scores (
  corpus_id     TEXT NOT NULL REFERENCES corpora(id) ON DELETE CASCADE,
  pr_key        TEXT NOT NULL,
  score_json    TEXT NOT NULL,
  recorded_unix INTEGER NOT NULL,
  PRIMARY KEY (corpus_id, pr_key)
);
`
