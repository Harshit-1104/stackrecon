# Build stage
FROM golang:alpine AS builder
WORKDIR /app

# Install dependencies first (for better caching)
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source code
COPY . .

# Build the Go application
RUN CGO_DISABLED=1 GOOS=linux go build -o /app/api-server ./cmd/api

# Runtime stage
FROM alpine:latest
WORKDIR /app

# Create a non-root user
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

# Copy necessary files from the builder stage
COPY --from=builder /app/api-server .
COPY --from=builder /app/alias_map.json .
COPY --from=builder /app/skill_blocklist_resolved.json .

# Ensure permissions
RUN chown -R appuser:appgroup /app

# Switch to the non-root user
USER appuser

EXPOSE 8080

CMD ["./api-server"]
