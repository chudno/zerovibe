-- Аутентификация: пользователи, сессии, токены сброса пароля, счётчики рейт-лимита.
-- Плюс заметки становятся личными (привязка к владельцу).
-- +goose Up
CREATE TABLE IF NOT EXISTS users (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    email         TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    role          TEXT NOT NULL DEFAULT 'user',
    created_at    TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS sessions (
    token      TEXT PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    expires_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions(user_id);

CREATE TABLE IF NOT EXISTS password_resets (
    token      TEXT PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    expires_at TEXT NOT NULL,
    used_at    TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_resets_user ON password_resets(user_id);

CREATE TABLE IF NOT EXISTS rate_counters (
    key          TEXT PRIMARY KEY,
    window_start TEXT NOT NULL,
    count        INTEGER NOT NULL DEFAULT 0
);

-- Заметки получают владельца. owner_id=0 — «ничей» (для возможных старых записей до
-- появления аутентификации); реальные пользователи начинаются с id=1.
ALTER TABLE notes ADD COLUMN owner_id INTEGER NOT NULL DEFAULT 0;
CREATE INDEX IF NOT EXISTS idx_notes_owner ON notes(owner_id);

-- +goose Down
DROP INDEX IF EXISTS idx_notes_owner;
ALTER TABLE notes DROP COLUMN owner_id;
DROP TABLE IF EXISTS rate_counters;
DROP INDEX IF EXISTS idx_resets_user;
DROP TABLE IF EXISTS password_resets;
DROP INDEX IF EXISTS idx_sessions_user;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS users;
