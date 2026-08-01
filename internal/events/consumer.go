package events

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/twmb/franz-go/pkg/kgo"
)

type Consumer struct {
	client *kgo.Client
}

func NewConsumer(brokers []string, topic, group string) (*Consumer, error) {
	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ConsumeTopics(topic),
		kgo.ConsumerGroup(group),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
	)
	if err != nil {
		return nil, fmt.Errorf("create kafka consumer: %w", err)
	}
	return &Consumer{client: client}, nil
}

func (c *Consumer) Close() {
	c.client.Close()
}

func (c *Consumer) ConsumeCheckJobs(ctx context.Context, handle func(context.Context, CheckJob) error) error {
	for {
		fetches := c.client.PollFetches(ctx)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if errs := fetches.Errors(); len(errs) > 0 {
			return fmt.Errorf("poll fetches: %v", errs)
		}

		fetches.EachRecord(func(r *kgo.Record) {
			var job CheckJob
			if err := json.Unmarshal(r.Value, &job); err != nil {
				return
			}
			if err := handle(ctx, job); err != nil {
				return
			}
		})
	}
}

func (c *Consumer) ConsumeCheckResults(ctx context.Context, handle func(context.Context, CheckResult) error) error {
	for {
		fetches := c.client.PollFetches(ctx)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if errs := fetches.Errors(); len(errs) > 0 {
			return fmt.Errorf("poll fetches: %v", errs)
		}

		fetches.EachRecord(func(r *kgo.Record) {
			var result CheckResult
			if err := json.Unmarshal(r.Value, &result); err != nil {
				return
			}
			if err := handle(ctx, result); err != nil {
				return
			}
		})
	}
}
