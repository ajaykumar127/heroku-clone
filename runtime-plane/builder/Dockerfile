FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o builder .

FROM ubuntu:22.04
RUN apt-get update && apt-get install -y git curl docker.io && rm -rf /var/lib/apt/lists/*
# Install pack CLI
RUN curl -sSL "https://github.com/buildpacks/pack/releases/download/v0.33.2/pack-v0.33.2-linux.tgz" | tar -xz -C /usr/local/bin
WORKDIR /app
COPY --from=builder /app/builder .
CMD ["./builder"]
