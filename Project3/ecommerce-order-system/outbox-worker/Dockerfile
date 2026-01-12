# syntax=docker/dockerfile:1

FROM --platform=linux/amd64 golang:1.24 AS build
WORKDIR /src

COPY shared/go.mod shared/go.sum ./shared/
COPY outbox-worker/go.mod outbox-worker/go.sum ./outbox-worker/

WORKDIR /src/outbox-worker
RUN go mod download

WORKDIR /src
COPY shared/ ./shared/
COPY outbox-worker/ ./outbox-worker/

WORKDIR /src/outbox-worker
RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -mod=mod -o /worker ./cmd/worker

FROM --platform=linux/amd64 alpine:3.20
RUN adduser -D -H -u 65532 appuser
COPY --from=build /worker /worker
RUN chmod +x /worker
USER 65532
ENTRYPOINT ["/worker"]
