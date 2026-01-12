package models

import (
	"encoding/json"
	"time"
)

type EventRaw struct {
	ID      string          `json:"id"`
	Type    string          `json:"type"`
	Version int             `json:"version"`
	Time    time.Time       `json:"time"`
	TraceID string          `json:"trace_id,omitempty"`
	OrderID string          `json:"order_id"`
	Payload json.RawMessage `json:"payload"`
}
