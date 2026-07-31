package events

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/twmb/franz-go/pkg/kgo"
)

type Producer struct {
	client *kgo.Client
}

func NewProducer(brokers []string) (*Producer, error) {
	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
	)
	if err != nil {
		return nil, fmt.Errorf("create kafka client: %w", err)
	}
	return &Producer{client: client}, nil
}

func (p *Producer) Close() {
	p.client.Close()
}

func (p *Producer) PublishCheckJob(ctx context.Context, topic string, job CheckJob) error {
	value, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("marshal check job: %w", err)
	}

	record := &kgo.Record{
		Topic: topic,
		Key:   []byte(fmt.Sprintf("%d", job.MonitorID)),
		Value: value,
	}

	if err := p.client.ProduceSync(ctx, record).FirstErr(); err != nil {
		return fmt.Errorf("produce check job: %w", err)
	}
	return nil
}
