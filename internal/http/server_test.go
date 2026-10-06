package http

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Bremcm/uptime/internal/billingclient"
	"github.com/Bremcm/uptime/internal/domain"
	"github.com/Bremcm/uptime/internal/storage"
	"github.com/labstack/echo/v4"
)

type mockStore struct {
	store
	monitorsByUser func(ctx context.Context, userID int64) ([]domain.Monitor, error)
	createMonitor  func(ctx context.Context, m domain.Monitor) (domain.Monitor, error)
	monitorByID    func(ctx context.Context, id int64) (domain.Monitor, error)
	updateMonitor  func(ctx context.Context, m domain.Monitor) error
	deleteMonitor  func(ctx context.Context, id, userID int64) error
}

func (m *mockStore) MonitorByID(ctx context.Context, id int64) (domain.Monitor, error) {
	return m.monitorByID(ctx, id)
}

func (m *mockStore) UpdateMonitor(ctx context.Context, mon domain.Monitor) error {
	return m.updateMonitor(ctx, mon)
}

func (m *mockStore) DeleteMonitor(ctx context.Context, id, userID int64) error {
	return m.deleteMonitor(ctx, id, userID)
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

func newCtx(method, body string, userID int64, id string) (echo.Context, *httptest.ResponseRecorder) {
	e := echo.New()
	req := httptest.NewRequest(method, "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set(userIDKey, userID)
	c.SetParamNames("id")
	c.SetParamValues(id)
	return c, rec
}

func statusOf(rec *httptest.ResponseRecorder, err error) int {
	if httpErr, ok := err.(*echo.HTTPError); ok {
		return httpErr.Code
	}
	return rec.Code
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

func TestHandleUpdateMonitor(t *testing.T) {
	owned := domain.Monitor{ID: 7, UserID: 1, Name: "old", URL: "https://old.com", IntervalSeconds: 300, Enabled: true}

	tests := []struct {
		name       string
		id         string
		body       string
		userID     int64
		monitor    domain.Monitor
		monitorErr error
		minInt     int
		wantStatus int
		wantEnable *bool
	}{
		{name: "bad id", id: "abc", body: `{}`, userID: 1, wantStatus: http.StatusBadRequest},
		{name: "not found", id: "7", body: `{}`, userID: 1, monitorErr: storage.ErrMonitorNotFound, wantStatus: http.StatusNotFound},
		{name: "someone else's monitor", id: "7", body: `{"name":"x"}`, userID: 2, monitor: owned, wantStatus: http.StatusNotFound},
		{name: "empty name", id: "7", body: `{"name":""}`, userID: 1, monitor: owned, wantStatus: http.StatusBadRequest},
		{name: "bad url scheme", id: "7", body: `{"url":"ftp://x.com"}`, userID: 1, monitor: owned, wantStatus: http.StatusBadRequest},
		{name: "interval below 30", id: "7", body: `{"interval_seconds":10}`, userID: 1, monitor: owned, wantStatus: http.StatusBadRequest},
		{name: "interval below plan minimum", id: "7", body: `{"interval_seconds":60}`, userID: 1, monitor: owned, minInt: 300, wantStatus: http.StatusForbidden},
		{name: "disable monitor", id: "7", body: `{"enabled":false}`, userID: 1, monitor: owned, minInt: 300, wantStatus: http.StatusOK, wantEnable: boolPtr(false)},
		{name: "rename only", id: "7", body: `{"name":"new"}`, userID: 1, monitor: owned, minInt: 300, wantStatus: http.StatusOK, wantEnable: boolPtr(true)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var saved domain.Monitor
			s := &Server{
				store: &mockStore{
					monitorByID: func(ctx context.Context, id int64) (domain.Monitor, error) {
						return tt.monitor, tt.monitorErr
					},
					updateMonitor: func(ctx context.Context, m domain.Monitor) error {
						saved = m
						return nil
					},
				},
				billing: &mockBilling{getLimits: func(ctx context.Context, userID int64) (billingclient.Limits, error) {
					return billingclient.Limits{MinIntervalSeconds: tt.minInt}, nil
				}},
				cache: &mockCache{},
			}

			c, rec := newCtx(http.MethodPatch, tt.body, tt.userID, tt.id)
			err := s.handleUpdateMonitor(c)

			if got := statusOf(rec, err); got != tt.wantStatus {
				t.Fatalf("status = %d, want %d", got, tt.wantStatus)
			}
			if tt.wantEnable != nil && saved.Enabled != *tt.wantEnable {
				t.Errorf("saved Enabled = %v, want %v", saved.Enabled, *tt.wantEnable)
			}
		})
	}
}

func boolPtr(b bool) *bool { return &b }

func TestHandleDeleteMonitor(t *testing.T) {
	owned := domain.Monitor{ID: 7, UserID: 1}

	tests := []struct {
		name       string
		monitor    domain.Monitor
		monitorErr error
		deleteErr  error
		userID     int64
		wantStatus int
		wantDelete bool
	}{
		{name: "not found", monitorErr: storage.ErrMonitorNotFound, userID: 1, wantStatus: http.StatusNotFound},
		{name: "someone else's monitor", monitor: owned, userID: 2, wantStatus: http.StatusNotFound},
		{name: "storage failure", monitor: owned, deleteErr: errors.New("boom"), userID: 1, wantStatus: http.StatusInternalServerError, wantDelete: true},
		{name: "success", monitor: owned, userID: 1, wantStatus: http.StatusNoContent, wantDelete: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deleteCalled := false
			s := &Server{
				store: &mockStore{
					monitorByID: func(ctx context.Context, id int64) (domain.Monitor, error) {
						return tt.monitor, tt.monitorErr
					},
					deleteMonitor: func(ctx context.Context, id, userID int64) error {
						deleteCalled = true
						return tt.deleteErr
					},
				},
				cache: &mockCache{},
			}

			c, rec := newCtx(http.MethodDelete, "", tt.userID, "7")
			err := s.handleDeleteMonitor(c)

			if got := statusOf(rec, err); got != tt.wantStatus {
				t.Errorf("status = %d, want %d", got, tt.wantStatus)
			}
			if deleteCalled != tt.wantDelete {
				t.Errorf("DeleteMonitor called = %v, want %v", deleteCalled, tt.wantDelete)
			}
		})
	}
}
