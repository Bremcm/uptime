package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Bremcm/uptime/internal/billingclient"
	"github.com/Bremcm/uptime/internal/domain"
	"github.com/labstack/echo/v4"
)

type mockStore struct {
	store
	monitorsByUser func(ctx context.Context, userID int64) ([]domain.Monitor, error)
	createMonitor  func(ctx context.Context, m domain.Monitor) (domain.Monitor, error)
}

func (m *mockStore) MonitorsByUser(ctx context.Context, userID int64) ([]domain.Monitor, error) {
	return m.monitorsByUser(ctx, userID)
}

func (m *mockStore) CreateMonitor(ctx context.Context, mon domain.Monitor) (domain.Monitor, error) {
	return m.createMonitor(ctx, mon)
}

type mockBilling struct {
	billing
	getLimits func(ctx context.Context, userID int64) (billingclient.Limits, error)
}

func (m *mockBilling) GetLimits(ctx context.Context, userID int64) (billingclient.Limits, error) {
	return m.getLimits(ctx, userID)
}

type mockCache struct {
	cache
	delCalled bool
}

func (m *mockCache) Del(ctx context.Context, key string) error {
	m.delCalled = true
	return nil
}

func TestHandleCreateMonitor(t *testing.T) {
	tests := []struct {
		name           string
		body           string
		userID         int64
		getLimits      func(ctx context.Context, userID int64) (billingclient.Limits, error)
		monitorsByUser func(ctx context.Context, userID int64) ([]domain.Monitor, error)
		createMonitor  func(ctx context.Context, m domain.Monitor) (domain.Monitor, error)
		wantStatus     int
	}{
		{
			name:       "missing name",
			body:       `{"url":"https://example.com"}`,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:   "limit reached",
			body:   `{"name":"test","url":"https://example.com","interval_seconds":300}`,
			userID: 1,
			getLimits: func(ctx context.Context, userID int64) (billingclient.Limits, error) {
				return billingclient.Limits{MaxMonitors: 3, MinIntervalSeconds: 300}, nil
			},
			monitorsByUser: func(ctx context.Context, userID int64) ([]domain.Monitor, error) {
				return []domain.Monitor{{}, {}, {}}, nil
			},
			wantStatus: http.StatusForbidden,
		},
		{
			name:   "interval too short for plan",
			body:   `{"name":"test","url":"https://example.com","interval_seconds":60}`,
			userID: 1,
			getLimits: func(ctx context.Context, userID int64) (billingclient.Limits, error) {
				return billingclient.Limits{MaxMonitors: 3, MinIntervalSeconds: 300}, nil
			},
			monitorsByUser: func(ctx context.Context, userID int64) ([]domain.Monitor, error) {
				return nil, nil
			},
			wantStatus: http.StatusForbidden,
		},
		{
			name:   "success",
			body:   `{"name":"test","url":"https://example.com","interval_seconds":300}`,
			userID: 1,
			getLimits: func(ctx context.Context, userID int64) (billingclient.Limits, error) {
				return billingclient.Limits{MaxMonitors: 3, MinIntervalSeconds: 300}, nil
			},
			monitorsByUser: func(ctx context.Context, userID int64) ([]domain.Monitor, error) {
				return nil, nil
			},
			createMonitor: func(ctx context.Context, m domain.Monitor) (domain.Monitor, error) {
				m.ID = 42
				m.CreatedAt = time.Now()
				return m, nil
			},
			wantStatus: http.StatusCreated,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Server{
				store:   &mockStore{monitorsByUser: tt.monitorsByUser, createMonitor: tt.createMonitor},
				billing: &mockBilling{getLimits: tt.getLimits},
				cache:   &mockCache{},
			}

			e := echo.New()
			req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)
			c.Set(userIDKey, tt.userID)

			err := s.handleCreateMonitor(c)

			gotStatus := rec.Code
			if httpErr, ok := err.(*echo.HTTPError); ok {
				gotStatus = httpErr.Code
			}
			if gotStatus != tt.wantStatus {
				t.Errorf("status = %d, want %d", gotStatus, tt.wantStatus)
			}
		})
	}
}
