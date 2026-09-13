-- +goose Up
-- +goose StatementBegin
ALTER TABLE subscriptions ADD COLUMN stripe_customer_id TEXT;
ALTER TABLE subscriptions ADD COLUMN stripe_subscription_id TEXT;
CREATE UNIQUE INDEX idx_subscriptions_stripe_customer ON subscriptions(stripe_customer_id) WHERE stripe_customer_id IS NOT NULL;
CREATE UNIQUE INDEX idx_subscriptions_stripe_subscription ON subscriptions(stripe_subscription_id) WHERE stripe_subscription_id IS NOT NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_subscriptions_stripe_subscription;
DROP INDEX IF EXISTS idx_subscriptions_stripe_customer;
ALTER TABLE subscriptions DROP COLUMN stripe_subscription_id;
ALTER TABLE subscriptions DROP COLUMN stripe_customer_id;
-- +goose StatementEnd