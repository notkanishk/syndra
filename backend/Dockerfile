# Build Stage
FROM golang:1.27.4-alpine3.24 AS builder

# The context is the repository root, not backend/: go.mod replaces
# syndra/addons/strictjson with ../addons/strictjson (the JSON decoder the
# add-ons share), so the build needs that module beside this one. The root
# .dockerignore keeps the context to the two.
WORKDIR /src/backend

# Exactly what go.mod and go.sum pin, downloaded before the source so an edit
# does not re-download the module graph. Never `go get` or `go mod tidy`
# here: either resolves versions at build time, so the image would build
# code CI never tested (repoguard TestNoImageResolvesItsOwnDependencies).
COPY addons/strictjson/go.mod /src/addons/strictjson/go.mod
COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY addons/strictjson /src/addons/strictjson
COPY backend .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -o syndra ./cmd/api
# Staging's stand-in Zitadel: run on production nightly, reads only.
RUN CGO_ENABLED=0 GOOS=linux go build -o syndra-world-export ./cmd/syndra-world-export

# Final Stage (Minimal)
FROM alpine:3.24.4

WORKDIR /app
COPY --from=builder /src/backend/syndra .
COPY --from=builder /src/backend/syndra-world-export .
COPY --from=builder /src/backend/db ./db

EXPOSE 8080
CMD ["./syndra"]
