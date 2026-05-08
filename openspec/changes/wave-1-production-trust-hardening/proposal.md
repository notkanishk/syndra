# Wave 1 — Production Trust Hardening

**Status:** In progress
**Source:** [May 2026 audit resolution design](../../../docs/superpowers/specs/2026-05-08-may-2026-audit-resolution-design.md) §3 Theme 1
**Phase:** 5.5

## Why
The May 2026 codebase audit identified five ship-blocker findings that erode operator trust in production:
- **C1** — Backend silently passes through Zitadel webhook/action requests when signing keys are unset, even with `ZITADEL_DOMAIN` configured.
- **C2 / D5** — OIDC member dashboard renders blank Title/Team/Location because the cookie is never populated from Zitadel metadata.
- **D1** — `GetWelcomeBundle` falls back to "first bundle by created_at" when no welcome bundle is configured, silently assigning the wrong bundle to new users.
- **C3** — Shadow-credential vault accepts mutations in dev mode without any actor attribution; the audit log records the target user as the actor.
- **B1** — `backend/cmd/test/main.go` is a destructive bootstrap script that runs `DELETE FROM mapping_rules` and has no callers.

These five fixes share no code paths but all gate the production deployment story. Shipping them as a single coordinated change keeps the audit-resolution wave structure visible.

## What changes
- Backend fails fast at startup if `ZITADEL_DOMAIN != ""` and either signing-key env is empty.
- New `GET /api/v1/me/profile` endpoint and OIDC callback wiring populates Title/Team/Location for OIDC sessions identically to demo sessions.
- `bundles.is_welcome` column with a single-true partial unique index; `GetWelcomeBundle` errors loudly when no bundle is marked.
- Vault `enforceSelfOnly` requires `?actor=<id>` for PUT/DELETE in dev mode and refuses 400 otherwise; reads unchanged.
- `backend/cmd/test/main.go` deleted.

## Out of scope
Theme 2's drift-control architecture, Theme 3's backend coherence refactors, Theme 4's UI palette migration, and Theme 5's operational polish all ship in later waves.
