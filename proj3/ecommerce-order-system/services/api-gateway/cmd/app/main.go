package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	httpx "ecommerce-order-system/services/api-gateway/internal/http"
	"ecommerce-order-system/services/api-gateway/internal/http/handlers"
	"ecommerce-order-system/services/api-gateway/internal/repo"
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

	ctxDB, cancelDB := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelDB()

	db, err := pgxpool.New(ctxDB, cfg.Postgres.DSN)
	if err != nil {
		log.Fatal().Err(err).Msg("pg connect failed")
	}
	defer db.Close()

	ordersRepo := &repo.OrdersPG{DB: db}

	create := &handlers.CreateOrderHandler{
		DB:     db,
		Outbox: &repo.OutboxPG{},
		Log:    log,
	}

	get := &handlers.GetOrderHandler{
		GetStatus: func(r *http.Request, orderID string) (string, error) {
			return ordersRepo.GetStatus(r.Context(), orderID)
		},
	}

	router := httpx.NewRouter(&httpx.Handlers{
		Health:      handlers.Health,
		CreateOrder: create.ServeHTTP,
		GetOrder:    get.ServeHTTP,
	})

	srv := &http.Server{
		Addr:              cfg.HTTP.Addr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Info().Str("addr", srv.Addr).Msg("http started")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("http failed")
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	log.Info().Msg("shutdown...")
	shCtx, shCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shCancel()
	_ = srv.Shutdown(shCtx)
}
