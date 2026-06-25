// E2E-тест транспорта заметок на полном стеке (реальный SQLite во временном файле).
// Заметки теперь личные — все запросы идут под сессией засиженного пользователя.
package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/chudno/zerovibe/internal/platform/db"
	"github.com/chudno/zerovibe/internal/repository/sqlite"
	"github.com/chudno/zerovibe/internal/usecase"
)

// noMailer — заглушка Mailer для тестов транспорта заметок (письма тут не нужны).
type noMailer struct{}

func (noMailer) Send(_ context.Context, _ usecase.Email) error { return nil }

// capturedToken — токен из последнего письма сброса (для e2e reset-флоу).
var capturedToken string

// captureMailer достаёт токен сброса из ссылки в тексте письма.
type captureMailer struct{}

var resetLinkRe = regexp.MustCompile(`/reset\?token=([0-9a-f]+)`)
var verifyLinkRe = regexp.MustCompile(`/verify-email\?token=([0-9a-f]+)`)

// capturedVerifyToken — токен из последнего письма подтверждения почты.
var capturedVerifyToken string

func (captureMailer) Send(_ context.Context, m usecase.Email) error {
	if mm := resetLinkRe.FindStringSubmatch(m.Text); mm != nil {
		capturedToken = mm[1]
	}
	if mm := verifyLinkRe.FindStringSubmatch(m.Text); mm != nil {
		capturedVerifyToken = mm[1]
	}
	return nil
}

// buildStackWithMailer — как buildStack, но с мейлером-перехватчиком токена сброса.
// testSetupToken — код первичной настройки, заданный в тестовом стеке. Совпадает с
// тем, что в проде задаёт плагин через env SETUP_TOKEN. /setup доступен, только пока
// в системе нет ни одного админа.
const testSetupToken = "test-setup-token"

