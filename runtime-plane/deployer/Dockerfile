FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o deployer .

FROM alpine:3.19
RUN apk add --no-cache curl bash
# Install kubectl
RUN curl -LO "https://dl.k8s.io/release/v1.28.0/bin/linux/amd64/kubectl" && \
    chmod +x kubectl && mv kubectl /usr/local/bin/
WORKDIR /app
COPY --from=builder /app/deployer .
CMD ["./deployer"]
