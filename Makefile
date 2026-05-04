.PHONY: dev dev-backend dev-ui test test-backend test-ui lint lint-backend lint-ui zitadel-actions-register zitadel-actions-verify zitadel-actions-verify-events zitadel-actions-remove zitadel-actions-rotate-key

dev-backend:
	cd backend && go run ./cmd/api

dev-ui:
	cd ui && bun run dev

dev:
	$(MAKE) -j2 dev-backend dev-ui

test-backend:
	cd backend && go test ./...

test-ui:
	cd ui && bun test

test: test-backend test-ui

lint-backend:
	cd backend && go vet ./...

lint-ui:
	cd ui && bun run lint

lint: lint-backend lint-ui

# --- Zitadel Actions v2 deployment (see zitadel/actions/README.md) ---
# Requires env: ZITADEL_DOMAIN, MKAUTH_EXTERNAL_URL, and either
# ZITADEL_M2M_TOKEN or ZITADEL_MACHINE_KEY_PATH.
zitadel-actions-register:
	@zitadel/actions/register.sh

zitadel-actions-remove:
	@zitadel/actions/register.sh --remove

# Rotate the Actions v2 target signing key(s) via POST /v2/actions/targets/{id}
# with expirationSigningKey:0s. Backs up the previous key per-target to
# zitadel/actions/.action-signing-key.<name>.previous; writes the new key to
# .action-signing-key.<name>; prints the operator env-swap + restart steps.
# Zitadel does not auto-expire signing keys — run on demand (incident
# response, compliance policy, operator handoff), not on a schedule.
#
# Usage:
#   make zitadel-actions-rotate-key                          # rotate every target
#   make zitadel-actions-rotate-key TARGET=mkauth-claim-injector  # rotate one
zitadel-actions-rotate-key:
ifdef TARGET
	@zitadel/actions/rotate.sh --target "$(TARGET)"
else
	@zitadel/actions/rotate.sh
endif

# Smoke-tests the /api/action/inject endpoint on a running backend. Pass
# BACKEND_URL=... to target a remote host; defaults to http://localhost:8080.
zitadel-actions-verify:
	@scripts/smoke-test-action-v2.sh $${BACKEND_URL:-http://localhost:8080}

# Smoke-tests the /api/webhooks/zitadel event-listener endpoint by POSTing a
# synthetic UNMAPPED event (user.password.changed) with a valid
# ZITADEL-Signature (or unsigned in dev pass-through mode). The unmapped
# event hits the translator's unknown-event passthrough (200 + log, no
# dispatch), so the check exercises auth + shape detection without touching
# onboarding/grant state — safe against staging and production. Pass
# BACKEND_URL=... to target a remote host; defaults to http://localhost:8080.
zitadel-actions-verify-events:
	@scripts/smoke-test-event-listener.sh $${BACKEND_URL:-http://localhost:8080}
