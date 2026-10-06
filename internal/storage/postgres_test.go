package storage

import (
	"context"
	"database/sql"
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
