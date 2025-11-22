FROM golang:1.22-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . ./
RUN go build -o ./out/server ./cmd/server

FROM alpine
WORKDIR /app
COPY --from=builder /app/out /app/bin
