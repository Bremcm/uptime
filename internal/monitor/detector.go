package monitor

import (
	"context"
	"errors"
	"log/slog"

	"github.com/Bremcm/uptime/internal/domain"
	"github.com/Bremcm/uptime/internal/storage"
)

type incidentStore interface {
	RecentChecks(ctx context.Context, monitorID int64, limit int) ([]domain.Check, error)
	OpenIncidentByMonitor(ctx context.Context, monitorID int64) (domain.Incident, error)
	CreateIncident(ctx context.Context, monitorID int64) (domain.Incident, error)
	ResolveIncident(ctx context.Context, incidentID int64) error
}

type Detector struct {
	store     incidentStore
	log       *slog.Logger
	threshold int
}

func NewDetector(store incidentStore, log *slog.Logger, threshold int) *Detector {
	return &Detector{
		store:     store,
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
