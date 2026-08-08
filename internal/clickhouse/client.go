package clickhouse

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/Bremcm/uptime/internal/events"
	ch "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

type Client struct {
	conn          driver.Conn
	mu            sync.Mutex
	buf           []events.CheckResult
	batchSize     int
	flushInterval time.Duration
	log           *slog.Logger
}

func New(ctx context.Context, addr string, batchSize int, flushInterval time.Duration, log *slog.Logger) (*Client, error) {
	conn, err := ch.Open(&ch.Options{
		Addr: []string{addr},
		Auth: ch.Auth{
			Database: "default",
			Username: "default",
			Password: "",
		},
	})
	if err != nil {
		return nil, err
	}

	if err := conn.Ping(ctx); err != nil {
		return nil, err
	}

	return &Client{
		conn:          conn,
		buf:           make([]events.CheckResult, 0, batchSize),
		batchSize:     batchSize,
		flushInterval: flushInterval,
		log:           log,
	}, nil
}

func (c *Client) Add(ctx context.Context, r events.CheckResult) error {
	c.mu.Lock()
	c.buf = append(c.buf, r)
	full := len(c.buf) >= c.batchSize
	c.mu.Unlock()

	if full {
		return c.flush(ctx)
	}
	return nil
}

func (c *Client) flush(ctx context.Context) error {
	c.mu.Lock()
	if len(c.buf) == 0 {
		c.mu.Unlock()
		return nil
	}
	rows := c.buf
	c.buf = make([]events.CheckResult, 0, c.batchSize)
	c.mu.Unlock()

	batch, err := c.conn.PrepareBatch(ctx, "INSERT INTO checks")
	if err != nil {
		return err
	}

	for _, r := range rows {
		if err := batch.Append(
			uint64(r.MonitorID),
			r.Status,
			uint16(r.StatusCode),
			uint32(r.LatencyMS),
			r.Error,
			r.CheckedAt,
		); err != nil {
			return err
		}
	}

	return batch.Send()
}

func (c *Client) Run(ctx context.Context) {
	ticker := time.NewTicker(c.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := c.flush(ctx); err != nil {
				c.log.Error("flush failed", "error", err)
			}
		case <-ctx.Done():
			c.flush(context.Background())
			return
		}
	}
}

func (c *Client) Close() error {
	return c.conn.Close()
}
