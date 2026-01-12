package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"ecommerce-order-system/shared/pkg/config"
	"ecommerce-order-system/shared/pkg/logger"
	"ecommerce-order-system/shared/pkg/rabbit"
	"ecommerce-order-system/shipping-service/internal/worker"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}

	log := logger.New("shipping-service", cfg.Common.LogLevel)

	rc, err := rabbit.Connect(cfg.Rabbit.URL)
	if err != nil {
		log.Fatal().Err(err).Msg("rabbit connect failed")
	}
	defer func() { _ = rc.Close() }()

	if err := rabbit.DeclareBase(rc.Ch); err != nil {
		log.Fatal().Err(err).Msg("declare base failed")
	}

	if err := rabbit.DeclareQueueWithDLQ(rc.Ch, rabbit.QueueSpec{
		Name:     "shipping.q",
		BindKeys: []string{"payment.processed"},
		DLQ:      "shipping.dlq",
		Prefetch: 10,
	}); err != nil {
		log.Fatal().Err(err).Msg("declare shipping topology failed")
	}

	if err := rabbit.DeclareRetryQueue(rc.Ch, "shipping.retry.payment.processed.5s", "shipping.payment.processed", "payment.processed", 5000); err != nil {
		log.Fatal().Err(err).Msg("declare shipping retry failed")
	}

	deliveries, err := rabbit.NewConsumer(rc.Ch).Consume("shipping.q", 10)
	if err != nil {
		log.Fatal().Err(err).Msg("consume failed")
	}

	w := &worker.Consumer{
		Log:         log,
		EventsPub:   rabbit.NewPublisher(rc.Ch, rabbit.ExchangeEvents),
		RetryPub:    rabbit.NewPublisher(rc.Ch, rabbit.ExchangeRetry),
		DLQPub:      rabbit.NewPublisher(rc.Ch, rabbit.ExchangeDLX),
		Service:     "shipping",
		MaxAttempts: 5,
		DLQKey:      "shipping.dlq",
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Run(ctx, deliveries)

	log.Info().Msg("shipping worker started")

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	log.Info().Msg("shutdown")
	cancel()
}
