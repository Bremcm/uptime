package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

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

	scheduler.Run(ctx)

	log.Info("shutdown complete")
}
