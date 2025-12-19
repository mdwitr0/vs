# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install dependencies
RUN apk add --no-cache git ca-certificates

# Copy go.work and all modules (go.work requires all modules)
COPY go.work go.work.sum ./
COPY go.mod go.sum ./
COPY pkg/ pkg/

# Copy all module definitions for go.work
COPY indexer/go.mod indexer/go.sum ./indexer/
COPY parser/go.mod parser/go.sum ./parser/

# Download dependencies
RUN go mod download

# Copy source code
COPY indexer/ indexer/
COPY parser/ parser/

# Build the application
RUN cd indexer && CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /indexer ./cmd/main.go

# Runtime stage
FROM alpine:3.21

WORKDIR /app

# Install ca-certificates for HTTPS requests
RUN apk add --no-cache ca-certificates tzdata

# Copy binary from builder
COPY --from=builder /indexer .

# Expose port
EXPOSE 8080

# Run the application
CMD ["./indexer"]
