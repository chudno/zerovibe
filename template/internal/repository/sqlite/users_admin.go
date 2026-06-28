package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/chudno/zerovibe/internal/domain"
)

// Admin-методы UserRepo: для встроенной админки (управление пользователями).
// ОСТОРОЖНО с аутентификацией: пароль обновляется ОТДЕЛЬНЫМ методом (а не через
// общий UPDATE), чтобы случайно не затереть хэш; удаление защищает последнего админа.

// ListAll возвращает всех пользователей (новые сверху) — для списка в админке.
func (r *UserRepo) ListAll(ctx context.Context) ([]domain.User, error) {
	var users []domain.User
	err := r.db.Read(func(s *sql.DB) error {
		rows, err := s.QueryContext(ctx,
			`SELECT id, email, password_hash, role, email_verified_at, created_at FROM users ORDER BY id DESC`)
		if err != nil {
			return fmt.Errorf("select all users: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			u, err := scanUserRow(rows)
			if err != nil {
				return err
			}
			users = append(users, u)
		}
		return rows.Err()
	})
	return users, err
}

// UpdateRoleAndEmail обновляет email, роль и статус подтверждения почты (НЕ трогает
// пароль). verified=true → проставляет email_verified_at (сохраняя уже имеющуюся дату,
// если почта была подтверждена раньше); verified=false → снимает подтверждение.
func (r *UserRepo) UpdateRoleAndEmail(ctx context.Context, id int64, email string, role domain.Role, verified bool) error {
	return r.db.Write(ctx, func(s *sql.DB) error {
		now := time.Now().UTC().Format(sqliteTime)
		// CASE сохраняет исходную дату подтверждения, если она уже была (не перетираем
		// её текущим временем при простом сохранении формы).
		res, err := s.ExecContext(ctx,
			`UPDATE users SET email = ?, role = ?,
			   email_verified_at = CASE
			     WHEN ? = 0 THEN ''
			     WHEN email_verified_at = '' THEN ?
			     ELSE email_verified_at
			   END
			 WHERE id = ?`,
			email, string(role), boolToInt(verified), now, id)
		if err != nil {
			if isUniqueViolation(err) {
				return domain.ErrEmailTaken
			}
			return fmt.Errorf("update user: %w", err)
		}
		if affected, _ := res.RowsAffected(); affected == 0 {
			return domain.ErrNotFound{Entity: "user", ID: id}
		}
		return nil
	})
}

// boolToInt — 1/0 для SQL-CASE (SQLite не имеет булева типа в параметрах).
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// DeleteUser удаляет пользователя по id, НЕ позволяя удалить последнего администратора
// (иначе в приложение будет не войти под админом). Проверка и удаление — в одной
// записи (writer-горутина сериализована, гонки нет).
func (r *UserRepo) DeleteUser(ctx context.Context, id int64) error {
	return r.db.Write(ctx, func(s *sql.DB) error {
		var role string
		if err := s.QueryRowContext(ctx, `SELECT role FROM users WHERE id = ?`, id).Scan(&role); err != nil {
			if err == sql.ErrNoRows {
				return domain.ErrNotFound{Entity: "user", ID: id}
			}
			return fmt.Errorf("get user role: %w", err)
		}
		if domain.Role(role) == domain.RoleAdmin {
			var admins int
			if err := s.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE role = 'admin'`).Scan(&admins); err != nil {
				return fmt.Errorf("count admins: %w", err)
			}
			if admins <= 1 {
				return domain.ErrValidation{Field: "role", Msg: "нельзя удалить последнего администратора"}
			}
		}
		if _, err := s.ExecContext(ctx, `DELETE FROM users WHERE id = ?`, id); err != nil {
			return fmt.Errorf("delete user: %w", err)
		}
		return nil
	})
}

// scanUserRow читает пользователя из *sql.Rows (порядок колонок как в ListAll).
func scanUserRow(rows *sql.Rows) (domain.User, error) {
	var u domain.User
	var role string
	var verified sql.NullString
	var created string
	if err := rows.Scan(&u.ID, &u.Email, &u.PasswordHash, &role, &verified, &created); err != nil {
		return domain.User{}, fmt.Errorf("scan user: %w", err)
	}
	u.Role = domain.Role(role)
	if verified.Valid {
		u.EmailVerifiedAt = parseTime(verified.String)
	}
	u.CreatedAt = parseTime(created)
	return u, nil
}
