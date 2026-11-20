ALTER TABLE users ADD COLUMN IF NOT EXISTS reminders_enabled BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE users ALTER COLUMN reminders_enabled SET DEFAULT FALSE;
ALTER TABLE bins ADD COLUMN IF NOT EXISTS material TEXT NOT NULL DEFAULT 'mixed';
ALTER TABLE dumps ADD COLUMN IF NOT EXISTS workflow_status TEXT NOT NULL DEFAULT 'new';

CREATE TABLE IF NOT EXISTS pickup_issues (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    bin_id BIGINT NOT NULL REFERENCES bins (id) ON DELETE CASCADE,
    description TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'new',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    resolved_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS pickup_issues_status_created_idx ON pickup_issues (status, created_at DESC);

CREATE TABLE IF NOT EXISTS collection_batches (
    id BIGSERIAL PRIMARY KEY,
    code TEXT NOT NULL UNIQUE,
    material TEXT NOT NULL,
    collector_telegram_id BIGINT NOT NULL,
    status TEXT NOT NULL DEFAULT 'open',
    collected_weight_kg NUMERIC(12, 3) NOT NULL DEFAULT 0,
    accepted_weight_kg NUMERIC(12, 3),
    accepted_by BIGINT,
    note TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    submitted_at TIMESTAMPTZ,
    accepted_at TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS collection_batches_collector_status_idx ON collection_batches (collector_telegram_id, status);

CREATE TABLE IF NOT EXISTS collection_events (
    id BIGSERIAL PRIMARY KEY,
    batch_id BIGINT NOT NULL REFERENCES collection_batches (id) ON DELETE CASCADE,
    bin_id BIGINT NOT NULL REFERENCES bins (id) ON DELETE CASCADE,
    collector_telegram_id BIGINT NOT NULL,
    weight_kg NUMERIC(12, 3) NOT NULL CHECK (weight_kg > 0),
    collected_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (batch_id, bin_id)
);
CREATE INDEX IF NOT EXISTS collection_events_bin_idx ON collection_events (bin_id, collected_at DESC);
