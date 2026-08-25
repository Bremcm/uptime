// Package domain описывает основные сущности сервиса — то, чем оперирует
// программа, на языке предметной области. Здесь нет ни базы, ни HTTP, ни JSON:
// только чистые понятия. Всё остальное зависит от domain, а domain — ни от чего.
package domain

import "time"

type Monitor struct {
	ID              int64
	URL             string
	Name            string
	UserID          int64
	IntervalSeconds int
	Enabled         bool
	CreatedAt       time.Time
}

type CheckStatus string

const (
	StatusUp   CheckStatus = "up"
	StatusDown CheckStatus = "down"
)

type Check struct {
	ID         int64
	MonitorID  int64
	Status     CheckStatus
	StatusCode int
	LatencyMS  int
	Error      string
	CheckedAt  time.Time
}

type User struct {
	ID             int64
	Email          string
	PasswordHash   string
	TelegramChatID string
	CreatedAt      time.Time
}

type Incident struct {
	ID         int64
	MonitorID  int64
	StartedAt  time.Time
	ResolvedAt *time.Time
}

func (i Incident) IsOpen() bool {
	return i.ResolvedAt == nil
}

type Plan struct {
	ID                 int64
	Name               string
	MaxMonitors        int
	MinIntervalSeconds int
	RateLimitPerMinute int
}

type Subscription struct {
	UserID int64
	Plan   Plan
	Status string
}
