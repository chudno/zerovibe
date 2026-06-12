// SQLite-репозиторий сессий. Время хранится в UTC в формате datetime('now'),
// чтобы запись/чтение/сравнение были консистентны.
package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/chudno/zerovibe/internal/domain"
)

// SessionRepo — SQLite-репозиторий сессий.
type SessionRepo struct {
	db writer
}

// NewSessionRepo собирает репозиторий сессий поверх платформенного db.
func NewSessionRepo(db writer) *SessionRepo {
	return &SessionRepo{db: db}
}

// Create сохраняет сессию.
func (r *SessionRepo) Create(ctx context.Context, sess domain.Session) error {
	return r.db.Write(ctx, func(s *sql.DB) error {
		_, err := s.ExecContext(ctx,
			`INSERT INTO sessions (token, user_id, created_at, expires_at) VALUES (?, ?, ?, ?)`,
			sess.Token, sess.UserID,
			sess.CreatedAt.UTC().Format(sqliteTime), sess.ExpiresAt.UTC().Format(sqliteTime),
		)
		if err != nil {
			return fmt.Errorf("insert session: %w", err)
		}
		return nil
	})
}

// ByToken находит сессию по токену; ErrNotFound если нет.
func (r *SessionRepo) ByToken(ctx context.Context, token string) (domain.Session, error) {
	var sess domain.Session
	err := r.db.Read(func(s *sql.DB) error {
		var created, expires string
		err := s.QueryRowContext(ctx,
			`SELECT token, user_id, created_at, expires_at FROM sessions WHERE token = ?`, token,
		).Scan(&sess.Token, &sess.UserID, &created, &expires)
		if errors.Is(err, sql.ErrNoRows) {
			return domain.ErrNotFound{Entity: "session"}
		}
		if err != nil {
			return fmt.Errorf("select session: %w", err)
		}
		sess.CreatedAt = parseTime(created)
		sess.ExpiresAt = parseTime(expires)
		return nil
	})
	if err != nil {
		return domain.Session{}, err
	}
	return sess, nil
}

// Delete удаляет сессию по токену (идемпотентно на уровне SQL — отсутствие не ошибка).
func (r *SessionRepo) Delete(ctx context.Context, token string) error {
	return r.db.Write(ctx, func(s *sql.DB) error {
		_, err := s.ExecContext(ctx, `DELETE FROM sessions WHERE token = ?`, token)
		if err != nil {
			return fmt.Errorf("delete session: %w", err)
		}
		return nil
	})
}

// DeleteByUser удаляет все сессии пользователя (бан/смена пароля/«выйти везде»).
func (r *SessionRepo) DeleteByUser(ctx context.Context, userID int64) error {
	return r.db.Write(ctx, func(s *sql.DB) error {
		_, err := s.ExecContext(ctx, `DELETE FROM sessions WHERE user_id = ?`, userID)
		if err != nil {
			return fmt.Errorf("delete sessions by user: %w", err)
		}
		return nil
	})
}

// DeleteExpired удаляет истёкшие сессии (фоновый GC).
func (r *SessionRepo) DeleteExpired(ctx context.Context, now time.Time) error {
	return r.db.Write(ctx, func(s *sql.DB) error {
		_, err := s.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at < ?`, now.UTC().Format(sqliteTime))
		if err != nil {
			return fmt.Errorf("delete expired sessions: %w", err)
		}
		return nil
	})
}
