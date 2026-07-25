package monitor

import (
	"context"
	"net/http"
	"time"

	"github.com/Bremcm/uptime/internal/domain"
)

type Prober struct {
	client *http.Client
}

func NewProber(timeout time.Duration) *Prober {
	return &Prober{
		client: &http.Client{Timeout: timeout},
	}
}

func (p *Prober) Probe(ctx context.Context, m domain.Monitor) domain.Check {
	start := time.Now()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.URL, nil)
	if err != nil {
		return domain.Check{
			MonitorID: m.ID,
			Status:    domain.StatusDown,
			Error:     "bad request: " + err.Error(),
			CheckedAt: start,
		}
	}

	resp, err := p.client.Do(req)
	latency := int(time.Since(start).Milliseconds())

	if err != nil {
		return domain.Check{
			MonitorID: m.ID,
			Status:    domain.StatusDown,
			LatencyMS: latency,
			Error:     err.Error(),
			CheckedAt: start,
		}
	}
	defer resp.Body.Close()

	status := domain.StatusUp
	errMsg := ""
	if resp.StatusCode >= 400 {
		status = domain.StatusDown
		errMsg = "unexpected status: " + resp.Status
	}

	return domain.Check{
		MonitorID:  m.ID,
		Status:     status,
		StatusCode: resp.StatusCode,
		LatencyMS:  latency,
		Error:      errMsg,
		CheckedAt:  start,
	}
}
