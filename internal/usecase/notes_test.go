// Unit-тест бизнес-логики на фейковом репозитории (без БД и сети).
// ОБРАЗЕЦ ПОКРЫТИЯ: каждый usecase покрывается так — фейк-реализация порта +
// проверка оркестрации и валидации. Быстро, детерминированно, без внешних
// зависимостей. Это обязательный уровень покрытия для каждой сущности.
package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/chudno/zerovibe/internal/domain"
)

// fakeNoteRepo — in-memory реализация порта NoteRepository для тестов.
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

func (f *fakeNoteRepo) List(_ context.Context) ([]domain.Note, error) {
	return f.items, nil
}

func (f *fakeNoteRepo) Delete(_ context.Context, id int64) error {
	for i, n := range f.items {
		if n.ID == id {
			f.items = append(f.items[:i], f.items[i+1:]...)
			return nil
		}
	}
	return domain.ErrNotFound{Entity: "note", ID: id}
}

func TestNoteService_Create_OK(t *testing.T) {
	svc := NewNoteService(&fakeNoteRepo{})
	n, err := svc.Create(context.Background(), "  Привет  ", "тело")
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if n.ID == 0 {
		t.Error("ожидался присвоенный id")
	}
	if n.Title != "Привет" {
		t.Errorf("заголовок должен быть усечён по пробелам, получено %q", n.Title)
	}
}

func TestNoteService_Create_EmptyTitle(t *testing.T) {
	svc := NewNoteService(&fakeNoteRepo{})
	_, err := svc.Create(context.Background(), "   ", "тело")
	var ve domain.ErrValidation
	if !errors.As(err, &ve) {
		t.Fatalf("ожидалась ErrValidation, получено %v", err)
	}
	if ve.Field != "title" {
		t.Errorf("ожидалось поле title, получено %q", ve.Field)
	}
}

func TestNoteService_List_NewestFirst(t *testing.T) {
	svc := NewNoteService(&fakeNoteRepo{})
	ctx := context.Background()
	_, _ = svc.Create(ctx, "первая", "")
	_, _ = svc.Create(ctx, "вторая", "")

	got, err := svc.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("ожидалось 2 заметки, получено %d", len(got))
	}
	if got[0].Title != "вторая" {
		t.Errorf("новые должны быть сверху, первый элемент %q", got[0].Title)
	}
}

func TestNoteService_Delete_NotFound(t *testing.T) {
	svc := NewNoteService(&fakeNoteRepo{})
	err := svc.Delete(context.Background(), 999)
	var nf domain.ErrNotFound
	if !errors.As(err, &nf) {
		t.Fatalf("ожидалась ErrNotFound, получено %v", err)
	}
}