func buildStackWithMailer(t *testing.T, allowSignup bool) (http.Handler, *usecase.AuthService, *usecase.SettingsService) {
	t.Helper()
	capturedToken = ""
	capturedVerifyToken = ""
	dsn := "file:" + filepath.Join(t.TempDir(), "test.db")
	database, err := db.Open(dsn)
	if err != nil {
		t.Fatalf("открыть БД: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	if err := database.MigrateUp(context.Background()); err != nil {
		t.Fatalf("миграции: %v", err)
	}
	settings := usecase.NewSettingsService(sqlite.NewSettingRepo(database))
	if allowSignup {
		_ = settings.Set(context.Background(), "allow_signup", "true")
	}
	notes := usecase.NewNoteService(sqlite.NewNoteRepo(database))
	auth := usecase.NewAuthService(
		sqlite.NewUserRepo(database), sqlite.NewSessionRepo(database),
		sqlite.NewResetRepo(database), sqlite.NewEmailVerificationRepo(database), sqlite.NewRateLimitRepo(database),
		usecase.NewBcryptHasher(), captureMailer{}, settings,
		usecase.AuthConfig{
			SessionTTL:      time.Hour,
			ResetTTL:        time.Hour,
			VerifyTTL:       24 * time.Hour,
			AppBaseURL:      "http://localhost",
			LoginRateLimit:  usecase.RateRule{Limit: 5, Window: 15 * time.Minute},
			ForgotRateLimit: usecase.RateRule{Limit: 3, Window: time.Hour},
			ResendShortRate: usecase.RateRule{Limit: 1, Window: time.Minute},
			ResendHourRate:  usecase.RateRule{Limit: 5, Window: time.Hour},
			SetupToken:      testSetupToken,
		},
	)
	srv, err := NewServer(notes, auth, settings, Config{SecureCookie: false, CookieName: "zv_session"})
	if err != nil {
		t.Fatalf("сервер: %v", err)
	}
	return srv.Routes(), auth, settings
}

// buildStack собирает полный стек на временной БД. Возвращает handler и сервисы для
// сидирования. allowSignup управляет открытой регистрацией.
func buildStack(t *testing.T, allowSignup bool) (http.Handler, *usecase.AuthService, *usecase.SettingsService) {
	t.Helper()
	dsn := "file:" + filepath.Join(t.TempDir(), "test.db")
	database, err := db.Open(dsn)
	if err != nil {
		t.Fatalf("открыть БД: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	if err := database.MigrateUp(context.Background()); err != nil {
		t.Fatalf("миграции: %v", err)
	}

	settings := usecase.NewSettingsService(sqlite.NewSettingRepo(database))
	if allowSignup {
		if err := settings.Set(context.Background(), "allow_signup", "true"); err != nil {
			t.Fatalf("настройка allow_signup: %v", err)
		}
	}
	notes := usecase.NewNoteService(sqlite.NewNoteRepo(database))
	auth := usecase.NewAuthService(
		sqlite.NewUserRepo(database), sqlite.NewSessionRepo(database),
		sqlite.NewResetRepo(database), sqlite.NewEmailVerificationRepo(database), sqlite.NewRateLimitRepo(database),
		usecase.NewBcryptHasher(), noMailer{}, settings,
		usecase.AuthConfig{
			SessionTTL:      time.Hour,
			ResetTTL:        time.Hour,
			VerifyTTL:       24 * time.Hour,
			AppBaseURL:      "http://localhost",
			LoginRateLimit:  usecase.RateRule{Limit: 5, Window: 15 * time.Minute},
			ForgotRateLimit: usecase.RateRule{Limit: 3, Window: time.Hour},
			ResendShortRate: usecase.RateRule{Limit: 1, Window: time.Minute},
			ResendHourRate:  usecase.RateRule{Limit: 5, Window: time.Hour},
			SetupToken:      testSetupToken,
		},
	)
	srv, err := NewServer(notes, auth, settings, Config{SecureCookie: false, CookieName: "zv_session"})
	if err != nil {
		t.Fatalf("сервер: %v", err)
	}
	return srv.Routes(), auth, settings
}

// seedAdminAndLogin создаёт первого админа (через первичную настройку по тестовому
// коду) и возвращает cookie его сессии. Идёт тем же путём, что и прод — /setup.
func seedAdminAndLogin(t *testing.T, h http.Handler, auth *usecase.AuthService, email, pass string) *http.Cookie {
	t.Helper()
	if err := auth.Setup(context.Background(), email, pass, testSetupToken); err != nil {
		t.Fatalf("сид админа: %v", err)
	}
	return loginCookie(t, h, email, pass)
}

// loginCookie логинится и возвращает cookie сессии.
func loginCookie(t *testing.T, h http.Handler, email, pass string) *http.Cookie {
	t.Helper()
	form := url.Values{"email": {email}, "password": {pass}}
	req := httptest.NewRequest("POST", "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	for _, c := range rec.Result().Cookies() {
		if c.Name == "zv_session" && c.Value != "" {
			return c
		}
	}
	t.Fatalf("логин не вернул cookie сессии (код %d): %s", rec.Code, rec.Body.String())
	return nil
}

var noteIDRe = regexp.MustCompile(`id="note-(\d+)"`)

// newAuthedServer — стек + cookie залогиненного пользователя (через сид-админа).
func newAuthedServer(t *testing.T) (http.Handler, *http.Cookie) {
	h, auth, _ := buildStack(t, false)
	c := seedAdminAndLogin(t, h, auth, "owner@example.com", "password123")
	return h, c
}

func TestCreateReturnsFragment(t *testing.T) {
	h, c := newAuthedServer(t)

	form := url.Values{"title": {"Купить хлеб"}, "body": {"и молоко"}}
	req := httptest.NewRequest("POST", "/notes", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(c)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("ожидался 200, получен %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if strings.Contains(body, "<html") {
		t.Error("ответ POST должен быть фрагментом, а не полной страницей")
	}
	if !strings.Contains(body, "Купить хлеб") || !strings.Contains(body, `id="note-`) {
		t.Errorf("фрагмент заметки не содержит ожидаемого, получено: %s", body)
	}
}

func TestStaticServed(t *testing.T) {
	h, _ := newAuthedServer(t)
	for _, name := range []string{"htmx.min.js", "app.css"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", "/static/"+name, nil))
		if rec.Code != http.StatusOK {
			t.Errorf("GET /static/%s: ожидался 200, получен %d", name, rec.Code)
		}
		if rec.Body.Len() == 0 {
			t.Errorf("GET /static/%s: пустое тело", name)
		}
	}
}

func TestIndexShowsCreatedNote(t *testing.T) {
	h, c := newAuthedServer(t)

	form := url.Values{"title": {"Видна в списке"}}
	postReq := httptest.NewRequest("POST", "/notes", strings.NewReader(form.Encode()))
	postReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	postReq.AddCookie(c)
	h.ServeHTTP(httptest.NewRecorder(), postReq)

	getReq := httptest.NewRequest("GET", "/", nil)
	getReq.AddCookie(c)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, getReq)

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
	h, c := newAuthedServer(t)

	form := url.Values{"title": {"Удалить меня"}}
	postReq := httptest.NewRequest("POST", "/notes", strings.NewReader(form.Encode()))
	postReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	postReq.AddCookie(c)
	postRec := httptest.NewRecorder()
	h.ServeHTTP(postRec, postReq)
	// id заметки берём из фрагмента ответа (устойчиво к нумерации).
	m := noteIDRe.FindStringSubmatch(postRec.Body.String())
	if m == nil {
		t.Fatalf("не нашли id заметки в ответе: %s", postRec.Body.String())
	}
	id := m[1]

	delReq := httptest.NewRequest("DELETE", "/notes/"+id, nil)
	delReq.AddCookie(c)
	delRec := httptest.NewRecorder()
	h.ServeHTTP(delRec, delReq)
	if delRec.Code != http.StatusOK {
		t.Fatalf("ожидался 200 на удаление, получен %d", delRec.Code)
	}

	getReq := httptest.NewRequest("GET", "/", nil)
	getReq.AddCookie(c)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, getReq)
	if strings.Contains(rec.Body.String(), "Удалить меня") {
		t.Error("заметка должна быть удалена из списка")
	}
}

func TestDeleteNotFound(t *testing.T) {
	h, c := newAuthedServer(t)
	req := httptest.NewRequest("DELETE", "/notes/999", nil)
	req.AddCookie(c)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("ожидался 404 на несуществующую заметку, получен %d", rec.Code)
	}
}
