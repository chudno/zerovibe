// Package web — HTTP/HTML транспорт на HTMX. Зависит от usecase и domain.
//
// КЛЮЧЕВОЙ HTMX-ПАТТЕРН:
//   - GET страницы — отдаёт ПОЛНУЮ страницу (layout + нужный content по .Page).
//   - Мутации (POST/PUT/DELETE) возвращают РОВНО изменившийся фрагмент, либо при
//     навигации просят клиента сделать редирект заголовком HX-Redirect.
//
// АУТЕНТИФИКАЦИЯ:
//   - loadUser — мягкое middleware: кладёт текущего пользователя в контекст (гость
//     проходит дальше без пользователя).
//   - requireAuth/requireRole — защита маршрутов (гость → на /login; не та роль → 403).
package web

import (
	"context"
	"embed"
	"errors"
	"html/template"
	"net"
	"net/http"
	"strconv"
	"strings"

	"github.com/chudno/zerovibe/internal/domain"
	"github.com/chudno/zerovibe/internal/usecase"
)

//go:embed templates/*.html
var templatesFS embed.FS

//go:embed static/*
var staticFS embed.FS

// Config — транспортный конфиг (поведение, не бизнес-правила).
type Config struct {
	SecureCookie bool   // ставить ли cookie с флагом Secure (true за TLS-edge; локально false)
	CookieName   string // имя cookie сессии (напр. "zv_session")
}

// pageData — данные для рендера страниц. Page выбирает, какой content показать.
type pageData struct {
	Title       string
	Page        string // "notes" | "login" | "register" | "forgot" | "reset" | "settings"
	User        *domain.User
	Notes       []domain.Note
	Settings    []usecase.SettingView
	Flash       string // нейтральное сообщение (forgot/reset/verify)
	Err         string // текст ошибки формы
	Token       string // для формы reset
	Email       string // для формы повторной отправки подтверждения
	AllowSignup bool
}

// Server держит зависимости транспорта.
type Server struct {
	tmpl     *template.Template
	notes    *usecase.NoteService
	auth     *usecase.AuthService
	settings *usecase.SettingsService
	cfg      Config
}

// NewServer парсит шаблоны и собирает сервер.
func NewServer(notes *usecase.NoteService, auth *usecase.AuthService, settings *usecase.SettingsService, cfg Config) (*Server, error) {
	if cfg.CookieName == "" {
		cfg.CookieName = "zv_session"
	}
	tmpl, err := template.ParseFS(templatesFS, "templates/*.html")
	if err != nil {
		return nil, err
	}
	return &Server{tmpl: tmpl, notes: notes, auth: auth, settings: settings, cfg: cfg}, nil
}

// Routes возвращает http.Handler со всеми маршрутами, обёрнутыми в loadUser.
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()

	// Публичные (аутентификация).
	mux.HandleFunc("GET /login", s.handleLoginPage)
	mux.HandleFunc("POST /login", s.handleLogin)
	mux.HandleFunc("POST /logout", s.handleLogout)
	mux.HandleFunc("GET /register", s.handleRegisterPage)
	mux.HandleFunc("POST /register", s.handleRegister)
	mux.HandleFunc("GET /forgot", s.handleForgotPage)
	mux.HandleFunc("POST /forgot", s.handleForgot)
	mux.HandleFunc("GET /reset", s.handleResetPage)
	mux.HandleFunc("POST /reset", s.handleReset)
	mux.HandleFunc("GET /verify-email", s.handleVerifyEmail)
	mux.HandleFunc("POST /resend-verification", s.handleResendVerification)

	// Служебное и статика.
	mux.HandleFunc("GET /healthz", s.handleHealth)
	mux.Handle("GET /static/", http.FileServerFS(staticFS))

	// Защищённые (демо-раздел заметок — личные).
	mux.HandleFunc("GET /", s.requireAuth(s.handleIndex))
	mux.HandleFunc("POST /notes", s.requireAuth(s.handleCreate))
	mux.HandleFunc("DELETE /notes/{id}", s.requireAuth(s.handleDelete))

	// Админ (настройки приложения).
	mux.HandleFunc("GET /admin/settings", s.requireRole(domain.RoleAdmin, s.handleSettingsPage))
	mux.HandleFunc("PUT /admin/settings", s.requireRole(domain.RoleAdmin, s.handleSetSetting))

	return s.loadUser(mux)
}

// --- контекст текущего пользователя ---

type ctxKey int

const userKey ctxKey = 0

// currentUser достаёт пользователя из контекста (nil если гость).
func currentUser(r *http.Request) *domain.User {
	u, _ := r.Context().Value(userKey).(*domain.User)
	return u
}

// --- middleware ---

