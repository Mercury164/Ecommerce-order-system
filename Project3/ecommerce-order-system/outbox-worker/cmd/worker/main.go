package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	httpx "ecommerce-order-system/outbox-worker/internal/http"
	"ecommerce-order-system/outbox-worker/internal/outbox"
	"ecommerce-order-system/shared/pkg/config"
	"ecommerce-order-system/shared/pkg/logger"
	"ecommerce-order-system/shared/pkg/rabbit"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}

	log := logger.New("outbox-worker", cfg.Common.LogLevel)

	// DB
	ctxDB, cancelDB := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelDB()

	db, err := pgxpool.New(ctxDB, cfg.Postgres.DSN)
	if err != nil {
		log.Fatal().Err(err).Msg("pg connect failed")
	}
	defer db.Close()

	// Rabbit
	rc, err := rabbit.Connect(cfg.Rabbit.URL)
	if err != nil {
		log.Fatal().Err(err).Msg("rabbit connect failed")
	}
	defer func() { _ = rc.Close() }()

	if err := rabbit.DeclareBase(rc.Ch); err != nil {
		log.Fatal().Err(err).Msg("declare base failed")
	}

	eventsPub := rabbit.NewPublisher(rc.Ch, rabbit.ExchangeEvents)

	runner := &outbox.Runner{
		Log:          log,
		DB:           db,
		EventsPub:    eventsPub,
		PollInterval: 500 * time.Millisecond,
		BatchSize:    50,
		MaxAttempts:  10,
		BackoffMax:   60 * time.Second,
	}

	appCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go runner.Run(appCtx)

	// HTTP
	addr := os.Getenv("OUTBOX_HTTP_ADDR")
	if addr == "" {
		// ВАЖНО: слушаем на всех интерфейсах, чтобы работал проброс порта
		addr = ":8085"
	}

	httpSrv := &http.Server{
		Addr:              addr,
		Handler:           (&httpx.Server{DB: db}).Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Info().Str("addr", httpSrv.Addr).Msg("http started")
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("http failed")
		}
	}()

	log.Info().Msg("outbox-worker started")

	// Graceful shutdown
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	log.Info().Msg("shutdown...")

	cancel()

	shCtx, shCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shCancel()
	_ = httpSrv.Shutdown(shCtx)
}
