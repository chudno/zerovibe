package sqlite

import "time"

// parseTime разбирает datetime('now')-строку SQLite ("2006-01-02 15:04:05") в
// time.Time. При ошибке возвращает нулевое время — слой представления покажет
// его как пустое, а не упадёт.
func parseTime(s string) time.Time {
	t, err := time.Parse("2006-01-02 15:04:05", s)
	if err != nil {
		return time.Time{}
	}
	return t
}
