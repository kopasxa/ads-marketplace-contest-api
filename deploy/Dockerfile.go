FROM golang:1.24-alpine AS builder

ARG SERVICE=api

WORKDIR /app

# Dependencies
COPY go.mod go.sum ./
RUN go mod download

# Source
COPY . .

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/service ./cmd/${SERVICE}

# Runtime
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /app/service .
COPY --from=builder /app/migrations ./migrations

EXPOSE 3000

CMD ["./service"]
