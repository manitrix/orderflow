package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"orderflow/internal/order"
	"orderflow/migrations"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(log); err != nil {
		log.Error("service stopped", "error", err)
		os.Exit(1)
	}
}
func run(log *slog.Logger) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		return errors.New("DATABASE_URL is required")
	}
	config, err := pgxpool.ParseConfig(url)
	if err != nil {
		return errors.New("invalid DATABASE_URL")
	}
	config.MaxConns = 10
	config.MinConns = 0
	config.ConnConfig.ConnectTimeout = 5 * time.Second
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return errors.New("cannot initialize database pool")
	}
	defer pool.Close()
	startup, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err = pool.Ping(startup); err != nil {
		return fmt.Errorf("database startup check: %w", err)
	}
	if len(os.Args) == 2 && os.Args[1] == "migrate" {
		// Migration is explicit: application startup never mutates the schema.
		if _, err = pool.Exec(startup, migrations.Initial); err != nil {
			return fmt.Errorf("migration: %w", err)
		}
		log.Info("migration applied", "version", "001")
		return nil
	}
	if len(os.Args) != 1 {
		return errors.New("usage: order-service [migrate]")
	}
	// Readiness also detects a reachable database with a missing migration.
	ready := func(ctx context.Context) error {
		_, err := pool.Exec(ctx, "SELECT id FROM orders.orders LIMIT 0")
		return err
	}
	if err = ready(startup); err != nil {
		return fmt.Errorf("schema check (run migrate): %w", err)
	}
	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	server := &http.Server{Addr: addr, Handler: order.NewHandler(order.NewService(order.NewPostgres(pool)), ready, log), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10}
	result := make(chan error, 1)
	go func() { log.Info("order service listening", "addr", addr); result <- server.ListenAndServe() }()
	select {
	case err = <-result:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		log.Info("shutting down")
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err = server.Shutdown(shutdown); err != nil {
			_ = server.Close()
			return err
		}
		return nil
	}
}
