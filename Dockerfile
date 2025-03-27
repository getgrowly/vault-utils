FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY auto-unseal-controller.go .
RUN go build -o auto-unseal

FROM alpine:3.19
WORKDIR /app
COPY --from=builder /app/auto-unseal .
ENTRYPOINT ["/app/auto-unseal"] 