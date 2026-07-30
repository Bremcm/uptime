-- +goose Up
-- +goose StatementBegin
ALTER TABLE users ADD COLUMN telegram_chat_id TEXT NOT NULL DEFAULT '';
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE users DROP COLUMN telegram_chat_id;
-- +goose StatementEnd