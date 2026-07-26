package http

import (
	"context"
	"net/http"
	"strconv"

	"github.com/Bremcm/uptime/internal/domain"
	"github.com/labstack/echo/v4"
)

type store interface {
	CreateMonitor(ctx context.Context, m domain.Monitor) (domain.Monitor, error)
	MonitorsByUser(ctx context.Context, userID int64) ([]domain.Monitor, error)
	RecentChecks(ctx context.Context, monitorID int64, limit int) ([]domain.Check, error)
}

type Server struct {
	echo  *echo.Echo
	store store
}

func NewServer(st store) *Server {
	e := echo.New()
	e.HideBanner = true

	s := &Server{echo: e, store: st}
	s.routes()
	return s
}

func (s *Server) routes() {
	s.echo.GET("/healthz", s.handleHealth)

	api := s.echo.Group("/api/v1")
	api.POST("/monitors", s.handleCreateMonitor)
	api.GET("/monitors", s.handleListMonitors)
	api.GET("/monitors/:id/checks", s.handleMonitorChecks)
}

func (s *Server) Start(addr string) error {
	return s.echo.Start(addr)
}

func (s *Server) handleHealth(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

type createMonitorRequest struct {
	Name            string `json:"name"`
	URL             string `json:"url"`
	IntervalSeconds int    `json:"interval_seconds"`
}

func (s *Server) handleCreateMonitor(c echo.Context) error {
	var req createMonitorRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	if req.URL == "" || req.Name == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "name and url are required")
	}
	if req.IntervalSeconds < 30 {
		req.IntervalSeconds = 300
	}

	m, err := s.store.CreateMonitor(c.Request().Context(), domain.Monitor{
		UserID:          1,
		Name:            req.Name,
		URL:             req.URL,
		IntervalSeconds: req.IntervalSeconds,
		Enabled:         true,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "could not create monitor")
	}

	return c.JSON(http.StatusCreated, m)
}

func (s *Server) handleListMonitors(c echo.Context) error {
	monitors, err := s.store.MonitorsByUser(c.Request().Context(), 1)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "could not list monitors")
	}
	return c.JSON(http.StatusOK, monitors)
}

func (s *Server) handleMonitorChecks(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid monitor id")
	}

	checks, err := s.store.RecentChecks(c.Request().Context(), id, 50)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "could not load checks")
	}
	return c.JSON(http.StatusOK, checks)
}
