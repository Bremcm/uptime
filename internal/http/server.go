package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/Bremcm/uptime/internal/auth"
	"github.com/Bremcm/uptime/internal/clickhouse"
	"github.com/Bremcm/uptime/internal/domain"
	"github.com/Bremcm/uptime/internal/storage"
	"github.com/labstack/echo/v4"
)

type store interface {
	CreateUser(ctx context.Context, email, passwordHash string) (domain.User, error)
	UserByEmail(ctx context.Context, email string) (domain.User, error)
	CreateMonitor(ctx context.Context, m domain.Monitor) (domain.Monitor, error)
	MonitorsByUser(ctx context.Context, userID int64) ([]domain.Monitor, error)
	RecentChecks(ctx context.Context, monitorID int64, limit int) ([]domain.Check, error)
	MonitorByID(ctx context.Context, id int64) (domain.Monitor, error)
	UpdateUserTelegramChatID(ctx context.Context, userID int64, chatID string) error
}

type analytics interface {
	QueryStats(ctx context.Context, monitorID int64, from, to time.Time) ([]clickhouse.StatPoint, error)
}

type cache interface {
	Get(ctx context.Context, key string) (string, bool, error)
	Set(ctx context.Context, key, value string, ttl time.Duration) error
	Del(ctx context.Context, key string) error
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type registerRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type authResponse struct {
	Token string `json:"token"`
}

type Server struct {
	echo   *echo.Echo
	store  store
	tokens *auth.TokenManager
	stats  analytics
	cache  cache
}

func NewServer(st store, tokens *auth.TokenManager, stats analytics, cache cache) *Server {
	e := echo.New()
	e.HideBanner = true

	s := &Server{echo: e, store: st, tokens: tokens, stats: stats, cache: cache}
	s.routes()
	return s
}

func (s *Server) routes() {
	s.echo.GET("/healthz", s.handleHealth)

	s.echo.POST("/api/v1/auth/register", s.handleRegister)
	s.echo.POST("/api/v1/auth/login", s.handleLogin)

	api := s.echo.Group("/api/v1")
	api.Use(s.authMiddleware)
	api.POST("/monitors", s.handleCreateMonitor)
	api.GET("/monitors", s.handleListMonitors)
	api.GET("/monitors/:id/checks", s.handleMonitorChecks)
	api.GET("/monitors/:id/stats", s.handleMonitorStats)
	api.PUT("/me/telegram", s.handleSetTelegram)
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

func userIDFrom(c echo.Context) int64 {
	id, _ := c.Get(userIDKey).(int64)
	return id
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
		UserID:          userIDFrom(c),
		Name:            req.Name,
		URL:             req.URL,
		IntervalSeconds: req.IntervalSeconds,
		Enabled:         true,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "could not create monitor")
	}

	key := fmt.Sprintf("monitors:user:%d", userIDFrom(c))
	_ = s.cache.Del(c.Request().Context(), key)

	return c.JSON(http.StatusCreated, toMonitorResponse(m))
}

func (s *Server) handleListMonitors(c echo.Context) error {
	ctx := c.Request().Context()
	userID := userIDFrom(c)
	key := fmt.Sprintf("monitors:user:%d", userID)

	if cached, found, err := s.cache.Get(ctx, key); err == nil && found {
		return c.JSONBlob(http.StatusOK, []byte(cached))
	}

	monitors, err := s.store.MonitorsByUser(ctx, userID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "could not list monitors")
	}

	resp := make([]monitorResponse, 0, len(monitors))
	for _, m := range monitors {
		resp = append(resp, toMonitorResponse(m))
	}

	payload, err := json.Marshal(resp)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "could not encode monitors")
	}
	_ = s.cache.Set(ctx, key, string(payload), 60*time.Second)

	return c.JSONBlob(http.StatusOK, payload)
}

func (s *Server) handleMonitorChecks(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid monitor id")
	}

	monitor, err := s.store.MonitorByID(c.Request().Context(), id)
	if err != nil {
		if errors.Is(err, storage.ErrMonitorNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "monitor not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "could not load monitor")
	}
	if monitor.UserID != userIDFrom(c) {
		return echo.NewHTTPError(http.StatusNotFound, "monitor not found")
	}

	checks, err := s.store.RecentChecks(c.Request().Context(), id, 50)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "could not load checks")
	}
	return c.JSON(http.StatusOK, checks)
}

func (s *Server) handleMonitorStats(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid monitor id")
	}

	monitor, err := s.store.MonitorByID(c.Request().Context(), id)
	if err != nil {
		if errors.Is(err, storage.ErrMonitorNotFound) {
			return echo.NewHTTPError(http.StatusNotFound, "monitor not found")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "could not load monitor")
	}
	if monitor.UserID != userIDFrom(c) {
		return echo.NewHTTPError(http.StatusNotFound, "monitor not found")
	}

	to := time.Now()
	from := to.Add(-24 * time.Hour)

	if v := c.QueryParam("from"); v != "" {
		parsed, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid from timestamp")
		}
		from = parsed
	}
	if v := c.QueryParam("to"); v != "" {
		parsed, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid to timestamp")
		}
		to = parsed
	}

	if !from.Before(to) {
		return echo.NewHTTPError(http.StatusBadRequest, "from must be before to")
	}

	points, err := s.stats.QueryStats(c.Request().Context(), id, from, to)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "could not load stats")
	}

	return c.JSON(http.StatusOK, points)
}

func (s *Server) handleRegister(c echo.Context) error {
	var req registerRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if req.Email == "" || len(req.Password) < 8 {
		return echo.NewHTTPError(http.StatusBadRequest, "email required and password must be at least 8 chars")
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "could not process password")
	}

	user, err := s.store.CreateUser(c.Request().Context(), req.Email, hash)
	if err != nil {
		if errors.Is(err, storage.ErrEmailTaken) {
			return echo.NewHTTPError(http.StatusConflict, "email already registered")
		}
		return echo.NewHTTPError(http.StatusInternalServerError, "could not create user")
	}

	token, err := s.tokens.Generate(user.ID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "could not issue token")
	}

	return c.JSON(http.StatusCreated, authResponse{Token: token})
}

func (s *Server) handleLogin(c echo.Context) error {
	var req loginRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	user, err := s.store.UserByEmail(c.Request().Context(), req.Email)
	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid email or password")
	}

	if err := auth.CheckPassword(req.Password, user.PasswordHash); err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "invalid email or password")
	}

	token, err := s.tokens.Generate(user.ID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "could not issue token")
	}

	return c.JSON(http.StatusOK, authResponse{Token: token})
}

type monitorResponse struct {
	ID              int64     `json:"id"`
	Name            string    `json:"name"`
	URL             string    `json:"url"`
	IntervalSeconds int       `json:"interval_seconds"`
	Enabled         bool      `json:"enabled"`
	CreatedAt       time.Time `json:"created_at"`
}

func toMonitorResponse(m domain.Monitor) monitorResponse {
	return monitorResponse{
		ID:              m.ID,
		Name:            m.Name,
		URL:             m.URL,
		IntervalSeconds: m.IntervalSeconds,
		Enabled:         m.Enabled,
		CreatedAt:       m.CreatedAt,
	}
}

type setTelegramRequest struct {
	ChatID string `json:"chat_id"`
}

func (s *Server) handleSetTelegram(c echo.Context) error {
	var req setTelegramRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}
	if req.ChatID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "chat_id is required")
	}

	if err := s.store.UpdateUserTelegramChatID(c.Request().Context(), userIDFrom(c), req.ChatID); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "could not update telegram")
	}
	return c.NoContent(http.StatusNoContent)
}
