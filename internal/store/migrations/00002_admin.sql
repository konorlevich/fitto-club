-- +goose Up
-- Admin accounts and sessions (DESIGN_BRIEF fitto-admin). Same portable DDL
-- rules as 00001: TEXT ids, INTEGER booleans, RFC 3339 TEXT timestamps.

CREATE TABLE users (
    id            TEXT PRIMARY KEY,
    login         TEXT NOT NULL UNIQUE,
    name          TEXT NOT NULL DEFAULT '',
    role          TEXT NOT NULL DEFAULT 'editor',   -- owner | editor
    password_hash TEXT NOT NULL DEFAULT '',         -- '' = not set yet (ENV bootstrap / temp password pending)
    must_change   INTEGER NOT NULL DEFAULT 1,
    blocked       INTEGER NOT NULL DEFAULT 0,
    lang          TEXT NOT NULL DEFAULT 'ru',
    theme         TEXT NOT NULL DEFAULT 'auto',     -- light | dark | auto
    created_at    TEXT NOT NULL DEFAULT '',
    updated_at    TEXT NOT NULL DEFAULT ''
);

-- id is the SHA-256 of the cookie value: a database leak does not leak sessions.
CREATE TABLE sessions (
    id         TEXT PRIMARY KEY,
    user_id    TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    csrf       TEXT NOT NULL,
    created_at TEXT NOT NULL,
    expires_at TEXT NOT NULL
);
CREATE INDEX idx_sessions_user ON sessions(user_id);

-- Failed logins for the rate limit; rows older than an hour are pruned.
CREATE TABLE login_attempts (
    id    TEXT PRIMARY KEY,
    login TEXT NOT NULL,
    ip    TEXT NOT NULL,
    at    TEXT NOT NULL
);
CREATE INDEX idx_login_attempts ON login_attempts(at);

-- Uploaded coach photo: base file name under DATA_DIR/uploads (without the
-- width suffix), and the focus point of the 4:5 crop as fractions 0..1.
ALTER TABLE coaches ADD COLUMN photo_file TEXT NOT NULL DEFAULT '';
ALTER TABLE coaches ADD COLUMN focus_x REAL NOT NULL DEFAULT 0.5;
ALTER TABLE coaches ADD COLUMN focus_y REAL NOT NULL DEFAULT 0.5;

-- +goose Down
ALTER TABLE coaches DROP COLUMN focus_y;
ALTER TABLE coaches DROP COLUMN focus_x;
ALTER TABLE coaches DROP COLUMN photo_file;
DROP TABLE login_attempts;
DROP TABLE sessions;
DROP TABLE users;
