-- +goose Up
-- +goose StatementBegin
ALTER TABLE users ADD COLUMN webhook_url TEXT NOT NULL DEFAULT '';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE users DROP COLUMN webhook_url;
-- +goose StatementEnd