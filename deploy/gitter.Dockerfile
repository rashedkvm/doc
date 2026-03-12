# Build stage
FROM golang:1.23 AS builder

WORKDIR /app

# Copy dependency files first for better layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -o gitter -v ./cmd/gitter/

# Runtime stage
FROM alpine:3
RUN apk add --no-cache ca-certificates

# Create non-root user
RUN addgroup -g 1000 appgroup && \
    adduser -u 1000 -G appgroup -D appuser

WORKDIR /app

# Copy the binary from builder
COPY --from=builder /app/gitter ./

# Change ownership and switch to non-root user (use numeric UID for K8s runAsNonRoot)
RUN chown -R appuser:appgroup /app
USER 1000

ENTRYPOINT ["/app/gitter"]
