package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/Bremcm/uptime/internal/config"
	"github.com/Bremcm/uptime/internal/domain"
	"github.com/Bremcm/uptime/internal/events"
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

	producer, err := events.NewProducer(cfg.KafkaBrokers)
	if err != nil {
		log.Error("failed to create producer", "error", err)
		os.Exit(1)
	}
	defer producer.Close()

	publish := func(ctx context.Context, topic string, event events.IncidentEvent) error {
		return events.Publish(ctx, producer, topic, event)
	}
	detector := monitor.NewDetector(store, publish, cfg.IncidentsTopic, log, cfg.DetectorThreshold)
	consumer, err := events.NewConsumer(cfg.KafkaBrokers, cfg.ResultsTopic, "detectors", log)
	if err != nil {
		log.Error("failed to create consumer", "error", err)
		os.Exit(1)
	}
	defer consumer.Close()

	log.Info("detector started", "topic", cfg.ResultsTopic)

	err = events.Consume(ctx, consumer, func(ctx context.Context, result events.CheckResult) error {
		check := domain.Check{
			MonitorID:  result.MonitorID,
			Status:     domain.CheckStatus(result.Status),
			StatusCode: result.StatusCode,
			LatencyMS:  result.LatencyMS,
			Error:      result.Error,
			CheckedAt:  result.CheckedAt,
		}
		if err := store.SaveCheck(ctx, check); err != nil {
			log.Error("failed to save check", "monitor", result.MonitorID, "error", err)
			return err
		}
		detector.Process(ctx, check)
		return nil
	})
	if err != nil && ctx.Err() == nil {
		log.Error("consumer stopped", "error", err)
	}

	log.Info("detector shutdown complete")
}
