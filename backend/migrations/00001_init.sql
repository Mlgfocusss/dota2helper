-- +goose Up

-- Пользователи и сессии -------------------------------------------------------

CREATE TABLE users (
    id                 bigserial   PRIMARY KEY,
    steam_id64         text        NOT NULL UNIQUE,
    account_id         bigint      NOT NULL UNIQUE,
    persona_name       text,
    avatar_url         text,
    tracked_account_id bigint,
    analysis_credits   int         NOT NULL DEFAULT 0 CHECK (analysis_credits >= 0),
    has_paid           boolean     NOT NULL DEFAULT false,
    created_at         timestamptz NOT NULL DEFAULT now(),
    last_login_at      timestamptz
);

CREATE TABLE sessions (
    id         text        PRIMARY KEY,
    user_id    bigint      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX sessions_user_id_idx ON sessions (user_id);
CREATE INDEX sessions_expires_at_idx ON sessions (expires_at);

-- Данные игроков (OpenDota) ---------------------------------------------------

CREATE TABLE player_cache (
    account_id       bigint      PRIMARY KEY,
    profile          jsonb,
    rank_tier        int,
    leaderboard_rank int,
    updated_at       timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE player_matches (
    account_id  bigint      NOT NULL,
    match_id    bigint      NOT NULL,
    hero_id     int         NOT NULL,
    player_slot int,
    radiant_win boolean,
    win         boolean     NOT NULL,
    kills       int         NOT NULL DEFAULT 0,
    deaths      int         NOT NULL DEFAULT 0,
    assists     int         NOT NULL DEFAULT 0,
    duration    int         NOT NULL DEFAULT 0,
    start_time  timestamptz NOT NULL,
    game_mode   int,
    lobby_type  int,
    lane_role   int,
    party_size  int,
    PRIMARY KEY (account_id, match_id)
);
CREATE INDEX player_matches_account_start_idx ON player_matches (account_id, start_time DESC);

-- Мета, матчапы, мини‑игры, новости -------------------------------------------

CREATE TABLE meta_hero_position (
    patch      text        NOT NULL,
    position   smallint    NOT NULL CHECK (position BETWEEN 1 AND 5),
    hero_id    int         NOT NULL,
    matches    int         NOT NULL,
    wins       int         NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (patch, position, hero_id)
);

CREATE TABLE hero_matchups (
    hero_id    int         NOT NULL,
    other_id   int         NOT NULL,
    kind       text        NOT NULL CHECK (kind IN ('vs', 'with')),
    matches    int         NOT NULL,
    advantage  real        NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (hero_id, other_id, kind)
);

CREATE TABLE hl_snapshot (
    mode       text        NOT NULL,
    hero_id    int         NOT NULL,
    value      real        NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (mode, hero_id)
);

CREATE TABLE news (
    gid     text        PRIMARY KEY,
    title   text        NOT NULL,
    url     text        NOT NULL,
    date    timestamptz NOT NULL,
    html    text        NOT NULL,
    excerpt text        NOT NULL
);
CREATE INDEX news_date_idx ON news (date DESC);

-- Рекорды и квизы -------------------------------------------------------------

CREATE TABLE game_records (
    user_id    bigint      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    game       text        NOT NULL,
    mode       text        NOT NULL,
    best       int         NOT NULL DEFAULT 0,
    plays      int         NOT NULL DEFAULT 0,
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, game, mode)
);

CREATE TABLE quiz_results (
    id         bigserial   PRIMARY KEY,
    user_id    bigint      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    quiz       text        NOT NULL,
    result     jsonb       NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX quiz_results_user_quiz_idx ON quiz_results (user_id, quiz, created_at DESC);

-- Драфтер ---------------------------------------------------------------------

CREATE TABLE drafter_trainings (
    id           uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      bigint      REFERENCES users (id) ON DELETE CASCADE,
    enemies      jsonb       NOT NULL,
    team         jsonb,
    score        real,
    breakdown    jsonb,
    status       text        NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'submitted')),
    created_at   timestamptz NOT NULL DEFAULT now(),
    submitted_at timestamptz
);
CREATE INDEX drafter_trainings_user_submitted_idx ON drafter_trainings (user_id, submitted_at);
CREATE INDEX drafter_trainings_month_idx ON drafter_trainings (submitted_at) WHERE status = 'submitted';

