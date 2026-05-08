# Wave 1 — Design

This design defers to the [May 2026 meta-spec](../../../docs/superpowers/specs/2026-05-08-may-2026-audit-resolution-design.md) for cross-cutting structure and timeline. Per-item notes:

## C1 — Production refuses missing signing keys
Two layers:
1. Startup gate in `cmd/api/main.go`. When `ZITADEL_DOMAIN != ""`, fail fast (log.Fatalf) if `ZITADEL_EVENT_SIGNING_KEY` or `ZITADEL_ACTION_SIGNING_KEY` is empty. Runs before the HTTP server is bound, so a misconfigured deploy never accepts traffic.
2. Middleware tightening in `withZitadelActionSignature`. Dev-mode passthrough is now conditional on `ZITADEL_DOMAIN == ""` *as well as* the secret being empty. If `ZITADEL_DOMAIN != ""` and (somehow) the secret is empty at request time, return 503 INTERNAL.

The two-layer design is belt-and-suspenders — startup catches misconfiguration; middleware refuses to silently degrade if startup is bypassed (e.g. signal-induced reload mid-flight).

## C2 / D5 — OIDC profile metadata
A new authenticated endpoint `GET /api/v1/me/profile` resolves the requester's user ID from their bearer token and returns the same `models.UserProfile` shape the directory layer already overlays (`title`, `team`, `location`, `name`, `email`, `status`).

The Next.js OIDC callback handler hits this endpoint immediately after token exchange, with the freshly-issued access token, and writes the full `UserProfile` into the session cookie. Cookie size impact: ~120 bytes worst-case — well below the 4 KB limit.

Demo sessions already populate these fields from the demo catalog; the change makes OIDC sessions render identically.

## D1 — Welcome bundle errors explicitly
Schema:
```sql
ALTER TABLE bundles ADD COLUMN is_welcome BOOLEAN NOT NULL DEFAULT FALSE;
CREATE UNIQUE INDEX idx_bundles_welcome_unique ON bundles (is_welcome) WHERE is_welcome = TRUE;
```

The partial unique index enforces "at most one welcome bundle" at the database layer. The default of `FALSE` keeps existing rows safe.

`GetWelcomeBundle` becomes a single-row select on `WHERE is_welcome = TRUE`. `pgx.ErrNoRows` is mapped to a domain error `ErrNoWelcomeBundleConfigured`. Onboarding propagates that error verbatim; the trigger row is marked `failed` with the same string so operators see "no welcome bundle configured" in the UI rather than a silent default-bundle assignment.

`SetWelcomeBundle(bundleID)` is a transactional clear-then-set: `UPDATE bundles SET is_welcome=FALSE WHERE is_welcome=TRUE; UPDATE bundles SET is_welcome=TRUE WHERE id=$1`. The partial unique index is a backstop, not the contract.

UI: `BundleRowCard` renders a `Welcome` badge when `is_welcome=true`; the bundle's expanded panel exposes a `Set as welcome bundle` button. No autopromote.

## C3 — Vault dev-mode self-attribution
`enforceSelfOnly` gains a `requireActor bool` parameter. Mutations (`PUT`/`DELETE`) call with `requireActor=true`; reads call with `requireActor=false`.

When `getAdminUserID(ctx) == ""` (dev-mode API-key auth) and `requireActor=true`, the handler reads `?actor=<id>` from the query. Empty → 400 `MISSING_ACTOR`. Non-empty → use it as `actorID`; log it with the prefix `[VAULT] dev-mode actor`.

The query parameter never propagates to the audit log as a JWT actor — it's stamped into `actorID` only after the JWT-actor branch is exhausted. The audit reads exactly the same.

## B1 — Delete test/main.go
Run `git log --diff-filter=A -- backend/cmd/test/main.go` for blame; verify nothing imports it (`go list ./...` from `backend/`). Remove the file and the empty `cmd/test/` directory.
