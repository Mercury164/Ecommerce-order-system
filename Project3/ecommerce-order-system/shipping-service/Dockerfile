# syntax=docker/dockerfile:1

FROM --platform=linux/amd64 golang:1.24 AS build
WORKDIR /src

# 1) Кешируем зависимости
COPY shared/go.mod shared/go.sum ./shared/
COPY shipping-service/go.mod shipping-service/go.sum ./shipping-service/

WORKDIR /src/shipping-service
RUN go mod download

# 2) Копируем исходники
WORKDIR /src
COPY shared/ ./shared/
COPY shipping-service/ ./shipping-service/

# 3) Добиваем суммы (транзитивные зависимости shared/pkg/config и т.д.)
WORKDIR /src/shipping-service
RUN go mod tidy

# 4) Сборка cmd/worker
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -mod=mod -o /worker ./cmd/worker

# ---- runtime ----
FROM --platform=linux/amd64 alpine:3.20
RUN adduser -D -H -u 65532 appuser
COPY --from=build /worker /worker
RUN chmod +x /worker
USER 65532
ENTRYPOINT ["/worker"]
