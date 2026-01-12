package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	apihttp "ecommerce-order-system/order-status-service/internal/http"
	"ecommerce-order-system/order-status-service/internal/http/handlers"
	"ecommerce-order-system/order-status-service/internal/repo"
	"ecommerce-order-system/order-status-service/internal/worker"
	"ecommerce-order-system/shared/pkg/cache"
	"ecommerce-order-system/shared/pkg/config"
	"ecommerce-order-system/shared/pkg/logger"
	"ecommerce-order-system/shared/pkg/rabbit"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Интерфейс для чтения/обновления статуса (PG или PG+Redis)
type OrdersStore interface {
	UpdateStatus(ctx context.Context, orderID string, status string) error
	GetStatus(ctx context.Context, orderID string) (string, error)
}

// Интерфейс для идемпотентности
type EventsStore interface {
	TryMarkProcessed(ctx context.Context, eventID, eventType, orderID string) (bool, error)
}

// Адаптер для worker.Consumer: UpdateStatus + TryMarkProcessed
type StatusRepo struct {
	Orders OrdersStore
	Events EventsStore
}

func (s *StatusRepo) GetStatus(ctx context.Context, orderID string) (string, error) {
	return s.Orders.GetStatus(ctx, orderID)
}

func (s *StatusRepo) UpdateStatus(ctx context.Context, orderID string, status string) error {
	return s.Orders.UpdateStatus(ctx, orderID, status)
}
func (s *StatusRepo) TryMarkProcessed(ctx context.Context, eventID, eventType, orderID string) (bool, error) {
	return s.Events.TryMarkProcessed(ctx, eventID, eventType, orderID)
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}

	log := logger.New("order-status-service", cfg.Common.LogLevel)

	// --- Postgres ---
	ctxDB, cancelDB := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelDB()

	db, err := pgxpool.New(ctxDB, cfg.Postgres.DSN)
	if err != nil {
		log.Fatal().Err(err).Msg("pg connect failed")
	}
	defer db.Close()

	pgOrders := &repo.OrdersPG{DB: db}
	eventsRepo := &repo.ProcessedEventsPG{DB: db}

	// --- Redis (optional cache) ---
	var ordersStore OrdersStore = pgOrders // fallback по умолчанию

	rdb := cache.New(cfg.Redis.Addr)
	defer func() { _ = rdb.Close() }()

	redisOK := false
	deadline := time.Now().Add(30 * time.Second)
	for {
		pingCtx, pingCancel := context.WithTimeout(context.Background(), 2*time.Second)
		err := rdb.Ping(pingCtx)
		pingCancel()

		if err == nil {
			redisOK = true
			break
		}
		if time.Now().After(deadline) {
			log.Warn().Err(err).Msg("redis unavailable, continue without cache")
			break
		}
		time.Sleep(1 * time.Second)
	}

	if redisOK {
		ordersStore = &repo.OrdersCached{
			PG:    pgOrders,
			Redis: rdb,
			TTL:   10 * time.Minute,
		}
	}

	statusRepo := &StatusRepo{
		Orders: ordersStore,
		Events: eventsRepo,
	}

	// --- RabbitMQ ---
	rc, err := rabbit.Connect(cfg.Rabbit.URL)
	if err != nil {
		log.Fatal().Err(err).Msg("rabbit connect failed")
	}
	defer func() { _ = rc.Close() }()

	if err := rabbit.DeclareBase(rc.Ch); err != nil {
		log.Fatal().Err(err).Msg("declare base failed")
	}

	// status queue (слушаем все события)
	if err := rabbit.DeclareQueueWithDLQ(rc.Ch, rabbit.QueueSpec{
		Name:     "status.q",
		BindKeys: []string{"#"},
		DLQ:      "status.dlq",
		Prefetch: 50,
	}); err != nil {
		log.Fatal().Err(err).Msg("declare status topology failed")
	}

	// retry queues (TTL 5s) для известных routing keys
	keys := []string{
		"orders.created",

		"inventory.reserved",
		"inventory.failed",
		"inventory.release_requested",
		"inventory.released",

		"payment.processed",
		"payment.failed",

		"shipping.scheduled",

		"order.cancelled",
		"order.completed",
	}

	for _, rk := range keys {
		qName := "status.retry." + rk + ".5s"
		bindKey := "status." + rk
		if err := rabbit.DeclareRetryQueue(rc.Ch, qName, bindKey, rk, 5000); err != nil {
			log.Fatal().Err(err).Str("rk", rk).Msg("declare status retry failed")
		}
	}

	deliveries, err := rabbit.NewConsumer(rc.Ch).Consume("status.q", 50)
	if err != nil {
		log.Fatal().Err(err).Msg("consume failed")
	}

	// --- Worker ---
	appCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	w := &worker.Consumer{
		Log:         log,
		Repo:        statusRepo,
		RetryPub:    rabbit.NewPublisher(rc.Ch, rabbit.ExchangeRetry),
		DLQPub:      rabbit.NewPublisher(rc.Ch, rabbit.ExchangeDLX),
		Service:     "status",
		MaxAttempts: 5,
		DLQKey:      "status.dlq",
	}
	go w.Run(appCtx, deliveries)

	// --- HTTP ---
	getOrder := &handlers.GetOrderHandler{
		GetStatus: func(r *http.Request, orderID string) (string, error) {
			return ordersStore.GetStatus(r.Context(), orderID)
		},
	}

	router := apihttp.NewRouter(&apihttp.Handlers{
		Health:   handlers.Health,
		GetOrder: getOrder.ServeHTTP,
	})

	srv := &http.Server{
		Addr:              cfg.HTTP.Addr, // HTTP_ADDR=:8090
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Info().Str("addr", srv.Addr).Msg("http started")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("http failed")
		}
	}()

	// --- Graceful shutdown ---
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	log.Info().Msg("shutdown...")

	shCtx, shCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shCancel()
	_ = srv.Shutdown(shCtx)
	cancel()
}
