package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Bremcm/uptime/internal/config"
	"github.com/Bremcm/uptime/internal/domain"
	"github.com/Bremcm/uptime/internal/events"
	"github.com/Bremcm/uptime/internal/monitor"
	"github.com/Bremcm/uptime/internal/notifier"
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

	prober := monitor.NewProber(10 * time.Second)
	telegram := notifier.NewTelegram(cfg.TelegramToken)
	detector := monitor.NewDetector(store, telegram, log, cfg.DetectorThreshold)

	consumer, err := events.NewConsumer(cfg.KafkaBrokers, cfg.ChecksTopic, "checkers")
	if err != nil {
		log.Error("failed to create consumer", "error", err)
		os.Exit(1)
	}
	defer consumer.Close()

	log.Info("checker started", "topic", cfg.ChecksTopic)

	err = consumer.ConsumeCheckJobs(ctx, func(ctx context.Context, job events.CheckJob) error {
		m := domainMonitorFromJob(job)
		check := prober.Probe(ctx, m)
		if err := store.SaveCheck(ctx, check); err != nil {
			log.Error("failed to save check", "monitor", job.MonitorID, "error", err)
			return err
		}
		detector.Process(ctx, check)
		return nil
	})
	if err != nil && ctx.Err() == nil {
		log.Error("consumer stopped", "error", err)
	}

	log.Info("checker shutdown complete")
}

func domainMonitorFromJob(job events.CheckJob) domain.Monitor {
	return domain.Monitor{
		ID:  job.MonitorID,
		URL: job.URL,
	}
}
