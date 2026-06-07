// E2E-тест транспорта на полном стеке (реальный SQLite во временном файле).
// ОБРАЗЕЦ ПОКРЫТИЯ: проверяем HTTP+HTML+HTMX поведение целиком —
// создание возвращает HTML-фрагмент, список отражает данные, удаление убирает.
// Поднимает настоящие слои (db→repo→usecase→web), но БД временная и изолированная.
package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/chudno/zerovibe/internal/platform/db"
	"github.com/chudno/zerovibe/internal/repository/sqlite"
	"github.com/chudno/zerovibe/internal/usecase"
)

// newTestServer собирает полный стек на временной БД (t.TempDir → авто-очистка).
func newTestServer(t *testing.T) http.Handler {
	t.Helper()
	dsn := "file:" + filepath.Join(t.TempDir(), "test.db")
	database, err := db.Open(dsn)
	if err != nil {
		t.Fatalf("открыть БД: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	if err := database.Migrate(context.Background(), sqlite.Schema); err != nil {
		t.Fatalf("миграция: %v", err)
	}
	srv, err := NewServer(usecase.NewNoteService(sqlite.NewNoteRepo(database)))
	if err != nil {
		t.Fatalf("сервер: %v", err)
	}
	return srv.Routes()
}

func TestCreateReturnsFragment(t *testing.T) {
	h := newTestServer(t)

	form := url.Values{"title": {"Купить хлеб"}, "body": {"и молоко"}}
	req := httptest.NewRequest("POST", "/notes", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("ожидался 200, получен %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	// Ответ — ФРАГМЕНТ одной заметки (не полная страница): без <html>, с классом note.
	if strings.Contains(body, "<html") {
		t.Error("ответ POST должен быть фрагментом, а не полной страницей")
	}
	if !strings.Contains(body, "Купить хлеб") || !strings.Contains(body, `class="note"`) {
		t.Errorf("фрагмент заметки не содержит ожидаемого, получено: %s", body)
	}
}

func TestIndexShowsCreatedNote(t *testing.T) {
	h := newTestServer(t)

	form := url.Values{"title": {"Видна в списке"}}
	postReq := httptest.NewRequest("POST", "/notes", strings.NewReader(form.Encode()))
	postReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	h.ServeHTTP(httptest.NewRecorder(), postReq)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("ожидался 200, получен %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "<html") {
		t.Error("GET / должен возвращать полную страницу")
	}
	if !strings.Contains(body, "Видна в списке") {
		t.Error("созданная заметка не отображается в списке")
	}
}

func TestDeleteRemovesNote(t *testing.T) {
	h := newTestServer(t)

	form := url.Values{"title": {"Удалить меня"}}
	postReq := httptest.NewRequest("POST", "/notes", strings.NewReader(form.Encode()))
	postReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	postRec := httptest.NewRecorder()
	h.ServeHTTP(postRec, postReq)
	// id присвоен 1 (первая запись в чистой БД); проверим через список после удаления.

	delReq := httptest.NewRequest("DELETE", "/notes/1", nil)
	delRec := httptest.NewRecorder()
	h.ServeHTTP(delRec, delReq)
	if delRec.Code != http.StatusOK {
		t.Fatalf("ожидался 200 на удаление, получен %d", delRec.Code)
	}

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
	if strings.Contains(rec.Body.String(), "Удалить меня") {
		t.Error("заметка должна быть удалена из списка")
	}
}

func TestDeleteNotFound(t *testing.T) {
	h := newTestServer(t)
	req := httptest.NewRequest("DELETE", "/notes/999", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("ожидался 404 на несуществующую заметку, получен %d", rec.Code)
	}
}
