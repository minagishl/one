# Multi-stage build for minimal image size

# Frontend build stage
FROM node:18-alpine AS frontend-builder

WORKDIR /app/frontend

# Copy package.json and package-lock.json
COPY frontend/package*.json ./

# Install dependencies
RUN npm ci

# Copy frontend source
COPY frontend/ .

# Build frontend
RUN npm run build

# Backend build stage
FROM golang:1.23-alpine AS backend-builder

# Install build dependencies
RUN apk add --no-cache git ca-certificates tzdata

# Set working directory
WORKDIR /app/backend

# Copy go mod files
COPY backend/go.mod backend/go.sum ./

# Download dependencies
RUN go mod download

# Copy backend source code
COPY backend/ .

# Build the application with optimizations
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-w -s' -o main .

# Final stage - minimal runtime image
FROM alpine:latest

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates tzdata

# Create non-root user
RUN addgroup -g 1000 appuser && \
    adduser -D -s /bin/sh -u 1000 -G appuser appuser

# Set working directory
WORKDIR /app

# Copy binary from builder stage
COPY --from=backend-builder /app/backend/main .

# Copy schema.sql file from builder stage
COPY --from=backend-builder /app/backend/schema.sql .

# Copy built frontend from frontend builder
COPY --from=frontend-builder /app/static ./static

# Create temp directory for file uploads in /app/temp (persistent) as root
RUN mkdir -p /app/temp && \
    mkdir -p /app/temp/files && \
    chmod 755 /app/temp

# Change ownership to non-root user (including temp directory)
RUN chown -R appuser:appuser /app

# Switch to non-root user
USER appuser

# Set environment variable for temp directory
ENV TEMP_DIR=/app/temp

# Expose port
EXPOSE 8080

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/ || exit 1

# Run the application
CMD ["./main"]