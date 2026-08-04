package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/Bremcm/uptime/internal/config"
	"github.com/Bremcm/uptime/internal/events"
	"github.com/Bremcm/uptime/internal/notifier"
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

	telegram := notifier.NewTelegram(cfg.TelegramToken)

	consumer, err := events.NewConsumer(cfg.KafkaBrokers, cfg.IncidentsTopic, "notifiers")
	if err != nil {
		log.Error("failed to create consumer", "error", err)
		os.Exit(1)
	}
	defer consumer.Close()

	log.Info("notifier started", "topic", cfg.IncidentsTopic)

	err = events.Consume(ctx, consumer, func(ctx context.Context, event events.IncidentEvent) error {
		if err := telegram.NotifyFromEvent(ctx, event); err != nil {
			log.Error("failed to notify", "incident", event.IncidentID, "error", err)
			return err
		}
		log.Info("notification sent", "incident", event.IncidentID, "resolved", event.Resolved)
		return nil
	})
	if err != nil && ctx.Err() == nil {
		log.Error("consumer stopped", "error", err)
	}

	log.Info("notifier shutdown complete")
}
