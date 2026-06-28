-- +goose Up
-- Срок заметки (необязательная дата) — демонстрирует редактируемое date-поле в админке
-- (нативный календарь). Пусто = срока нет.
ALTER TABLE notes ADD COLUMN due_date TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE notes DROP COLUMN due_date;
