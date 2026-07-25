package storage

import (
	"context"
	"fmt"

	"github.com/Bremcm/uptime/internal/domain"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool *pgxpool.Pool
}

func New(ctx context.Context, dsn string) (*Store, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("connect to postgres: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return &Store{pool: pool}, nil
}

func (s *Store) Close() {
	s.pool.Close()
}

func (s *Store) CreateMonitor(ctx context.Context, m domain.Monitor) (domain.Monitor, error) {
	const q = `
		INSERT INTO monitors (user_id, name, url, interval_seconds, enabled)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at`

	err := s.pool.QueryRow(ctx, q, m.UserID, m.Name, m.URL, m.IntervalSeconds, m.Enabled).
		Scan(&m.ID, &m.CreatedAt)
	if err != nil {
		return domain.Monitor{}, fmt.Errorf("create monitor: %w", err)
	}
	return m, nil
}

func (s *Store) MonitorsByUser(ctx context.Context, userID int64) ([]domain.Monitor, error) {
	const q = `
		SELECT id, user_id, name, url, interval_seconds, enabled, created_at
		FROM monitors
		WHERE user_id = $1
		ORDER BY created_at DESC`

	rows, err := s.pool.Query(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("query monitors: %w", err)
	}
	defer rows.Close()

	var monitors []domain.Monitor
	for rows.Next() {
		var m domain.Monitor
		err := rows.Scan(&m.ID, &m.UserID, &m.Name, &m.URL,
			&m.IntervalSeconds, &m.Enabled, &m.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("scan monitor: %w", err)
		}
		monitors = append(monitors, m)
	}
	return monitors, rows.Err()
}

func (s *Store) CreateUser(ctx context.Context, email, passwordHash string) (domain.User, error) {
	const q = `
		INSERT INTO users (email, password_hash)
		VALUES ($1, $2)
		RETURNING id, created_at`

	u := domain.User{Email: email, PasswordHash: passwordHash}
	err := s.pool.QueryRow(ctx, q, email, passwordHash).Scan(&u.ID, &u.CreatedAt)
	if err != nil {
		return domain.User{}, fmt.Errorf("create user: %w", err)
	}
	return u, nil
}
