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
	"syscall"
	"time"

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

	// db (платформенный слой: SQLite + очередь записи)
	database, err := db.Open(dbPath)
	if err != nil {
		return err
	}
	defer database.Close()

	ctx := context.Background()
	if err := database.Migrate(ctx, sqlite.Schema); err != nil {
		return err
	}

	// repository → usecase → transport
	repo := sqlite.NewNoteRepo(database)
	notes := usecase.NewNoteService(repo)
	srv, err := web.NewServer(notes)
	if err != nil {
		return err
	}

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
