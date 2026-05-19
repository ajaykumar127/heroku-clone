FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY . .
RUN go build -o agent .

FROM alpine:3.19
RUN apk add --no-cache git curl bash kubectl
WORKDIR /app
COPY --from=builder /app/agent .
CMD ["./agent"]
