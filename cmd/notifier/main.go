package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Bremcm/uptime/internal/config"
	"github.com/Bremcm/uptime/internal/events"
	"github.com/Bremcm/uptime/internal/notifier"
	"github.com/Bremcm/uptime/internal/redis"
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

	redisClient, err := redis.New(ctx, cfg.RedisAddr)
	if err != nil {
		log.Error("failed to connect to redis", "error", err)
		os.Exit(1)
	}

	consumer, err := events.NewConsumer(cfg.KafkaBrokers, cfg.IncidentsTopic, "notifiers", log)
	if err != nil {
		log.Error("failed to create consumer", "error", err)
		os.Exit(1)
	}
	defer consumer.Close()

	log.Info("notifier started", "topic", cfg.IncidentsTopic)

	err = events.Consume(ctx, consumer, func(ctx context.Context, event events.IncidentEvent) error {
		kind := "open"
		if event.Resolved {
			kind = "resolved"
		}
		key := fmt.Sprintf("notif:incident:%d:%s", event.IncidentID, kind)

		ok, err := redisClient.SetNX(ctx, key, 10*time.Minute)
		if err == nil && !ok {
			log.Info("duplicate notification skipped", "incident", event.IncidentID, "resolved", event.Resolved)
			return nil
		}

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