// loadUser читает cookie сессии и кладёт пользователя в контекст. Гость проходит
// дальше без пользователя — это «мягкая» аутентификация для всех маршрутов.
func (s *Server) loadUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := r.Cookie(s.cfg.CookieName)
		if err == nil && c.Value != "" {
			if u, err := s.auth.Authenticate(r.Context(), c.Value); err == nil {
				r = r.WithContext(context.WithValue(r.Context(), userKey, &u))
			} else if errors.Is(err, domain.ErrUnauthenticated) {
				// сессия истекла/недействительна — чистим cookie
				s.clearSessionCookie(w)
			}
		}
		next.ServeHTTP(w, r)
	})
}

// requireAuth пропускает только аутентифицированных; гостя отправляет на вход.
func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if currentUser(r) == nil {
			s.fail(w, r, domain.ErrUnauthenticated)
			return
		}
		next(w, r)
	}
}

// requireRole пропускает только пользователей с нужной ролью.
func (s *Server) requireRole(role domain.Role, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := currentUser(r)
		if u == nil {
			s.fail(w, r, domain.ErrUnauthenticated)
			return
		}
		if u.Role != role {
			s.fail(w, r, domain.ErrForbidden)
			return
		}
		next(w, r)
	}
}

// --- хендлеры: страницы ---

// handleIndex — полная страница со списком личных заметок.
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	u := currentUser(r)
	notes, err := s.notes.List(r.Context(), u.ID)
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.renderPage(w, r, pageData{Title: "Заметки", Page: "notes", User: u, Notes: notes})
}

func (s *Server) handleLoginPage(w http.ResponseWriter, r *http.Request) {
	if u := currentUser(r); u != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	allow, _ := s.settings.Bool(r.Context(), "allow_signup")
	s.renderPage(w, r, pageData{Title: "Вход", Page: "login", AllowSignup: allow})
}

func (s *Server) handleRegisterPage(w http.ResponseWriter, r *http.Request) {
	if u := currentUser(r); u != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	allow, _ := s.settings.Bool(r.Context(), "allow_signup")
	s.renderPage(w, r, pageData{Title: "Регистрация", Page: "register", AllowSignup: allow})
}

func (s *Server) handleForgotPage(w http.ResponseWriter, r *http.Request) {
	s.renderPage(w, r, pageData{Title: "Восстановление пароля", Page: "forgot"})
}

func (s *Server) handleResetPage(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	s.renderPage(w, r, pageData{Title: "Новый пароль", Page: "reset", Token: token})
}

func (s *Server) handleSettingsPage(w http.ResponseWriter, r *http.Request) {
	views, err := s.settings.All(r.Context())
	if err != nil {
		s.fail(w, r, err)
		return
	}
	s.renderPage(w, r, pageData{Title: "Настройки", Page: "settings", User: currentUser(r), Settings: views})
}

// --- хендлеры: мутации ---

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	email := r.FormValue("email")
	password := r.FormValue("password")
	rateKey := domain.NormalizeEmail(email) + "|" + clientIP(r)

	sess, err := s.auth.Login(r.Context(), email, password, rateKey)
	if err != nil {
		// Почта не подтверждена — показываем страницу подтверждения с кнопкой повтора.
		if errors.Is(err, domain.ErrEmailNotVerified) {
			s.renderPage(w, r, pageData{
				Title: "Подтвердите почту", Page: "verify",
				Email: domain.NormalizeEmail(email),
				Flash: "Аккаунт создан, но почта ещё не подтверждена. Перейдите по ссылке из письма.",
			})
			return
		}
		s.failForm(w, r, "login", err)
		return
	}
	s.setSessionCookie(w, sess)
	s.redirect(w, r, "/")
}

// handleVerifyEmail подтверждает почту по токену из ссылки в письме.
func (s *Server) handleVerifyEmail(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if err := s.auth.ConfirmEmailVerification(r.Context(), token); err != nil {
		s.renderPage(w, r, pageData{Title: "Подтверждение почты", Page: "verify",
			Err: "Ссылка недействительна или устарела. Запросите письмо повторно."})
		return
	}
	s.renderPage(w, r, pageData{Title: "Вход", Page: "login",
		Flash: "Почта подтверждена. Теперь можно войти."})
}

