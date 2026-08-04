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

func Publish[T Keyer](ctx context.Context, p *Producer, topic string, msg T) error {
	value, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}

	record := &kgo.Record{
		Topic: topic,
		Key:   []byte(msg.Key()),
		Value: value,
	}

	if err := p.client.ProduceSync(ctx, record).FirstErr(); err != nil {
		return fmt.Errorf("produce message: %w", err)
	}
	return nil
}
