docker compose up -d --build

Write-Host "Create order..."
$resp = Invoke-RestMethod -Method Post `
  -Uri http://localhost:8080/api/v1/orders `
  -ContentType "application/json" `
  -Body '{"user_id":"u1","email":"u1@test.com","items":[{"sku":"SKU1","qty":1,"price_cents":1000}]}' 

$resp

Write-Host "Order status (order-status-service)..."
Invoke-RestMethod "http://localhost:8090/api/v1/orders/$($resp.id)"

Write-Host "Pending outbox..."
Invoke-RestMethod http://localhost:8085/outbox/pending
