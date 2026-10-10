-- +goose Up
-- Комментарии к новостям и патчам; писать могут только вошедшие через Steam
CREATE TABLE news_comments (
    id         bigserial   PRIMARY KEY,
    gid        text        NOT NULL REFERENCES news (gid) ON DELETE CASCADE,
    user_id    bigint      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    text       text        NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX news_comments_gid_idx ON news_comments (gid, id DESC);

-- +goose Down
DROP TABLE IF EXISTS news_comments;