-- Поиск пати ------------------------------------------------------------------

CREATE TABLE party_profiles (
    user_id     bigint      PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    rank_medal  smallint    NOT NULL CHECK (rank_medal BETWEEN 0 AND 8),
    hours       int         NOT NULL DEFAULT 0 CHECK (hours BETWEEN 0 AND 50000),
    positions   smallint[]  NOT NULL,
    modes       text[]      NOT NULL,
    mic         boolean     NOT NULL DEFAULT false,
    discord     boolean     NOT NULL DEFAULT false,
    discord_tag text,
    telegram    text,
    fav_heroes  int[]       NOT NULL DEFAULT '{}',
    about       text        NOT NULL DEFAULT '',
    is_active   boolean     NOT NULL DEFAULT true,
    bumped_at   timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX party_profiles_active_bumped_idx ON party_profiles (bumped_at DESC) WHERE is_active;

CREATE TABLE party_reports (
    id          bigserial   PRIMARY KEY,
    reporter_id bigint      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    target_id   bigint      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    reason      text        NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    -- один пользователь жалуется на анкету один раз
    UNIQUE (reporter_id, target_id)
);
CREATE INDEX party_reports_target_idx ON party_reports (target_id);

-- Платежи и кредиты -----------------------------------------------------------

CREATE TABLE products (
    code                text    PRIMARY KEY,
    title               text    NOT NULL,
    description         text    NOT NULL DEFAULT '',
    price_kopecks       int     NOT NULL CHECK (price_kopecks > 0),
    credits             int     NOT NULL CHECK (credits > 0),
    first_purchase_only boolean NOT NULL DEFAULT false,
    active              boolean NOT NULL DEFAULT true,
    sort                int     NOT NULL DEFAULT 0
);

CREATE TABLE orders (
    id                  uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id             bigint      NOT NULL REFERENCES users (id),
    product_code        text        NOT NULL REFERENCES products (code),
    amount_kopecks      int         NOT NULL,
    status              text        NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'succeeded', 'canceled')),
    provider            text        NOT NULL,
    provider_payment_id text        UNIQUE,
    created_at          timestamptz NOT NULL DEFAULT now(),
    paid_at             timestamptz
);
CREATE INDEX orders_user_idx ON orders (user_id, created_at DESC);

CREATE TABLE credit_ledger (
    id         bigserial   PRIMARY KEY,
    user_id    bigint      NOT NULL REFERENCES users (id),
    delta      int         NOT NULL,
    reason     text        NOT NULL,
    ref        text,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX credit_ledger_user_idx ON credit_ledger (user_id, created_at DESC);
-- идемпотентность: одно начисление на заказ, одно списание и один возврат на разбор
CREATE UNIQUE INDEX credit_ledger_reason_ref_uniq ON credit_ledger (reason, ref) WHERE ref IS NOT NULL;

-- Глубокий разбор -------------------------------------------------------------

CREATE TABLE analyses (
    id         uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    bigint      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    account_id bigint      NOT NULL,
    match_id   bigint      NOT NULL,
    status     text        NOT NULL DEFAULT 'queued' CHECK (status IN ('queued', 'parsing', 'analyzing', 'done', 'failed')),
    error      text,
    result     jsonb,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX analyses_user_idx ON analyses (user_id, created_at DESC);
CREATE INDEX analyses_queue_idx ON analyses (created_at) WHERE status = 'queued';
-- по одному матчу — один действующий разбор; после ошибки можно заказать снова
CREATE UNIQUE INDEX analyses_user_match_uniq ON analyses (user_id, match_id) WHERE status <> 'failed';

-- Обратная связь --------------------------------------------------------------

CREATE TABLE feedback (
    id         bigserial   PRIMARY KEY,
    user_id    bigint      REFERENCES users (id) ON DELETE SET NULL,
    text       text        NOT NULL,
    contact    text,
    created_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS
    feedback, analyses, credit_ledger, orders, products, party_reports, party_profiles,
    drafter_trainings, quiz_results, game_records, news, hl_snapshot, hero_matchups,
    meta_hero_position, player_matches, player_cache, sessions, users;
