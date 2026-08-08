package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/Bremcm/uptime/internal/clickhouse"
	"github.com/Bremcm/uptime/internal/config"
	"github.com/Bremcm/uptime/internal/events"
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

	chClient, err := clickhouse.New(ctx, cfg.ClickHouseAddr, cfg.ClickHouseBatchSize, cfg.ClickHouseFlushInterval, log)
	if err != nil {
		log.Error("failed to connect to clickhouse", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := chClient.Close(); err != nil {
			log.Error("failed to close clickhouse", "error", err)
		}
	}()

	go chClient.Run(ctx)

	consumer, err := events.NewConsumer(cfg.KafkaBrokers, cfg.ResultsTopic, "archivers", log)
	if err != nil {
		log.Error("failed to create consumer", "error", err)
		os.Exit(1)
	}
	defer consumer.Close()

	log.Info("archiver started", "topic", cfg.ResultsTopic, "batch_size", cfg.ClickHouseBatchSize)

	err = events.Consume(ctx, consumer, func(ctx context.Context, result events.CheckResult) error {
		if err := chClient.Add(ctx, result); err != nil {
			log.Error("failed to add check", "monitor", result.MonitorID, "error", err)
			return err
		}
		return nil
	})
	if err != nil && ctx.Err() == nil {
		log.Error("consumer stopped", "error", err)
	}

	log.Info("archiver shutdown complete")
}
