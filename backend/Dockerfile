# Build Stage
FROM golang:alpine AS builder

WORKDIR /app

# Copy Go mod files and resolve dependencies first
COPY go.mod ./
# RUN go mod download # Uncomment once go.sum exists

COPY . .

# Fetch dependencies
RUN go get github.com/jackc/pgx/v5/pgxpool
RUN go get github.com/jackc/pgx/v5/stdlib
RUN go get github.com/redis/go-redis/v9
RUN go get github.com/golang-migrate/migrate/v4/database/pgx/v5
RUN go get github.com/golang-migrate/migrate/v4/source/file
RUN go mod tidy

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -o mkauth ./cmd/api

# Final Stage (Minimal)
FROM alpine:latest

WORKDIR /app
COPY --from=builder /app/mkauth .
COPY --from=builder /app/db ./db

EXPOSE 8080
CMD ["./mkauth"]
