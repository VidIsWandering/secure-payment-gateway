# ---- Build Stage ----
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /app

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build static binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o /app/server ./cmd/api

# ---- Runtime Stage ----
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata

# Non-root user
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

WORKDIR /app

COPY --from=builder /app/server .
COPY --from=builder /app/config/config.yaml ./config/config.yaml

RUN chown -R appuser:appgroup /app

USER appuser

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --retries=3 \
    CMD wget -qO- http://localhost:8080/health || exit 1

ENTRYPOINT ["./server"]
