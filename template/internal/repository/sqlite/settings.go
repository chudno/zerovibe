// SQLite-репозиторий настроек приложения. Запись (Set) — через очередь записи
// (db.Write), чтения (Get/List) — через db.Read. Схема таблицы settings заводится
// goose-миграцией, здесь только доступ.
package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/chudno/zerovibe/internal/domain"
)

// SettingRepo — SQLite-репозиторий настроек.
type SettingRepo struct {
	db writer
}

// NewSettingRepo собирает репозиторий настроек поверх платформенного db.
func NewSettingRepo(db writer) *SettingRepo {
	return &SettingRepo{db: db}
}

// Get возвращает настройку по ключу; ErrNotFound, если не задана.
func (r *SettingRepo) Get(ctx context.Context, key string) (domain.Setting, error) {
	var st domain.Setting
	err := r.db.Read(func(s *sql.DB) error {
		var updated string
		err := s.QueryRowContext(ctx,
			`SELECT key, value, updated_at FROM settings WHERE key = ?`, key,
		).Scan(&st.Key, &st.Value, &updated)
		if errors.Is(err, sql.ErrNoRows) {
			return domain.ErrNotFound{Entity: "setting"}
		}
		if err != nil {
			return fmt.Errorf("select setting: %w", err)
		}
		st.UpdatedAt = parseTime(updated)
		return nil
	})
	if err != nil {
		return domain.Setting{}, err
	}
	return st, nil
}

// Set вставляет или обновляет настройку (UPSERT по ключу).
func (r *SettingRepo) Set(ctx context.Context, st domain.Setting) error {
	return r.db.Write(ctx, func(s *sql.DB) error {
		_, err := s.ExecContext(ctx,
			`INSERT INTO settings (key, value, updated_at) VALUES (?, ?, ?)
			 ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
			st.Key, st.Value, st.UpdatedAt.UTC().Format(sqliteTime),
		)
		if err != nil {
			return fmt.Errorf("upsert setting: %w", err)
		}
		return nil
	})
}

// List возвращает все заданные настройки.
func (r *SettingRepo) List(ctx context.Context) ([]domain.Setting, error) {
	var out []domain.Setting
	err := r.db.Read(func(s *sql.DB) error {
		rows, err := s.QueryContext(ctx, `SELECT key, value, updated_at FROM settings`)
		if err != nil {
			return fmt.Errorf("select settings: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var st domain.Setting
			var updated string
			if err := rows.Scan(&st.Key, &st.Value, &updated); err != nil {
				return fmt.Errorf("scan setting: %w", err)
			}
			st.UpdatedAt = parseTime(updated)
			out = append(out, st)
		}
		return rows.Err()
	})
	return out, err
}
