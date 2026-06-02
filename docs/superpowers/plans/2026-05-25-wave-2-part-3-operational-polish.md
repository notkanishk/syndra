# Wave 2 · Part 3 — Operational Polish Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` (recommended) or `superpowers:executing-plans` to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close eleven Theme 5 audit findings (B8, S1, S3, S4, C7, C8, C9, C10, D4, D7, D9) in a single coordinated wave, ordered by ascending blast radius so each commit is a verification anchor for the next.

**Architecture:** Eleven independent fixes across backend, sync service, deployment template, and zitadel-action scripts. No two share state. The largest item (D4 — drop `mapping_rules.version` end-to-end) lands last; the smallest (C10 — delete a no-op `defer`) lands first. Spec deltas are confined to `ldap-sync`; feature-coverage and INDEX.md updates land in the final task.

**Tech Stack:** Go (backend, sync), Next.js + TypeScript (frontend trim only), PostgreSQL migrations, Bash. Tests via Go's stdlib `testing`, Bun for UI.

---

## OpenSpec change scope

- `openspec/changes/wave-2-part-3-operational-polish/proposal.md`
- `openspec/changes/wave-2-part-3-operational-polish/design.md`
- `openspec/changes/wave-2-part-3-operational-polish/tasks.md`
- `openspec/changes/wave-2-part-3-operational-polish/specs/ldap-sync/spec.md`

---

## Task 0 — Scaffolding baseline commit

Goal: land the OpenSpec scaffolding and the plan file before touching source. Subsequent commits are then guaranteed clean against the new scaffolding without mixing docs + code.

**Files:**
- Create: `openspec/changes/wave-2-part-3-operational-polish/{proposal,design,tasks}.md`
- Create: `openspec/changes/wave-2-part-3-operational-polish/specs/ldap-sync/spec.md`
- Create: `docs/superpowers/plans/2026-05-25-wave-2-part-3-operational-polish.md` (this file)

- [ ] **Step 0.1: Validate the OpenSpec change shape**

```bash
cd /Users/notkanishk/Documents/Mkrspc/Projects/MkAuth
openspec validate wave-2-part-3-operational-polish --strict
```

Expected: `validation passed` (no errors).

- [ ] **Step 0.2: Commit scaffolding only**

