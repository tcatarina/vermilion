CREATE TABLE board_cache (
    key TEXT PRIMARY KEY,
    payload JSONB NOT NULL,
    cached_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
