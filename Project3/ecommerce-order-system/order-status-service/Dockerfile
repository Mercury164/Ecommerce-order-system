FROM golang:1.24 AS build
WORKDIR /src

COPY shared/go.mod shared/go.sum ./shared/
COPY order-status-service/go.mod order-status-service/go.sum ./order-status-service/

WORKDIR /src/order-status-service
RUN go mod download

WORKDIR /src
COPY shared/ ./shared/
COPY order-status-service/ ./order-status-service/

WORKDIR /src/order-status-service
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/app ./cmd/app

FROM gcr.io/distroless/static:nonroot
COPY --from=build /bin/app /app
USER nonroot:nonroot
ENTRYPOINT ["/app"]
