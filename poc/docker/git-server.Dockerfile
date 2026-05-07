FROM golang:1.21-alpine AS builder

WORKDIR /build

# Install dependencies
RUN apk add --no-cache git openssh-keygen

# Copy go mod files
COPY git-server/go.mod git-server/go.sum ./

# Download dependencies
RUN go mod download

# Copy source
COPY git-server/ ./

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -o git-server .

# Final stage
FROM alpine:latest

RUN apk add --no-cache ca-certificates git openssh-keygen curl bash

WORKDIR /app

COPY --from=builder /build/git-server .

# Generate host key
RUN ssh-keygen -t ed25519 -f ssh_host_key -N ""

EXPOSE 2222

CMD ["./git-server"]
