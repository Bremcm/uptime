package http

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
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

const rateLimitPerMinute = 100

func (s *Server) rateLimitMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		userID := userIDFrom(c)
		key := fmt.Sprintf("ratelimit:user:%d", userID)

		count, err := s.cache.IncrWithTTL(c.Request().Context(), key, time.Minute)
		if err != nil {
			return next(c)
		}

		if count > rateLimitPerMinute {
			return echo.NewHTTPError(http.StatusTooManyRequests, "rate limit exceeded")
		}

		return next(c)
	}
}
