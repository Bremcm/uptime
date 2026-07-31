package events

type CheckJob struct {
	MonitorID int64  `json:"monitor_id"`
	URL       string `json:"url"`
}
