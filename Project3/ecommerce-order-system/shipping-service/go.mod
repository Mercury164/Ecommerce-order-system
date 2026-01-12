module ecommerce-order-system/shipping-service

go 1.24

require (
	ecommerce-order-system/shared v0.0.0
	github.com/rabbitmq/amqp091-go v1.10.0
	github.com/rs/zerolog v1.33.0
)

require (
	github.com/caarlos0/env/v11 v11.1.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.19 // indirect
	golang.org/x/sys v0.12.0 // indirect
)

replace ecommerce-order-system/shared => ../shared
