FROM golang:1.24 AS build
WORKDIR /src

COPY shared/go.mod shared/go.sum ./shared/
COPY inventory-service/go.mod inventory-service/go.sum ./inventory-service/

WORKDIR /src/inventory-service
RUN go mod download

WORKDIR /src
COPY shared/ ./shared/
COPY inventory-service/ ./inventory-service/

WORKDIR /src/inventory-service
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/worker ./cmd/worker

FROM gcr.io/distroless/static:nonroot
COPY --from=build /bin/worker /worker
USER nonroot:nonroot
ENTRYPOINT ["/worker"]
