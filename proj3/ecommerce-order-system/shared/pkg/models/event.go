package models

import "time"

type Event[T any] struct {
	ID      string    `json:"id"`
	Type    string    `json:"type"`
	Version int       `json:"version"`
	Time    time.Time `json:"time"`
	OrderID string    `json:"order_id"`
	Payload T         `json:"payload"`
}
