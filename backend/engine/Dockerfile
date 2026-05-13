# ---------- Stage 1: Build ----------
FROM golang:1.25 AS builder

WORKDIR /app

# Install required tools
RUN apt-get update && apt-get install -y \
    git \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Set Go proxy (important for reliability)
RUN go env -w GOPROXY=https://proxy.golang.org,direct

# Copy only dependency files first
COPY go.mod go.sum ./

# Download dependencies
RUN  go mod download

# Copy full source
COPY . .

# Build binary
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -o omnimarket-engine ./cmd/server

# ---------- Stage 2: Run ----------
FROM debian:bookworm-slim

WORKDIR /

# Install runtime certs
RUN apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*

# Copy binary
COPY --from=builder /app/omnimarket-engine /omnimarket-engine

EXPOSE 8080

CMD ["/omnimarket-engine"]