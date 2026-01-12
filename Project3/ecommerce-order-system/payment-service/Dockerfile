FROM golang:1.24 AS build
WORKDIR /src

# mod/sum для кеша зависимостей
COPY shared/go.mod shared/go.sum ./shared/
COPY payment-service/go.mod payment-service/go.sum ./payment-service/

WORKDIR /src/payment-service
RUN go mod download

# исходники
WORKDIR /src
COPY shared/ ./shared/
COPY payment-service/ ./payment-service/

WORKDIR /src/payment-service
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/worker ./cmd/worker

FROM gcr.io/distroless/static:nonroot
COPY --from=build /bin/worker /worker
USER nonroot:nonroot
ENTRYPOINT ["/worker"]
