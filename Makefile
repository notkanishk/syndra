.PHONY: dev dev-backend dev-ui test test-backend test-ui lint lint-backend lint-ui zitadel-actions-register zitadel-actions-verify zitadel-actions-remove zitadel-actions-rotate-key

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

# Rotate the Actions v2 target signing key via POST /v2/actions/targets/{id}
# with expirationSigningKey:0s. Backs up the previous key to
# zitadel/actions/.action-signing-key.previous; writes the new key to
# .action-signing-key; prints the operator env-swap + restart steps. Zitadel
# does not auto-expire signing keys — run on demand (incident response,
# compliance policy, operator handoff), not on a schedule.
zitadel-actions-rotate-key:
	@zitadel/actions/rotate.sh

# Smoke-tests the /api/action/inject endpoint on a running backend. Pass
# BACKEND_URL=... to target a remote host; defaults to http://localhost:8080.
zitadel-actions-verify:
	@scripts/smoke-test-action-v2.sh $${BACKEND_URL:-http://localhost:8080}
