// SQLite-репозиторий токенов подтверждения почты. used_at=” = не использован.
package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/chudno/zerovibe/internal/domain"
)

// EmailVerificationRepo — SQLite-репозиторий токенов подтверждения почты.
type EmailVerificationRepo struct {
	db writer
}

// NewEmailVerificationRepo собирает репозиторий поверх платформенного db.
func NewEmailVerificationRepo(db writer) *EmailVerificationRepo {
	return &EmailVerificationRepo{db: db}
}

// Create сохраняет токен подтверждения.
func (r *EmailVerificationRepo) Create(ctx context.Context, v domain.EmailVerification) error {
	return r.db.Write(ctx, func(s *sql.DB) error {
		_, err := s.ExecContext(ctx,
			`INSERT INTO email_verifications (token, user_id, created_at, expires_at) VALUES (?, ?, ?, ?)`,
			v.Token, v.UserID, v.CreatedAt.UTC().Format(sqliteTime), v.ExpiresAt.UTC().Format(sqliteTime),
		)
		if err != nil {
			return fmt.Errorf("insert email verification: %w", err)
		}
		return nil
	})
}

// ByToken находит токен подтверждения; ErrNotFound если нет.
func (r *EmailVerificationRepo) ByToken(ctx context.Context, token string) (domain.EmailVerification, error) {
	var v domain.EmailVerification
	err := r.db.Read(func(s *sql.DB) error {
		var created, expires, used string
		err := s.QueryRowContext(ctx,
			`SELECT token, user_id, created_at, expires_at, used_at FROM email_verifications WHERE token = ?`, token,
		).Scan(&v.Token, &v.UserID, &created, &expires, &used)
		if errors.Is(err, sql.ErrNoRows) {
			return domain.ErrNotFound{Entity: "email verification"}
		}
		if err != nil {
			return fmt.Errorf("select email verification: %w", err)
		}
		v.CreatedAt = parseTime(created)
		v.ExpiresAt = parseTime(expires)
		if used != "" {
			v.UsedAt = parseTime(used)
		}
		return nil
	})
	if err != nil {
		return domain.EmailVerification{}, err
	}
	return v, nil
}

// MarkUsed помечает токен использованным.
func (r *EmailVerificationRepo) MarkUsed(ctx context.Context, token string, usedAt time.Time) error {
	return r.db.Write(ctx, func(s *sql.DB) error {
		_, err := s.ExecContext(ctx,
			`UPDATE email_verifications SET used_at = ? WHERE token = ?`,
			usedAt.UTC().Format(sqliteTime), token,
		)
		if err != nil {
			return fmt.Errorf("mark email verification used: %w", err)
		}
		return nil
	})
}

// DeleteByUser удаляет все токены подтверждения пользователя (выдача нового
// инвалидирует старые).
func (r *EmailVerificationRepo) DeleteByUser(ctx context.Context, userID int64) error {
	return r.db.Write(ctx, func(s *sql.DB) error {
		_, err := s.ExecContext(ctx, `DELETE FROM email_verifications WHERE user_id = ?`, userID)
		if err != nil {
			return fmt.Errorf("delete email verifications by user: %w", err)
		}
		return nil
	})
}
