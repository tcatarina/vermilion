package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"vermilion/internal/api"
	"vermilion/internal/db"
	"vermilion/migrations"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	addr := strings.TrimSpace(os.Getenv("VERMILION_ADDR"))
	if addr == "" {
		addr = ":8080"
	}

	mux := chi.NewRouter()

	if isDevMode() {
		origins := corsOriginsFromEnv()
		if len(origins) > 0 {
			mux.Use(api.DevCORS(origins))
		}
	}

	h := &api.Handlers{}

	var pool *pgxpool.Pool
	if dsn := db.DSNFromEnv(); dsn != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		p, err := db.Open(ctx, dsn)
		cancel()
		if err != nil {
			log.Printf("vermilion backend: cannot reach database (%s); starting in degraded mode", db.RedactDSN(dsn))
		} else {
			migrator := db.NewMigrator(p, migrations.FS)
			migrator.SetVerbose(false)
			migrateCtx, migrateCancel := context.WithTimeout(context.Background(), 30*time.Second)
			applied, err := migrator.Up(migrateCtx)
			migrateCancel()
			if err != nil {
				p.Close()
				log.Printf("vermilion backend: migrate database: %v; starting in degraded mode", err)
			} else {
				if applied > 0 {
					log.Printf("vermilion backend: applied %d database migration(s)", applied)
				}
				h.Repo = db.NewRepo(p)
				pool = p
				log.Printf("vermilion backend: database pool ready")
			}
		}
	} else {
		log.Printf("vermilion backend: VERMILION_PG_DSN not set; starting in degraded mode")
	}

	h.RegisterRoutes(mux)

	mux.NotFound(api.NotFoundJSON)
	mux.MethodNotAllowed(api.MethodNotAllowedJSON)

	srv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		log.Printf("vermilion backend listening on %s", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
		close(serverErr)
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	select {
	case sig := <-stop:
		log.Printf("vermilion backend: caught %s, shutting down", sig)
	case err := <-serverErr:
		if err != nil {
			log.Fatalf("vermilion backend: %v", err)
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("vermilion backend: shutdown: %v", err)
	}
	if pool != nil {
		pool.Close()
	}
}

func isDevMode() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("VERMILION_ENV"))) {
	case "dev", "development":
		return true
	default:
		return false
	}
}

func corsOriginsFromEnv() []string {
	raw := strings.TrimSpace(os.Getenv("VERMILION_CORS_ORIGINS"))
	if raw == "" {
		if isDevMode() {
			return []string{"http://localhost:5173"}
		}
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
