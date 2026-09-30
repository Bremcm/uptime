package notifier

import (
	"context"
	"fmt"
	"net/smtp"
	"time"

	"github.com/Bremcm/uptime/internal/events"
)

type Email struct {
	host string
	port string
	from string
	auth smtp.Auth
}

func NewEmail(host, port, username, password, from string) *Email {
	return &Email{
		host: host,
		port: port,
		from: from,
		auth: smtp.PlainAuth("", username, password, host),
	}
}

func (e *Email) NotifyFromEvent(ctx context.Context, event events.IncidentEvent) error {
	var subject, body string
	if !event.Resolved {
		subject = fmt.Sprintf("🔴 %s is DOWN", event.MonitorName)
		body = fmt.Sprintf("%s\nsince %s", event.MonitorURL, event.StartedAt.Format("15:04:05"))
	} else {
		duration := event.ResolvedAt.Sub(event.StartedAt).Round(time.Second)
		subject = fmt.Sprintf("🟢 %s is UP again", event.MonitorName)
		body = fmt.Sprintf("%s\nwas down for %s", event.MonitorURL, duration)
	}

	msg := fmt.Sprintf("To: %s\r\nSubject: %s\r\n\r\n%s\r\n", event.Email, subject, body)

	addr := fmt.Sprintf("%s:%s", e.host, e.port)
	return smtp.SendMail(addr, e.auth, e.from, []string{event.Email}, []byte(msg))
}
