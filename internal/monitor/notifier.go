package monitor

import (
	"context"

	"github.com/Bremcm/uptime/internal/domain"
)

type Notifier interface {
	NotifyIncident(ctx context.Context, chatID string, monitor domain.Monitor, incident domain.Incident) error
}
