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
	"github.com/Bremcm/uptime/internal/tracing"
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

	shutdownTracing, err := tracing.Setup(ctx, "notifier", cfg.JaegerAddr)
	if err != nil {
		log.Error("failed to setup tracing", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := shutdownTracing(context.Background()); err != nil {
			log.Error("failed to shutdown tracing", "error", err)
		}
	}()

	telegram := notifier.NewTelegram(cfg.TelegramToken)
	email := notifier.NewEmail(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUsername, cfg.SMTPPassword, cfg.SMTPFrom)
	webhook := notifier.NewWebhook()

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

		var sendErr error
		if event.ChatID != "" {
			if err := telegram.NotifyFromEvent(ctx, event); err != nil {
				log.Error("failed to notify via telegram", "incident", event.IncidentID, "error", err)
				sendErr = err
			}
		}
		if event.Email != "" {
			if err := email.NotifyFromEvent(ctx, event); err != nil {
				log.Error("failed to notify via email", "incident", event.IncidentID, "error", err)
				sendErr = err
			}
		}
		if event.WebhookURL != "" {
			if err := webhook.NotifyFromEvent(ctx, event); err != nil {
				log.Error("failed to notify via webhook", "incident", event.IncidentID, "error", err)
				sendErr = err
			}
		}
		if sendErr != nil {
			return sendErr
		}
		log.Info("notification sent", "incident", event.IncidentID, "resolved", event.Resolved)
		return nil
	})
	if err != nil && ctx.Err() == nil {
		log.Error("consumer stopped", "error", err)
	}

	log.Info("notifier shutdown complete")
}
