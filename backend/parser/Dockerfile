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
RUN cd parser && CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /parser ./cmd/main.go

# Runtime stage - using full Chrome for better anti-detection bypass
FROM zenika/alpine-chrome:with-chromedriver

USER root

WORKDIR /app

# Install ca-certificates and dumb-init
RUN apk add --no-cache ca-certificates dumb-init

# Copy binary from builder
COPY --from=builder /parser .

# Use dumb-init as entrypoint for proper signal handling
ENTRYPOINT ["dumb-init", "--"]

# Run the application
CMD ["./parser"]
