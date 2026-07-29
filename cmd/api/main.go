package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Bremcm/uptime/internal/auth"
	"github.com/Bremcm/uptime/internal/config"
	httpserver "github.com/Bremcm/uptime/internal/http"
	"github.com/Bremcm/uptime/internal/monitor"
	"github.com/Bremcm/uptime/internal/storage"
)

func main() {
	log := slog.New(slog.NewTextHandler(os.Stdout, nil))

	cfg, err := config.Load()
	if err != nil {
		log.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	store, err := storage.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer store.Close()
	log.Info("connected to database")

	prober := monitor.NewProber(10 * time.Second)
	detector := monitor.NewDetector(store, log, 3)
	scheduler := monitor.NewScheduler(store, prober, detector, log, cfg.SchedulerWorkers, cfg.SchedulerTick)

	tokenManager := auth.NewTokenManager(cfg.JWTSecret, 24*time.Hour)
	srv := httpserver.NewServer(store, tokenManager)
	go func() {
		log.Info("http server starting", "addr", cfg.HTTPAddr)
		if err := srv.Start(cfg.HTTPAddr); err != nil {
			log.Error("http server stopped", "error", err)
		}
	}()

	scheduler.Run(ctx)

	log.Info("shutdown complete")
}
