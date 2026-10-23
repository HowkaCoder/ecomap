CREATE TABLE IF NOT EXISTS users (
    id          BIGSERIAL PRIMARY KEY,
    telegram_id BIGINT      NOT NULL UNIQUE,
    full_name   TEXT        NOT NULL,
    photo_url   TEXT        NOT NULL DEFAULT '',
    reputation  BIGINT      NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS bins (
    id         BIGSERIAL PRIMARY KEY,
    owner_id   BIGINT           NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    qr_code    TEXT             NOT NULL UNIQUE,
    photo_url  TEXT             NOT NULL DEFAULT '',
    lat        DOUBLE PRECISION NOT NULL,
    lon        DOUBLE PRECISION NOT NULL,
    address    TEXT             NOT NULL DEFAULT '',
    landmark   TEXT             NOT NULL DEFAULT '',
    status     TEXT             NOT NULL DEFAULT 'Пустая',
    verified   BOOLEAN          NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ      NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS bins_owner_id_idx ON bins (owner_id);

CREATE TABLE IF NOT EXISTS dumps (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT           NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    photo_url   TEXT             NOT NULL,
    lat         DOUBLE PRECISION NOT NULL,
    lon         DOUBLE PRECISION NOT NULL,
    address     TEXT             NOT NULL DEFAULT '',
    landmark    TEXT             NOT NULL DEFAULT '',
    description TEXT             NOT NULL DEFAULT '',
    status      TEXT             NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ      NOT NULL DEFAULT now()
);
