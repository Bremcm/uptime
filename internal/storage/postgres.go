package storage

import (
	"context"
	"errors"
	"fmt"

	"github.com/Bremcm/uptime/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrIncidentNotFound = errors.New("incident not found")
var ErrEmailTaken = errors.New("email already taken")
var ErrUserNotFound = errors.New("user not found")
var ErrMonitorNotFound = errors.New("monitor not found")

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

func (s *Store) MonitorByID(ctx context.Context, id int64) (domain.Monitor, error) {
	const q = `
		SELECT id, user_id, name, url, interval_seconds, enabled, created_at
		FROM monitors
		WHERE id = $1`

	var m domain.Monitor
	err := s.pool.QueryRow(ctx, q, id).Scan(&m.ID, &m.UserID, &m.Name, &m.URL,
		&m.IntervalSeconds, &m.Enabled, &m.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Monitor{}, ErrMonitorNotFound
		}
		return domain.Monitor{}, fmt.Errorf("monitor by id: %w", err)
	}
	return m, nil
}

func (s *Store) CreateUser(ctx context.Context, email, passwordHash string) (domain.User, error) {
	const q = `
		INSERT INTO users (email, password_hash)
		VALUES ($1, $2)
		RETURNING id, created_at`

	u := domain.User{Email: email, PasswordHash: passwordHash}
	err := s.pool.QueryRow(ctx, q, email, passwordHash).Scan(&u.ID, &u.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return domain.User{}, ErrEmailTaken
		}
		return domain.User{}, fmt.Errorf("create user: %w", err)
	}
	return u, nil
}

func (s *Store) UserByEmail(ctx context.Context, email string) (domain.User, error) {
	const q = `
		SELECT id, email, password_hash, created_at
		FROM users
		WHERE email = $1`

	var u domain.User
	err := s.pool.QueryRow(ctx, q, email).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.User{}, ErrUserNotFound
		}
		return domain.User{}, fmt.Errorf("user by email: %w", err)
	}
	return u, nil
}

func (s *Store) EnabledMonitors(ctx context.Context) ([]domain.Monitor, error) {
	const q = `
		SELECT id, user_id, name, url, interval_seconds, enabled, created_at
		FROM monitors
		WHERE enabled = true`

	rows, err := s.pool.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("query enabled monitors: %w", err)
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

func (s *Store) SaveCheck(ctx context.Context, c domain.Check) error {
	const q = `
		INSERT INTO checks (monitor_id, status, status_code, latency_ms, error, checked_at)
		VALUES ($1, $2, $3, $4, $5, $6)`

	_, err := s.pool.Exec(ctx, q, c.MonitorID, c.Status, c.StatusCode,
		c.LatencyMS, c.Error, c.CheckedAt)
	if err != nil {
		return fmt.Errorf("save check: %w", err)
	}
	return nil
}

func (s *Store) RecentChecks(ctx context.Context, monitorID int64, limit int) ([]domain.Check, error) {
	const q = `
		SELECT id, monitor_id, status, status_code, latency_ms, error, checked_at
		FROM checks
		WHERE monitor_id = $1
		ORDER BY checked_at DESC
		LIMIT $2`

	rows, err := s.pool.Query(ctx, q, monitorID, limit)
	if err != nil {
		return nil, fmt.Errorf("query checks: %w", err)
	}
	defer rows.Close()

	var checks []domain.Check
	for rows.Next() {
		var c domain.Check
		err := rows.Scan(&c.ID, &c.MonitorID, &c.Status, &c.StatusCode,
			&c.LatencyMS, &c.Error, &c.CheckedAt)
		if err != nil {
			return nil, fmt.Errorf("scan check: %w", err)
		}
		checks = append(checks, c)
	}
	return checks, rows.Err()
}

func (s *Store) OpenIncidentByMonitor(ctx context.Context, monitorID int64) (domain.Incident, error) {
	const q = `
		SELECT id, monitor_id, started_at, resolved_at
		FROM incidents
		WHERE monitor_id = $1 AND resolved_at IS NULL`

	var inc domain.Incident
	err := s.pool.QueryRow(ctx, q, monitorID).Scan(&inc.ID, &inc.MonitorID, &inc.StartedAt, &inc.ResolvedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Incident{}, ErrIncidentNotFound
		}
		return domain.Incident{}, fmt.Errorf("open incident by monitor: %w", err)
	}
	return inc, nil
}

func (s *Store) CreateIncident(ctx context.Context, monitorID int64) (domain.Incident, error) {
	const q = `
		INSERT INTO incidents (monitor_id)
		VALUES ($1)
		RETURNING id, monitor_id, started_at, resolved_at`

	var inc domain.Incident
	err := s.pool.QueryRow(ctx, q, monitorID).Scan(&inc.ID, &inc.MonitorID, &inc.StartedAt, &inc.ResolvedAt)
	if err != nil {
		return domain.Incident{}, fmt.Errorf("create incident: %w", err)
	}
	return inc, nil
}

func (s *Store) ResolveIncident(ctx context.Context, incidentID int64) error {
	const q = `
		UPDATE incidents
		SET resolved_at = now()
		WHERE id = $1 AND resolved_at IS NULL`

	_, err := s.pool.Exec(ctx, q, incidentID)
	if err != nil {
		return fmt.Errorf("resolve incident: %w", err)
	}
	return nil
}
