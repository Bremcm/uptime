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

	prober := monitor.NewProber(10 * time.Second)

	producer, err := events.NewProducer(cfg.KafkaBrokers)
	if err != nil {
		log.Error("failed to create producer", "error", err)
		os.Exit(1)
	}
	defer producer.Close()

	consumer, err := events.NewConsumer(cfg.KafkaBrokers, cfg.ChecksTopic, "checkers", log)
	if err != nil {
		log.Error("failed to create consumer", "error", err)
		os.Exit(1)
	}
	defer consumer.Close()

	log.Info("checker started", "in", cfg.ChecksTopic, "out", cfg.ResultsTopic)

	err = events.Consume(ctx, consumer, func(ctx context.Context, job events.CheckJob) error {
		m := domain.Monitor{ID: job.MonitorID, URL: job.URL}
		check := prober.Probe(ctx, m)

		result := events.CheckResult{
			MonitorID:  check.MonitorID,
			Status:     string(check.Status),
			StatusCode: check.StatusCode,
			LatencyMS:  check.LatencyMS,
			Error:      check.Error,
			CheckedAt:  check.CheckedAt,
		}
		if err := events.Publish(ctx, producer, cfg.ResultsTopic, result); err != nil {
			log.Error("failed to publish result", "monitor", job.MonitorID, "error", err)
			return err
		}
		return nil
	})
	if err != nil && ctx.Err() == nil {
		log.Error("consumer stopped", "error", err)
	}

	log.Info("checker shutdown complete")
}
