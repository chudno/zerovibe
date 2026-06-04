// Package usecase содержит бизнес-логику (оркестрацию) и порты — интерфейсы
// репозиториев. Зависит ТОЛЬКО от domain. Конкретные реализации хранилища
// (sqlite) внедряются снаружи и реализуют эти интерфейсы — инверсия зависимостей.
//
// ОБРАЗЕЦ ДЛЯ ГЕНЕРАЦИИ: на каждую сущность — порт (NoteRepository) + сервис
// (NoteService) с методами-операциями. Сервис валидирует через domain-конструкторы
// и делегирует хранение порту. Здесь нет ни SQL, ни HTTP.
package usecase

import (
	"context"

	"github.com/chudno/zerovibe/internal/domain"
)

// NoteRepository — порт хранилища заметок. Реализуется в repository/sqlite.
type NoteRepository interface {
	Create(ctx context.Context, n domain.Note) (domain.Note, error)
	List(ctx context.Context) ([]domain.Note, error)
	Delete(ctx context.Context, id int64) error
}

// NoteService — бизнес-операции над заметками.
type NoteService struct {
	repo NoteRepository
}

// NewNoteService собирает сервис с внедрённым репозиторием.
func NewNoteService(repo NoteRepository) *NoteService {
	return &NoteService{repo: repo}
}

// Create валидирует ввод через доменный конструктор и сохраняет заметку.
func (s *NoteService) Create(ctx context.Context, title, body string) (domain.Note, error) {
	n, err := domain.NewNote(title, body)
	if err != nil {
		return domain.Note{}, err
	}
	return s.repo.Create(ctx, n)
}

// List возвращает все заметки (новые сверху — порядок задаёт репозиторий).
func (s *NoteService) List(ctx context.Context) ([]domain.Note, error) {
	return s.repo.List(ctx)
}

// Delete удаляет заметку по id.
func (s *NoteService) Delete(ctx context.Context, id int64) error {
	return s.repo.Delete(ctx, id)
}
