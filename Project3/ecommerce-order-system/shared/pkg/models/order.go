package models

import "time"

type Order struct {
	ID         string
	UserID     string
	Email      string
	Status     OrderStatus
	TotalCents int
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type OrderItem struct {
	SKU        string
	Qty        int
	PriceCents int
}