// handleResendVerification повторно отправляет письмо подтверждения (рейт-лимит).
func (s *Server) handleResendVerification(w http.ResponseWriter, r *http.Request) {
	email := r.FormValue("email")
	rateKey := domain.NormalizeEmail(email) + "|" + clientIP(r)
	if err := s.auth.ResendVerification(r.Context(), email, rateKey); err != nil {
		// Рейт-лимит — единственная ошибка наружу (анти-enumeration в сервисе).
		var limited domain.ErrRateLimited
		if errors.As(err, &limited) {
			w.Header().Set("Retry-After", strconv.Itoa(int(limited.RetryAfter.Seconds())))
		}
		s.renderPage(w, r, pageData{Title: "Подтвердите почту", Page: "verify",
			Email: domain.NormalizeEmail(email), Err: errText(err)})
		return
	}
	s.renderPage(w, r, pageData{Title: "Подтвердите почту", Page: "verify",
		Email: domain.NormalizeEmail(email),
		Flash: "Если адрес ещё не подтверждён, мы отправили новое письмо. Проверьте почту."})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(s.cfg.CookieName); err == nil && c.Value != "" {
		_ = s.auth.Logout(r.Context(), c.Value)
	}
	s.clearSessionCookie(w)
	s.redirect(w, r, "/login")
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	email := r.FormValue("email")
	password := r.FormValue("password")

	if _, err := s.auth.Register(r.Context(), email, password); err != nil {
		s.failForm(w, r, "register", err)
		return
	}
	// Автологин после регистрации.
	rateKey := domain.NormalizeEmail(email) + "|" + clientIP(r)
	sess, err := s.auth.Login(r.Context(), email, password, rateKey)
	if err != nil {
		// Регистрация прошла, но автологин не удался (напр. включено подтверждение
		// почты → ErrEmailNotVerified) — отправляем на вход.
		s.redirect(w, r, "/login")
		return
	}
	s.setSessionCookie(w, sess)
	s.redirect(w, r, "/")
}

func (s *Server) handleForgot(w http.ResponseWriter, r *http.Request) {
	email := r.FormValue("email")
	rateKey := domain.NormalizeEmail(email) + "|" + clientIP(r)

	if err := s.auth.RequestReset(r.Context(), email, rateKey); err != nil {
		s.failForm(w, r, "forgot", err)
		return
	}
	// Анти-enumeration: всегда нейтральный ответ.
	s.renderPage(w, r, pageData{
		Title: "Восстановление пароля", Page: "forgot",
		Flash: "Если такой email зарегистрирован, мы отправили на него ссылку для сброса пароля.",
	})
}

func (s *Server) handleReset(w http.ResponseWriter, r *http.Request) {
	token := r.FormValue("token")
	password := r.FormValue("password")

	if err := s.auth.ConfirmReset(r.Context(), token, password); err != nil {
		s.renderPage(w, r, pageData{Title: "Новый пароль", Page: "reset", Token: token, Err: errText(err)})
		return
	}
	s.renderPage(w, r, pageData{
		Title: "Вход", Page: "login",
		Flash: "Пароль изменён. Войдите с новым паролем.",
	})
}

