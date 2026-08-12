#!/bin/bash
# Mint the transport material for the backend <-> add-on channel.
#
# One private CA, one server certificate for the add-on, one client certificate
# for the backend, and one signing key. Both authentication modes come out of a
# single run because the expensive half is shared: there is no plaintext mode,
# so signed mode still needs the add-on's server certificate and the CA that
# authenticates it. Choosing signed mode drops the client certificate and
# nothing else.
#
# Two output directories, never one. The add-on holds the NAS credential and is
# the least trusted container in the deployment; it must not be able to read the
# key that authenticates the backend to it.
#
# Re-runnable only on purpose: replacing the CA invalidates both leaves at once,
# and a half-replaced pair fails as a handshake error during whatever the
# operator was actually doing.
set -euo pipefail

cd "$(dirname "$0")/.."

command -v openssl >/dev/null || { echo "error: openssl not found" >&2; exit 1; }

# The add-on's identity on the Compose network. It is the SERVICE name because
# that is the name the backend dials in ADDON_TRUENAS_BASE_URL, and a
# certificate whose SAN does not contain the dialed name fails verification
# whatever else is right about it. Override only if the service is renamed.
ADDON_HOST="${ADDON_HOST:-truenas-addon}"
ADDON_DIR="${ADDON_SECRETS_DIR:-./secrets/truenas-addon}"
BACKEND_DIR="${BACKEND_SECRETS_DIR:-./secrets/backend}"
DAYS="${DAYS:-825}"

# The add-on runs as this uid (addons/truenas/Dockerfile) and the mount is
# read-only, so the files have to be readable by it or the container exits on a
# certificate it can see but not open.
ADDON_UID=65532

if [ -e "$ADDON_DIR/tls.crt" ] || [ -e "$BACKEND_DIR/truenas-client.crt" ]; then
  echo "error: add-on transport material already exists" >&2
  echo "       $ADDON_DIR/tls.crt or $BACKEND_DIR/truenas-client.crt" >&2
  echo "" >&2
  echo "       Re-running mints a new CA, which invalidates BOTH ends at once." >&2
  echo "       To rotate deliberately: set the add-on to LIFECYCLE_STATE=draining," >&2
  echo "       move this directory aside, re-run, then restart both containers." >&2
  exit 1
fi

mkdir -p "$ADDON_DIR" "$BACKEND_DIR"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

echo "==> private CA"
openssl req -x509 -newkey ec -pkeyopt ec_paramgen_curve:P-256 -nodes \
  -keyout "$WORK/ca.key" -out "$WORK/ca.crt" -days "$DAYS" \
  -subj "/CN=Syndra add-on CA" \
  -addext "basicConstraints=critical,CA:TRUE,pathlen:0" \
  -addext "keyUsage=critical,keyCertSign,cRLSign" 2>/dev/null

# Signed by the CA above rather than self-signed: the backend verifies the
# add-on against ADDON_TRUENAS_CA_CERT, and in mTLS mode the add-on verifies the
# backend against the same anchor. One CA, both directions.
issue() { # issue <name> <subject-cn> <extensions>
  openssl req -newkey ec -pkeyopt ec_paramgen_curve:P-256 -nodes \
    -keyout "$WORK/$1.key" -out "$WORK/$1.csr" \
    -subj "/CN=$2" 2>/dev/null
  printf '%s\n' "$3" > "$WORK/$1.ext"
  openssl x509 -req -in "$WORK/$1.csr" -CA "$WORK/ca.crt" -CAkey "$WORK/ca.key" \
    -CAcreateserial -out "$WORK/$1.crt" -days "$DAYS" \
    -extfile "$WORK/$1.ext" 2>/dev/null
}

echo "==> add-on server certificate (CN=$ADDON_HOST)"
# localhost and 127.0.0.1 alongside the service name so `curl` from inside the
# container is a usable check. Both are reachable only from within the add-on's
# own network namespace, so neither widens who can connect.
issue addon "$ADDON_HOST" "subjectAltName=DNS:$ADDON_HOST,DNS:localhost,IP:127.0.0.1
extendedKeyUsage=serverAuth
keyUsage=critical,digitalSignature,keyEncipherment"

echo "==> backend client certificate"
issue client "syndra-backend" "extendedKeyUsage=clientAuth
keyUsage=critical,digitalSignature"

echo "==> signing key"
# Written to BOTH directories as the same bytes. The two ends HMAC the file's
# CONTENTS, and an earlier deployment had one end HMAC the literal path string
# instead; the only symptom was "no matching signature".
openssl rand -hex 32 > "$WORK/signing.key"

install -m 0644 "$WORK/addon.crt"   "$ADDON_DIR/tls.crt"
install -m 0640 "$WORK/addon.key"   "$ADDON_DIR/tls.key"
install -m 0644 "$WORK/ca.crt"      "$ADDON_DIR/ca.crt"
install -m 0640 "$WORK/signing.key" "$ADDON_DIR/signing.key"

install -m 0644 "$WORK/client.crt"  "$BACKEND_DIR/truenas-client.crt"
install -m 0600 "$WORK/client.key"  "$BACKEND_DIR/truenas-client.key"
install -m 0644 "$WORK/ca.crt"      "$BACKEND_DIR/addon-ca.crt"
install -m 0600 "$WORK/signing.key" "$BACKEND_DIR/truenas-signing.key"

# The CA private key is deliberately NOT kept. Nothing in the deployment issues
# a second certificate from it, and a CA key on the same host as its leaves adds
# a way to mint a client certificate without adding a way to use one. Rotation
# is: move the directory aside and re-run.

if chown -R "$ADDON_UID:$ADDON_UID" "$ADDON_DIR" 2>/dev/null; then
  echo "==> $ADDON_DIR owned by uid $ADDON_UID"
else
  chmod 0644 "$ADDON_DIR/tls.key" "$ADDON_DIR/signing.key"
  echo "warning: could not chown $ADDON_DIR to uid $ADDON_UID (not root)." >&2
  echo "         Its private key is now world-readable so the container can" >&2
  echo "         start. On the deployment host, re-run this under sudo." >&2
fi

cat <<EOF

Done. Add to .env — MUTUAL TLS (stronger; the add-on refuses to start if both
modes are configured, so set one block or the other, never both):

  ADDON_TARGETS=truenas
  ADDON_TRUENAS_BASE_URL=https://$ADDON_HOST:8443
  ADDON_TRUENAS_CLIENT_CERT=/run/secrets/truenas-client.crt
  ADDON_TRUENAS_CLIENT_KEY=/run/secrets/truenas-client.key
  ADDON_TRUENAS_CA_CERT=/run/secrets/addon-ca.crt
  ADDON_CLIENT_CA_FILE=/run/secrets/truenas-addon/ca.crt

or SIGNED REQUESTS (where issuing a client certificate is impractical):

  ADDON_TARGETS=truenas
  ADDON_TRUENAS_BASE_URL=https://$ADDON_HOST:8443
  ADDON_TRUENAS_CA_CERT=/run/secrets/addon-ca.crt
  ADDON_TRUENAS_SIGNING_KEY=/run/secrets/truenas-signing.key
  ADDON_SIGNING_KEY_FILE=/run/secrets/truenas-addon/signing.key

Certificates expire $(date -u -d "+$DAYS days" +%Y-%m-%d 2>/dev/null || date -u -v+"${DAYS}"d +%Y-%m-%d).
The backend surfaces that date on the target's health response; it does not
renew them.
EOF
