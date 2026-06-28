// Command server — точка входа эталонного приложения zerovibe.
// Composition root: читает конфиг из окружения, собирает слои
// (db → repository → usecase → transport) и поднимает HTTP-сервер с graceful
// shutdown. Это единственное место, где слои «склеиваются».
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/chudno/zerovibe/internal/adapter/platformfiles"
	"github.com/chudno/zerovibe/internal/adapter/platformmail"
	"github.com/chudno/zerovibe/internal/admin"
	"github.com/chudno/zerovibe/internal/adminres"
	"github.com/chudno/zerovibe/internal/platform/db"
	"github.com/chudno/zerovibe/internal/repository/sqlite"
	"github.com/chudno/zerovibe/internal/transport/web"
	"github.com/chudno/zerovibe/internal/usecase"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	addr := env("ADDR", ":8080")
	dbPath := env("DB_PATH", "file:zerovibe.db")
	appBaseURL := env("APP_BASE_URL", "http://localhost:8080")
	cookieName := env("SESSION_COOKIE", "zv_session")
	secureCookie := envBool("SECURE_COOKIE", true)
	// Платформа подставляет адрес API и сервис-ключ для отправки писем при деплое.
	platformURL := os.Getenv("PLATFORM_API_URL")
	platformKey := os.Getenv("PLATFORM_API_KEY")
	// Первый админ создаётся через POST /setup по одноразовому коду SETUP_TOKEN.
	// Код задаёт и передаёт платформа при деплое (env); локально его можно задать
	// в .env. Это единственный путь — отдельного сида из env-кредов нет.
	setupToken := os.Getenv("SETUP_TOKEN")

	// db (платформенный слой: SQLite + очередь записи)
	database, err := db.Open(dbPath)
	if err != nil {
		return err
	}
	defer database.Close()

	ctx := context.Background()
	if err := database.MigrateUp(ctx); err != nil {
		return err
	}

	// repository
	noteRepo := sqlite.NewNoteRepo(database)
	userRepo := sqlite.NewUserRepo(database)
	sessRepo := sqlite.NewSessionRepo(database)
	resetRepo := sqlite.NewResetRepo(database)
	verifyRepo := sqlite.NewEmailVerificationRepo(database)
	rlRepo := sqlite.NewRateLimitRepo(database)
	settingRepo := sqlite.NewSettingRepo(database)

	// adapters
	mailer := platformmail.New(platformURL, platformKey)
	// Файловое хранилище: прод — presigned-URL платформы (S3); локально — каталог
	// /data/uploads (DB_PATH рядом), раздаётся как /uploads/*.
	files := platformfiles.New(platformURL, platformKey, env("UPLOADS_DIR", "uploads"))
	hasher := usecase.NewBcryptHasher()

	// usecase
	settings := usecase.NewSettingsService(settingRepo)
	notes := usecase.NewNoteService(noteRepo)
	auth := usecase.NewAuthService(
		userRepo, sessRepo, resetRepo, verifyRepo, rlRepo,
		hasher, mailer, settings,
		usecase.AuthConfig{
			SessionTTL:      30 * 24 * time.Hour,
			ResetTTL:        time.Hour,
			VerifyTTL:       24 * time.Hour,
			AppBaseURL:      appBaseURL,
			LoginRateLimit:  usecase.RateRule{Limit: 5, Window: 15 * time.Minute},
			ForgotRateLimit: usecase.RateRule{Limit: 3, Window: time.Hour},
			ResendShortRate: usecase.RateRule{Limit: 1, Window: time.Minute},
			ResendHourRate:  usecase.RateRule{Limit: 5, Window: time.Hour},
			SetupToken:      setupToken,
		},
	)

	// Первый администратор создаётся через /setup по коду SETUP_TOKEN: код задаёт и
	// передаёт платформа при деплое (env), приложение его только читает. После деплоя
	// вайбкодер открывает /setup и задаёт свой email/пароль. /setup работает, только
	// пока админа ещё нет — после создания первого закрывается навсегда.
	if needed, err := auth.SetupNeeded(ctx); err != nil {
		return err
	} else if needed {
		log.Print("ПЕРВИЧНАЯ НАСТРОЙКА: админ ещё не создан. Создайте его вызовом POST /setup " +
			"с полями email, password и кодом настройки (SETUP_TOKEN).")
	}

	// transport
	srv, err := web.NewServer(notes, auth, settings, web.Config{
		SecureCookie: secureCookie,
		CookieName:   cookieName,
	})
	if err != nil {
		return err
	}

	// Встроенная админка: реестр сущностей + generic-CRUD. Регистрируем сущности
	// приложения (эталон — Note и User). Добавить новую сущность в админку = одна
	// строка adminres.RegisterX(reg, repo) здесь (см. skill new-feature).
	reg := admin.NewRegistry()
	adminres.RegisterUser(reg, userRepo, hasher)
	adminres.RegisterNote(reg, noteRepo, adminres.UserOptions(userRepo))
	adminSrv, err := admin.NewServer(reg, files)
	if err != nil {
		return err
	}
	srv.SetAdmin(adminSrv)

	httpSrv := &http.Server{
		Addr:              addr,
		Handler:           srv.Routes(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	// graceful shutdown по SIGINT/SIGTERM
	go func() {
		log.Printf("zerovibe слушает %s (db=%s)", addr, dbPath)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("listen: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	log.Println("остановка...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return httpSrv.Shutdown(shutdownCtx)
}

// env возвращает значение переменной окружения или fallback.
func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// envBool парсит булеву переменную окружения (1/true/yes/on → true); иначе fallback.
func envBool(key string, fallback bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	if b, err := strconv.ParseBool(v); err == nil {
		return b
	}
	switch v {
	case "yes", "on", "YES", "ON":
		return true
	case "no", "off", "NO", "OFF":
		return false
	}
	return fallback
}
