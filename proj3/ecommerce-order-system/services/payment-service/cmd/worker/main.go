package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"ecommerce-order-system/services/payment-service/internal/worker"
	"ecommerce-order-system/shared/pkg/config"
	"ecommerce-order-system/shared/pkg/logger"
	"ecommerce-order-system/shared/pkg/rabbit"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}
	log := logger.New("payment-service", cfg.Common.LogLevel)

	rc, err := rabbit.Connect(cfg.Rabbit.URL)
	if err != nil {
		log.Fatal().Err(err).Msg("rabbit connect failed")
	}
	defer func() { _ = rc.Close() }()

	if err := rabbit.DeclareBase(rc.Ch); err != nil {
		log.Fatal().Err(err).Msg("declare base failed")
	}

	if err := rabbit.DeclareQueueWithDLQ(rc.Ch, rabbit.QueueSpec{
		Name:     "payment.q",
		BindKeys: []string{"inventory.reserved"},
		DLQKey:   "payment.dlq",
		Prefetch: 20,
	}); err != nil {
		log.Fatal().Err(err).Msg("declare payment topology failed")
	}

	_ = rabbit.DeclareRetryQueue(rc.Ch, "payment.retry.inventory.reserved.5s", "payment.inventory.reserved", "inventory.reserved", 5000)

	deliveries, err := rabbit.NewConsumer(rc.Ch).Consume("payment.q", 20)
	if err != nil {
		log.Fatal().Err(err).Msg("consume failed")
	}

	w := &worker.Consumer{
		Log:         log,
		EventsPub:   rabbit.NewPublisher(rc.Ch, rabbit.ExchangeEvents),
		RetryPub:    rabbit.NewPublisher(rc.Ch, rabbit.ExchangeRetry),
		DLQPub:      rabbit.NewPublisher(rc.Ch, rabbit.ExchangeDLX),
		Service:     "payment",
		MaxAttempts: 5,
		DLQKey:      "payment.dlq",
		FailRate:    cfg.Payment.FailRate,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Run(ctx, deliveries)

	log.Info().Msg("payment worker started")

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	log.Info().Msg("shutdown")
	cancel()
}
