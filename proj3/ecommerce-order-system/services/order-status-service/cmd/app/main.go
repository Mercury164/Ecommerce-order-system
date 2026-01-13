package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	httpx "ecommerce-order-system/services/order-status-service/internal/http"
	"ecommerce-order-system/services/order-status-service/internal/repo"
	"ecommerce-order-system/services/order-status-service/internal/worker"
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
	log := logger.New("order-status-service", cfg.Common.LogLevel)

	ctxDB, cancelDB := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelDB()
	db, err := pgxpool.New(ctxDB, cfg.Postgres.DSN)
	if err != nil {
		log.Fatal().Err(err).Msg("pg connect failed")
	}
	defer db.Close()

	repoOrders := &repo.OrdersPG{DB: db}

	rc, err := rabbit.Connect(cfg.Rabbit.URL)
	if err != nil {
		log.Fatal().Err(err).Msg("rabbit connect failed")
	}
	defer func() { _ = rc.Close() }()

	if err := rabbit.DeclareBase(rc.Ch); err != nil {
		log.Fatal().Err(err).Msg("declare base failed")
	}

	if err := rabbit.DeclareQueueWithDLQ(rc.Ch, rabbit.QueueSpec{
		Name:     "status.q",
		BindKeys: []string{"#"},
		DLQKey:   "status.dlq",
		Prefetch: 50,
	}); err != nil {
		log.Fatal().Err(err).Msg("declare status topology failed")
	}

	deliveries, err := rabbit.NewConsumer(rc.Ch).Consume("status.q", 50)
	if err != nil {
		log.Fatal().Err(err).Msg("consume failed")
	}

	w := &worker.Consumer{
		Log:         log,
		Repo:        repoOrders,
		RetryPub:    rabbit.NewPublisher(rc.Ch, rabbit.ExchangeRetry),
		DLQPub:      rabbit.NewPublisher(rc.Ch, rabbit.ExchangeDLX),
		Service:     "status",
		MaxAttempts: 0,
		DLQKey:      "status.dlq",
	}

	appCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Run(appCtx, deliveries)

	addr := os.Getenv("ORDER_STATUS_HTTP_ADDR")
	if addr == "" {
		addr = ":8090"
	}

	get := &httpx.GetOrderHandler{
		GetStatus: func(r *http.Request, orderID string) (string, error) {
			return repoOrders.GetStatus(r.Context(), orderID)
		},
	}

	srv := &http.Server{
		Addr:              addr,
		Handler:           httpx.NewRouter(&httpx.Handlers{Health: httpx.Health, GetOrder: get.ServeHTTP}),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Info().Str("addr", srv.Addr).Msg("http started")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("http failed")
		}
	}()

	log.Info().Msg("order-status-service started")

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	log.Info().Msg("shutdown...")

	cancel()
	shCtx, shCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shCancel()
	_ = srv.Shutdown(shCtx)
}
