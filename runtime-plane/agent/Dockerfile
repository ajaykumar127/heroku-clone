FROM golang:1.21.13-alpine3.20 AS builder
WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY . .
RUN go build -o agent .

FROM alpine:3.19.1
RUN apk add --no-cache git curl bash
# Install kubectl with official SHA256 verification
RUN KUBECTL_VERSION="v1.28.0" \
    && curl -LO "https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/linux/amd64/kubectl" \
    && curl -LO "https://dl.k8s.io/release/${KUBECTL_VERSION}/bin/linux/amd64/kubectl.sha256" \
    && sha256sum -c kubectl.sha256 \
    && chmod +x kubectl \
    && mv kubectl /usr/local/bin/kubectl \
    && rm kubectl.sha256
RUN addgroup -S nonroot && adduser -S nonroot -G nonroot
WORKDIR /app
COPY --from=builder /app/agent .
USER 65534:65534
CMD ["./agent"]
