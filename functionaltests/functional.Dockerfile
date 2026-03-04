FROM golang:1.24-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . ./
RUN go build -o ./out/server ./cmd/server
RUN go build -o ./out/backfill-aliases ./cmd/backfill-aliases
RUN go build -o ./out/migrate ./cmd/migrate

FROM alpine
WORKDIR /app
COPY --from=builder /app/out /app/bin
COPY --from=builder /app/functionaltests/google-test-credentials.json /app/google-test-credentials.json
