# Build Stage
FROM golang:alpine AS builder

# The context is the repository root, not backend/: go.mod replaces
# syndra/addons/strictjson with ../addons/strictjson (the JSON decoder the
# add-ons share), so the build needs that module beside this one. The root
# .dockerignore keeps the context to the two.
WORKDIR /src/backend

# Copy Go mod files and resolve dependencies first
COPY addons/strictjson /src/addons/strictjson
COPY backend/go.mod ./
# RUN go mod download # Uncomment once go.sum exists

COPY backend .

# Fetch dependencies
RUN go get github.com/jackc/pgx/v5/pgxpool
RUN go get github.com/jackc/pgx/v5/stdlib
RUN go get github.com/redis/go-redis/v9
RUN go get github.com/golang-migrate/migrate/v4/database/pgx/v5
RUN go get github.com/golang-migrate/migrate/v4/source/file
RUN go mod tidy

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -o syndra ./cmd/api
# Staging's stand-in Zitadel: run on production nightly, reads only.
RUN CGO_ENABLED=0 GOOS=linux go build -o syndra-world-export ./cmd/syndra-world-export

# Final Stage (Minimal)
FROM alpine:latest

WORKDIR /app
COPY --from=builder /src/backend/syndra .
COPY --from=builder /src/backend/syndra-world-export .
COPY --from=builder /src/backend/db ./db

EXPOSE 8080
CMD ["./syndra"]
