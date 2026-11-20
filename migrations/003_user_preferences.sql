CREATE TABLE IF NOT EXISTS user_preferences (
    telegram_id BIGINT PRIMARY KEY,
    language TEXT NOT NULL DEFAULT 'kaa' CHECK (language IN ('kaa', 'ru')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
