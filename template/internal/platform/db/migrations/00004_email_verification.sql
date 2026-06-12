-- Подтверждение почты: токены подтверждения + отметка у пользователя.
-- +goose Up
CREATE TABLE IF NOT EXISTS email_verifications (
    token      TEXT PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TEXT NOT NULL DEFAULT (datetime('now')),
    expires_at TEXT NOT NULL,
    used_at    TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_email_verifications_user ON email_verifications(user_id);

-- Отметка времени подтверждения почты у пользователя (пусто = не подтверждена).
ALTER TABLE users ADD COLUMN email_verified_at TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE users DROP COLUMN email_verified_at;
DROP INDEX IF EXISTS idx_email_verifications_user;
DROP TABLE IF EXISTS email_verifications;
