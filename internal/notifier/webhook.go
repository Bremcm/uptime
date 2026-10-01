package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Bremcm/uptime/internal/events"
)

type Webhook struct {
	client *http.Client
}

func NewWebhook() *Webhook {
	return &Webhook{
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

type webhookPayload struct {
	IncidentID  int64      `json:"incident_id"`
	MonitorName string     `json:"monitor_name"`
	MonitorURL  string     `json:"monitor_url"`
	Resolved    bool       `json:"resolved"`
	StartedAt   time.Time  `json:"started_at"`
	ResolvedAt  *time.Time `json:"resolved_at"`
}

func (w *Webhook) NotifyFromEvent(ctx context.Context, event events.IncidentEvent) error {
	payload := webhookPayload{
		IncidentID:  event.IncidentID,
		MonitorName: event.MonitorName,
		MonitorURL:  event.MonitorURL,
		Resolved:    event.Resolved,
		StartedAt:   event.StartedAt,
		ResolvedAt:  event.ResolvedAt,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, event.WebhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := w.client.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}
	return nil
}
