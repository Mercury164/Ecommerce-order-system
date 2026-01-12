package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	apihttp "ecommerce-order-system/api-gateway/internal/http"
	"ecommerce-order-system/api-gateway/internal/http/handlers"
	"ecommerce-order-system/api-gateway/internal/repo"
	"ecommerce-order-system/shared/pkg/config"
	"ecommerce-order-system/shared/pkg/logger"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}

	log := logger.New("api-gateway", cfg.Common.LogLevel)

	// Postgres
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	db, err := pgxpool.New(ctx, cfg.Postgres.DSN)
	if err != nil {
		log.Fatal().Err(err).Msg("pg connect failed")
	}
	defer db.Close()

	// HTTP handlers
	create := &handlers.CreateOrderHandler{
		DB:     db,
		Outbox: &repo.OutboxPG{},
		Log:    log,
	}

	statusBase := os.Getenv("ORDER_STATUS_URL")
	if statusBase == "" {
		statusBase = "http://order-status-service:8090"
	}

	getOrder := &handlers.GetOrderProxyHandler{
		BaseURL: statusBase,
		Client:  handlers.DefaultHTTPClient(),
		Log:     log,
	}

	router := apihttp.NewRouter(&apihttp.Handlers{
		Health:      handlers.Health,
		CreateOrder: create.ServeHTTP,
		GetOrder:    getOrder.ServeHTTP,
	})

	srv := &http.Server{
		Addr:              cfg.HTTP.Addr, // по умолчанию :8080
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	// run server
	go func() {
		log.Info().Str("addr", srv.Addr).Msg("http server started")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("http server failed")
		}
	}()

	// graceful shutdown
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	log.Info().Msg("shutdown...")

	shCtx, shCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shCancel()
	_ = srv.Shutdown(shCtx)
}
