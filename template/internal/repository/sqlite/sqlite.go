package sqlite

import (
	"log/slog"
	"time"
)

// sqliteTime — формат хранения времени в текстовых колонках (как datetime('now')).
// Единый формат для записи и чтения по всем репозиториям.
const sqliteTime = "2006-01-02 15:04:05"

// parseTime разбирает datetime('now')-строку SQLite в time.Time. При ошибке
// возвращает нулевое время, но ЛОГИРУЕТ — иначе битый формат тихо делал бы
// сессию/токен мгновенно невалидными (нулевое время = «истекло»), и причину было
// бы не найти. Пустая строка (не заданное значение) — норма, не логируем.
func parseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse(sqliteTime, s)
	if err != nil {
		slog.Warn("sqlite: не удалось разобрать время", "value", s, "error", err)
		return time.Time{}
	}
	return t
}
