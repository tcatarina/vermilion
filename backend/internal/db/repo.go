package db

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("not found")

type CacheEntry struct {
	Key      string
	Payload  []byte
	CachedAt time.Time
}

type Repo struct {
	pool *pgxpool.Pool
}

func NewRepo(pool *pgxpool.Pool) *Repo {
	return &Repo{pool: pool}
}

func (r *Repo) Pool() *pgxpool.Pool { return r.pool }

func (r *Repo) GetCache(ctx context.Context, key string) (CacheEntry, error) {
	var e CacheEntry
	err := r.pool.QueryRow(ctx,
		`SELECT key, payload, cached_at FROM board_cache WHERE key = $1`, key,
	).Scan(&e.Key, &e.Payload, &e.CachedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return CacheEntry{}, ErrNotFound
		}
		return CacheEntry{}, err
	}
	return e, nil
}

func (r *Repo) DeleteCache(ctx context.Context, key string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM board_cache WHERE key = $1`, key)
	return err
}

func (r *Repo) SetCache(ctx context.Context, key string, payload []byte) error {
	_, err := r.pool.Exec(ctx, `
INSERT INTO board_cache (key, payload, cached_at)
VALUES ($1, $2, now())
ON CONFLICT (key) DO UPDATE SET payload = EXCLUDED.payload, cached_at = EXCLUDED.cached_at
`, key, payload)
	return err
}
