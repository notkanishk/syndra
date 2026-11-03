# Claims shaping: one shape per app (A-14)

## Why
A token carried every app's claims on its project: the project default plus every override. So each app's override reached its siblings, and two apps on one project couldn't be told different things. The research (`docs/research/2026-11-03-app-claims.md`) shows real apps need choices this couldn't make:
- which roles an app sees, and the word each is sent as;
- nested claims, for Supabase;
- where the claims land, for NetBird and Supabase;
- always `[]`, never a missing claim.

A-14 redraws the page around one shape per app, with templates, copying, history and debug mode. Its rulings are settled by the owner (3 Nov).

## What changes
- **One shape per app (ADR-003).** The data plane maps `application.client_id` to its app. An app with no shape of its own gets its project's default, and a project with none gets the built-in `roles` list.
- **The shaper grows.** It gains role maps, ordered and nested rows, if-role, bundles, role ends and where claims land. `Diff` and `Breaking` feed every before-line, summary and history entry. A static template catalog follows the research exactly.
- **Facts grow.** The cache compiler persists given name, family name, username, bundles, role ends and is_service_account.
- **Per-app versions** (migration 000071). Saves are immutable; going back saves an old one again. Drafts come from a template or from any app's shape. Saves the app will act on need `acknowledge_breaking`.
- **One explain step** with five checks, for the live token and every draft. Check 1 reads Zitadel's token settings, read-only.
- **Debug mode.** One chosen account at a time, 15 minutes enforced server-side, in memory only, audited.
- **A lazy seed.** Every app that existed under Actions v1 gets v1's exact shape as version 1, so turning v1 off changes nothing.

The API contract is in `design.md`.

## Not in this change
The UI. That's a later slice built on this API. Also out: test people, "what the app reads", and the cross-app comparison (see `design.md`, "Not built here").
