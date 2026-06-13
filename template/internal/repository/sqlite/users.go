// SQLite-репозиторий пользователей. Записи — через db.Write, чтения — через db.Read.
// Конфликт уникальности email мапится в domain.ErrEmailTaken.
package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/chudno/zerovibe/internal/domain"
)

// UserRepo — SQLite-репозиторий пользователей.
type UserRepo struct {
	db writer
}

// NewUserRepo собирает репозиторий пользователей поверх платформенного db.
func NewUserRepo(db writer) *UserRepo {
	return &UserRepo{db: db}
}

// Create вставляет пользователя; дубль email → domain.ErrEmailTaken.
func (r *UserRepo) Create(ctx context.Context, u domain.User) (domain.User, error) {
	err := r.db.Write(ctx, func(s *sql.DB) error {
		var created string
		err := s.QueryRowContext(ctx,
			`INSERT INTO users (email, password_hash, role) VALUES (?, ?, ?)
			 RETURNING id, created_at`,
			u.Email, u.PasswordHash, string(u.Role),
		).Scan(&u.ID, &created)
		if err != nil {
			if isUniqueViolation(err) {
				return domain.ErrEmailTaken
			}
			return fmt.Errorf("insert user: %w", err)
		}
		u.CreatedAt = parseTime(created)
		return nil
	})
	if err != nil {
		return domain.User{}, err
	}
	return u, nil
}

// ByEmail находит пользователя по email; ErrNotFound если нет.
func (r *UserRepo) ByEmail(ctx context.Context, email string) (domain.User, error) {
	return r.scanOne(ctx, `SELECT id, email, password_hash, role, email_verified_at, created_at FROM users WHERE email = ?`, email)
}

// ByID находит пользователя по id; ErrNotFound если нет.
func (r *UserRepo) ByID(ctx context.Context, id int64) (domain.User, error) {
	return r.scanOne(ctx, `SELECT id, email, password_hash, role, email_verified_at, created_at FROM users WHERE id = ?`, id)
}

// scanOne читает одного пользователя по запросу с одним аргументом.
func (r *UserRepo) scanOne(ctx context.Context, query string, arg any) (domain.User, error) {
	var u domain.User
	err := r.db.Read(func(s *sql.DB) error {
		var role, verified, created string
		err := s.QueryRowContext(ctx, query, arg).
			Scan(&u.ID, &u.Email, &u.PasswordHash, &role, &verified, &created)
		if errors.Is(err, sql.ErrNoRows) {
			return domain.ErrNotFound{Entity: "user"}
		}
		if err != nil {
			return fmt.Errorf("select user: %w", err)
		}
		u.Role = domain.Role(role)
		if verified != "" {
			u.EmailVerifiedAt = parseTime(verified)
		}
		u.CreatedAt = parseTime(created)
		return nil
	})
	if err != nil {
		return domain.User{}, err
	}
	return u, nil
}

// UpdatePasswordHash меняет хеш пароля; ErrNotFound если пользователя нет.
func (r *UserRepo) UpdatePasswordHash(ctx context.Context, userID int64, hash string) error {
	return r.db.Write(ctx, func(s *sql.DB) error {
		res, err := s.ExecContext(ctx, `UPDATE users SET password_hash = ? WHERE id = ?`, hash, userID)
		if err != nil {
			return fmt.Errorf("update password: %w", err)
		}
		n, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if n == 0 {
			return domain.ErrNotFound{Entity: "user", ID: userID}
		}
		return nil
	})
}

// MarkEmailVerified проставляет время подтверждения почты; ErrNotFound если нет.
func (r *UserRepo) MarkEmailVerified(ctx context.Context, userID int64, at time.Time) error {
	return r.db.Write(ctx, func(s *sql.DB) error {
		res, err := s.ExecContext(ctx,
			`UPDATE users SET email_verified_at = ? WHERE id = ?`,
			at.UTC().Format(sqliteTime), userID)
		if err != nil {
			return fmt.Errorf("mark email verified: %w", err)
		}
		n, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if n == 0 {
			return domain.ErrNotFound{Entity: "user", ID: userID}
		}
		return nil
	})
}

// CountAdmins возвращает число пользователей с ролью admin (для сида первого админа).
func (r *UserRepo) CountAdmins(ctx context.Context) (int, error) {
	var n int
	err := r.db.Read(func(s *sql.DB) error {
		return s.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE role = ?`, string(domain.RoleAdmin)).Scan(&n)
	})
	return n, err
}

// isUniqueViolation распознаёт нарушение UNIQUE-ограничения SQLite (modernc отдаёт
// сообщение вида "UNIQUE constraint failed: users.email").
func isUniqueViolation(err error) bool {
	return strings.Contains(strings.ToLower(err.Error()), "unique constraint failed")
}
