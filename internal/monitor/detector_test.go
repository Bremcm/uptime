package monitor

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/Bremcm/uptime/internal/domain"
	"github.com/Bremcm/uptime/internal/events"
	"github.com/Bremcm/uptime/internal/storage"
)

type mockStore struct {
	recentChecks       []domain.Check
	openIncident       domain.Incident
	openIncidentErr    error
	createdIncident    domain.Incident
	monitor            domain.Monitor
	user               domain.User
	resolveIncidentErr error

	resolveIncidentCalled bool
	createIncidentCalled  bool
}

func (m *mockStore) RecentChecks(ctx context.Context, monitorID int64, limit int) ([]domain.Check, error) {
	return m.recentChecks, nil
}

func (m *mockStore) OpenIncidentByMonitor(ctx context.Context, monitorID int64) (domain.Incident, error) {
	return m.openIncident, m.openIncidentErr
}

func (m *mockStore) CreateIncident(ctx context.Context, monitorID int64) (domain.Incident, error) {
	m.createIncidentCalled = true
	return m.createdIncident, nil
}

func (m *mockStore) ResolveIncident(ctx context.Context, incidentID int64) error {
	m.resolveIncidentCalled = true
	return m.resolveIncidentErr
}

func (m *mockStore) MonitorByID(ctx context.Context, id int64) (domain.Monitor, error) {
	return m.monitor, nil
}

func (m *mockStore) UserByID(ctx context.Context, id int64) (domain.User, error) {
	return m.user, nil
}

func TestDetectorProcess(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	tests := []struct {
		name              string
		checkStatus       domain.CheckStatus
		threshold         int
		recentChecks      []domain.Check
		openIncidentErr   error
		user              domain.User
		wantCreateCalled  bool
		wantResolveCalled bool
		wantPublishCalled bool
	}{
		{
			name:             "down but threshold not reached",
			checkStatus:      domain.StatusDown,
			threshold:        3,
			recentChecks:     []domain.Check{{Status: domain.StatusDown}},
			openIncidentErr:  storage.ErrIncidentNotFound,
			wantCreateCalled: false,
		},
		{
			name:        "down, threshold reached, no existing incident, user has telegram",
			checkStatus: domain.StatusDown,
			threshold:   2,
			recentChecks: []domain.Check{
				{Status: domain.StatusDown},
				{Status: domain.StatusDown},
			},
			openIncidentErr:   storage.ErrIncidentNotFound,
			user:              domain.User{TelegramChatID: "12345"},
			wantCreateCalled:  true,
			wantPublishCalled: true,
		},
		{
			name:              "up, existing incident resolves",
			checkStatus:       domain.StatusUp,
			openIncidentErr:   nil,
			user:              domain.User{TelegramChatID: "12345"},
			wantResolveCalled: true,
			wantPublishCalled: true,
		},
		{
			name:            "up, no existing incident, nothing happens",
			checkStatus:     domain.StatusUp,
			openIncidentErr: storage.ErrIncidentNotFound,
		},
		{
			name:        "down, threshold reached, user has no notification channels",
			checkStatus: domain.StatusDown,
			threshold:   2,
			recentChecks: []domain.Check{
				{Status: domain.StatusDown},
				{Status: domain.StatusDown},
			},
			openIncidentErr:   storage.ErrIncidentNotFound,
			user:              domain.User{},
			wantCreateCalled:  true,
			wantPublishCalled: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			publishCalled := false
			publisher := func(ctx context.Context, topic string, event events.IncidentEvent) error {
				publishCalled = true
				return nil
			}

			store := &mockStore{
				recentChecks:    tt.recentChecks,
				openIncidentErr: tt.openIncidentErr,
				monitor:         domain.Monitor{ID: 1, UserID: 1},
				user:            tt.user,
			}

			d := NewDetector(store, publisher, "incidents", log, tt.threshold)
			d.Process(context.Background(), domain.Check{
				MonitorID: 1,
				Status:    tt.checkStatus,
				CheckedAt: time.Now(),
			})

			if store.createIncidentCalled != tt.wantCreateCalled {
				t.Errorf("CreateIncident called = %v, want %v", store.createIncidentCalled, tt.wantCreateCalled)
			}
			if store.resolveIncidentCalled != tt.wantResolveCalled {
				t.Errorf("ResolveIncident called = %v, want %v", store.resolveIncidentCalled, tt.wantResolveCalled)
			}
			if publishCalled != tt.wantPublishCalled {
				t.Errorf("publisher called = %v, want %v", publishCalled, tt.wantPublishCalled)
			}
		})
	}
}
