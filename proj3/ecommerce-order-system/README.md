# ecommerce-order-system (RabbitMQ + Outbox)

Микросервисная система обработки заказов на Go с RabbitMQ (topic exchange), Postgres и паттерном **Transactional Outbox**.

## Сервисы
- **api-gateway** (8080): REST `POST /api/v1/orders`, `GET /api/v1/orders/{id}`, `/metrics`, `/health`
- **outbox-worker** (8085): публикует события из `outbox_events` в RabbitMQ
- **inventory-service**: `orders.created` -> `inventory.reserved`, + компенсация `inventory.release_requested` -> `inventory.released`
- **payment-service**: `inventory.reserved` -> `payment.processed` или `payment.failed` + `inventory.release_requested` + `order.cancelled`
- **shipping-service**: `payment.processed` -> `shipping.scheduled` + `order.completed`
- **order-status-service** (8090): слушает все события и обновляет статус заказа в Postgres

## Запуск
```bash
docker compose up -d --build
```

RabbitMQ UI: http://localhost:15672 (guest/guest)  
Grafana: http://localhost:3000 (admin/admin)  
Prometheus: http://localhost:9090

## Тест
PowerShell:

```powershell
$resp = Invoke-RestMethod -Method Post `
  -Uri http://localhost:8080/api/v1/orders `
  -ContentType "application/json" `
  -Body '{"user_id":"u1","email":"u1@test.com","items":[{"sku":"SKU1","qty":1,"price_cents":1000}]}'

$resp

Invoke-RestMethod "http://localhost:8090/api/v1/orders/$($resp.id)"
```

Посмотреть pending outbox:
```powershell
Invoke-RestMethod http://localhost:8085/outbox/pending
```

Логи:
```powershell
docker compose logs -f api-gateway outbox-worker inventory-service payment-service shipping-service order-status-service
```
