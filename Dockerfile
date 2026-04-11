# Stage 1: Build
FROM golang:1.22-alpine AS builder

WORKDIR /app

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Copy go module files first for layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w -extldflags '-static'" \
    -o /app/bin/notification-service \
    ./cmd/notification/...

# Stage 2: Final distroless image
FROM gcr.io/distroless/static-debian12:nonroot

# Copy timezone data and CA certs from builder
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Copy binary
COPY --from=builder /app/bin/notification-service /notification-service

# Copy migrations
COPY --from=builder /app/migrations /migrations

EXPOSE 8080

USER nonroot:nonroot

ENTRYPOINT ["/notification-service"]
