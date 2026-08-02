package monitor

import (
	"context"
	"errors"
	"log/slog"

	"github.com/Bremcm/uptime/internal/domain"
	"github.com/Bremcm/uptime/internal/events"
	"github.com/Bremcm/uptime/internal/storage"
)

type incidentStore interface {
	RecentChecks(ctx context.Context, monitorID int64, limit int) ([]domain.Check, error)
	OpenIncidentByMonitor(ctx context.Context, monitorID int64) (domain.Incident, error)
	CreateIncident(ctx context.Context, monitorID int64) (domain.Incident, error)
	ResolveIncident(ctx context.Context, incidentID int64) error
	MonitorByID(ctx context.Context, id int64) (domain.Monitor, error)
	UserByID(ctx context.Context, id int64) (domain.User, error)
}

type incidentPublisher interface {
	PublishIncident(ctx context.Context, topic string, event events.IncidentEvent) error
}

type Detector struct {
	store     incidentStore
	publisher incidentPublisher
	topic     string
	log       *slog.Logger
	threshold int
}

func NewDetector(store incidentStore, publisher incidentPublisher, topic string, log *slog.Logger, threshold int) *Detector {
	return &Detector{
		store:     store,
		publisher: publisher,
		topic:     topic,
		log:       log,
		threshold: threshold,
	}
}

func (d *Detector) Process(ctx context.Context, check domain.Check) {
	open, err := d.store.OpenIncidentByMonitor(ctx, check.MonitorID)
	hasOpen := err == nil
	if err != nil && !errors.Is(err, storage.ErrIncidentNotFound) {
		d.log.Error("check open incident", "monitor", check.MonitorID, "error", err)
		return
	}

	switch {
	case check.Status == domain.StatusUp && hasOpen:
		if err := d.store.ResolveIncident(ctx, open.ID); err != nil {
			d.log.Error("resolve incident", "incident", open.ID, "error", err)
			return
		}
		d.log.Info("incident resolved", "incident", open.ID, "monitor", check.MonitorID)
		open.ResolvedAt = &check.CheckedAt
		d.notify(ctx, check.MonitorID, open)

	case check.Status == domain.StatusDown && !hasOpen:
		if !d.recentAllDown(ctx, check.MonitorID) {
			return
		}
		inc, err := d.store.CreateIncident(ctx, check.MonitorID)
		if err != nil {
			d.log.Error("create incident", "monitor", check.MonitorID, "error", err)
			return
		}
		d.log.Info("incident opened", "incident", inc.ID, "monitor", check.MonitorID)
		d.notify(ctx, check.MonitorID, inc)
	}
}

func (d *Detector) recentAllDown(ctx context.Context, monitorID int64) bool {
	checks, err := d.store.RecentChecks(ctx, monitorID, d.threshold)
	if err != nil {
		d.log.Error("recent checks", "monitor", monitorID, "error", err)
		return false
	}
	if len(checks) < d.threshold {
		return false
	}
	for _, c := range checks {
		if c.Status != domain.StatusDown {
			return false
		}
	}
	return true
}

func (d *Detector) notify(ctx context.Context, monitorID int64, incident domain.Incident) {
	monitor, err := d.store.MonitorByID(ctx, monitorID)
	if err != nil {
		d.log.Error("load monitor for notification", "monitor", monitorID, "error", err)
		return
	}

	user, err := d.store.UserByID(ctx, monitor.UserID)
	if err != nil {
		d.log.Error("load user for notification", "user", monitor.UserID, "error", err)
		return
	}
	if user.TelegramChatID == "" {
		return
	}

	event := events.IncidentEvent{
		IncidentID:  incident.ID,
		MonitorName: monitor.Name,
		MonitorURL:  monitor.URL,
		ChatID:      user.TelegramChatID,
		Resolved:    !incident.IsOpen(),
		StartedAt:   incident.StartedAt,
		ResolvedAt:  incident.ResolvedAt,
	}
	if err := d.publisher.PublishIncident(ctx, d.topic, event); err != nil {
		d.log.Error("publish incident event", "monitor", monitorID, "error", err)
	}
}
