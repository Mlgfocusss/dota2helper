-- +goose Up
-- Данные внешних API, которые должны пережить перезапуск (константы героев, патч)
CREATE TABLE kv_cache (
    key        text        PRIMARY KEY,
    value      jsonb       NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS kv_cache;
