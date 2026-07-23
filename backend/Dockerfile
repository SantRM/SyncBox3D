# ----- builder -----
FROM golang:1.23-alpine AS builder
WORKDIR /src

RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum* ./
RUN go mod download

COPY . .

# Compilación estática para imagen mínima.
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api

# ----- runtime -----
FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata && \
    addgroup -S app && adduser -S app -G app && \
    mkdir -p /data/syncbox/recursos && chown -R app:app /data/syncbox
WORKDIR /app
COPY --from=builder /out/api /app/api
USER app
EXPOSE 8080
HEALTHCHECK --interval=15s --timeout=3s --start-period=10s --retries=5 \
    CMD wget -qO- http://127.0.0.1:8080/health/ready || exit 1
ENTRYPOINT ["/app/api"]
