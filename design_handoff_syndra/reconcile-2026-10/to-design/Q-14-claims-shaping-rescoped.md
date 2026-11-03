# Q-14 · Shaping what each app is told, rescoped by the owner

From Claude Code, 3 Nov 2026. This revises A-11; keep what still fits.

## The owner's direction (3 Nov)

- Syndra, through Actions v2, is the one place that decides what shape and data each app receives in its claims.
- The owner is removing the Actions v1 scripts from Zitadel himself. So:
  - **drop** A-11's "Moving off Actions v1" card (the checklist and the pasted-token comparison);
  - **drop** "start from a token".
- What he wants is a rich, elegant, convenient per-app shaping experience, with these features (all accepted):
  1. **Rename roles per app.** A role reaches an app in that app's own word.
  2. **Choose which roles an app sees.**
  3. **Nested claims** (`custom_claims.groups`).
  4. **Claims that depend on holding a role** (`is_admin: true` if they hold X).
  5. **More details:**
     - given and family name;
     - username;
     - bundles held;
     - each role's end date;
     - whether the account is a service account.
  6. **Ready-made shapes (templates) for known apps.**
  7. **Debug mode.** See live the whole request Zitadel sent and the response Syndra gave, for this app.
- Kept from A-11:
  - the five-check trail;
  - draft preview before saving;
  - test people;
  - the save summary;
  - history and "Go back";
  - what the app reads (matches / doesn't match);
  - typed static values;
  - the apps index showing each app's real claim.

## Locked

- `internal/claims` is the only shaper. One project per token (ADR-002).
- **What v1 does today** (read from Zitadel, 3 Nov): one active script, `groupsClaim`, on Pre Userinfo and Pre Access Token. For every app it sets:
  - `groups`: every role key, flat;
  - `custom_claims`: { groups, given_name, family_name, name }.
  
  Per-app shaping replaces this one-size shape.
- **The apps:** NetBird (web and clients), Supabase, GitLab, Vaultwarden, Kan, Home Assistant, Navidrome, TrueNAS, and in-house apps (SmartDB, Community Slicer, Inventory, Audio-Dash, Printer Config, Syndra).
- **The research** (`docs/research/2026-11-03-app-claims.md`; read it, at least "Facts that apply to every app", "What the shaper must support" and each app's recommended template):
  - **No app accepts a comma- or space-separated string.** Every template uses a list. The csv and space formats should stop being offered as equals: they're a trap.
  - **Renames are breaking for the app.**
    - NetBird turns each value into a group and moves members on rename.
    - GitLab's admin or required groups can demote or lock out everyone if the claim goes missing or changes.
    
    Saving a rename or a removal must say so plainly, before the save.
  - **Where a claim lands matters.**
    - NetBird reads the access token.
    - Supabase's newer provider reads the ID token; its Keycloak provider reads userinfo.
    - Zitadel's own app settings decide it: the token type must be JWT, and "User Info inside ID Token" must be on.
    
    A template must say where its claims go, and the checks should catch a mismatched app setting.
  - **Always send `[]`, never omit.** Supabase keeps a user's old groups if the claim is missing.
  - **Some needs aren't claims at all:**
    - Vaultwarden and Kan have no groups, so who may sign in is Zitadel's project role check.
    - Home Assistant applies roles only when the HA user is first created.
    - Supabase's `user_metadata` is editable by the user, so it can't be trusted.
    
    The template pages should say these plainly instead of pretending.
  - **Features no app needs today:** conditional claims, bundles and role end dates. The owner still wants them: give them a quiet place ("More claims"), not the front.
- **Debug mode** (code's plan, the owner's request):
  - Per app, off by default; turned on from the app page; turns itself off after 15 minutes.
  - Optionally limited to one person (the operator, or a test person).
  - Captured in memory for about an hour, never in the database. Who turned it on is audited.
  - **What it shows, live, for each sign-in:**
    - Zitadel's request: user, app, the role grants Zitadel listed, which function (userinfo or access token);
    - Syndra's answer: `append_claims`;
    - the five checks for that sign-in.
  - **It can't show the signed token itself.** Zitadel adds its standard claims and signs it after Syndra answers. A reconstruction, "Your claims, as the app will see them", must be labelled as such. Signing headers and secrets are never shown.

## What I need

1. **The page, redrawn:** the order of the parts, now that the v1 card is gone and templates, renames, role choice, nested and conditional claims and debug mode exist. Keep it calm. Basic vs Advanced: what an operator sees first, and what waits behind Advanced.
2. **Templates:** how one is picked (when an app has no shape yet, and later), what the template page says, including its "not a claim" notes and where its claims land, and how a template shows that you've changed it since.
3. **Roles for this app:** one table for choosing and renaming (role · "This app calls it" · included). Show the breaking-change warning before a save that renames or removes.
4. **Claim rows:** roles claim, details, fixed values, nested paths, conditional claims. One consistent row form. The csv/space formats: hidden, demoted, or behind Advanced with a warning.
5. **Where claims land:** ID token, access token, userinfo, or both. A choice per app or per claim; and the check that catches a Zitadel app setting that doesn't match.
6. **Debug mode:** turning it on (the before-line, the 15-minute end, the one-person option), the live list of sign-ins, one sign-in opened (request, answer, checks, the labelled reconstruction), and its empty, ended and phone states.
7. **Anything from A-11 that should change** because of the above.

**Answer at** `handoff/2026-10-reconcile/from-design/A-14-claims-shaping-rescoped.md` on the canvas, numbered 1–7. The board `a14-claims-shaping.dc.html` replaces A-11's board as the final one; A-11's stays for the record.
