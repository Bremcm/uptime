package monitor

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/Bremcm/uptime/internal/domain"
)

type checkStore interface {
	EnabledMonitors(ctx context.Context) ([]domain.Monitor, error)
	SaveCheck(ctx context.Context, c domain.Check) error
}

type Scheduler struct {
	store    checkStore
	prober   *Prober
	detector *Detector
	log      *slog.Logger
	workers  int
	interval time.Duration
}

func NewScheduler(store checkStore, prober *Prober, detector *Detector, log *slog.Logger, workers int, interval time.Duration) *Scheduler {
	return &Scheduler{
		store:    store,
		prober:   prober,
		detector: detector,
		log:      log,
		workers:  workers,
		interval: interval,
	}
}

func (s *Scheduler) Run(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	s.log.Info("scheduler started", "workers", s.workers, "interval", s.interval)

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
	if len(monitors) == 0 {
		return
	}

	jobs := make(chan domain.Monitor)
	var wg sync.WaitGroup

	for i := 0; i < s.workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for m := range jobs {
				check := s.prober.Probe(ctx, m)
				if err := s.store.SaveCheck(ctx, check); err != nil {
					s.log.Error("failed to save check", "monitor_id", m.ID, "error", err)
					continue
				}
				s.detector.Process(ctx, check)
			}
		}()
	}

	for _, m := range monitors {
		jobs <- m
	}
	close(jobs)

	wg.Wait()
}
