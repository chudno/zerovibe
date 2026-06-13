-- Первая миграция: исходная схема приложения (заметки-образец).
-- Совпадает со схемой, которая раньше применялась через CREATE TABLE IF NOT EXISTS,
-- чтобы у уже работающих приложений база не разошлась при переходе на goose.
-- +goose Up
CREATE TABLE IF NOT EXISTS notes (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    title      TEXT NOT NULL,
    body       TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT (datetime('now'))
);

-- +goose Down
DROP TABLE IF EXISTS notes;
