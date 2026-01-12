package http

import "net/http"

type Handlers struct {
	Health      http.HandlerFunc
	CreateOrder http.HandlerFunc
	GetOrder    http.HandlerFunc
}
