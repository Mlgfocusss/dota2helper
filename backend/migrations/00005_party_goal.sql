-- +goose Up

-- Анкета пати: цель игры и кого ищет автор («ищу команду» / «ищу игроков в команду»).
ALTER TABLE party_profiles
    ADD COLUMN goal        text NOT NULL DEFAULT 'rating' CHECK (goal IN ('rating', 'fun', 'stack', 'scrim')),
    ADD COLUMN looking_for text NOT NULL DEFAULT 'team'   CHECK (looking_for IN ('team', 'players'));

-- поиск всегда по виду анкеты и свежести
CREATE INDEX party_profiles_looking_bumped_idx ON party_profiles (looking_for, bumped_at DESC) WHERE is_active;

-- +goose Down
DROP INDEX IF EXISTS party_profiles_looking_bumped_idx;
ALTER TABLE party_profiles DROP COLUMN IF EXISTS looking_for, DROP COLUMN IF EXISTS goal;
