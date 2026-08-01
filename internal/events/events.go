package events

import "time"

type CheckJob struct {
	MonitorID int64  `json:"monitor_id"`
	URL       string `json:"url"`
}

type CheckResult struct {
	MonitorID  int64     `json:"monitor_id"`
	Status     string    `json:"status"`
	StatusCode int       `json:"status_code"`
	LatencyMS  int       `json:"latency_ms"`
	Error      string    `json:"error"`
	CheckedAt  time.Time `json:"checked_at"`
}
