// SQLite-репозиторий рейт-лимитов (оконные счётчики). Учёт попытки — атомарный
// read-modify-write СТРОГО внутри одного db.Write: единственная writer-горутина
// сериализует записи, поэтому между чтением и обновлением счётчика не вклинится
// другой писатель — гонок нет без явных транзакций. Счётчики переживают рестарт
// (хранятся в БД), как и требовалось.
package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// RateLimitRepo — SQLite-репозиторий оконных счётчиков.
type RateLimitRepo struct {
	db writer
}

// NewRateLimitRepo собирает репозиторий рейт-лимитов поверх платформенного db.
func NewRateLimitRepo(db writer) *RateLimitRepo {
	return &RateLimitRepo{db: db}
}

// Allow учитывает попытку по ключу и сообщает, не превышен ли лимit. Fixed-window:
// если текущее окно истекло (now - window_start >= window) — начинаем новое.
// Возвращает (разрешено, сколько ждать до сброса окна при запрете, ошибка).
func (r *RateLimitRepo) Allow(ctx context.Context, key string, limit int, window time.Duration, now time.Time) (bool, time.Duration, error) {
	var allowed bool
	var retryAfter time.Duration

	err := r.db.Write(ctx, func(s *sql.DB) error {
		var startStr string
		var count int
		err := s.QueryRowContext(ctx,
			`SELECT window_start, count FROM rate_counters WHERE key = ?`, key,
		).Scan(&startStr, &count)

		windowStart := now
		switch {
		case errors.Is(err, sql.ErrNoRows):
			// первой попытки ещё не было — заводим окно
			count = 0
			windowStart = now
		case err != nil:
			return fmt.Errorf("select rate counter: %w", err)
		default:
			windowStart = parseTime(startStr)
			if now.Sub(windowStart) >= window {
				// окно истекло — новое окно
				count = 0
				windowStart = now
			}
		}

		count++
		allowed = count <= limit
		if !allowed {
			retryAfter = window - now.Sub(windowStart)
			if retryAfter < 0 {
				retryAfter = 0
			}
		}

		_, err = s.ExecContext(ctx,
			`INSERT INTO rate_counters (key, window_start, count) VALUES (?, ?, ?)
			 ON CONFLICT(key) DO UPDATE SET window_start = excluded.window_start, count = excluded.count`,
			key, windowStart.UTC().Format(sqliteTime), count,
		)
		if err != nil {
			return fmt.Errorf("upsert rate counter: %w", err)
		}
		return nil
	})
	if err != nil {
		return false, 0, err
	}
	return allowed, retryAfter, nil
}
