package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"school-platform/fees-service/internal/service"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	dsn := getenv("FEES_DATABASE_URL", getenv("DATABASE_URL", ""))
	if dsn == "" {
		log.Fatal("FEES_DATABASE_URL or DATABASE_URL is required")
	}
	db, err := pgxpool.New(ctx, dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	var pingErr error
	for attempt := 0; attempt < 30; attempt++ {
		if pingErr = db.Ping(ctx); pingErr == nil { break }
		select {
		case <-ctx.Done(): log.Fatal(ctx.Err())
		case <-time.After(time.Second):
		}
	}
	if pingErr != nil { log.Fatal(pingErr) }
	interval := time.Minute
	if value := os.Getenv("BILLING_INTERVAL_SECONDS"); value != "" {
		if seconds, parseErr := strconv.Atoi(value); parseErr == nil && seconds > 0 {
			interval = time.Duration(seconds) * time.Second
		}
	}
	if err := service.NewScheduler(db).Run(ctx, interval); err != nil && ctx.Err() == nil {
		log.Fatal(err)
	}
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
