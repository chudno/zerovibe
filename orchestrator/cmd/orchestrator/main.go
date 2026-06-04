// Command orchestrator — управляющий сервис: разворачивает приложения
// вайбкодеров в контейнерах на наших VM. Composition root.
//
// Фаза 3: Provider и Deployer — заглушки (доказываем логику без облака).
// Реальные адаптеры (Timeweb + SSH) подключаются за теми же портами позже.
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

	"github.com/chudno/zerovibe/orchestrator/internal/deployer/ssh"
	"github.com/chudno/zerovibe/orchestrator/internal/provider/timeweb"
	"github.com/chudno/zerovibe/orchestrator/internal/store/sqlite"
	httptransport "github.com/chudno/zerovibe/orchestrator/internal/transport/http"
	"github.com/chudno/zerovibe/orchestrator/internal/usecase"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	addr := env("ADDR", ":8090")
	dbPath := env("DB_PATH", "file:orchestrator.db")
	baseDomain := env("BASE_DOMAIN", "zerovibe.ru")
	fakeIP := env("FAKE_NODE_IP", "127.0.0.1")

	ctx := context.Background()
	store, err := sqlite.Open(ctx, dbPath)
	if err != nil {
		return err
	}
	defer store.Close()

	orch := usecase.New(
		timeweb.NewFake(fakeIP), // Provider (заглушка)
		ssh.NewNoOp(),           // Deployer (заглушка)
		store,
		usecase.UUIDGen{},
		usecase.SystemClock{},
		usecase.Config{BaseDomain: baseDomain, NodeCapacity: 10},
	)

	srv := httptransport.NewServer(orch)
	httpSrv := &http.Server{
		Addr:              addr,
		Handler:           srv.Routes(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("orchestrator слушает %s (db=%s, base=%s)", addr, dbPath, baseDomain)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("listen: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return httpSrv.Shutdown(shutdownCtx)
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
