-- +goose Up
-- +goose StatementBegin
CREATE TABLE plans (
    id                    BIGSERIAL PRIMARY KEY,
    name                  TEXT NOT NULL UNIQUE,
    max_monitors          INT NOT NULL,
    min_interval_seconds  INT NOT NULL,
    rate_limit_per_minute INT NOT NULL
);

CREATE TABLE subscriptions (
    id         BIGSERIAL PRIMARY KEY,
    user_id    BIGINT NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
    plan_id    BIGINT NOT NULL REFERENCES plans(id),
    status     TEXT NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO plans (name, max_monitors, min_interval_seconds, rate_limit_per_minute) VALUES
    ('free', 3,  300, 100),
    ('pro',  50, 30,  1000);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS subscriptions;
DROP TABLE IF EXISTS plans;
-- +goose StatementEnd