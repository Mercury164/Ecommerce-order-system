package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"ecommerce-order-system/services/inventory-service/internal/worker"
	"ecommerce-order-system/shared/pkg/config"
	"ecommerce-order-system/shared/pkg/logger"
	"ecommerce-order-system/shared/pkg/rabbit"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}
	log := logger.New("inventory-service", cfg.Common.LogLevel)

	rc, err := rabbit.Connect(cfg.Rabbit.URL)
	if err != nil {
		log.Fatal().Err(err).Msg("rabbit connect failed")
	}
	defer func() { _ = rc.Close() }()

	if err := rabbit.DeclareBase(rc.Ch); err != nil {
		log.Fatal().Err(err).Msg("declare base failed")
	}

	// main queue + dlq
	if err := rabbit.DeclareQueueWithDLQ(rc.Ch, rabbit.QueueSpec{
		Name:     "inventory.q",
		BindKeys: []string{"orders.created", "inventory.release_requested"},
		DLQKey:   "inventory.dlq",
		Prefetch: 20,
	}); err != nil {
		log.Fatal().Err(err).Msg("declare inventory topology failed")
	}

	// retry per routing key
	_ = rabbit.DeclareRetryQueue(rc.Ch, "inventory.retry.orders.created.5s", "inventory.orders.created", "orders.created", 5000)
	_ = rabbit.DeclareRetryQueue(rc.Ch, "inventory.retry.inventory.release_requested.5s", "inventory.inventory.release_requested", "inventory.release_requested", 5000)

	deliveries, err := rabbit.NewConsumer(rc.Ch).Consume("inventory.q", 20)
	if err != nil {
		log.Fatal().Err(err).Msg("consume failed")
	}

	w := &worker.Consumer{
		Log:         log,
		EventsPub:   rabbit.NewPublisher(rc.Ch, rabbit.ExchangeEvents),
		RetryPub:    rabbit.NewPublisher(rc.Ch, rabbit.ExchangeRetry),
		DLQPub:      rabbit.NewPublisher(rc.Ch, rabbit.ExchangeDLX),
		Service:     "inventory",
		MaxAttempts: 5,
		DLQKey:      "inventory.dlq",
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Run(ctx, deliveries)

	log.Info().Msg("inventory worker started")

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	log.Info().Msg("shutdown")
	cancel()
}
