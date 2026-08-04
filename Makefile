.PHONY: dev dev-backend dev-ui test test-backend test-ui lint lint-backend lint-ui zitadel-actions-register zitadel-actions-verify zitadel-actions-verify-events zitadel-actions-remove zitadel-actions-purge zitadel-actions-rotate-key reset-demo-data reset-all-data

dev-backend:
	cd backend && go run ./cmd/api

dev-ui:
	cd ui && bun run dev

dev:
	$(MAKE) -j2 dev-backend dev-ui

test-backend:
	cd backend && go test ./...

# `bun run test` — NOT `bun test`. The latter is Bun's own test runner, which
# picks up the same files but knows nothing about the vitest API they are
# written against (`vi.hoisted`, the jsdom environment, the setup file), and
# reports ~75 failures on a perfectly healthy tree. `bun run test` invokes the
# "test" script in package.json, which is `vitest run`.
test-ui:
	cd ui && bun run test

test: test-backend test-ui

lint-backend:
	cd backend && go vet ./...

lint-ui:
	cd ui && bun run lint

lint: lint-backend lint-ui

# --- Starting state ---
# Both targets are DRY RUN by default: they print per-table counts and delete
# nothing. Append APPLY=1 to commit, which then asks for typed confirmation.
#
#   make reset-demo-data           # what would go
#   make reset-demo-data APPLY=1   # remove only rows referencing demo fixtures
#   make reset-all-data APPLY=1    # truncate every operator table — blank slate
#
# Neither touches Zitadel. Clearing Syndra's ledger does not revoke anything
# upstream; the next reconciliation sweep re-detects those grants as
# unexplained access, which is how they get re-adopted deliberately.
reset-demo-data:
	@scripts/reset-data.sh demo $(if $(APPLY),--apply)

reset-all-data:
	@scripts/reset-data.sh all $(if $(APPLY),--apply)

# --- Zitadel Actions v2 deployment (see zitadel/actions/README.md) ---
# Requires env: ZITADEL_DOMAIN, SYNDRA_EXTERNAL_URL, and either
# ZITADEL_M2M_TOKEN or ZITADEL_MACHINE_KEY_PATH.
zitadel-actions-register:
	@zitadel/actions/register.sh

zitadel-actions-remove:
	@zitadel/actions/register.sh --remove

# Full teardown: unbinds executions AND deletes the targets themselves via
# DELETE /v2/actions/targets/{id}, then removes the local
# .action-signing-key.<name>{,.previous,.rotated_at} files. Destructive —
# re-running `make zitadel-actions-register` mints fresh signing keys, so
# you also need to clear ZITADEL_ACTION_SIGNING_KEY / ZITADEL_EVENT_SIGNING_KEY
# from .env and restart the backend before re-registering.
zitadel-actions-purge:
	@zitadel/actions/register.sh --purge

# Rotate the Actions v2 target signing key(s) via POST /v2/actions/targets/{id}
# with expirationSigningKey:0s. Backs up the previous key per-target to
# zitadel/actions/.action-signing-key.<name>.previous; writes the new key to
# .action-signing-key.<name>; prints the operator env-swap + restart steps.
# Zitadel does not auto-expire signing keys — run on demand (incident
# response, compliance policy, operator handoff), not on a schedule.
#
# Usage:
#   make zitadel-actions-rotate-key                          # rotate every target
#   make zitadel-actions-rotate-key TARGET=syndra-claim-injector  # rotate one
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
