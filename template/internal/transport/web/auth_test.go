// E2E-тесты аутентификации на полном стеке (httptest + временный SQLite).
package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// postForm — POST с form-encoded телом и опциональной cookie.
func postForm(h http.Handler, path string, vals url.Values, c *http.Cookie) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", path, strings.NewReader(vals.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if c != nil {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestLogin_SetsCookie(t *testing.T) {
	h, auth, _ := buildStack(t, false)
	c := seedAdminAndLogin(t, h, auth, "a@b.com", "password123")
	if c.Value == "" {
		t.Fatal("ожидалась непустая cookie сессии")
	}
	if !c.HttpOnly {
		t.Error("cookie сессии должна быть HttpOnly")
	}
}

func TestLogin_WrongPassword_401(t *testing.T) {
	h, auth, _ := buildStack(t, false)
	if err := auth.Setup(context.Background(), "a@b.com", "password123", testSetupToken); err != nil {
		t.Fatal(err)
	}
	rec := postForm(h, "/login", url.Values{"email": {"a@b.com"}, "password": {"wrongpass1"}}, nil)
	// форма перерисовывается с ошибкой; cookie не ставится
	for _, ck := range rec.Result().Cookies() {
		if ck.Name == "zv_session" && ck.Value != "" {
			t.Error("при неверном пароле cookie сессии не должна ставиться")
		}
	}
}

