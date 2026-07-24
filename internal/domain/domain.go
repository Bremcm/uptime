// Package domain описывает основные сущности сервиса — то, чем оперирует
// программа, на языке предметной области. Здесь нет ни базы, ни HTTP, ни JSON:
// только чистые понятия. Всё остальное зависит от domain, а domain — ни от чего.
package domain

import "time"

type Monitor struct {
	URL             string
	Name            string
	UserID          int64
	IntervalSeconds int
	Enabled         bool
}

type CheckStatus string

const (
	StatusUp   CheckStatus = "up"
	StatusDown CheckStatus = "down"
)

type Check struct {
	MonitorID  int64
	Status     CheckStatus
	StatusCode int
	LatencyMS  int
	Error      string
	CheckedAt  time.Time
}
