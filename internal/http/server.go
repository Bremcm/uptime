package http

import (
	"context"
	"net/http"
	"strconv"

	"github.com/Bremcm/uptime/internal/auth"
	"github.com/Bremcm/uptime/internal/domain"
	"github.com/labstack/echo/v4"
)

type store interface {
	CreateUser(ctx context.Context, email, passwordHash string) (domain.User, error)
	UserByEmail(ctx context.Context, email string) (domain.User, error)
	CreateMonitor(ctx context.Context, m domain.Monitor) (domain.Monitor, error)
	MonitorsByUser(ctx context.Context, userID int64) ([]domain.Monitor, error)
	RecentChecks(ctx context.Context, monitorID int64, limit int) ([]domain.Check, error)
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
}

func NewServer(st store, tokens *auth.TokenManager) *Server {
	e := echo.New()
	e.HideBanner = true

	s := &Server{echo: e, store: st, tokens: tokens}
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

	return c.JSON(http.StatusCreated, m)
}

func (s *Server) handleListMonitors(c echo.Context) error {
	monitors, err := s.store.MonitorsByUser(c.Request().Context(), userIDFrom(c))
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
		return echo.NewHTTPError(http.StatusConflict, "could not create user (email may be taken)")
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
