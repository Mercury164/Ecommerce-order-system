FROM golang:1.24 AS build
WORKDIR /src

COPY shared/go.mod shared/go.sum ./shared/
COPY notification-service/go.mod notification-service/go.sum ./notification-service/

WORKDIR /src/notification-service
RUN go mod download

WORKDIR /src
COPY shared/ ./shared/
COPY notification-service/ ./notification-service/

WORKDIR /src/notification-service
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/worker ./cmd/worker

FROM gcr.io/distroless/static:nonroot
COPY --from=build /bin/worker /worker
USER nonroot:nonroot
ENTRYPOINT ["/worker"]
