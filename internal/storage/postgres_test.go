package storage

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/Bremcm/uptime/internal/domain"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()

	container, err := tcpostgres.Run(ctx, "postgres:17-alpine",
		tcpostgres.WithDatabase("uptime"),
		tcpostgres.WithUsername("uptime"),
		tcpostgres.WithPassword("uptime"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		t.Fatalf("start postgres container: %v", err)
	}
	t.Cleanup(func() { _ = container.Terminate(ctx) })

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}

	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer db.Close()

	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatalf("goose dialect: %v", err)
	}
	if err := goose.Up(db, "../../migrations"); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}

	store, err := New(ctx, dsn)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	t.Cleanup(store.Close)

	return store
}

func TestUserChannelsRoundTrip(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	created, err := store.CreateUser(ctx, "alice@example.com", "hash")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	if err := store.UpdateUserTelegramChatID(ctx, created.ID, "12345"); err != nil {
		t.Fatalf("update telegram: %v", err)
	}
	if err := store.UpdateUserNotificationEmail(ctx, created.ID, "alerts@example.com"); err != nil {
		t.Fatalf("update email: %v", err)
	}
	if err := store.UpdateUserWebhookURL(ctx, created.ID, "https://hooks.example.com/x"); err != nil {
		t.Fatalf("update webhook: %v", err)
	}

	byID, err := store.UserByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("user by id: %v", err)
	}
	byEmail, err := store.UserByEmail(ctx, "alice@example.com")
	if err != nil {
		t.Fatalf("user by email: %v", err)
	}

	for name, u := range map[string]domain.User{"UserByID": byID, "UserByEmail": byEmail} {
		if u.TelegramChatID != "12345" {
			t.Errorf("%s: TelegramChatID = %q, want %q", name, u.TelegramChatID, "12345")
		}
		if u.NotificationEmail != "alerts@example.com" {
			t.Errorf("%s: NotificationEmail = %q, want %q", name, u.NotificationEmail, "alerts@example.com")
		}
		if u.WebhookURL != "https://hooks.example.com/x" {
			t.Errorf("%s: WebhookURL = %q, want %q", name, u.WebhookURL, "https://hooks.example.com/x")
		}
	}
}

func TestMonitorUpdateDelete(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	owner, err := store.CreateUser(ctx, "owner@example.com", "hash")
	if err != nil {
		t.Fatalf("create owner: %v", err)
	}
	other, err := store.CreateUser(ctx, "other@example.com", "hash")
	if err != nil {
		t.Fatalf("create other: %v", err)
	}

	mon, err := store.CreateMonitor(ctx, domain.Monitor{
		UserID: owner.ID, Name: "orig", URL: "https://orig.com", IntervalSeconds: 300, Enabled: true,
	})
	if err != nil {
		t.Fatalf("create monitor: %v", err)
	}

	t.Run("update by owner changes fields", func(t *testing.T) {
		upd := mon
		upd.Name = "renamed"
		upd.URL = "https://new.com"
		upd.IntervalSeconds = 600
		upd.Enabled = false

		if err := store.UpdateMonitor(ctx, upd); err != nil {
			t.Fatalf("update: %v", err)
		}
		got, err := store.MonitorByID(ctx, mon.ID)
		if err != nil {
			t.Fatalf("read back: %v", err)
		}
		if got.Name != "renamed" || got.URL != "https://new.com" || got.IntervalSeconds != 600 || got.Enabled {
			t.Errorf("unexpected state after update: %+v", got)
		}
	})

	t.Run("update by another user is rejected and changes nothing", func(t *testing.T) {
		hijack := mon
		hijack.UserID = other.ID
		hijack.Name = "hijacked"

		err := store.UpdateMonitor(ctx, hijack)
		if !errors.Is(err, ErrMonitorNotFound) {
			t.Fatalf("err = %v, want ErrMonitorNotFound", err)
		}
		got, _ := store.MonitorByID(ctx, mon.ID)
		if got.Name == "hijacked" {
			t.Errorf("another user's update was applied")
		}
	})

	t.Run("delete by another user is rejected and monitor survives", func(t *testing.T) {
		err := store.DeleteMonitor(ctx, mon.ID, other.ID)
		if !errors.Is(err, ErrMonitorNotFound) {
			t.Fatalf("err = %v, want ErrMonitorNotFound", err)
		}
		if _, err := store.MonitorByID(ctx, mon.ID); err != nil {
			t.Errorf("monitor should still exist: %v", err)
		}
	})

	t.Run("delete by owner cascades to checks and incidents", func(t *testing.T) {
		if err := store.SaveCheck(ctx, domain.Check{
			MonitorID: mon.ID, Status: domain.StatusDown, CheckedAt: time.Now(),
		}); err != nil {
			t.Fatalf("save check: %v", err)
		}
		if _, err := store.CreateIncident(ctx, mon.ID); err != nil {
			t.Fatalf("create incident: %v", err)
		}

		if err := store.DeleteMonitor(ctx, mon.ID, owner.ID); err != nil {
			t.Fatalf("delete: %v", err)
		}
		if _, err := store.MonitorByID(ctx, mon.ID); !errors.Is(err, ErrMonitorNotFound) {
			t.Errorf("monitor should be gone, err = %v", err)
		}
		checks, err := store.RecentChecks(ctx, mon.ID, 10)
		if err != nil {
			t.Fatalf("recent checks: %v", err)
		}
		if len(checks) != 0 {
			t.Errorf("checks should be cascaded away, got %d", len(checks))
		}
		if _, err := store.OpenIncidentByMonitor(ctx, mon.ID); !errors.Is(err, ErrIncidentNotFound) {
			t.Errorf("incident should be cascaded away, err = %v", err)
		}
	})

	t.Run("delete of a missing monitor returns not found", func(t *testing.T) {
		if err := store.DeleteMonitor(ctx, 999999, owner.ID); !errors.Is(err, ErrMonitorNotFound) {
			t.Errorf("err = %v, want ErrMonitorNotFound", err)
		}
	})
}
