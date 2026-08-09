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

type StatPoint struct {
	Hour       time.Time `json:"hour"`
	AvgLatency float64   `json:"avg_latency"`
	UptimePct  float64   `json:"uptime_pct"`
}

type Client struct {
	conn          driver.Conn
	mu            sync.Mutex
	buf           []events.CheckResult
	batchSize     int
	flushInterval time.Duration
	log           *slog.Logger
	done          chan struct{}
	running       bool
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
		done:          make(chan struct{}),
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

	if err := batch.Send(); err != nil {
		c.log.Error("flush failed, dropping batch", "rows", len(rows), "error", err)
		return err
	}
	return nil
}

func (c *Client) Run(ctx context.Context) {
	c.running = true
	ticker := time.NewTicker(c.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if err := c.flush(ctx); err != nil {
				c.log.Error("flush failed", "error", err)
			}
		case <-ctx.Done():
			flushCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			c.flush(flushCtx)
			cancel()
			close(c.done)
			return
		}
	}
}

func (c *Client) Close() error {
	if c.running {
		<-c.done
	}
	return c.conn.Close()
}

func (c *Client) QueryStats(ctx context.Context, monitorID int64, from, to time.Time) ([]StatPoint, error) {
	const query = `
		SELECT
			hour,
			sum(sum_latency) / sum(count_checks)     AS avg_latency,
			100.0 * sum(count_up) / sum(count_checks) AS uptime_pct
		FROM checks_hourly
		WHERE monitor_id = ? AND hour >= ? AND hour < ?
		GROUP BY hour
		ORDER BY hour`

	rows, err := c.conn.Query(ctx, query, uint64(monitorID), from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var points []StatPoint
	for rows.Next() {
		var p StatPoint
		if err := rows.Scan(&p.Hour, &p.AvgLatency, &p.UptimePct); err != nil {
			return nil, err
		}
		points = append(points, p)
	}

	return points, rows.Err()
}