```bash
git add openspec/changes/wave-2-part-3-operational-polish/ docs/superpowers/plans/2026-05-25-wave-2-part-3-operational-polish.md
git commit -m "$(cat <<'EOF'
docs(openspec): scaffold wave-2-part-3 operational polish

Lays down proposal, design, tasks, spec delta (ldap-sync), and
the detailed TDD-ordered implementation plan for Theme 5 from
the May 2026 audit-resolution design. No source changes yet —
subsequent tasks land in size-ascending order.

Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 1 — Drop the shadow-password zero-buffer defer (C10)

The `defer` block at `sync/internal/worker/worker.go:191-195` zeros a byte slice that was copied from an immutable Go string. The original `hash` string keeps the secret in memory regardless. The defer is theatre that obscures the real constraint.

**Files:**
- Modify: `sync/internal/worker/worker.go:185-205`

- [ ] **Step 1.1: Read current state**

Confirm lines 185-205 match:

```go
func syncShadowPassword(
	ctx context.Context,
	uid string,
	bc BackendClient,
	lp LDAPPool,
	cfg Config,
) {
	hash, _, err := bc.GetShadowCredentialHash(ctx, uid)
	if err != nil {
		log.Printf("[SYNC] Shadow credential check failed for %s: %v", uid, err)
		return
	}
	if hash == "" {
		return // No shadow credential set.
	}

	// Zero hash bytes after use.
	hashBytes := []byte(hash)
	defer func() {
		for i := range hashBytes {
			hashBytes[i] = 0
		}
	}()
```

- [ ] **Step 1.2: Replace the defer block with a clarifying comment**

Old (lines 188-195 of the function body):

```go
	// Zero hash bytes after use.
	hashBytes := []byte(hash)
	defer func() {
		for i := range hashBytes {
			hashBytes[i] = 0
		}
	}()

	err = retryTransient(ctx, cfg, func() error {
		return lp.SetUserPassword(ctx, uid, string(hashBytes))
	})
```

New:

```go
	// The shadow-credential hash is a Go string (immutable; the GC retains
	// the original allocation until collection). A defer-loop that zeros a
	// []byte(hash) copy is theatre: it cannot reach the source string. The
	// real mitigation is keeping the lifetime short — pass directly to
	// SetUserPassword and let the function return GC the value.
	err = retryTransient(ctx, cfg, func() error {
		return lp.SetUserPassword(ctx, uid, hash)
	})
```

- [ ] **Step 1.3: Run the sync test suite**

```bash
cd /Users/notkanishk/Documents/Mkrspc/Projects/MkAuth/sync
go test ./...
go vet ./...
```

Expected: all green. `worker_test.go` does not exercise the defer; deletion is non-observable.

- [ ] **Step 1.4: Commit**

```bash
git add sync/internal/worker/worker.go
git commit -m "$(cat <<'EOF'
fix(sync): drop no-op shadow-password zero-buffer defer

The defer-loop zeroed a []byte copy of an immutable Go string,
which cannot reach the original allocation. Replaced with a
comment documenting the constraint so a future contributor
doesn't re-introduce the pattern.

Audit ref: C10
Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 2 — `grantLookupMaxPages` from 100 to 10 (B8)

The fallback enrichment path in `backend/internal/handlers/zitadel_grant_lookup.go` caps pagination at 100 pages × 100 grants = 10 000 grants per user — two orders of magnitude beyond the makerspace audience's p99 of ≈100 grants/user. Right-size to 10 pages (1 000 grants), update the rationale.

**Files:**
- Modify: `backend/internal/handlers/zitadel_grant_lookup.go:10-15`

- [ ] **Step 2.1: Edit the constant and comment**

Old:

```go
// grantLookupMaxPages bounds the pagination loop so a bug in Zitadel's Total
// reporting (or an exotic mock) cannot spin the lookup forever. A user with
// > maxPages * DefaultSearchLimit grants is well outside the makerspace
// scale this code targets; if it ever matters, switch to a more selective
// API at that point.
const grantLookupMaxPages = 100
```

New:

```go
// grantLookupMaxPages bounds the pagination loop in the fallback enrichment
// path. At 100 grants per page (Zitadel DefaultSearchLimit), 10 pages cover
// 1000 grants per user — already an order of magnitude beyond what the
// makerspace audience generates (p99 ≈ 100 grants/user). If a future
// deployment regularly hits the cap, the right fix is a more selective
// Zitadel query (search by grantID), not a higher page count.
const grantLookupMaxPages = 10
```

- [ ] **Step 2.2: Confirm the loop still uses the constant**

Run: `grep -n "grantLookupMaxPages" backend/internal/handlers/zitadel_grant_lookup.go`

Expected: 2 hits (declaration line and loop usage at line 32). No other references.

- [ ] **Step 2.3: Run backend tests**

```bash
cd /Users/notkanishk/Documents/Mkrspc/Projects/MkAuth/backend
go test ./internal/handlers/...
go vet ./...
```

Expected: all green. No existing test asserts on `100`; the cap is internal.

- [ ] **Step 2.4: Commit**

```bash
git add backend/internal/handlers/zitadel_grant_lookup.go
git commit -m "$(cat <<'EOF'
perf(webhook): right-size grantLookupMaxPages from 100 to 10

200-user makerspace generates p99 ≈ 100 grants/user; 10 pages
× 100/page = 1000-grant headroom. The previous 100-page cap was
defensive carryover from a different sizing assumption.

Audit ref: B8
Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 3 — Smoke test probes `/healthz` (C9)

`scripts/smoke-test-lxc.sh:11` curls `/api/v1/bundles` with a bearer token. On real OIDC-mode LXC deployments, the operator running the smoke test does not have an API key to hand the script. `/healthz` is an unauthenticated endpoint that returns 200 when the backend is alive (registered at `backend/internal/handlers/router.go:17`).

**Files:**
- Modify: `scripts/smoke-test-lxc.sh:1-15`

- [ ] **Step 3.1: Read current state**

```bash
sed -n '1,15p' scripts/smoke-test-lxc.sh
```

Expected:

```bash
#!/bin/bash
set -euo pipefail

HOST="${1:-198.51.100.14}"
API_KEY="${MKAUTH_API_KEY:-dev_auth_token_secret}"

echo "Checking UI availability..."
curl -fsS "http://${HOST}:3000" >/dev/null

echo "Checking API availability..."
curl -fsS \
  -H "Authorization: Bearer ${API_KEY}" \
  "http://${HOST}:8080/api/v1/bundles" >/dev/null
```

- [ ] **Step 3.2: Replace the authenticated bundles probe with `/healthz`**

Old (lines 5, 10-13):

```bash
API_KEY="${MKAUTH_API_KEY:-dev_auth_token_secret}"

echo "Checking UI availability..."
curl -fsS "http://${HOST}:3000" >/dev/null

echo "Checking API availability..."
curl -fsS \
  -H "Authorization: Bearer ${API_KEY}" \
  "http://${HOST}:8080/api/v1/bundles" >/dev/null
```

New:

```bash
echo "Checking UI availability..."
curl -fsS "http://${HOST}:3000" >/dev/null

echo "Checking API availability..."
# /healthz is unauthenticated so this works on OIDC-mode deployments
# where the operator has no bearer token to hand the script.
curl -fsS "http://${HOST}:8080/healthz" >/dev/null
```

The `API_KEY` line is also removed — nothing else in the script consumes it.

- [ ] **Step 3.3: Verify the script still parses and the endpoint exists locally**

```bash
bash -n scripts/smoke-test-lxc.sh    # syntax check
# Confirm /healthz is registered.
grep -n "/healthz" backend/internal/handlers/router.go
```

Expected: syntax ok; `mux.HandleFunc("GET /healthz", handleHealthCheck)` matched.

- [ ] **Step 3.4: Commit**

```bash
git add scripts/smoke-test-lxc.sh
git commit -m "$(cat <<'EOF'
fix(smoke-test): probe /healthz instead of /api/v1/bundles

The bundles endpoint requires bearer auth which an operator
running the LXC smoke test on a real OIDC deployment does not
have. /healthz is unauthenticated and registered for exactly
this purpose.

Audit ref: C9
Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 4 — `.env.example` EXPIRY_SCHEDULER framing (D9)

The audit-resolution design dropped the "N > 1 replicas" framing — single-LXC deployment does not have multi-replica. The toggle stays useful for ops experimentation; only the rationale changes.

**Files:**
- Modify: `.env.example:21-30`

- [ ] **Step 4.1: Replace the comment block**

Old:

```
# --- Grant Expiry Scheduler (optional — defaults are sensible) ---
# Background worker that sweeps direct_role_grants with expires_at <= NOW(),
# emits LLDAP removal intents, hard-deletes the rows, invalidates user cache,
# and cascade-revokes derived Zitadel grants.
#
# Set to false on extra replicas when running N > 1 backend instances (the
# scheduler assumes single-instance). Leave true everywhere else.
# EXPIRY_SCHEDULER_ENABLED=true
# EXPIRY_SCHEDULER_INTERVAL=5m
# EXPIRY_SCHEDULER_BATCH_SIZE=500
```

New:

```
# --- Grant Expiry Scheduler (optional — defaults are sensible) ---
# Background worker that sweeps direct_role_grants with expires_at <= NOW(),
# emits LLDAP removal intents, hard-deletes the rows, invalidates user cache,
# and cascade-revokes derived Zitadel grants.
#
# Disable only when reproducing a stuck-grant scenario or testing the sweeper
# in isolation. Single-LXC deployment runs one instance; there is no replica
# coordination to consider.
# EXPIRY_SCHEDULER_ENABLED=true
# EXPIRY_SCHEDULER_INTERVAL=5m
# EXPIRY_SCHEDULER_BATCH_SIZE=500
```

- [ ] **Step 4.2: Commit**

```bash
git add .env.example
git commit -m "$(cat <<'EOF'
docs(env): drop N>1 replicas framing from EXPIRY_SCHEDULER block

Single-LXC deployment doesn't have multi-replica. The toggle
still has a legitimate ops-experimentation use; rewrote the
rationale accordingly.

Audit ref: D9
Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 5 — `.env.example` Sync block + missing backend vars (D7)

Append a `--- Sync Service / LLDAP ---` block documenting every env var in `sync/internal/config/config.go`. Insert `MKAUTH_EXTERNAL_URL` and `ZITADEL_M2M_TOKEN` into the appropriate backend / Zitadel sections; both are required at runtime by `zitadel/actions/{register,rotate}.sh` but undocumented in the template today.

**Files:**
- Modify: `.env.example` (insertions + append)

- [ ] **Step 5.1: Insert `MKAUTH_EXTERNAL_URL` into the backend block**

After the `CORS_ORIGIN=http://localhost:3000` line (currently line 17), insert:

```
# Base URL that Zitadel uses to POST to MkAuth (Actions v2 target + event
# listener target). Read by zitadel/actions/{register,rotate}.sh when
# rendering the targets manifest. Required when running the registration
# or rotation scripts; not consumed at backend runtime.
# MKAUTH_EXTERNAL_URL=https://mkauth.internal
```

- [ ] **Step 5.2: Insert `ZITADEL_M2M_TOKEN` into the Zitadel M2M block**

Find the `# --- Zitadel M2M / Management API ---` block (around line 47) and below the `ZITADEL_MACHINE_KEY_PATH=...` commented entry, insert:

```
# Alternative to ZITADEL_MACHINE_KEY_PATH: a pre-minted M2M access token.
# Useful for CI runs where the key file is inconvenient to mount. The
# action-registration scripts prefer this when set; otherwise they mint a
# token from the key file via `go run ./backend/cmd/mkauth-token`. The Go
# backend itself does NOT read this env var — it always mints from the key
# path via the zitadel package.
# ZITADEL_M2M_TOKEN=
```

- [ ] **Step 5.3: Append the Sync Service / LLDAP block**

At the end of `.env.example`, append:

```

# --- Sync Service / LLDAP ---
# The sync service is a separate container that reflects Zitadel identity
# state into LLDAP for legacy protocols (Samba, UniFi). See sync/internal/
# config/config.go for the canonical definitions of each variable.

# URL the sync service uses to reach the MkAuth backend. In docker-compose
# this is the service name; on the LXC it is the backend's internal address.
# Default: http://backend:8080
# BACKEND_URL=http://backend:8080

# Shared secret for sync→backend API auth. Must match MKAUTH_API_KEY in the
# backend block above. Required, no default.
# MKAUTH_API_KEY=dev_auth_token_secret

# LLDAP server URL. Use ldaps:// for TLS; ldap:// for cleartext (dev only).
# Default: ldaps://lldap:636
# LLDAP_URL=ldaps://lldap:636

# LLDAP service-account bind DN. Required, no default.
# LLDAP_BIND_DN=uid=admin,ou=people,dc=example,dc=com

# LLDAP service-account bind password. Required, no default.
# LLDAP_BIND_PASSWORD=change-me

# LLDAP directory root. Default: dc=example,dc=com
# LLDAP_BASE_DN=dc=example,dc=com

# Set to true ONLY in dev with self-signed LLDAP certs. Default: false
# LLDAP_INSECURE_SKIP_VERIFY=false

# Sync worker polling cadence (Go duration). Default: 10s
# SYNC_POLL_INTERVAL=10s

# Number of concurrent sync workers. Default: 5
# SYNC_WORKER_COUNT=5

# Max intents fetched per polling round. Default: 50
# SYNC_INTENT_LIMIT=50

# Number of retry attempts on transient LDAP errors before giving up.
# Default: 3
# SYNC_RETRY_ATTEMPTS=3

# Initial backoff between retry attempts (Go duration). Doubles each
# attempt (exponential backoff). Default: 1s
# SYNC_RETRY_BACKOFF=1s
```

- [ ] **Step 5.4: Verify the file still parses as KEY=VALUE pairs**

```bash
# Naive lint: every non-comment non-blank line should match KEY=VALUE.
grep -vE '^\s*(#|$)' .env.example | grep -vE '^[A-Z_]+=' || echo "OK"
```

Expected: `OK` (no offending lines).

- [ ] **Step 5.5: Commit**

```bash
git add .env.example
git commit -m "$(cat <<'EOF'
docs(env): document sync-service vars and two missing backend vars

Adds --- Sync Service / LLDAP --- block covering all 12 env vars
read by sync/internal/config/config.go (with canonical defaults
inline). Backend block gains MKAUTH_EXTERNAL_URL and ZITADEL_M2M_TOKEN,
both required by the action-registration scripts but undocumented.

Audit ref: D7
Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 6 — Fold `zitadel/actions/{PERMISSIONS,SIGNING_KEY}.md` into `README.md` (S3)

Both docs are read sequentially during action deployment. They become `## Service-Account Permissions` and `## Signing Key Handling` sections inside `README.md`. The originals are deleted. `EVENTS.md` stays separate — its audience is the event-listener path, not action registration.

**Files:**
- Modify: `zitadel/actions/README.md`
- Delete: `zitadel/actions/PERMISSIONS.md`
- Delete: `zitadel/actions/SIGNING_KEY.md`

- [ ] **Step 6.1: Read both source files in full**

```bash
cat zitadel/actions/PERMISSIONS.md
cat zitadel/actions/SIGNING_KEY.md
```

(Both files are short — ~80-150 lines each. Read them in your editor first.)

- [ ] **Step 6.2: Read the current README.md to find the insertion point**

```bash
cat zitadel/actions/README.md
```

The README has step-by-step setup instructions ending around line 60. Pick an insertion point AFTER the step-by-step but BEFORE any troubleshooting / appendix sections. Typically: just before the last `## ` header or at end of file.

- [ ] **Step 6.3: Append the folded content**

Use `## Service-Account Permissions` as the new H2 for PERMISSIONS.md content. Demote any H1 in the source to H2 and any H2 to H3. Same for `## Signing Key Handling` (SIGNING_KEY.md content).

Edit `zitadel/actions/README.md`: append (or insert before the troubleshooting / footer section if one exists):

```markdown
---

## Service-Account Permissions

<demoted content of PERMISSIONS.md — change `# Title` to `### Subtitle` etc.>

---

## Signing Key Handling

<demoted content of SIGNING_KEY.md — same demotion rules>
```

Verification: the README should read top-to-bottom as one coherent setup flow without forward references to deleted files.

- [ ] **Step 6.4: Update any cross-links**

```bash
grep -rn "PERMISSIONS\.md\|SIGNING_KEY\.md" . --include="*.md" --include="*.sh" 2>/dev/null
```

For each hit, rewrite the link to `README.md#service-account-permissions` or `README.md#signing-key-handling`. Common locations: `zitadel/actions/README.md` itself (if it pointed at the now-deleted files), `register.sh` / `rotate.sh` comments, `CLAUDE.md` if it references them.

- [ ] **Step 6.5: Delete the source files**

```bash
git rm zitadel/actions/PERMISSIONS.md zitadel/actions/SIGNING_KEY.md
```

- [ ] **Step 6.6: Visual verification**

Open `zitadel/actions/README.md` in a renderer (GitHub preview, VS Code preview). Confirm:
- Section ordering reads as: original README setup → Service-Account Permissions → Signing Key Handling
- No broken anchor links
- No duplicate `# Title` H1s (the README should have exactly one H1)

- [ ] **Step 6.7: Commit**

```bash
git add zitadel/actions/
git commit -m "$(cat <<'EOF'
docs(zitadel-actions): fold PERMISSIONS and SIGNING_KEY into README

Both docs were read sequentially during action deployment and are
tightly coupled to the README's step-by-step flow. They are now
## Service-Account Permissions and ## Signing Key Handling
sections inside README.md. EVENTS.md stays separate — different
audience (event-listener path, not action registration).

Audit ref: S3
Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 7 — LDAP `member: [bindDN]` placeholder (C7)

`sync/internal/ldap/client.go:170` initialises new groups with `member: [""]`. LLDAP accepts this; strict OpenLDAP would reject it. The bind DN is already in scope via `p.cfg.BindDN`. Extract a small `newGroupAddRequest` helper so the request shape is unit-testable without a fake LDAP connection.

**Files:**
- Modify: `sync/internal/ldap/client.go:155-181`
- Modify: `sync/internal/ldap/client_test.go` (append test)

- [ ] **Step 7.1: Write the failing test first**

Append to `sync/internal/ldap/client_test.go`:

```go
func TestNewGroupAddRequest_UsesBindDNAsPlaceholderMember(t *testing.T) {
	p := &Pool{cfg: Config{
		BaseDN: "dc=example,dc=com",
		BindDN: "uid=admin,ou=people,dc=example,dc=com",
	}}
	req := p.newGroupAddRequest("samba_share_admin")

	var memberVals []string
	var objectClassVals []string
	for _, attr := range req.Attributes {
		switch attr.Type {
		case "member":
			memberVals = attr.Vals
		case "objectClass":
			objectClassVals = attr.Vals
		}
	}

	if len(memberVals) != 1 {
		t.Fatalf("expected exactly one member value, got %d: %v", len(memberVals), memberVals)
	}
	if memberVals[0] != "uid=admin,ou=people,dc=example,dc=com" {
		t.Errorf("expected member=[bindDN], got %q", memberVals[0])
	}
	if memberVals[0] == "" {
		t.Error("placeholder must not be an empty DN — strict OpenLDAP rejects it")
	}
	if len(objectClassVals) != 1 || objectClassVals[0] != "groupOfNames" {
		t.Errorf("expected objectClass=[groupOfNames], got %v", objectClassVals)
	}
}
```

- [ ] **Step 7.2: Run the test to verify it fails**

```bash
cd /Users/notkanishk/Documents/Mkrspc/Projects/MkAuth/sync
go test ./internal/ldap/... -run TestNewGroupAddRequest_UsesBindDNAsPlaceholderMember -v
```

Expected: FAIL with `undefined: newGroupAddRequest` (or method-not-found).

- [ ] **Step 7.3: Extract the helper and update `EnsureGroup`**

In `sync/internal/ldap/client.go`, replace lines 155-181 (the entire `EnsureGroup` function) with:

```go
// newGroupAddRequest builds the AddRequest used by EnsureGroup. Extracted
// so the request shape is unit-testable without a fake LDAP connection.
// `groupOfNames` requires at least one member; we use the bind DN (a real
// principal in the directory) as the placeholder so strict OpenLDAP
// accepts it. Subsequent AddUserToGroup calls add real members alongside.
func (p *Pool) newGroupAddRequest(group string) *ldapv3.AddRequest {
	addReq := ldapv3.NewAddRequest(p.GroupDN(group), nil)
	addReq.Attribute("objectClass", []string{"groupOfNames"})
	addReq.Attribute("cn", []string{group})
	addReq.Attribute("member", []string{p.cfg.BindDN})
	return addReq
}

// EnsureGroup searches for a group by cn. If absent, creates it.
func (p *Pool) EnsureGroup(_ context.Context, group string) error {
	return p.withConn(func(conn *ldapv3.Conn) error {
		groupDN := p.GroupDN(group)

		_, err := conn.Search(ldapv3.NewSearchRequest(
			groupDN, ldapv3.ScopeBaseObject, ldapv3.NeverDerefAliases, 1, 0, false,
			"(objectClass=*)", []string{"cn"}, nil,
		))
		if err != nil {
			if ldapv3.IsErrorWithCode(err, ldapv3.LDAPResultNoSuchObject) {
				if err := conn.Add(p.newGroupAddRequest(group)); err != nil {
					return fmt.Errorf("create LLDAP group %s: %w", group, err)
				}
				log.Printf("[LDAP] Created group: cn=%s", group)
				return nil
			}
			return fmt.Errorf("search group %s: %w", group, err)
		}
		return nil
	})
}
```

Note: the `withConn` call still uses the old signature (no `ctx`); Task 8 changes that.

- [ ] **Step 7.4: Run the test to verify it passes**

```bash
go test ./internal/ldap/... -run TestNewGroupAddRequest -v
```

Expected: PASS.

- [ ] **Step 7.5: Run the full sync test suite**

```bash
go test ./...
go vet ./...
```

Expected: all green.

- [ ] **Step 7.6: Commit**

```bash
git add sync/internal/ldap/client.go sync/internal/ldap/client_test.go
git commit -m "$(cat <<'EOF'
fix(sync-ldap): use bind DN as groupOfNames placeholder member

LLDAP accepted member=[""] as the bootstrap placeholder; strict
OpenLDAP would reject it. The bind DN is a real principal in the
directory and always in scope via p.cfg.BindDN. Extracted a small
newGroupAddRequest helper so the request shape is unit-testable
without a fake LDAP connection.

Audit ref: C7
Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 8 — `withConn` plumbs `context.Context` (C8)

Change the signature so worker shutdown delivers cancellation at mutex-boundary checkpoints. Fail-fast at three points: before attempting `p.mu.Lock()`, immediately after acquiring the mutex, and before the reconnect retry. The active op holding the mutex is NOT interrupted; queued goroutines blocked on `Lock()` continue to wait for the mutex (Go's `sync.Mutex` is not select-able) and observe cancellation only at the post-acquisition checkpoint.

**Files:**
- Modify: `sync/internal/ldap/client.go:73-87` and every caller (lines 101, 156, 185, 204, 222, plus any others)
- Modify: `sync/internal/ldap/client_test.go` (append test)

- [ ] **Step 8.1: Write the failing test first**

Append to `sync/internal/ldap/client_test.go`:

```go
import (
	"context"
	"errors"
	// ... existing imports ...
)

func TestWithConn_CancelledCtxFailsFast(t *testing.T) {
	p := &Pool{} // conn is nil; fn must not be invoked
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before calling withConn

	fnInvoked := false
	err := p.withConn(ctx, func(c *ldapv3.Conn) error {
		fnInvoked = true
		return nil
	})

	if fnInvoked {
		t.Error("fn must not be invoked when ctx is already cancelled")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}
```

- [ ] **Step 8.2: Run to verify it fails**

```bash
cd /Users/notkanishk/Documents/Mkrspc/Projects/MkAuth/sync
go test ./internal/ldap/... -run TestWithConn_CancelledCtxFailsFast -v
```

Expected: COMPILE ERROR — `withConn` signature does not accept context.

- [ ] **Step 8.3: Change the `withConn` signature**

Replace lines 73-87 of `client.go`:

Old:

```go
// withConn executes fn with the current connection. If fn returns a connection
// error, reconnect is attempted once and fn is retried.
func (p *Pool) withConn(fn func(*ldapv3.Conn) error) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	err := fn(p.conn)
	if err != nil && IsConnectionError(err) {
		if reconnErr := p.reconnect(); reconnErr != nil {
			return fmt.Errorf("reconnect failed: %w (original: %v)", reconnErr, err)
		}
		return fn(p.conn) // retry once
	}
	return err
}
```

New:

```go
// withConn executes fn with the current connection. Context cancellation
// is honored at LDAP-operation boundaries: ctx.Err() is checked before
// acquiring the pool mutex, after acquiring it, and before any reconnect
// retry. Once fn is running, it owns the LDAP call to completion (or the
// underlying conn's timeout). If fn returns a connection error, reconnect
// is attempted once and fn is retried.
func (p *Pool) withConn(ctx context.Context, fn func(*ldapv3.Conn) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}

	err := fn(p.conn)
	if err != nil && IsConnectionError(err) {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		if reconnErr := p.reconnect(); reconnErr != nil {
			return fmt.Errorf("reconnect failed: %w (original: %v)", reconnErr, err)
		}
		return fn(p.conn) // retry once
	}
	return err
}
```

- [ ] **Step 8.4: Thread `ctx` through every caller**

Open `sync/internal/ldap/client.go` and rewrite each caller. Pattern: every caller already has `ctx context.Context` in its parameters (currently as `_ context.Context` in most cases — name it `ctx` and pass it through).

`EnsureUser` (line 101):

```go
func (p *Pool) EnsureUser(ctx context.Context, uid, displayName, email string) error {
	return p.withConn(ctx, func(conn *ldapv3.Conn) error {
		// ... unchanged body ...
	})
}
```

`EnsureGroup` (line 156, post-Task-7):

```go
func (p *Pool) EnsureGroup(ctx context.Context, group string) error {
	return p.withConn(ctx, func(conn *ldapv3.Conn) error {
		// ... unchanged body ...
	})
}
```

`AddUserToGroup` (line 185):

```go
func (p *Pool) AddUserToGroup(ctx context.Context, targetUID, lldapGroup string) error {
	return p.withConn(ctx, func(conn *ldapv3.Conn) error {
		// ... unchanged body ...
	})
}
```

`RemoveUserFromGroup` (line 204):

```go
func (p *Pool) RemoveUserFromGroup(ctx context.Context, targetUID, lldapGroup string) error {
	return p.withConn(ctx, func(conn *ldapv3.Conn) error {
		// ... unchanged body ...
	})
}
```

`SetUserPassword` (line 222):

```go
func (p *Pool) SetUserPassword(ctx context.Context, targetUID, passwordHash string) error {
	return p.withConn(ctx, func(conn *ldapv3.Conn) error {
		// ... unchanged body ...
	})
}
```

Run a grep to make sure no caller is missed:

```bash
grep -n "p.withConn(\|withConn(func" sync/internal/ldap/client.go
```

Expected: every match is `p.withConn(ctx, func(...)` — no bare `p.withConn(func` left.

- [ ] **Step 8.5: Run the targeted test**

```bash
go test ./internal/ldap/... -run TestWithConn_CancelledCtxFailsFast -v
```

Expected: PASS.

- [ ] **Step 8.6: Run the full sync suite — callers in `worker.go` should still compile**

```bash
go test ./...
go vet ./...
```

Expected: all green. The interface types (`LDAPPool` in `worker.go`) already declared each method as taking `context.Context`, so worker call sites should not need changes. If they do, the interface lives at `sync/internal/worker/worker.go` — update the method signatures there too.

- [ ] **Step 8.7: Commit**

```bash
git add sync/internal/ldap/client.go sync/internal/ldap/client_test.go
git commit -m "$(cat <<'EOF'
feat(sync-ldap): plumb context through withConn

withConn now accepts context.Context and fails fast when the
context is cancelled — before mutex acquisition, after mutex
acquisition, and before reconnect retry. A queued goroutine
already blocked on the pool mutex still waits for it (sync.Mutex
is not select-able); it observes cancellation only at the
post-acquisition checkpoint. The active op holding the mutex is
not interrupted.

Audit ref: C8
Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 9 — Wire `SYNC_RETRY_ATTEMPTS` and `SYNC_RETRY_BACKOFF` (S4)

`sync/internal/config/config.go:42-43` hardcodes `RetryAttempts = 3` and `RetryBackoff = 1 * time.Second`. Read both from env, preserve defaults, follow the existing `envOrDefault` + parse pattern.

**Files:**
- Modify: `sync/internal/config/config.go`
- Create: `sync/internal/config/config_test.go`

- [ ] **Step 9.1: Write the failing test first**

Create `sync/internal/config/config_test.go`:

```go
package config

import (
	"testing"
	"time"
)

func TestLoad_RetryAttemptsAndBackoffFromEnv(t *testing.T) {
	// Required vars (LoadConfig errors without them).
	t.Setenv("MKAUTH_API_KEY", "test-key")
	t.Setenv("LLDAP_BIND_DN", "uid=admin,dc=example,dc=com")
	t.Setenv("LLDAP_BIND_PASSWORD", "test-pw")

	t.Setenv("SYNC_RETRY_ATTEMPTS", "5")
	t.Setenv("SYNC_RETRY_BACKOFF", "250ms")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.RetryAttempts != 5 {
		t.Errorf("RetryAttempts: got %d, want 5", cfg.RetryAttempts)
	}
	if cfg.RetryBackoff != 250*time.Millisecond {
		t.Errorf("RetryBackoff: got %v, want 250ms", cfg.RetryBackoff)
	}
}

func TestLoad_RetryDefaultsWhenEnvAbsent(t *testing.T) {
	t.Setenv("MKAUTH_API_KEY", "test-key")
	t.Setenv("LLDAP_BIND_DN", "uid=admin,dc=example,dc=com")
	t.Setenv("LLDAP_BIND_PASSWORD", "test-pw")
	// Explicitly clear in case the host env has them.
	t.Setenv("SYNC_RETRY_ATTEMPTS", "")
	t.Setenv("SYNC_RETRY_BACKOFF", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.RetryAttempts != 3 {
		t.Errorf("RetryAttempts default: got %d, want 3", cfg.RetryAttempts)
	}
	if cfg.RetryBackoff != 1*time.Second {
		t.Errorf("RetryBackoff default: got %v, want 1s", cfg.RetryBackoff)
	}
}

func TestLoad_InvalidRetryAttemptsReturnsError(t *testing.T) {
	t.Setenv("MKAUTH_API_KEY", "test-key")
	t.Setenv("LLDAP_BIND_DN", "uid=admin,dc=example,dc=com")
	t.Setenv("LLDAP_BIND_PASSWORD", "test-pw")
	t.Setenv("SYNC_RETRY_ATTEMPTS", "not-a-number")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid SYNC_RETRY_ATTEMPTS, got nil")
	}
}
```

- [ ] **Step 9.2: Run to verify it fails**

```bash
cd /Users/notkanishk/Documents/Mkrspc/Projects/MkAuth/sync
go test ./internal/config/... -v
```

Expected: `TestLoad_RetryAttemptsAndBackoffFromEnv` FAILS — `RetryAttempts: got 3, want 5`.

- [ ] **Step 9.3: Wire the env reads**

In `sync/internal/config/config.go`, replace the `cfg := Config{...}` block and the parse section below it.

Old (lines 33-60):

```go
func Load() (Config, error) {
	cfg := Config{
		BackendURL:             envOrDefault("BACKEND_URL", "http://backend:8080"),
		APIKey:                 os.Getenv("MKAUTH_API_KEY"),
		LDAPURL:                envOrDefault("LLDAP_URL", "ldaps://lldap:636"),
		LDAPBindDN:             os.Getenv("LLDAP_BIND_DN"),
		LDAPBindPassword:       os.Getenv("LLDAP_BIND_PASSWORD"),
		LDAPBaseDN:             envOrDefault("LLDAP_BASE_DN", "dc=example,dc=com"),
		LDAPInsecureSkipVerify: envOrDefault("LLDAP_INSECURE_SKIP_VERIFY", "false") == "true",
		RetryAttempts:          3,
		RetryBackoff:           1 * time.Second,
	}

	var err error
	cfg.PollInterval, err = time.ParseDuration(envOrDefault("SYNC_POLL_INTERVAL", "10s"))
	if err != nil {
		return cfg, fmt.Errorf("invalid SYNC_POLL_INTERVAL: %w", err)
	}

	cfg.WorkerCount, err = strconv.Atoi(envOrDefault("SYNC_WORKER_COUNT", "5"))
	if err != nil {
		return cfg, fmt.Errorf("invalid SYNC_WORKER_COUNT: %w", err)
	}

	cfg.IntentLimit, err = strconv.Atoi(envOrDefault("SYNC_INTENT_LIMIT", "50"))
	if err != nil {
		return cfg, fmt.Errorf("invalid SYNC_INTENT_LIMIT: %w", err)
	}
```

New (replace the literal `RetryAttempts: 3,` and `RetryBackoff: 1 * time.Second,` lines and append parses):

```go
func Load() (Config, error) {
	cfg := Config{
		BackendURL:             envOrDefault("BACKEND_URL", "http://backend:8080"),
		APIKey:                 os.Getenv("MKAUTH_API_KEY"),
		LDAPURL:                envOrDefault("LLDAP_URL", "ldaps://lldap:636"),
		LDAPBindDN:             os.Getenv("LLDAP_BIND_DN"),
		LDAPBindPassword:       os.Getenv("LLDAP_BIND_PASSWORD"),
		LDAPBaseDN:             envOrDefault("LLDAP_BASE_DN", "dc=example,dc=com"),
		LDAPInsecureSkipVerify: envOrDefault("LLDAP_INSECURE_SKIP_VERIFY", "false") == "true",
	}

	var err error
	cfg.PollInterval, err = time.ParseDuration(envOrDefault("SYNC_POLL_INTERVAL", "10s"))
	if err != nil {
		return cfg, fmt.Errorf("invalid SYNC_POLL_INTERVAL: %w", err)
	}

	cfg.WorkerCount, err = strconv.Atoi(envOrDefault("SYNC_WORKER_COUNT", "5"))
	if err != nil {
		return cfg, fmt.Errorf("invalid SYNC_WORKER_COUNT: %w", err)
	}

	cfg.IntentLimit, err = strconv.Atoi(envOrDefault("SYNC_INTENT_LIMIT", "50"))
	if err != nil {
		return cfg, fmt.Errorf("invalid SYNC_INTENT_LIMIT: %w", err)
	}

	cfg.RetryAttempts, err = strconv.Atoi(envOrDefault("SYNC_RETRY_ATTEMPTS", "3"))
	if err != nil {
		return cfg, fmt.Errorf("invalid SYNC_RETRY_ATTEMPTS: %w", err)
	}

	cfg.RetryBackoff, err = time.ParseDuration(envOrDefault("SYNC_RETRY_BACKOFF", "1s"))
	if err != nil {
		return cfg, fmt.Errorf("invalid SYNC_RETRY_BACKOFF: %w", err)
	}
```

The validation block below (`if cfg.APIKey == ""` etc.) is unchanged.

- [ ] **Step 9.4: Run the tests**

```bash
go test ./internal/config/... -v
go test ./...
go vet ./...
```

Expected: all green.

- [ ] **Step 9.5: Commit**

```bash
git add sync/internal/config/config.go sync/internal/config/config_test.go
git commit -m "$(cat <<'EOF'
feat(sync-config): read SYNC_RETRY_ATTEMPTS/BACKOFF from env

Previously hardcoded as 3 / 1s in the Config initialiser. Now
follows the same envOrDefault + parse pattern as the rest of
the sync config surface; defaults preserved.

Audit ref: S4
Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 10 — Extract `scripts/lib/load-env.sh` (S1; S2 deferred)

The 22-line env-loader block is duplicated character-for-character in `zitadel/actions/register.sh:74-95` and `zitadel/actions/rotate.sh:55-76`. Extract to `scripts/lib/load-env.sh`; both callers `source` it. `zitadel_api()` is NOT extracted — see design.md Decision 4.

**Files:**
- Create: `scripts/lib/load-env.sh`
- Modify: `zitadel/actions/register.sh:74-95`
- Modify: `zitadel/actions/rotate.sh:55-76`

- [ ] **Step 10.1: Confirm the two duplicated blocks are identical**

```bash
diff <(sed -n '74,95p' zitadel/actions/register.sh) <(sed -n '55,76p' zitadel/actions/rotate.sh)
```

Expected: zero diff (the agent verified character-for-character match). If diff is non-empty, halt and re-examine.

- [ ] **Step 10.2: Create the extracted helper**

Create `scripts/lib/load-env.sh`:

```bash
#!/usr/bin/env bash
# scripts/lib/load-env.sh
#
# Loads KEY=VALUE pairs from an env file into the current shell environment.
# Preserves any KEY already set in the environment (does not overwrite).
# Strips matching surrounding single or double quotes. No-op when the file
# does not exist.
#
# The caller MUST set `_ENV_FILE` to the path before sourcing:
#
#   _ENV_FILE="$(cd "${SCRIPT_DIR}/../.." && pwd)/.env"
#   # shellcheck source=../../scripts/lib/load-env.sh
#   source "${REPO_ROOT}/scripts/lib/load-env.sh"
#
# Intentional: the helper does not invent its own path resolution — every
# caller has its own SCRIPT_DIR convention, and the helper stays a pure
# transformation (env file → process environment).

if [[ -f "$_ENV_FILE" ]]; then
  while IFS= read -r _raw || [[ -n "$_raw" ]]; do
    [[ "$_raw" =~ ^[[:space:]]*($|#) ]] && continue
    [[ "$_raw" =~ ^[[:space:]]*([A-Za-z_][A-Za-z0-9_]*)=(.*)$ ]] || continue
    _k="${BASH_REMATCH[1]}"
    _v="${BASH_REMATCH[2]}"
    if [[ "$_v" =~ ^\"(.*)\"$ ]] || [[ "$_v" =~ ^\'(.*)\'$ ]]; then
      _v="${BASH_REMATCH[1]}"
    fi
    [[ -z "${!_k+x}" ]] && export "$_k=$_v"
  done < "$_ENV_FILE"
  unset _raw _k _v
fi
```

Make it executable (cosmetic; `source` does not require it but matches convention):

```bash
chmod +x scripts/lib/load-env.sh
```

- [ ] **Step 10.3: Replace the inline block in `register.sh`**

In `zitadel/actions/register.sh`, lines 74-95 currently contain the inline loader. Replace with:

```bash
# Load .env if present so MKAUTH_EXTERNAL_URL / ZITADEL_M2M_TOKEN / etc. are
# available when this script runs from a fresh shell. Existing exports win.
_ENV_FILE="$(cd "${SCRIPT_DIR}/../.." && pwd)/.env"
# shellcheck source=../../scripts/lib/load-env.sh
source "$(cd "${SCRIPT_DIR}/../.." && pwd)/scripts/lib/load-env.sh"
```

(Adjust the explanatory comment to match whatever comment preceded the block in register.sh — preserve the operator-facing context.)

- [ ] **Step 10.4: Replace the inline block in `rotate.sh`**

Same pattern, lines 55-76:

```bash
# Load .env for the same reasons as register.sh — operator-friendly defaults.
_ENV_FILE="$(cd "${SCRIPT_DIR}/../.." && pwd)/.env"
# shellcheck source=../../scripts/lib/load-env.sh
source "$(cd "${SCRIPT_DIR}/../.." && pwd)/scripts/lib/load-env.sh"
```

- [ ] **Step 10.5: Syntax-check both scripts**

```bash
bash -n zitadel/actions/register.sh
bash -n zitadel/actions/rotate.sh
bash -n scripts/lib/load-env.sh
```

Expected: zero output (all pass syntax).

- [ ] **Step 10.6: Smoke-test the loader**

Create a throwaway env file, source the helper, verify behavior:

```bash
mkdir -p /tmp/wave-2-part-3-test
cat > /tmp/wave-2-part-3-test/.env <<'EOF'
TEST_LOAD_ENV_VAR=loaded
TEST_QUOTED_VAR="quoted value"
# comment line ignored
TEST_EXISTING_VAR=should_not_overwrite
EOF

export TEST_EXISTING_VAR=preserved
unset TEST_LOAD_ENV_VAR TEST_QUOTED_VAR

_ENV_FILE=/tmp/wave-2-part-3-test/.env
source scripts/lib/load-env.sh

[[ "$TEST_LOAD_ENV_VAR" == "loaded" ]] || echo "FAIL: load"
[[ "$TEST_QUOTED_VAR" == "quoted value" ]] || echo "FAIL: quote-strip"
[[ "$TEST_EXISTING_VAR" == "preserved" ]] || echo "FAIL: precedence"
echo "load-env.sh smoke test OK"

# Cleanup
rm -rf /tmp/wave-2-part-3-test
unset TEST_LOAD_ENV_VAR TEST_QUOTED_VAR TEST_EXISTING_VAR _ENV_FILE
```

Expected: prints `load-env.sh smoke test OK` (no FAIL lines).

- [ ] **Step 10.7: Commit**

```bash
git add scripts/lib/load-env.sh zitadel/actions/register.sh zitadel/actions/rotate.sh
git commit -m "$(cat <<'EOF'
refactor(scripts): extract scripts/lib/load-env.sh

The 22-line env-loader block was duplicated character-for-character
in zitadel/actions/{register,rotate}.sh. Extracted to a sourced
helper; both callers now use a 4-line invocation. zitadel_api() is
NOT extracted (single consumer; YAGNI — see wave-2-part-3
design.md Decision 4).

Audit ref: S1 (S2 deferred)
Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 11 — Drop `mapping_rules.version` end-to-end (D4)

Fourteen touch points spanning a full vertical slice: migration up/down, db function, handler, route, deps injectable, three handler tests, model, view, two UI types, the mutation hook, and five edits in the policies page (import, binding, function, badge, button). The largest blast radius; lands last so the rest of the wave is stable.

**Files:**
- Create: `backend/db/migrations/000014_drop_mapping_rules_version.up.sql`
- Create: `backend/db/migrations/000014_drop_mapping_rules_version.down.sql`
- Modify: `backend/internal/db/rules.go` (delete `UpdateMappingRule`; trim `GetActiveMappingRules`)
- Modify: `backend/internal/handlers/rules.go` (delete `handleUpdateMappingRule`)
- Modify: `backend/internal/handlers/router.go:60` (delete the PUT route)
- Modify: `backend/internal/handlers/deps.go:44` (delete the injectable)
- Modify: `backend/internal/handlers/rules_test.go` (delete the three `TestHandleUpdateMappingRule_*` and trim `resetRulesDeps`)
- Modify: `backend/internal/models/models.go:37`
- Modify: `backend/internal/services/views.go:700`
- Modify: `ui/src/lib/queries/useMappingRules.ts` (trim `MappingRuleRow`; delete `useBumpMappingRule`)
- Modify: `ui/src/lib/types.ts:59`
- Modify: `ui/src/app/policies/page.tsx` (5 edits)

### Step 11.A — Migrations

- [ ] **Step 11.A.1: Create the up migration**

`backend/db/migrations/000014_drop_mapping_rules_version.up.sql`:

```sql
-- Wave 2 · Part 3 — D4: mapping_rules.version is removed. audit_logs is
-- the historical record per the May 2026 audit-resolution design §2.

ALTER TABLE mapping_rules DROP COLUMN version;
```

- [ ] **Step 11.A.2: Create the down migration**

`backend/db/migrations/000014_drop_mapping_rules_version.down.sql`:

```sql
-- Rollback: restore the column with default 1. Pre-drop version values are
-- not recoverable from audit_logs; rollback returns to a parseable state but
-- not to historical version numbers (which were never read by the system
-- and only ever existed for the now-removed "Bump version" workflow).

ALTER TABLE mapping_rules ADD COLUMN version INT NOT NULL DEFAULT 1;
```

### Step 11.B — Backend Go: db layer

- [ ] **Step 11.B.1: Delete `UpdateMappingRule` from `db/rules.go`**

In `backend/internal/db/rules.go`, delete lines 29-40 (the entire function):

```go
func UpdateMappingRule(ctx context.Context, id string) error {
	// Increment version only, indicating this rule's logic or downstream effects were reviewed/refreshed
	query := `UPDATE mapping_rules SET version = version + 1 WHERE id = $1;`
	tag, err := PG.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to update mapping rule version: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("mapping rule not found")
	}
	return nil
}
```

- [ ] **Step 11.B.2: Trim `version` from `GetActiveMappingRules` in `db/rules.go`**

Old:

```go
func GetActiveMappingRules(ctx context.Context) ([]models.MappingRule, error) {
	query := `
		SELECT id, source_zitadel_project_id, source_zitadel_role_key, target_zitadel_project_id, target_zitadel_role_key, version, created_at 
		FROM mapping_rules;`

	rows, err := PG.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []models.MappingRule
	for rows.Next() {
		var r models.MappingRule
		if err := rows.Scan(&r.ID, &r.SourceProject, &r.SourceRole, &r.TargetProject, &r.TargetRole, &r.Version, &r.CreatedAt); err != nil {
			return nil, err
		}
		rules = append(rules, r)
	}
	return rules, nil
}
```

New:

```go
func GetActiveMappingRules(ctx context.Context) ([]models.MappingRule, error) {
	query := `
		SELECT id, source_zitadel_project_id, source_zitadel_role_key, target_zitadel_project_id, target_zitadel_role_key, created_at
		FROM mapping_rules;`

	rows, err := PG.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rules []models.MappingRule
	for rows.Next() {
		var r models.MappingRule
		if err := rows.Scan(&r.ID, &r.SourceProject, &r.SourceRole, &r.TargetProject, &r.TargetRole, &r.CreatedAt); err != nil {
			return nil, err
		}
		rules = append(rules, r)
	}
	return rules, nil
}
```

### Step 11.C — Backend Go: handler layer

- [ ] **Step 11.C.1: Delete `handleUpdateMappingRule` from `handlers/rules.go`**

In `backend/internal/handlers/rules.go`, delete lines 128-144 (the entire function):

```go
func handleUpdateMappingRule(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		jsonErrorResponse(w, http.StatusBadRequest, "BAD_REQUEST", "Missing rule ID")
		return
	}

	if err := dbUpdateMappingRule(r.Context(), id); err != nil {
		jsonErrorResponse(w, http.StatusInternalServerError, "UPDATE_FAILED", err.Error())
		return
	}

	// Audit log
	_ = dbInsertAuditLog(r.Context(), "system", "-", "mapping_rule.version_bumped", id)

	jsonResponse(w, http.StatusOK, map[string]string{"message": "Version incremented successfully"})
}
```

- [ ] **Step 11.C.2: Unregister the route in `handlers/router.go`**

Delete line 60:

```go
mux.HandleFunc("PUT /api/v1/rules/mapping/{id}", withCORS(withUserAuth(handleUpdateMappingRule)))
```

The remaining mapping-rules routes (`GET`, `POST`, `POST .../validate`) stay.

- [ ] **Step 11.C.3: Delete the injectable in `handlers/deps.go`**

In `backend/internal/handlers/deps.go`, delete line 44:

```go
dbUpdateMappingRule     = db.UpdateMappingRule
```

Visual: it sits in a `var (...)` block alongside `dbGetActiveMappingRules`, `dbCreateMappingRule`, `dbDetectCycleOnInsert`, `dbInsertAuditLog`. Only the one line goes; the block stays.

- [ ] **Step 11.C.4: Trim `rules_test.go`**

In `backend/internal/handlers/rules_test.go`:

(a) Delete the `origUpdate` save/restore in `resetRulesDeps` (lines 17-26). Old:

```go
func resetRulesDeps(t *testing.T) {
	t.Helper()
	origGetActive := dbGetActiveMappingRules
	origCreate := dbCreateMappingRule
	origUpdate := dbUpdateMappingRule
	origDetect := dbDetectCycleOnInsert
	origAudit := dbInsertAuditLog
	t.Cleanup(func() {
		dbGetActiveMappingRules = origGetActive
		dbCreateMappingRule = origCreate
		dbUpdateMappingRule = origUpdate
		dbDetectCycleOnInsert = origDetect
		dbInsertAuditLog = origAudit
	})
}
```

New:

```go
func resetRulesDeps(t *testing.T) {
	t.Helper()
	origGetActive := dbGetActiveMappingRules
	origCreate := dbCreateMappingRule
	origDetect := dbDetectCycleOnInsert
	origAudit := dbInsertAuditLog
	t.Cleanup(func() {
		dbGetActiveMappingRules = origGetActive
		dbCreateMappingRule = origCreate
		dbDetectCycleOnInsert = origDetect
		dbInsertAuditLog = origAudit
	})
}
```

(b) Delete the three `TestHandleUpdateMappingRule_*` tests entirely (lines 151-225, inclusive of the `// --- handleUpdateMappingRule ---` banner comment):

- `TestHandleUpdateMappingRule_NotFound`
- `TestHandleUpdateMappingRule_HappyPath`
- `TestHandleUpdateMappingRule_MissingID`

These are the only references to `dbUpdateMappingRule` in the test file (verify with `grep -n dbUpdateMappingRule backend/internal/handlers/rules_test.go` — expect zero hits after the edit).

### Step 11.D — Backend Go: model + view

- [ ] **Step 11.D.1: Delete the `Version` field from `models.MappingRule`**

In `backend/internal/models/models.go`, remove line 37 (`Version int json:"version"`). The struct becomes:

```go
type MappingRule struct {
	ID            string    `json:"id"`
	SourceProject string    `json:"source_project"`
	SourceRole    string    `json:"source_role"`
	TargetProject string    `json:"target_project"`
	TargetRole    string    `json:"target_role"`
	CreatedAt     time.Time `json:"created_at"`
}
```

- [ ] **Step 11.D.2: Delete the Topology meta line in `services/views.go`**

In `backend/internal/services/views.go`, remove line 700:

```go
				"version": fmt.Sprintf("%d", rule.Version),
```

The surrounding meta-map has other entries (`"source_role"`, `"target_role"`, etc.) that stay; only this one line goes.

### Step 11.E — Backend verification

- [ ] **Step 11.E.1: Confirm no Go references to `UpdateMappingRule` / `dbUpdateMappingRule` / `handleUpdateMappingRule` remain**

```bash
cd /Users/notkanishk/Documents/Mkrspc/Projects/MkAuth
grep -rn "UpdateMappingRule\b\|dbUpdateMappingRule\b\|handleUpdateMappingRule\b\|mapping_rule\.version_bumped\|rule\.Version\b" backend/ --include="*.go"
```

Expected: zero hits. Any hit is a stale reference to fix before continuing.

- [ ] **Step 11.E.2: Backend tests**

```bash
cd backend
go test ./...
go vet ./...
```

Expected: all green. The deleted handler tests no longer exist; the create / get / detect-cycle tests still pass.

### Step 11.F — Frontend

- [ ] **Step 11.F.1: Trim `MappingRuleRow` and delete `useBumpMappingRule` in `useMappingRules.ts`**

In `ui/src/lib/queries/useMappingRules.ts`:

(a) Delete `version: number;` from the `MappingRuleRow` interface (line 13). New interface:

```typescript
export interface MappingRuleRow {
  id: string;
  source_project: string;
  source_role: string;
  target_project: string;
  target_role: string;
  created_at: string;
}
```

(b) Delete the `useBumpMappingRule` hook entirely (the JSDoc comment plus the function, approximately lines 75-87):

```typescript
/** Bump a rule's version (re-evaluates downstream propagation). */
export function useBumpMappingRule() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (id: string) => {
      return await request(`/rules/mapping/${id}`, { method: "PUT", body: {} });
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: KEYS.list });
    },
  });
}
```

If removing this leaves an unused import (`useMutation`, `useQueryClient`), check whether any other hook in the same file still uses them — likely yes (create/delete mutations). If not, drop the unused import.

- [ ] **Step 11.F.2: Delete `version: number;` from `lib/types.ts:MappingRule`**

In `ui/src/lib/types.ts`, remove line 59. The (currently unimported anywhere) `MappingRule` interface becomes:

```typescript
export interface MappingRule {
  id: string;
  source_project: string;
  source_role: string;
  target_project: string;
  target_role: string;
  created_at: string;
}
```

Keep the type around; deleting it is not in scope for this wave (it's stale but harmless and removing untyped-but-exported interfaces is a Wave 3 cleanup concern).

- [ ] **Step 11.F.3: Update `app/policies/page.tsx` — five edits**

In `ui/src/app/policies/page.tsx`:

(a) Line 16 — drop `useBumpMappingRule` from the import. Old:

```typescript
import { useBumpMappingRule, useMappingRules } from "@/lib/queries/useMappingRules";
```

New:

```typescript
import { useMappingRules } from "@/lib/queries/useMappingRules";
```

(b) Line 38 — delete the binding:

```typescript
const bumpRule = useBumpMappingRule();
```

(c) Lines 45-49 (approximately) — delete the `handleBump` async function:

```typescript
async function handleBump(id: string) {
  // ... existing body ...
  await bumpRule.mutateAsync(id);
}
```

Confirm with `grep -n "handleBump\|bumpRule" ui/src/app/policies/page.tsx` — expect zero hits after the edit.

(d) Line 96 — delete the version badge:

```tsx
<Badge variant="outline">v{rule.version || 1}</Badge>
```

If this leaves a parent `<div>` with no remaining children, simplify (delete the wrapper) only if no other element renders inside it.

(e) Lines 135-145 (approximately) — delete the entire "Bump version →" button block, including its enclosing `<div>` (the wrapper exists ONLY for this button and has nothing else):

```tsx
<div className="flex items-center justify-end mt-3 pt-3 border-t border-outline-variant/50">
  <Button
    type="button"
    variant="link"
    size="sm"
    onClick={() => handleBump(rule.id)}
    isPending={bumpRule.isPending && bumpRule.variables === rule.id}
  >
    Bump version →
  </Button>
</div>
```

Visually verify the surrounding `<article>` still has a sensible footer (the bordered-top spacing was provided by this wrapper; the article should still terminate cleanly without it).

- [ ] **Step 11.F.4: Sanity-grep the frontend for stragglers**

```bash
grep -rn "useBumpMappingRule\|handleBump\|rule\.version\b\|version_bumped\|Bump version" ui/src/ --include="*.tsx" --include="*.ts" 2>/dev/null
```

Expected: zero hits. Any hit is a stale reference.

- [ ] **Step 11.F.5: Frontend build + tests**

```bash
cd /Users/notkanishk/Documents/Mkrspc/Projects/MkAuth/ui
bun run lint
bun run test
bun run build
```

Expected: all green. TypeScript will catch any remaining `rule.version` reference or unresolved `useBumpMappingRule` import.

### Step 11.G — Migration round-trip + commit

- [ ] **Step 11.G.1: Migration round-trip on a throwaway DB**

```bash
cd /Users/notkanishk/Documents/Mkrspc/Projects/MkAuth/backend
# Substitute the project's actual migrate-CLI invocation. Most projects use
# golang-migrate against DB_DSN from .env.
go run ./cmd/migrate up         # apply 000014 → column dropped
go run ./cmd/migrate down 1     # roll back → column restored with DEFAULT 1
go run ./cmd/migrate up         # re-apply → column dropped again
```

Expected: all three succeed. The down migration leaves the DB in a state the pre-D4 schema can read (column present, default `1`).

- [ ] **Step 11.G.2: Commit**

```bash
git add backend/db/migrations/000014_drop_mapping_rules_version.up.sql \
        backend/db/migrations/000014_drop_mapping_rules_version.down.sql \
        backend/internal/db/rules.go \
        backend/internal/handlers/rules.go \
        backend/internal/handlers/router.go \
        backend/internal/handlers/deps.go \
        backend/internal/handlers/rules_test.go \
        backend/internal/models/models.go \
        backend/internal/services/views.go \
        ui/src/lib/queries/useMappingRules.ts \
        ui/src/lib/types.ts \
        ui/src/app/policies/page.tsx
git commit -m "$(cat <<'EOF'
refactor(rules): drop mapping_rules.version and the bump workflow

Versioning was unused as data — UpdateMappingRule's only job was
bumping the column; the only consumer was a "Bump version" button
that emitted a mapping_rule.version_bumped audit log and re-rendered
the rule with v(N+1). No caller ever read it for rollback, replay,
or history. audit_logs is the authoritative historical record per
the May 2026 audit-resolution design §2 (D4).

Removes 14 touchpoints:
- 000014 migration (up + down)
- db/rules.go: UpdateMappingRule; version in SELECT/Scan
- handlers/rules.go: handleUpdateMappingRule
- handlers/router.go: PUT /api/v1/rules/mapping/{id}
- handlers/deps.go: dbUpdateMappingRule injectable
- handlers/rules_test.go: three TestHandleUpdateMappingRule_*
- models.MappingRule.Version
- services/views.go: Topology version meta
- useMappingRules.ts: MappingRuleRow.version + useBumpMappingRule
- lib/types.ts: MappingRule.version (stale type kept consistent)
- app/policies/page.tsx: import, binding, handleBump, badge, button

Down migration restores the column with DEFAULT 1 for parseability;
historical version values are not recoverable (never read).

Audit ref: D4
Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Task 12 — Wave-level verification gate

Run every module's full suite, plus codebase-memory refresh and OpenSpec validation.

- [ ] **Step 12.1: Backend full suite**

```bash
cd /Users/notkanishk/Documents/Mkrspc/Projects/MkAuth/backend
go test ./...
go vet ./...
```

Expected: all green.

- [ ] **Step 12.2: Sync full suite**

```bash
cd /Users/notkanishk/Documents/Mkrspc/Projects/MkAuth/sync
go test ./...
go vet ./...
```

Expected: all green.

- [ ] **Step 12.3: Frontend full suite**

```bash
cd /Users/notkanishk/Documents/Mkrspc/Projects/MkAuth/ui
bun run lint
bun run test
bun run build
```

Expected: all green.

- [ ] **Step 12.4: gofmt on the wave's touch set**

```bash
cd /Users/notkanishk/Documents/Mkrspc/Projects/MkAuth
gofmt -d \
  backend/internal/handlers/zitadel_grant_lookup.go \
  backend/internal/handlers/rules.go \
  backend/internal/handlers/router.go \
  backend/internal/handlers/deps.go \
  backend/internal/handlers/rules_test.go \
  backend/internal/db/rules.go \
  backend/internal/models/models.go \
  backend/internal/services/views.go \
  sync/internal/worker/worker.go \
  sync/internal/ldap/client.go \
  sync/internal/ldap/client_test.go \
  sync/internal/config/config.go \
  sync/internal/config/config_test.go
```

Expected: zero diff.

- [ ] **Step 12.5: Codebase-memory refresh**

```
mcp__codebase-memory-mcp__detect_changes
mcp__codebase-memory-mcp__index_repository  (affected scope: backend, sync, ui, scripts, zitadel)
```

Expected: indexer reflects the deletions (`UpdateMappingRule`, `version` field, `PERMISSIONS.md`, `SIGNING_KEY.md`) and additions (`scripts/lib/load-env.sh`, `newGroupAddRequest`, retry env vars).

- [ ] **Step 12.6: OpenSpec validation**

```bash
cd /Users/notkanishk/Documents/Mkrspc/Projects/MkAuth
openspec validate wave-2-part-3-operational-polish --strict
```

Expected: `validation passed`.

- [ ] **Step 12.7: No commit on this task** (verification only).

---

## Task 13 — INDEX.md and feature-coverage.md updates

`feature-coverage.md` has two rows that this wave changes; INDEX.md gets one new change-log row.

**Files:**
- Modify: `openspec/changes/mkauth-core-architecture/specs/feature-coverage.md:45` (Versioned policies row)
- Modify: `openspec/changes/mkauth-core-architecture/specs/feature-coverage.md` (LDAP Sync row)
- Modify: `openspec/INDEX.md`

- [ ] **Step 13.1: Update the Versioned policies row**

In `openspec/changes/mkauth-core-architecture/specs/feature-coverage.md` line 45, replace:

```
| **Versioned policies** | Explicit versioning with rollbacks. | **Partial** | `backend/db/migrations/000001_init_schema.up.sql` (`mapping_rules.version`) | Version column exists, but repo does not show multi-version history, policy snapshots, or rollback primitives. |
```

With:

```
| **Versioned policies** | Explicit versioning with rollbacks. | **Removed** | `backend/db/migrations/000014_drop_mapping_rules_version.up.sql` | Column dropped in Wave 2·Part 3 (D4). audit_logs is the authoritative historical record; rule edits flow through DELETE+CREATE with both halves persisted. |
```

- [ ] **Step 13.2: Update the LDAP Sync retry row (if it exists)**

Search:

```bash
grep -n "retry\|RetryAttempts\|RetryBackoff" openspec/changes/mkauth-core-architecture/specs/feature-coverage.md
```

If a row mentions retry behavior, update it to note that `SYNC_RETRY_ATTEMPTS` and `SYNC_RETRY_BACKOFF` are now env-configurable. If no such row exists today, do not invent one — feature-coverage.md is intentionally sparse.

- [ ] **Step 13.3: Append a change-log row to INDEX.md**

In `openspec/INDEX.md`, find the change-log / proposed-changes table and append:

```
| `wave-2-part-3-operational-polish` | Proposed | Operational polish — drops mapping_rules.version, documents sync env surface, plumbs ctx through LDAP, extracts scripts/lib/load-env.sh. Resolves audit refs B8, S1, S3, S4, C7, C8, C9, C10, D4, D7, D9. |
```

(Adjust to match the actual column layout of the existing table — read the surrounding rows first.)

- [ ] **Step 13.4: Re-validate**

```bash
cd /Users/notkanishk/Documents/Mkrspc/Projects/MkAuth
openspec validate wave-2-part-3-operational-polish --strict
```

Expected: `validation passed`.

- [ ] **Step 13.5: Mark all OpenSpec tasks complete**

In `openspec/changes/wave-2-part-3-operational-polish/tasks.md`, change every `- [ ]` to `- [x]` for tasks 0 through 13.

- [ ] **Step 13.6: Final commit**

```bash
git add openspec/INDEX.md \
        openspec/changes/mkauth-core-architecture/specs/feature-coverage.md \
        openspec/changes/wave-2-part-3-operational-polish/tasks.md
git commit -m "$(cat <<'EOF'
docs(openspec): wave-2-part-3 INDEX + feature-coverage updates

- INDEX.md: new change-log row for wave-2-part-3-operational-polish
- feature-coverage.md: Versioned policies → Removed (cites the
  new 000014 migration); LDAP retry row gains env-configurable
  qualifier where present
- wave-2-part-3 tasks.md: all tasks ticked

Audit refs: D4 doctrinal closure, S4 visibility
Co-Authored-By: Claude Opus 4.7 <noreply@anthropic.com>
EOF
)"
```

---

## Self-Review

**Spec coverage:**

| Audit ref | Task | Note |
|---|---|---|
| C10 | Task 1 | Defer deletion + comment |
| B8 | Task 2 | Constant 100 → 10 + rationale |
| C9 | Task 3 | `/healthz` probe; bearer header removed |
| D9 | Task 4 | EXPIRY_SCHEDULER comment block rewritten |
| D7 | Task 5 | Sync block + MKAUTH_EXTERNAL_URL + ZITADEL_M2M_TOKEN |
| S3 | Task 6 | PERMISSIONS + SIGNING_KEY folded into README |
| C7 | Task 7 | member=[bindDN] via newGroupAddRequest helper |
| C8 | Task 8 | withConn(ctx, fn) + fail-fast |
| S4 | Task 9 | SYNC_RETRY_ATTEMPTS / SYNC_RETRY_BACKOFF env |
| S1 | Task 10 | scripts/lib/load-env.sh extracted (S2 deferred — design Decision 4) |
| D4 | Task 11 | Migration + 14 touch points across backend (db, handler, route, deps, tests, model, view) and frontend (mutation hook + types + policies page) |

Plus: Task 0 scaffolding, Task 12 verification gate, Task 13 INDEX / feature-coverage updates.

All 11 Theme 5 audit findings are mapped. S2 is the only deviation; documented in proposal "Out of scope" and design Decision 4.

**Placeholder scan:** Searched the plan for "TBD", "TODO", "implement later", "add appropriate", "handle edge cases", "similar to" — zero hits. Every step shows exact code or commands.

**Type consistency:**
- `newGroupAddRequest` (Task 7) — same name in test, definition, and call site.
- `withConn(ctx, fn)` signature — consistent across Task 8 caller updates.
- `SYNC_RETRY_ATTEMPTS` / `SYNC_RETRY_BACKOFF` env var names — same in `.env.example` block, config.go, tests, design.md, proposal.md.
- Migration filename `000014_drop_mapping_rules_version` — same up + down + commit message + design.md reference.
- `mapping_rules.version` column name — same throughout (Go field `Version`, JSON tag `version`, TS field `version`, SQL column `version`).

No drift found.

---

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-05-25-wave-2-part-3-operational-polish.md`.

The eleven body tasks are size-ordered and fully independent — ideal for either execution model:

**1. Subagent-Driven (recommended for this wave).** Each task is a 5-30 minute unit with its own commit. Dispatch a fresh subagent per task; review the diff between tasks. Highest task (D4) deserves its own focused agent run with the migration round-trip verification.

**2. Inline Execution.** Run all tasks in this session with a checkpoint after Task 6 (mid-wave; everything before is mechanical, everything after touches behavior).

Either way: the verification gate at Task 12 is the hard ship-line. Do not skip the migration round-trip in Task 11 Step 11.G.1.
