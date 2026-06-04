package db

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const migrationsTable = "schema_migrations"

type Migration struct {
	Version int
	Name    string
	UpSQL   string
	DownSQL string
}

type Applied struct {
	Version   int
	Name      string
	AppliedAt time.Time
}

type Migrator struct {
	pool    *pgxpool.Pool
	dir     fs.FS
	verbose bool
}

func NewMigrator(pool *pgxpool.Pool, dir fs.FS) *Migrator {
	return &Migrator{pool: pool, dir: dir}
}

func (m *Migrator) SetVerbose(v bool) {
	m.verbose = v
}

func (m *Migrator) EnsureTable(ctx context.Context) error {
	stmt := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s (
    version    integer PRIMARY KEY,
    name       text NOT NULL,
    applied_at timestamptz NOT NULL DEFAULT now()
)`, migrationsTable)
	_, err := m.pool.Exec(ctx, stmt)
	return err
}

func (m *Migrator) LoadAll(ctx context.Context) ([]Migration, error) {
	entries, err := fs.ReadDir(m.dir, ".")
	if err != nil {
		return nil, fmt.Errorf("read migrations dir: %w", err)
	}
	upByVer := map[int]Migration{}
	downByVer := map[int]string{}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() {
			continue
		}
		if !strings.HasSuffix(name, ".sql") {
			continue
		}
		version, base, ok := parseMigrationName(name)
		if !ok {
			return nil, fmt.Errorf("invalid migration filename %q (expected NNNN_name.up.sql or NNNN_name.down.sql)", name)
		}
		body, err := fs.ReadFile(m.dir, name)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", name, err)
		}
		switch base {
		case "up":
			mig, ok := upByVer[version]
			if !ok {
				mig = Migration{Version: version}
			}
			mig.Name = stripVersion(name)
			mig.UpSQL = string(body)
			upByVer[version] = mig
		case "down":
			downByVer[version] = string(body)
		}
	}
	out := make([]Migration, 0, len(upByVer))
	for v, mig := range upByVer {
		mig.DownSQL = downByVer[v]
		out = append(out, mig)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Version < out[j].Version })
	return out, nil
}

func parseMigrationName(name string) (int, string, bool) {
	idx := strings.Index(name, "_")
	if idx <= 0 {
		return 0, "", false
	}
	v, err := strconv.Atoi(name[:idx])
	if err != nil {
		return 0, "", false
	}
	rest := name[idx+1:]
	switch {
	case strings.HasSuffix(rest, ".up.sql"):
		return v, "up", true
	case strings.HasSuffix(rest, ".down.sql"):
		return v, "down", true
	}
	return 0, "", false
}

func stripVersion(name string) string {
	idx := strings.Index(name, "_")
	if idx < 0 {
		return name
	}
	rest := name[idx+1:]
	rest = strings.TrimSuffix(rest, ".up.sql")
	rest = strings.TrimSuffix(rest, ".down.sql")
	return rest
}

func (m *Migrator) Applied(ctx context.Context) ([]Applied, error) {
	rows, err := m.pool.Query(ctx,
		fmt.Sprintf("SELECT version, name, applied_at FROM %s ORDER BY version", migrationsTable))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Applied
	for rows.Next() {
		var a Applied
		if err := rows.Scan(&a.Version, &a.Name, &a.AppliedAt); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (m *Migrator) Up(ctx context.Context) (int, error) {
	if err := m.EnsureTable(ctx); err != nil {
		return 0, err
	}
	all, err := m.LoadAll(ctx)
	if err != nil {
		return 0, err
	}
	appliedMap, err := m.appliedMap(ctx)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, mig := range all {
		if _, ok := appliedMap[mig.Version]; ok {
			continue
		}
		if mig.UpSQL == "" {
			return n, fmt.Errorf("migration %d has no up SQL", mig.Version)
		}
		if err := m.applyUp(ctx, mig); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

func (m *Migrator) Down(ctx context.Context) (int, error) {
	if err := m.EnsureTable(ctx); err != nil {
		return 0, err
	}
	all, err := m.LoadAll(ctx)
	if err != nil {
		return 0, err
	}
	appliedMap, err := m.appliedMap(ctx)
	if err != nil {
		return 0, err
	}
	sort.Slice(all, func(i, j int) bool { return all[i].Version > all[j].Version })
	n := 0
	for _, mig := range all {
		if _, ok := appliedMap[mig.Version]; !ok {
			continue
		}
		if mig.DownSQL == "" {
			return n, fmt.Errorf("migration %d has no down SQL", mig.Version)
		}
		if err := m.applyDown(ctx, mig); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

func (m *Migrator) applyUp(ctx context.Context, mig Migration) error {
	return pgx.BeginFunc(ctx, m.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, mig.UpSQL); err != nil {
			return fmt.Errorf("apply up %d (%s): %w", mig.Version, mig.Name, err)
		}
		_, err := tx.Exec(ctx,
			fmt.Sprintf("INSERT INTO %s (version, name) VALUES ($1, $2)", migrationsTable),
			mig.Version, mig.Name)
		if err != nil {
			return fmt.Errorf("record %d: %w", mig.Version, err)
		}
		if m.verbose {
			fmt.Printf("applied %04d %s\n", mig.Version, mig.Name)
		}
		return nil
	})
}

func (m *Migrator) applyDown(ctx context.Context, mig Migration) error {
	return pgx.BeginFunc(ctx, m.pool, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, mig.DownSQL); err != nil {
			return fmt.Errorf("apply down %d (%s): %w", mig.Version, mig.Name, err)
		}
		_, err := tx.Exec(ctx,
			fmt.Sprintf("DELETE FROM %s WHERE version = $1", migrationsTable),
			mig.Version)
		if err != nil {
			return fmt.Errorf("unrecord %d: %w", mig.Version, err)
		}
		if m.verbose {
			fmt.Printf("reverted %04d %s\n", mig.Version, mig.Name)
		}
		return nil
	})
}

func (m *Migrator) appliedMap(ctx context.Context) (map[int]Applied, error) {
	applied, err := m.Applied(ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[int]Applied, len(applied))
	for _, a := range applied {
		out[a.Version] = a
	}
	return out, nil
}

func (m *Migrator) Status(ctx context.Context) (pending []Migration, applied []Applied, err error) {
	if err = m.EnsureTable(ctx); err != nil {
		return nil, nil, err
	}
	all, err := m.LoadAll(ctx)
	if err != nil {
		return nil, nil, err
	}
	appliedMap, err := m.appliedMap(ctx)
	if err != nil {
		return nil, nil, err
	}
	for _, mig := range all {
		if a, ok := appliedMap[mig.Version]; ok {
			applied = append(applied, a)
			continue
		}
		pending = append(pending, mig)
	}
	return pending, applied, nil
}

var ErrNoDatabase = errors.New("no DSN provided")
