FROM golang:1.24 AS build
WORKDIR /src

# Сначала только mod/sum для кеша
COPY shared/go.mod shared/go.sum ./shared/
COPY api-gateway/go.mod api-gateway/go.sum ./api-gateway/

# Важно: WORKDIR в модуле, чтобы replace ../shared работал
WORKDIR /src/api-gateway
RUN go mod download

# Потом исходники
WORKDIR /src
COPY shared/ ./shared/
COPY api-gateway/ ./api-gateway/

WORKDIR /src/api-gateway
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/app ./cmd/app

FROM gcr.io/distroless/static:nonroot
COPY --from=build /bin/app /app
USER nonroot:nonroot
ENTRYPOINT ["/app"]
