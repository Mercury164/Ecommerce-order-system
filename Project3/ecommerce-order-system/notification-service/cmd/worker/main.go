package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"ecommerce-order-system/notification-service/internal/worker"
	"ecommerce-order-system/shared/pkg/config"
	"ecommerce-order-system/shared/pkg/logger"
	"ecommerce-order-system/shared/pkg/rabbit"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}

	log := logger.New("notification-service", cfg.Common.LogLevel)

	rc, err := rabbit.Connect(cfg.Rabbit.URL)
	if err != nil {
		log.Fatal().Err(err).Msg("rabbit connect failed")
	}
	defer func() { _ = rc.Close() }()

	if err := rabbit.DeclareBase(rc.Ch); err != nil {
		log.Fatal().Err(err).Msg("declare base failed")
	}

	if err := rabbit.DeclareQueueWithDLQ(rc.Ch, rabbit.QueueSpec{
		Name:     "notification.q",
		BindKeys: []string{"#"},
		DLQ:      "notification.dlq",
		Prefetch: 50,
	}); err != nil {
		log.Fatal().Err(err).Msg("declare notification topology failed")
	}

	deliveries, err := rabbit.NewConsumer(rc.Ch).Consume("notification.q", 50)
	if err != nil {
		log.Fatal().Err(err).Msg("consume failed")
	}

	w := &worker.Consumer{Log: log}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.Run(ctx, deliveries)

	log.Info().Msg("notification worker started")

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	log.Info().Msg("shutdown")
	cancel()
}
