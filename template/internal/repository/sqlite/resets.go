// SQLite-репозиторий токенов сброса пароля. used_at=” означает «не использован».
package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/chudno/zerovibe/internal/domain"
)

// ResetRepo — SQLite-репозиторий токенов сброса.
type ResetRepo struct {
	db writer
}

// NewResetRepo собирает репозиторий токенов сброса поверх платформенного db.
func NewResetRepo(db writer) *ResetRepo {
	return &ResetRepo{db: db}
}

// Create сохраняет токен сброса.
func (r *ResetRepo) Create(ctx context.Context, p domain.PasswordReset) error {
	return r.db.Write(ctx, func(s *sql.DB) error {
		_, err := s.ExecContext(ctx,
			`INSERT INTO password_resets (token, user_id, created_at, expires_at) VALUES (?, ?, ?, ?)`,
			p.Token, p.UserID, p.CreatedAt.UTC().Format(sqliteTime), p.ExpiresAt.UTC().Format(sqliteTime),
		)
		if err != nil {
			return fmt.Errorf("insert reset: %w", err)
		}
		return nil
	})
}

// ByToken находит токен сброса; ErrNotFound если нет.
func (r *ResetRepo) ByToken(ctx context.Context, token string) (domain.PasswordReset, error) {
	var p domain.PasswordReset
	err := r.db.Read(func(s *sql.DB) error {
		var created, expires, used string
		err := s.QueryRowContext(ctx,
			`SELECT token, user_id, created_at, expires_at, used_at FROM password_resets WHERE token = ?`, token,
		).Scan(&p.Token, &p.UserID, &created, &expires, &used)
		if errors.Is(err, sql.ErrNoRows) {
			return domain.ErrNotFound{Entity: "reset"}
		}
		if err != nil {
			return fmt.Errorf("select reset: %w", err)
		}
		p.CreatedAt = parseTime(created)
		p.ExpiresAt = parseTime(expires)
		if used != "" {
			p.UsedAt = parseTime(used)
		}
		return nil
	})
	if err != nil {
		return domain.PasswordReset{}, err
	}
	return p, nil
}

// MarkUsed помечает токен использованным.
func (r *ResetRepo) MarkUsed(ctx context.Context, token string, usedAt time.Time) error {
	return r.db.Write(ctx, func(s *sql.DB) error {
		_, err := s.ExecContext(ctx,
			`UPDATE password_resets SET used_at = ? WHERE token = ?`,
			usedAt.UTC().Format(sqliteTime), token,
		)
		if err != nil {
			return fmt.Errorf("mark reset used: %w", err)
		}
		return nil
	})
}

// DeleteByUser удаляет все токены сброса пользователя (выдача нового инвалидирует старые).
func (r *ResetRepo) DeleteByUser(ctx context.Context, userID int64) error {
	return r.db.Write(ctx, func(s *sql.DB) error {
		_, err := s.ExecContext(ctx, `DELETE FROM password_resets WHERE user_id = ?`, userID)
		if err != nil {
			return fmt.Errorf("delete resets by user: %w", err)
		}
		return nil
	})
}
