FROM golang:1.23-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . ./
RUN go build -o ./out/server ./cmd/server

FROM alpine
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /app/out /app/bin
