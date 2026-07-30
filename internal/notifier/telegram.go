package notifier

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Bremcm/uptime/internal/domain"
)

type Telegram struct {
	token  string
	client *http.Client
}

func NewTelegram(token string) *Telegram {
	return &Telegram{
		token:  token,
		client: &http.Client{Timeout: 10 * time.Second},
	}
}

func (t *Telegram) NotifyIncident(ctx context.Context, chatID string, monitor domain.Monitor, incident domain.Incident) error {
	var text string
	if incident.IsOpen() {
		text = fmt.Sprintf("🔴 %s is DOWN\n%s\nsince %s",
			monitor.Name, monitor.URL, incident.StartedAt.Format("15:04:05"))
	} else {
		duration := incident.ResolvedAt.Sub(incident.StartedAt).Round(time.Second)
		text = fmt.Sprintf("🟢 %s is UP again\n%s\nwas down for %s",
			monitor.Name, monitor.URL, duration)
	}

	return t.send(ctx, chatID, text)
}

func (t *Telegram) send(ctx context.Context, chatID, text string) error {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", t.token)

	payload := map[string]string{
		"chat_id": chatID,
		"text":    text,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram returned status %d", resp.StatusCode)
	}
	return nil
}
