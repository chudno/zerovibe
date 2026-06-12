// Unit-тест бизнес-логики заметок на фейковом репозитории (без БД и сети).
// Заметки личные: проверяем валидацию, владельца и изоляцию между пользователями.
package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/chudno/zerovibe/internal/domain"
)

// fakeNoteRepo — in-memory реализация порта NoteRepository (owner-scoped).
type fakeNoteRepo struct {
	items  []domain.Note
	nextID int64
}

func (f *fakeNoteRepo) Create(_ context.Context, n domain.Note) (domain.Note, error) {
	f.nextID++
	n.ID = f.nextID
	f.items = append([]domain.Note{n}, f.items...) // новые сверху
	return n, nil
}

func (f *fakeNoteRepo) ListByOwner(_ context.Context, ownerID int64) ([]domain.Note, error) {
	var out []domain.Note
	for _, n := range f.items {
		if n.OwnerID == ownerID {
			out = append(out, n)
		}
	}
	return out, nil
}

func (f *fakeNoteRepo) Delete(_ context.Context, id, ownerID int64) error {
	for i, n := range f.items {
		if n.ID == id && n.OwnerID == ownerID {
			f.items = append(f.items[:i], f.items[i+1:]...)
			return nil
		}
	}
	return domain.ErrNotFound{Entity: "note", ID: id}
}

func TestNoteService_Create_OK(t *testing.T) {
	svc := NewNoteService(&fakeNoteRepo{})
	n, err := svc.Create(context.Background(), 7, "  Привет  ", "тело")
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if n.ID == 0 {
		t.Error("ожидался присвоенный id")
	}
	if n.OwnerID != 7 {
		t.Errorf("ожидался владелец 7, получено %d", n.OwnerID)
	}
	if n.Title != "Привет" {
		t.Errorf("заголовок должен быть усечён по пробелам, получено %q", n.Title)
	}
}

func TestNoteService_Create_EmptyTitle(t *testing.T) {
	svc := NewNoteService(&fakeNoteRepo{})
	_, err := svc.Create(context.Background(), 1, "   ", "тело")
	var ve domain.ErrValidation
	if !errors.As(err, &ve) {
		t.Fatalf("ожидалась ErrValidation, получено %v", err)
	}
	if ve.Field != "title" {
		t.Errorf("ожидалось поле title, получено %q", ve.Field)
	}
}

func TestNoteService_List_OnlyOwner(t *testing.T) {
	svc := NewNoteService(&fakeNoteRepo{})
	ctx := context.Background()
	_, _ = svc.Create(ctx, 1, "моя", "")
	_, _ = svc.Create(ctx, 2, "чужая", "")

	got, err := svc.List(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("ожидалась 1 заметка владельца 1, получено %d", len(got))
	}
	if got[0].Title != "моя" {
		t.Errorf("должна вернуться только своя заметка, получено %q", got[0].Title)
	}
}

func TestNoteService_Delete_OtherOwner_NotFound(t *testing.T) {
	svc := NewNoteService(&fakeNoteRepo{})
	ctx := context.Background()
	n, _ := svc.Create(ctx, 2, "чужая", "")

	// пользователь 1 пытается удалить заметку пользователя 2
	err := svc.Delete(ctx, n.ID, 1)
	var nf domain.ErrNotFound
	if !errors.As(err, &nf) {
		t.Fatalf("ожидалась ErrNotFound при удалении чужой заметки, получено %v", err)
	}
}

func TestNoteService_Delete_NotFound(t *testing.T) {
	svc := NewNoteService(&fakeNoteRepo{})
	err := svc.Delete(context.Background(), 999, 1)
	var nf domain.ErrNotFound
	if !errors.As(err, &nf) {
		t.Fatalf("ожидалась ErrNotFound, получено %v", err)
	}
}
