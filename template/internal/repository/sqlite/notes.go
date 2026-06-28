// Package sqlite — реализация портов usecase поверх SQLite.
//
// КЛЮЧЕВОЙ ПАТТЕРН: записи (Create/Delete) идут через db.Write — попадают в
// единую writer-горутину и сериализуются (нет SQLITE_BUSY). Чтения (List) идут
// через db.Read — параллельно. Конвертация domain↔строка БД — в этом слое.
//
// ОБРАЗЕЦ ДЛЯ ГЕНЕРАЦИИ: на каждую сущность — свой репозиторий, реализующий
// порт из usecase. INSERT/UPDATE/DELETE оборачивать в db.Write, SELECT — в db.Read.
package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/chudno/zerovibe/internal/domain"
)

// writer — минимальный интерфейс к платформенному db.DB (Write/Read).
// Принимаем интерфейсом, а не *db.DB, чтобы репозиторий было легко тестировать.
type writer interface {
	Write(ctx context.Context, fn func(*sql.DB) error) error
	Read(fn func(*sql.DB) error) error
}

// NoteRepo — SQLite-репозиторий заметок.
type NoteRepo struct {
	db writer
}

// NewNoteRepo собирает репозиторий поверх платформенного db.
func NewNoteRepo(db writer) *NoteRepo {
	return &NoteRepo{db: db}
}

// Create вставляет заметку владельца (через очередь записи) и возвращает её с id и
// проставленным базой created_at — чтобы фрагмент сразу показывал время.
func (r *NoteRepo) Create(ctx context.Context, n domain.Note) (domain.Note, error) {
	err := r.db.Write(ctx, func(s *sql.DB) error {
		var created string
		// RETURNING поддерживается SQLite ≥3.35 (есть в modernc) — отдаёт id и
		// время одним запросом, без отдельного SELECT.
		err := s.QueryRowContext(ctx,
			`INSERT INTO notes (owner_id, title, body, due_date) VALUES (?, ?, ?, ?) RETURNING id, created_at`,
			n.OwnerID, n.Title, n.Body, n.DueDate,
		).Scan(&n.ID, &created)
		if err != nil {
			return fmt.Errorf("insert note: %w", err)
		}
		n.CreatedAt = parseTime(created)
		return nil
	})
	if err != nil {
		return domain.Note{}, err
	}
	return n, nil
}

// ListByOwner возвращает заметки владельца, новые сверху.
func (r *NoteRepo) ListByOwner(ctx context.Context, ownerID int64) ([]domain.Note, error) {
	var notes []domain.Note
	err := r.db.Read(func(s *sql.DB) error {
		rows, err := s.QueryContext(ctx,
			`SELECT id, owner_id, title, body, due_date, created_at FROM notes WHERE owner_id = ? ORDER BY id DESC`, ownerID)
		if err != nil {
			return fmt.Errorf("select notes: %w", err)
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

// Delete удаляет заметку владельца по id (чужую не трогает — отдаёт ErrNotFound,
// скрывая существование чужих заметок).
func (r *NoteRepo) Delete(ctx context.Context, id, ownerID int64) error {
	return r.db.Write(ctx, func(s *sql.DB) error {
		res, err := s.ExecContext(ctx, `DELETE FROM notes WHERE id = ? AND owner_id = ?`, id, ownerID)
		if err != nil {
			return fmt.Errorf("delete note: %w", err)
		}
		affected, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if affected == 0 {
			return domain.ErrNotFound{Entity: "note", ID: id}
		}
		return nil
	})
}
