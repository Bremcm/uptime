package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Bremcm/uptime/internal/auth"
	httpserver "github.com/Bremcm/uptime/internal/http"
	"github.com/Bremcm/uptime/internal/monitor"
	"github.com/Bremcm/uptime/internal/storage"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	dsn := "postgres://uptime:uptime@localhost:5433/uptime?sslmode=disable"
	store, err := storage.New(ctx, dsn)
	if err != nil {
		log.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer store.Close()
	log.Info("connected to database")

	prober := monitor.NewProber(10 * time.Second)
	scheduler := monitor.NewScheduler(store, prober, log, 20, 15*time.Second)

	// TODO: вынести секрет в переменную окружения перед деплоем
	tokenManager := auth.NewTokenManager("super-secret-change-me", 24*time.Hour)
	srv := httpserver.NewServer(store, tokenManager)
	go func() {
		log.Info("http server starting", "addr", ":8080")
		if err := srv.Start(":8080"); err != nil {
			log.Error("http server stopped", "error", err)
		}
	}()

	scheduler.Run(ctx)

	log.Info("shutdown complete")
}
