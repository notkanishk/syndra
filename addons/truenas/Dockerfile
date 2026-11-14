# The TrueNAS add-on. A target adapter behind Syndra's policy engine.
#
# It holds the NAS API key, which is why it is a separate container at all: in
# the backend's process, one memory disclosure would expose identity, storage
# and physical access together. Everything below follows from treating this as
# the least trusted component in the deployment.

FROM golang:1.25-alpine AS build
# The context is addons/, not addons/truenas: go.mod replaces
# syndra/addons/contract with ../contract (the request decoder both add-ons
# share), so the build needs that module beside this one.
WORKDIR /src/truenas

# Dependencies first, so a source edit does not re-download the module graph.
COPY contract/go.mod /src/contract/go.mod
COPY truenas/go.mod truenas/go.sum ./
RUN go mod download

COPY contract /src/contract
COPY truenas .
# CGO off for a static binary the scratch stage can run; trimpath so the image
# carries no build-host paths.
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/truenas-addon .

FROM alpine:3.22
# Certificates for verifying the NAS's TLS. TrueNAS ships a self-signed one, so
# a lab deployment sets TRUENAS_VERIFY_TLS=false explicitly — the roots are here
# so that stays a deliberate choice rather than the only option.
RUN apk add --no-cache ca-certificates \
 && addgroup -g 65532 -S addon \
 && adduser -u 65532 -S addon -G addon \
 && mkdir -p /var/lib/syndra-truenas \
 && chown addon:addon /var/lib/syndra-truenas

COPY --from=build /out/truenas-addon /usr/local/bin/truenas-addon

# Non-root, and its own uid. The state volume holds a mirror naming every
# account on the target and the mutation log names who did what to whom.
USER 65532:65532
VOLUME ["/var/lib/syndra-truenas"]

# No EXPOSE and no published port anywhere. The backend reaches this over the
# internal Compose network and nothing else should be able to.
ENTRYPOINT ["/usr/local/bin/truenas-addon"]
