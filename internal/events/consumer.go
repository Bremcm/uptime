package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
)

type Consumer struct {
	client *kgo.Client
	log    *slog.Logger
}

func NewConsumer(brokers []string, topic, group string, log *slog.Logger) (*Consumer, error) {
	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ConsumeTopics(topic),
		kgo.ConsumerGroup(group),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
	)
	if err != nil {
		return nil, fmt.Errorf("create kafka consumer: %w", err)
	}
	return &Consumer{client: client, log: log}, nil
}

func (c *Consumer) Close() {
	c.client.Close()
}

func Consume[T any](ctx context.Context, c *Consumer, handle func(context.Context, T) error) error {
	for {
		fetches := c.client.PollFetches(ctx)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if errs := fetches.Errors(); len(errs) > 0 {
			return fmt.Errorf("poll fetches: %v", errs)
		}

		fetches.EachRecord(func(r *kgo.Record) {
			var msg T
			if err := json.Unmarshal(r.Value, &msg); err != nil {
				c.log.Error("skipping malformed message", "error", err, "offset", r.Offset)
				return
			}

			if err := handleWithRetry(ctx, handle, msg, c.log); err != nil {
				c.log.Error("message dropped after retries (would go to DLQ)",
					"error", err, "offset", r.Offset, "partition", r.Partition)
			}
		})
	}
}

func handleWithRetry[T any](ctx context.Context, handle func(context.Context, T) error, msg T, log *slog.Logger) error {
	const maxAttempts = 3
	delay := 500 * time.Millisecond

	var err error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		err = handle(ctx, msg)
		if err == nil {
			return nil
		}

		if attempt < maxAttempts {
			log.Warn("handler failed, retrying", "attempt", attempt, "error", err, "next_delay", delay)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delay):
			}
			delay *= 2
		}
	}
	return err
}
