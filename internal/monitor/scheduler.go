package monitor

import (
	"context"
	"log/slog"
	"time"

	"github.com/Bremcm/uptime/internal/domain"
	"github.com/Bremcm/uptime/internal/events"
)

type jobPublisher interface {
	PublishCheckJob(ctx context.Context, topic string, job events.CheckJob) error
}

type monitorLister interface {
	EnabledMonitors(ctx context.Context) ([]domain.Monitor, error)
}

type Scheduler struct {
	store     monitorLister
	publisher jobPublisher
	topic     string
	log       *slog.Logger
	interval  time.Duration
}

func NewScheduler(store monitorLister, publisher jobPublisher, topic string, log *slog.Logger, interval time.Duration) *Scheduler {
	return &Scheduler{
		store:     store,
		publisher: publisher,
		topic:     topic,
		log:       log,
		interval:  interval,
	}
}

func (s *Scheduler) Run(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	s.log.Info("scheduler started", "interval", s.interval, "topic", s.topic)
	for {
		select {
		case <-ctx.Done():
			s.log.Info("scheduler stopped")
			return
		case <-ticker.C:
			s.runOnce(ctx)
		}
	}
}

func (s *Scheduler) runOnce(ctx context.Context) {
	monitors, err := s.store.EnabledMonitors(ctx)
	if err != nil {
		s.log.Error("failed to load monitors", "error", err)
		return
	}

	for _, m := range monitors {
		job := events.CheckJob{MonitorID: m.ID, URL: m.URL}
		if err := s.publisher.PublishCheckJob(ctx, s.topic, job); err != nil {
			s.log.Error("failed to publish check job", "monitor", m.ID, "error", err)
		}
	}
}
