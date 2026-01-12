# ecommerce-order-system (RabbitMQ + Outbox + Saga)

Микросервисная система обработки заказов: API принимает заказ, сохраняет в Postgres и кладёт событие в Outbox. Outbox-worker публикует события в RabbitMQ, остальные сервисы обрабатывают асинхронно. Есть retry + DLQ, идемпотентность (processed events), saga-компенсация и terminal-guard (completed/cancelled не перезатираются).

## Архитектура (коротко)

Client → **api-gateway (HTTP)** → Postgres (orders + outbox)  
**outbox-worker** → RabbitMQ exchange (topic) → workers:
- **inventory-service**: reserve / release (компенсация)
- **payment-service**: processed / failed (+ compensation events)
- **shipping-service**: scheduled + completed
- **order-status-service**: агрегирует статусы в Postgres (+ terminal guard)

## Сервисы и порты

- api-gateway: `http://localhost:8080`
- order-status-service: `http://localhost:8090`
- outbox-worker (health/metrics/pending): `http://localhost:8085`
- RabbitMQ UI: `http://localhost:15672` (обычно `guest/guest`)
- Prometheus: `http://localhost:9090`
- Grafana: `http://localhost:3000` (обычно `admin/admin`)

## Основные события (routing keys)

- `orders.created`
- `inventory.reserved`, `inventory.failed`
- `payment.processed`, `payment.failed`
- `shipping.scheduled`
- `order.completed`, `order.cancelled`
- компенсация: `inventory.release_requested`, `inventory.released`
- retry / dlq на уровне Rabbit (через retry-exchange + DLX)

## Запуск

```powershell
docker compose up -d --build
docker compose ps
