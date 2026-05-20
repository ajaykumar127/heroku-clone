FROM golang:1.21.13-alpine3.20 AS builder
WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o builder .

FROM ubuntu:22.04
# NOTE: builder runs as root because it requires Docker socket access.
# docker.io is installed so the builder binary can invoke the Docker CLI
# to push images to the registry after pack builds them.
RUN apt-get update && apt-get install -y git curl docker.io && rm -rf /var/lib/apt/lists/*
# Install pack CLI with official SHA256 verification
RUN PACK_VERSION="v0.33.2" \
    && curl -sSL "https://github.com/buildpacks/pack/releases/download/${PACK_VERSION}/pack-${PACK_VERSION}-linux.tgz" \
       -o /tmp/pack.tgz \
    && curl -sSL "https://github.com/buildpacks/pack/releases/download/${PACK_VERSION}/pack-${PACK_VERSION}-linux.tgz.sha256" \
       -o /tmp/pack.tgz.sha256 \
    && (cd /tmp && sha256sum -c pack.tgz.sha256) \
    && tar -xz -C /usr/local/bin -f /tmp/pack.tgz \
    && rm /tmp/pack.tgz /tmp/pack.tgz.sha256
WORKDIR /app
COPY --from=builder /app/builder .
CMD ["./builder"]
