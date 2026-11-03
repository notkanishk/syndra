# Q-11 · Shaping what an app is told: a clearer flow

From Claude Code, 3 Nov 2026. The owner asked for "more features, a clearer flow and a better framework" for shaping the claims Actions v2 sends.

## Locked

- **One shaper.** `internal/claims` builds every token, both the live one and the preview (CLAUDE.md). A preview made by other code previews nothing.
- **One project per token** (ADR-002, `docs/adr/002-one-project-per-token.md`). A token carries one project's claims or none. A request that spans projects gets none, and the log names the client. Keys are unique within a project.
- **Where it lives today:**
  - The app page (`route-applications-id`; `ui/src/components/apps/TokenFormatEditor.tsx`) has the claim name, the format, the static values, and a preview for one person.
  - The preview uses `GET /applications/{id}/simulate` (`router.go:87`). The project's shape is read from `GET /projects/{id}/claim-shape` (`router.go:93`).
  - The board draws "This preview shows the saved format; save to preview".
- **Gaps the outside review found** (`docs/from another side/ACTIONS-V2-REVIEW-2026-10-27.md`, from line 284):
  - The preview doesn't run the live path: live reads a cache, gated by the project Zitadel lists for the token.
  - An empty token can't say why.
  - The apps index shows the project default, not each app's actual claim (`applications/page.tsx:142`).
  - "Unset" is guessed: `looksUnset` treats `roles` + array as unset (`TokenFormatEditor.tsx:322`).
  - Static values become text.
  - A clash error doesn't name the key.
- **Code will build one "explain" step** that the live path and every preview call. It returns the token plus the result of five checks:
  1. Zitadel lists the app's project for this token. That needs "assert roles" on the app, or the app requesting role scopes, and no second project.
  2. The person holds roles in that project, as observed in Zitadel.
  3. Syndra's claims for them are ready in the cache, and when they were built.
  4. Which shape applies (the project default or this app's own) and which keys it produces.
  5. What Zitadel receives.
  
  A draft (unsaved edits) can run through the same step.
- **History:** a save is an immutable snapshot (NEXT.md C5), not draft and publish.

## What I need

1. **The flow.** How the app page reads, top to bottom, once the five checks exist: where the checks sit (a column beside the token, a strip above it, or folded under it), and how a failed check shows its fix. Some fixes are in Zitadel, not Syndra ("Turn on Assert roles in Zitadel").
2. **Previewing an edit before saving.** This reverses the "save to preview" line. Show the before and after, and how a difference is marked.
3. **Test people.** Pin 3–5 people to an app and show their tokens as a before/after table on every edit. Where do they live, and how are they picked (the role picker's person form)?
4. **Saving.** The summary before the save confirms: "Changes the token for 3 apps and 41 people; adds `team`, renames `roles` to `printing.roles`". Is it a dialog, or a before-line on the Save button?
5. **History.** A version list and "Go back to this version". Is it a tab, a fold, or a sheet?
6. **What the app reads.** Let an operator note "this app reads `printing.roles` as a list", and show matches or doesn't match. Where does it go? Also, the apps index shows each app's real claim name.
7. **Ready to move off Actions v1.** A checklist for each app, with a comparison against a pasted v1 token. Is it on the app page, or one page for all apps? Whether it gates anything is the owner's call. Draw it as advice.
8. **Static values:** how the type (text, number, yes/no, list) is chosen and shown.

**Answer at** `handoff/2026-10-reconcile/from-design/A-11-claims-shaping.md` on the canvas, numbered 1–8, with the board beside it.
