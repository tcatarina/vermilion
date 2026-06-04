package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"vermilion/internal/db"
	"vermilion/migrations"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "vermilion-db: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	dsn := flag.String("dsn", db.DSNFromEnv(), "Postgres DSN (overrides VERMILION_PG_DSN)")
	migrationsDir := flag.String("migrations", "", "path to migrations directory (defaults to embedded migrations)")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: vermilion-db [flags] <up|down|status>\n\nFlags:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nCommands:\n  up      apply all pending migrations\n  down    revert the most recent applied migration\n  status  list applied and pending migrations\n")
	}
	flag.Parse()

	if *dsn == "" {
		return errors.New("no DSN: pass --dsn or set VERMILION_PG_DSN")
	}
	if flag.NArg() != 1 {
		flag.Usage()
		return errors.New("expected exactly one command argument")
	}
	command := flag.Arg(0)

	migrationsFS, err := resolveMigrationsFS(*migrationsDir)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.Open(ctx, *dsn)
	if err != nil {
		return fmt.Errorf("connect to %s: %w", db.RedactDSN(*dsn), err)
	}
	defer pool.Close()

	mig := db.NewMigrator(pool, migrationsFS)
	mig.SetVerbose(true)

	switch command {
	case "up":
		n, err := mig.Up(ctx)
		if err != nil {
			return err
		}
		fmt.Printf("applied %d migration(s)\n", n)
	case "down":
		n, err := mig.Down(ctx)
		if err != nil {
			return err
		}
		fmt.Printf("reverted %d migration(s)\n", n)
	case "status":
		pending, applied, err := mig.Status(ctx)
		if err != nil {
			return err
		}
		fmt.Println("Applied:")
		for _, a := range applied {
			fmt.Printf("  %04d %s at %s\n", a.Version, a.Name, a.AppliedAt.Format("2006-01-02 15:04:05Z07:00"))
		}
		fmt.Println("Pending:")
		for _, p := range pending {
			fmt.Printf("  %04d %s\n", p.Version, p.Name)
		}
	default:
		flag.Usage()
		return fmt.Errorf("unknown command %q", command)
	}
	return nil
}

func resolveMigrationsFS(dir string) (fs.FS, error) {
	if dir == "" {
		return migrations.FS, nil
	}
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("resolve migrations path: %w", err)
	}
	info, err := os.Stat(absDir)
	if err != nil || !info.IsDir() {
		return nil, fmt.Errorf("migrations dir not found: %s", absDir)
	}
	return os.DirFS(absDir), nil
}