func TestProtectedRedirectsWhenAnonymous(t *testing.T) {
	h, _, _ := buildStack(t, false)
	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("гость на / должен получить редирект 303, получен %d", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "/login" {
		t.Errorf("ожидался редирект на /login, получено %q", loc)
	}
}

func TestProtectedAccessibleWithCookie(t *testing.T) {
	h, auth, _ := buildStack(t, false)
	c := seedAdminAndLogin(t, h, auth, "a@b.com", "password123")
	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(c)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("с cookie / должен открываться, получен %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "<html") {
		t.Error("ожидалась полная страница")
	}
}

func TestLogout_ClearsCookie(t *testing.T) {
	h, auth, _ := buildStack(t, false)
	c := seedAdminAndLogin(t, h, auth, "a@b.com", "password123")
	rec := postForm(h, "/logout", url.Values{}, c)
	var cleared bool
	for _, ck := range rec.Result().Cookies() {
		if ck.Name == "zv_session" && ck.MaxAge < 0 {
			cleared = true
		}
	}
	if !cleared {
		t.Error("logout должен очищать cookie сессии (MaxAge<0)")
	}
}

func TestRegisterClosed_GET_ShowsStub(t *testing.T) {
	h, _, _ := buildStack(t, false)
	req := httptest.NewRequest("GET", "/register", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("ожидался 200, получен %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "закрыта") {
		t.Error("при закрытой регистрации страница должна показывать заглушку")
	}
}

func TestRegisterClosed_POST_Rejected(t *testing.T) {
	h, _, _ := buildStack(t, false)
	rec := postForm(h, "/register", url.Values{"email": {"x@y.com"}, "password": {"password123"}}, nil)
	// failForm перерисовывает форму регистрации; но cookie не ставится и юзер не входит
	for _, ck := range rec.Result().Cookies() {
		if ck.Name == "zv_session" && ck.Value != "" {
			t.Error("при закрытой регистрации сессия не должна создаваться")
		}
	}
}

func TestRegisterOpen_CreatesAndLogsIn(t *testing.T) {
	h, _, _ := buildStack(t, true) // allowSignup=true
	rec := postForm(h, "/register", url.Values{"email": {"new@user.com"}, "password": {"password123"}}, nil)
	var got *http.Cookie
	for _, ck := range rec.Result().Cookies() {
		if ck.Name == "zv_session" && ck.Value != "" {
			got = ck
		}
	}
	if got == nil {
		t.Fatalf("после открытой регистрации ожидался автологин (cookie), код %d, тело: %s", rec.Code, rec.Body.String())
	}
	// и доступ к / есть
	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(got)
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req)
	if rec2.Code != http.StatusOK {
		t.Errorf("новый пользователь должен иметь доступ к /, получен %d", rec2.Code)
	}
}

func TestForgot_UnknownEmail_NeutralNoMail(t *testing.T) {
	h, _, _ := buildStack(t, false)
	rec := postForm(h, "/forgot", url.Values{"email": {"nobody@x.com"}}, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("forgot должен отдавать нейтральный 200, получен %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Если такой email") {
		t.Error("ожидался нейтральный ответ (анти-enumeration)")
	}
}

func TestReset_ChangesPassword(t *testing.T) {
	// Соберём стек с мейлером-перехватчиком, чтобы достать токен из письма.
	h, auth, _ := buildStackWithMailer(t, false)
	ctx := context.Background()
	if err := auth.Setup(ctx, "a@b.com", "password123", testSetupToken); err != nil {
		t.Fatal(err)
	}
	// запросим сброс через usecase напрямую (транспорт forgot не отдаёт токен)
	if err := auth.RequestReset(ctx, "a@b.com", "k"); err != nil {
		t.Fatal(err)
	}
	token := capturedToken
	if token == "" {
		t.Fatal("токен сброса не перехвачен")
	}

	rec := postForm(h, "/reset", url.Values{"token": {token}, "password": {"newpassword1"}}, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("reset должен пройти, получен %d: %s", rec.Code, rec.Body.String())
	}
	// войти новым паролем
	c := loginCookie(t, h, "a@b.com", "newpassword1")
	if c.Value == "" {
		t.Error("вход с новым паролем должен работать")
	}
}

func TestReset_BadToken_400Ish(t *testing.T) {
	h, _, _ := buildStack(t, false)
	rec := postForm(h, "/reset", url.Values{"token": {"garbage"}, "password": {"newpassword1"}}, nil)
	// reset перерисовывает страницу с ошибкой (200 с текстом), не 500
	if rec.Code == http.StatusInternalServerError {
		t.Fatalf("плохой токен не должен давать 500, получен %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "недействительна") {
		t.Error("ожидалось сообщение о недействительной ссылке")
	}
}

func TestLoginRateLimited_429(t *testing.T) {
	h, auth, _ := buildStack(t, false)
	if err := auth.Setup(context.Background(), "a@b.com", "password123", testSetupToken); err != nil {
		t.Fatal(err)
	}
	// лимит логина 5/15мин → 6-я неверная попытка по тому же email → 429
	var last *httptest.ResponseRecorder
	for i := 0; i < 6; i++ {
		last = postForm(h, "/login", url.Values{"email": {"a@b.com"}, "password": {"wrongpass1"}}, nil)
	}
	if last.Header().Get("Retry-After") == "" {
		t.Errorf("после превышения лимита ожидался заголовок Retry-After, тело: %s", last.Body.String())
	}
}

func TestNotesScopedToOwner(t *testing.T) {
	h, auth, settings := buildStack(t, true)
	_ = settings
	// два пользователя: админ (сид) и второй через регистрацию
	cA := seedAdminAndLogin(t, h, auth, "a@owner.com", "password123")
	recB := postForm(h, "/register", url.Values{"email": {"b@owner.com"}, "password": {"password123"}}, nil)
	var cB *http.Cookie
	for _, ck := range recB.Result().Cookies() {
		if ck.Name == "zv_session" && ck.Value != "" {
			cB = ck
		}
	}
	if cB == nil {
		t.Fatal("второй пользователь не залогинен")
	}

	// A создаёт заметку
	postA := postForm(h, "/notes", url.Values{"title": {"Секрет A"}}, cA)
	m := noteIDRe.FindStringSubmatch(postA.Body.String())
	if m == nil {
		t.Fatalf("не нашли id заметки A: %s", postA.Body.String())
	}
	idA := m[1]

	// B не видит заметку A
	getB := httptest.NewRequest("GET", "/", nil)
	getB.AddCookie(cB)
	recGetB := httptest.NewRecorder()
	h.ServeHTTP(recGetB, getB)
	if strings.Contains(recGetB.Body.String(), "Секрет A") {
		t.Error("пользователь B не должен видеть заметки пользователя A")
	}

	// B не может удалить заметку A → 404
	delB := httptest.NewRequest("DELETE", "/notes/"+idA, nil)
	delB.AddCookie(cB)
	recDelB := httptest.NewRecorder()
	h.ServeHTTP(recDelB, delB)
	if recDelB.Code != http.StatusNotFound {
		t.Errorf("удаление чужой заметки должно дать 404, получен %d", recDelB.Code)
	}
	_ = auth
	_ = settings
}

func TestEmailVerification_BlocksLoginThenConfirms(t *testing.T) {
	h, auth, settings := buildStackWithMailer(t, true) // allowSignup + перехват токенов
	ctx := context.Background()
	if err := settings.Set(ctx, "require_email_verification", "true"); err != nil {
		t.Fatalf("включить подтверждение: %v", err)
	}

	// регистрация: аккаунт создаётся, письмо с токеном уходит (перехватываем)
	recReg := postForm(h, "/register", url.Values{"email": {"v@user.com"}, "password": {"password123"}}, nil)
	// после регистрации с verify вход заблокирован — cookie не должно быть
	for _, ck := range recReg.Result().Cookies() {
		if ck.Name == "zv_session" && ck.Value != "" {
			t.Error("при включённом подтверждении автологин не должен происходить")
		}
	}

	// попытка входа до подтверждения → страница «подтвердите почту»
	recLogin := postForm(h, "/login", url.Values{"email": {"v@user.com"}, "password": {"password123"}}, nil)
	if !strings.Contains(recLogin.Body.String(), "подтвержден") && !strings.Contains(recLogin.Body.String(), "подтверд") {
		t.Errorf("ожидалась страница подтверждения почты, тело: %s", recLogin.Body.String())
	}

	// подтверждаем по перехваченному токену
	if capturedVerifyToken == "" {
		t.Fatal("токен подтверждения не перехвачен из письма")
	}
	recVerify := httptest.NewRequest("GET", "/verify-email?token="+capturedVerifyToken, nil)
	recVerifyRec := httptest.NewRecorder()
	h.ServeHTTP(recVerifyRec, recVerify)
	if recVerifyRec.Code != http.StatusOK {
		t.Fatalf("подтверждение должно пройти, код %d", recVerifyRec.Code)
	}

	// теперь вход проходит и ставит cookie
	c := loginCookie(t, h, "v@user.com", "password123")
	if c.Value == "" {
		t.Error("после подтверждения вход должен работать")
	}
	_ = auth
}

func TestAdminSettings_RequiresAdmin(t *testing.T) {
	h, _, _ := buildStack(t, true)
	// обычный пользователь (через регистрацию) — не админ
	recReg := postForm(h, "/register", url.Values{"email": {"plain@user.com"}, "password": {"password123"}}, nil)
	var c *http.Cookie
	for _, ck := range recReg.Result().Cookies() {
		if ck.Name == "zv_session" && ck.Value != "" {
			c = ck
		}
	}
	if c == nil {
		t.Fatal("пользователь не залогинен")
	}
	req := httptest.NewRequest("GET", "/admin/settings", nil)
	req.AddCookie(c)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("не-админ на /admin/settings должен получить 403, получен %d", rec.Code)
	}
}

func TestSetup_CreatesFirstAdminThenClosed(t *testing.T) {
	h, auth, _ := buildStack(t, false)
	needed, err := auth.SetupNeeded(context.Background())
	if err != nil || !needed {
		t.Fatalf("ожидалась доступная настройка: needed=%v err=%v", needed, err)
	}

	// неверный токен → 403
	bad := postForm(h, "/setup", url.Values{"email": {"a@b.com"}, "password": {"password123"}, "token": {"nope"}}, nil)
	if bad.Code != http.StatusForbidden {
		t.Errorf("неверный токен → ожидался 403, получен %d", bad.Code)
	}

	// верный токен → 201, админ создан
	ok := postForm(h, "/setup", url.Values{"email": {"a@b.com"}, "password": {"password123"}, "token": {testSetupToken}}, nil)
	if ok.Code != http.StatusCreated {
		t.Fatalf("создание админа → ожидался 201, получен %d: %s", ok.Code, ok.Body.String())
	}
	// созданный админ может войти
	if c := loginCookie(t, h, "a@b.com", "password123"); c.Value == "" {
		t.Error("созданный через /setup админ должен входить")
	}

	// повторный /setup → 410 (закрыто)
	again := postForm(h, "/setup", url.Values{"email": {"x@b.com"}, "password": {"password123"}, "token": {testSetupToken}}, nil)
	if again.Code != http.StatusGone {
		t.Errorf("повторный /setup → ожидался 410, получен %d", again.Code)
	}
}
