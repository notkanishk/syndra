# Zitadel pages: notes for code (6 Oct 2026)

Settled on explore-zitadel-questions.dc.html (4a 5b 6b+6a 7c 8a, 2c) and drawn on route-zitadel.dc.html.

## Reading and writing apart
- Look inside opens three read-only pages: Projects (one folding list), People (list beside detail, with search), Roles held (one table, with search). No write control on any of them.
- Change Zitadel directly (folded, dashed red) has two red ButtonLinks to two change pages: Change people's roles, Change projects and roles. The one deliberate exception to "going somewhere is a violet link".
- Each change page: back link, title, the standing line in danger text, then the list inside the same dashed red frame. Verbs on rows, each opening a dialog that asks for the tick and becomes its result (#104).

## People's roles go straight to Zitadel, like projects and roles
- Today the direct grant path (`EnqueueDirectGrantPropagation`) records a ledger row and queues for Pending changes, which is exactly what a person's page already does. Two routes to one outcome.
- Change: direct people's-role writes go straight to Zitadel, like project and role edits: one Audit line, no ledger row, no queue. Same tick and copy as projects ("no preview and no record in Syndra"). Retire the `traced` branch of DirectWriteWarning.
- Nudge to the Syndra route, evident but not nagging:
  - The page lede says the person's page does the same with a reason, a plan and a record, with a link.
  - The selected person's card opens with one accent note linking to their Syndra page ("Change it on Priya Nair's page →").
  - Each dialog ends with a one-line alternative link above the tick.
  - A role that already matches Syndra says so in the dialog: removing it here is listed under Drift (Only in Syndra) for somebody to decide, and roles derived from it by automatic rules end with it.

## Checks line (7c)
- One line under the header: reply time always visible, then "all four checks pass" / "1 of 4 checks needs a look", then Show checks. Opens by itself when a check needs a look.
- Backend owed:
  - Allowed to change: a permission self-check of Syndra's Zitadel service account (read and write on projects, roles, user grants).
  - Events: compare the latest event sequence in Zitadel's event API with the last one Syndra received; report the gap count and oldest missing time. Never judge by silence.
  - Sign-in details: count Action calls today and failures (signature, timeout), from Syndra's own Action handler.

## Recent direct changes (8a)
- Inside the fold only: the last three direct writes, who, what, when, and what happened after (listed under Drift, given back by the next pass, explained since). Then All in Audit →. Needs an Audit filter for direct writes.

## Signing key (2c)
- One row; How to replace it opens a four-step guide. Every command is a CommandBlock and every setting a CopyableValue. Step 4 re-reads the key age from Syndra and becomes its result.

## For Claude Code to deliberate (raised 7 Oct 2026)

### What happens after a direct removal of a person's role
What the code does today, read from source:
- The sweep (services/drift/sweep.go) does NOT give back a role Zitadel was seen holding and no longer does: it records a `syndra_only` drift row for triage. It replays only a grant Zitadel was never seen holding (never projected).
- The webhook (handlers/webhook.go, processGrantRemoved) records the same `syndra_only` finding via detectWebhookRemoval, then revokes roles derived from it by mapping rules (webhookRevokeMappingRules). So a direct removal also takes away automatic roles that depended on it, at once.
- The removal arrives as `user.grant.removed` and shows in Incoming events.
- The drawings now say exactly this: listed under Drift, derived automatic roles end too, shows in Incoming events. Earlier copy ("the next pass gives it back") was wrong.
Open questions:
- Should the derived-role cascade wait for the Drift decision instead of firing on the event? Today a direct removal someone then reverts in Drift (give it back) has already cost the person their automatic roles in between.
- Should a removal Syndra made itself through the change page (one Audit line, known actor) still raise a `syndra_only` row, or be recorded as explained by that Audit line with the operator named? The second avoids a person reviewing their own change in Drift, but makes the direct route quieter than it should be.
- Drift should name the actor from the event (`UpstreamActor`) so the row reads "Removed in Zitadel by Kanishk Singh" rather than nobody.

### Fewer steps to replace the signing key
Today: four steps, three of them on the server (rotate, edit .env twice, restart, verify).
Options to weigh:
- One command does it all: `make zitadel-actions-rotate-key` writes both settings to .env, restarts the backend and verifies. The guide becomes one copy row and a live "waiting for the new key" line that turns healthy when Syndra reads it. Still server-side; no secret in the browser.
- Syndra rotates it itself: the backend creates the new key in Zitadel through its API, stores it, and accepts both old and new for a short overlap, so there is no gap where sign-ins miss their access details. The UI becomes one button with a confirm. Needs Syndra's Zitadel account to be allowed to manage Actions, and the key to live outside .env.
- Either way, keep the age check and the 90-day reminder on Home.
