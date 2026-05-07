FROM golang:1.21-alpine AS builder

WORKDIR /build

# Install dependencies
RUN apk add --no-cache git gcc musl-dev

# Copy go mod files
COPY api-server/go.mod api-server/go.sum ./

# Download dependencies
RUN go mod download

# Copy source
COPY api-server/ ./

# Build
RUN CGO_ENABLED=1 GOOS=linux go build -o api-server .

# Final stage
FROM alpine:latest

RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY --from=builder /build/api-server .

EXPOSE 8080

CMD ["./api-server"]
