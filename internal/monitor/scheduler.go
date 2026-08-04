package monitor

import (
	"context"
	"log/slog"
	"time"

	"github.com/Bremcm/uptime/internal/domain"
	"github.com/Bremcm/uptime/internal/events"
)

type publishFunc func(ctx context.Context, topic string, job events.CheckJob) error

type monitorLister interface {
	EnabledMonitors(ctx context.Context) ([]domain.Monitor, error)
}

type Scheduler struct {
	store         monitorLister
	publisher     publishFunc
	topic         string
	log           *slog.Logger
	interval      time.Duration
	lastPublished map[int64]time.Time
}

func NewScheduler(store monitorLister, publisher publishFunc, topic string, log *slog.Logger, interval time.Duration) *Scheduler {
	return &Scheduler{
		store:         store,
		publisher:     publisher,
		topic:         topic,
		log:           log,
		interval:      interval,
		lastPublished: make(map[int64]time.Time),
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

	now := time.Now()
	for _, m := range monitors {
		if !s.isDue(m, now) {
			continue
		}

		job := events.CheckJob{MonitorID: m.ID, URL: m.URL}
		if err := s.publisher(ctx, s.topic, job); err != nil {
			s.log.Error("failed to publish check job", "monitor", m.ID, "error", err)
			continue
		}
		s.lastPublished[m.ID] = now
	}
}

func (s *Scheduler) isDue(m domain.Monitor, now time.Time) bool {
	last, seen := s.lastPublished[m.ID]
	if !seen {
		return true
	}
	interval := time.Duration(m.IntervalSeconds) * time.Second
	return now.Sub(last) >= interval
}
