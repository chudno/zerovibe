package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/chudno/zerovibe/internal/domain"
)

// Admin-методы NoteRepo: доступ ко ВСЕМ заметкам без фильтра по владельцу. Их
// использует встроенная админка (internal/admin) — администратор видит и правит
// данные всех пользователей. Владельческие методы (ListByOwner/Delete) остаются
// для обычного пользовательского потока. ОБРАЗЕЦ: новая сущность под админку
// получает такой же набор ListAll/GetByID/UpdateAny/DeleteAny.

// ListAll возвращает все заметки (новые сверху) — для списка в админке.
func (r *NoteRepo) ListAll(ctx context.Context) ([]domain.Note, error) {
	var notes []domain.Note
	err := r.db.Read(func(s *sql.DB) error {
		rows, err := s.QueryContext(ctx,
			`SELECT id, owner_id, title, body, due_date, created_at FROM notes ORDER BY id DESC`)
		if err != nil {
			return fmt.Errorf("select all notes: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var n domain.Note
			var created string
			if err := rows.Scan(&n.ID, &n.OwnerID, &n.Title, &n.Body, &n.DueDate, &created); err != nil {
				return fmt.Errorf("scan note: %w", err)
			}
			n.CreatedAt = parseTime(created)
			notes = append(notes, n)
		}
		return rows.Err()
	})
	return notes, err
}

// GetByID возвращает заметку по id (для формы редактирования в админке).
func (r *NoteRepo) GetByID(ctx context.Context, id int64) (domain.Note, error) {
	var n domain.Note
	err := r.db.Read(func(s *sql.DB) error {
		var created string
		err := s.QueryRowContext(ctx,
			`SELECT id, owner_id, title, body, due_date, created_at FROM notes WHERE id = ?`, id).
			Scan(&n.ID, &n.OwnerID, &n.Title, &n.Body, &n.DueDate, &created)
		if errors.Is(err, sql.ErrNoRows) {
			return domain.ErrNotFound{Entity: "note", ID: id}
		}
		if err != nil {
			return fmt.Errorf("get note: %w", err)
		}
		n.CreatedAt = parseTime(created)
		return nil
	})
	return n, err
}

// UpdateAny обновляет заголовок/текст заметки по id (админ правит любую).
func (r *NoteRepo) UpdateAny(ctx context.Context, n domain.Note) error {
	return r.db.Write(ctx, func(s *sql.DB) error {
		res, err := s.ExecContext(ctx,
			`UPDATE notes SET title = ?, body = ?, due_date = ? WHERE id = ?`, n.Title, n.Body, n.DueDate, n.ID)
		if err != nil {
			return fmt.Errorf("update note: %w", err)
		}
		if affected, _ := res.RowsAffected(); affected == 0 {
			return domain.ErrNotFound{Entity: "note", ID: n.ID}
		}
		return nil
	})
}

// DeleteAny удаляет заметку по id без проверки владельца (админ удаляет любую).
func (r *NoteRepo) DeleteAny(ctx context.Context, id int64) error {
	return r.db.Write(ctx, func(s *sql.DB) error {
		res, err := s.ExecContext(ctx, `DELETE FROM notes WHERE id = ?`, id)
		if err != nil {
			return fmt.Errorf("delete note: %w", err)
		}
		if affected, _ := res.RowsAffected(); affected == 0 {
			return domain.ErrNotFound{Entity: "note", ID: id}
		}
		return nil
	})
}
