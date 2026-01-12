package service

import (
	"context"
	"time"

	"ecommerce-order-system/shared/pkg/models"
	"ecommerce-order-system/shared/pkg/rabbit"

	amqp "github.com/rabbitmq/amqp091-go"
)

type OrdersRepo interface {
	Create(ctx context.Context, o models.Order, items []models.OrderItem) (string, error)
}

type OrdersService struct {
	Repo      OrdersRepo
	Publisher *rabbit.Publisher
}

type CreateOrderInput struct {
	UserID string
	Email  string
	Items  []struct {
		SKU        string `json:"sku"`
		Qty        int    `json:"qty"`
		PriceCents int    `json:"price_cents"`
	}
}

func (s *OrdersService) CreateOrder(ctx context.Context, in CreateOrderInput) (string, error) {
	total := 0
	items := make([]models.OrderItem, 0, len(in.Items))
	for _, it := range in.Items {
		total += it.PriceCents * it.Qty
		items = append(items, models.OrderItem{
			SKU:        it.SKU,
			Qty:        it.Qty,
			PriceCents: it.PriceCents,
		})
	}

	order := models.Order{
		UserID:     in.UserID,
		Email:      in.Email,
		Status:     models.OrderStatusCreated,
		TotalCents: total,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	orderID, err := s.Repo.Create(ctx, order, items)
	if err != nil {
		return "", err
	}

	evt := models.NewOrderCreatedEvent(orderID, in.UserID, in.Email, total, in.Items)

	pubCtx, cancel := rabbit.WithTimeout(ctx)
	defer cancel()

	if err := s.Publisher.PublishJSON(pubCtx, "orders.created", evt, amqp.Table{
		"x-attempts": int32(0),
	}); err != nil {
		return "", err
	}

	return orderID, nil
}