func (s *Server) handleCreate(w http.ResponseWriter, r *http.Request) {
	u := currentUser(r)
	n, err := s.notes.Create(r.Context(), u.ID, r.FormValue("title"), r.FormValue("body"))
	if err != nil {
		s.fail(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.ExecuteTemplate(w, "note", n); err != nil {
		s.fail(w, r, err)
	}
}

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "некорректный id", http.StatusBadRequest)
		return
	}
	u := currentUser(r)
	if err := s.notes.Delete(r.Context(), id, u.ID); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleSetSetting(w http.ResponseWriter, r *http.Request) {
	key := r.FormValue("key")
	value := r.FormValue("value")
	if err := s.settings.Set(r.Context(), key, value); err != nil {
		s.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

// --- рендер и ошибки ---

func (s *Server) renderPage(w http.ResponseWriter, r *http.Request, data pageData) {
	if data.User == nil {
		data.User = currentUser(r)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.ExecuteTemplate(w, "layout", data); err != nil {
		http.Error(w, "внутренняя ошибка", http.StatusInternalServerError)
	}
}

// failForm перерисовывает страницу-форму с текстом ошибки (для login/register/forgot).
func (s *Server) failForm(w http.ResponseWriter, r *http.Request, page string, err error) {
	// Рейт-лимит — отдельный код и заголовок, даже для форм.
	var limited domain.ErrRateLimited
	if errors.As(err, &limited) {
		w.Header().Set("Retry-After", strconv.Itoa(int(limited.RetryAfter.Seconds())))
	}
	allow, _ := s.settings.Bool(r.Context(), "allow_signup")
	title := map[string]string{"login": "Вход", "register": "Регистрация", "forgot": "Восстановление пароля"}[page]
	s.renderPage(w, r, pageData{Title: title, Page: page, Err: errText(err), AllowSignup: allow})
}

// fail мапит доменные ошибки в HTTP-коды. Единая точка обработки ошибок транспорта.
func (s *Server) fail(w http.ResponseWriter, r *http.Request, err error) {
	var notFound domain.ErrNotFound
	var validation domain.ErrValidation
	var limited domain.ErrRateLimited
	switch {
	case errors.As(err, &notFound):
		http.Error(w, err.Error(), http.StatusNotFound)
	case errors.As(err, &validation):
		http.Error(w, err.Error(), http.StatusBadRequest)
	case errors.Is(err, domain.ErrInvalidCredentials):
		http.Error(w, err.Error(), http.StatusUnauthorized)
	case errors.Is(err, domain.ErrSignupClosed):
		http.Error(w, err.Error(), http.StatusForbidden)
	case errors.Is(err, domain.ErrEmailTaken):
		http.Error(w, err.Error(), http.StatusConflict)
	case errors.Is(err, domain.ErrForbidden):
		http.Error(w, err.Error(), http.StatusForbidden)
	case errors.Is(err, domain.ErrInvalidToken):
		http.Error(w, err.Error(), http.StatusBadRequest)
	case errors.Is(err, domain.ErrEmailNotVerified):
		http.Error(w, err.Error(), http.StatusForbidden)
	case errors.Is(err, domain.ErrUnauthenticated):
		if isHTMX(r) {
			w.Header().Set("HX-Redirect", "/login")
			w.WriteHeader(http.StatusUnauthorized)
		} else {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
		}
	case errors.As(err, &limited):
		w.Header().Set("Retry-After", strconv.Itoa(int(limited.RetryAfter.Seconds())))
		http.Error(w, err.Error(), http.StatusTooManyRequests)
	default:
		http.Error(w, "внутренняя ошибка", http.StatusInternalServerError)
	}
}

// errText возвращает безопасный для показа пользователю текст ошибки.
func errText(err error) string {
	var validation domain.ErrValidation
	if errors.As(err, &validation) {
		return validation.Error()
	}
	switch {
	case errors.Is(err, domain.ErrInvalidCredentials),
		errors.Is(err, domain.ErrSignupClosed),
		errors.Is(err, domain.ErrEmailTaken),
		errors.Is(err, domain.ErrInvalidToken),
		errors.Is(err, domain.ErrEmailNotVerified):
		return err.Error()
	}
	var limited domain.ErrRateLimited
	if errors.As(err, &limited) {
		return limited.Error()
	}
	return "что-то пошло не так, попробуйте ещё раз"
}

// --- cookie и вспомогательные ---

// Защита от CSRF: cookie сессии помечена SameSite=Lax — браузер не отправляет её при
// межсайтовых POST-запросах, что закрывает классический CSRF на мутации. Отдельных
// CSRF-токенов нет: для приложения такого класса (формы того же origin, Lax-cookie)
// это осознанное упрощение. Если понадобится строже — добавить токен в формы.
func (s *Server) setSessionCookie(w http.ResponseWriter, sess domain.Session) {
	http.SetCookie(w, &http.Cookie{
		Name:     s.cfg.CookieName,
		Value:    sess.Token,
		Path:     "/",
		HttpOnly: true,
		Secure:   s.cfg.SecureCookie,
		SameSite: http.SameSiteLaxMode,
		Expires:  sess.ExpiresAt,
	})
}

func (s *Server) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     s.cfg.CookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   s.cfg.SecureCookie,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

// redirect делает навигацию: для htmx — заголовком HX-Redirect, иначе 303.
func (s *Server) redirect(w http.ResponseWriter, r *http.Request, to string) {
	if isHTMX(r) {
		w.Header().Set("HX-Redirect", to)
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, to, http.StatusSeeOther)
}

// isHTMX сообщает, пришёл ли запрос от htmx.
func isHTMX(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true"
}

// clientIP определяет IP клиента для ключа рейт-лимита.
//
// БЕЗОПАСНОСТЬ: НЕ доверяем X-Forwarded-For — этот заголовок полностью подделывается
// клиентом, и доверие к нему позволяет обходить рейт-лимиты, меняя «IP» в каждом
// запросе. Доверяем только X-Real-IP, который выставляет НАШ доверенный edge-прокси
// (Caddy) и перезаписывает на каждом запросе. Если запрос пришёл напрямую (без edge),
// X-Real-IP не будет — используем RemoteAddr фактического соединения.
//
// Важно для прода: edge должен ОБЯЗАТЕЛЬНО устанавливать X-Real-IP (header_up), а
// прямой доступ к порту приложения, минуя edge, должен быть закрыт на сетевом уровне.
func clientIP(r *http.Request) string {
	if rip := strings.TrimSpace(r.Header.Get("X-Real-IP")); rip != "" {
		return rip
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
