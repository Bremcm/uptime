package http

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const userIDKey = "userID"

func (s *Server) authMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		header := c.Request().Header.Get("Authorization")
		if header == "" {
			return echo.NewHTTPError(http.StatusUnauthorized, "missing authorization header")
		}

		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			return echo.NewHTTPError(http.StatusUnauthorized, "invalid authorization header")
		}

		userID, err := s.tokens.Parse(parts[1])
		if err != nil {
			return echo.NewHTTPError(http.StatusUnauthorized, "invalid or expired token")
		}

		c.Set(userIDKey, userID)
		return next(c)
	}
}

const defaultRateLimitPerMinute = 100

func (s *Server) rateLimitMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		ctx := c.Request().Context()
		userID := userIDFrom(c)

		limit := s.rateLimitFor(ctx, userID)

		key := fmt.Sprintf("ratelimit:user:%d", userID)
		count, err := s.cache.IncrWithTTL(ctx, key, time.Minute)
		if err != nil {
			return next(c)
		}

		if count > int64(limit) {
			return echo.NewHTTPError(http.StatusTooManyRequests, "rate limit exceeded")
		}

		return next(c)
	}
}

func (s *Server) rateLimitFor(ctx context.Context, userID int64) int {
	cacheKey := fmt.Sprintf("billing:limit:user:%d", userID)

	if cached, found, err := s.cache.Get(ctx, cacheKey); err == nil && found {
		if n, err := strconv.Atoi(cached); err == nil {
			return n
		}
	}

	limits, err := s.billing.GetLimits(ctx, userID)
	if err != nil {
		return defaultRateLimitPerMinute
	}

	_ = s.cache.Set(ctx, cacheKey, strconv.Itoa(limits.RateLimitPerMinute), time.Minute)
	return limits.RateLimitPerMinute
}

func (s *Server) metricsMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		start := time.Now()

		err := next(c)

		duration := time.Since(start).Seconds()
		status := c.Response().Status

		attrs := metric.WithAttributes(
			attribute.String("method", c.Request().Method),
			attribute.String("path", c.Path()),
			attribute.Int("status", status),
		)

		s.requestCounter.Add(c.Request().Context(), 1, attrs)
		s.requestDuration.Record(c.Request().Context(), duration, attrs)

		return err
	}
}
