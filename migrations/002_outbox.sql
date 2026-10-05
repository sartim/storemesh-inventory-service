CREATE TABLE IF NOT EXISTS event_outbox (
    event_id UUID PRIMARY KEY,
    aggregate_type TEXT NOT NULL,
    aggregate_id UUID NOT NULL,
    event_type TEXT NOT NULL,
    payload JSONB NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL,
    published_at TIMESTAMPTZ,
    claimed_by TEXT,
    claimed_until TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS event_outbox_claimable_idx
    ON event_outbox (claimed_until, occurred_at)
    WHERE published_at IS NULL;
