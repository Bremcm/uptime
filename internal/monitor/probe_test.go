package monitor

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Bremcm/uptime/internal/domain"
)

func TestProbe(t *testing.T) {
	tests := []struct {
		name       string
		code       int
		wantStatus domain.CheckStatus
	}{
		{name: "200 is up", code: 200, wantStatus: domain.StatusUp},
		{name: "301 is up", code: 301, wantStatus: domain.StatusUp},
		{name: "404 is down", code: 404, wantStatus: domain.StatusDown},
		{name: "500 is down", code: 500, wantStatus: domain.StatusDown},
	}

	prober := NewProber(2 * time.Second)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(
				func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(tt.code)
				}))
			defer srv.Close()

			check := prober.Probe(context.Background(), domain.Monitor{ID: 1, URL: srv.URL})

			if check.Status != tt.wantStatus {
				t.Errorf("code %d: got status %q, want %q",
					tt.code, check.Status, tt.wantStatus)
			}
			if check.StatusCode != tt.code {
				t.Errorf("code %d: got StatusCode %d, want %d",
					tt.code, check.StatusCode, tt.code)
			}
		})
	}
}
