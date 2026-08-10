# Add-on platform — UI handover

**For:** the design agent that already built Syndra's design system.
**Status of this work:** routes, data wiring and behavioural tests are in and
green. Visual design is deliberately not. Everything below is a real screen with
real data behind it — what it needs is the design system applied to it.

**Read first:** `openspec/changes/basic-advanced-ia/design.md` (the Basic /
Advanced contract, the navigation rules, where the token shape lives) and
`openspec/changes/obsidian-clarity-redesign/` if it exists. The two rules that
constrain everything here:

1. **Design tokens live only in `ui/src/app/globals.css`.** No hardcoded hex in a
   component, both themes authored in full.
2. **Structure never moves in response to data.** A section with nothing in it
   renders in place with a hollow `0`. It never disappears and nothing ever
   appears above it.

---

## What changed in the navigation

`ui/src/lib/nav.ts`:

| Change | Where | Why |
|---|---|---|
| `Hardware sync` **removed** | Advanced › System | It named the LLDAP bridge, which is deleted. A nav row for a subsystem that no longer exists is worse than a missing one — an operator clicks it before they read anything. |
| **One row per registered add-on** | Advanced › System, between `Identity provider` and `Event activity` | `targetNav(targets)` builds them from `GET /api/v1/targets`, which is **deployment configuration**. A row appears because the deployment registered an add-on — never because this operator can see data. |
| `Withdrawn access` **added** | Advanced › Review, after `Unexplained access` | Its own destination, never a tab inside drift. Drift is access that appeared and cannot be explained; this is access somebody decided to take away that is still there. |
| `Network storage` **added** | Member nav (third row) | Present for **every** member whatever they can reach. |

Two things a visual pass must not undo:

- The member storage row is **ungated**. Gating it on entitlement would make the
  rail move as somebody's roles change, and it answers the wrong question: a
  member without access is asking whether they can get it, and a missing row
  does not answer that.
- The target rows carry **no badge**. A count on them would be data driving
  structure.

---

## Screen 1 — Member › Network storage (`/storage`)

`ui/src/components/storage/MyStorage.tsx` · tests in `__tests__/MyStorage.test.tsx`

**Three states, and they must stay three.** This is the design constraint that
matters most on this screen; a two-state "access / no access" design produces a
page that lies.

| State | What renders | What must NOT render |
|---|---|---|
| **No entitlement** — no role of theirs is mapped here | An explanation that access comes with a role | No credential form, no account name, no connection instructions |
| **Entitled, no account yet** — the change is queued and the drain has not run | "Recorded, not created yet, nothing needed from you" | **No credential form.** Setting one dispatches at an account that does not exist and tells them their password was set |
| **Account present** | Account name, what they can reach, the credential form | — |

The middle state is not an edge case. Add-on changes wait for an operator to
resume the drain, so it is the ordinary experience of every new member until
that happens. It deserves real design attention, not a spinner.

**Also on this screen:**

- **Withheld access.** When an operator has suspended something, the reason is
  rendered. A member seeing access they expect and do not have, with no
  explanation, asks an operator; one who can read the reason does not.
- **The scope sentence** under the password field ("used only for TrueNAS, not
  your Syndra sign-in and not your Google account") is load-bearing copy, not a
  hint. Members reasonably assume one password.
- **Unreachable target**: the form is replaced by an explanation. A credential
  set against an add-on that never answered must never be reported as done.

**Design questions worth answering:** how the three states read as a progression
rather than as three unrelated panels; whether the account name wants the
treatment a connection string gets elsewhere; what the pending state looks like
when a member refreshes it four times in a day.

---

## Screen 2 — Advanced › System › ‹target› (`/system/targets/[target]`)

`ui/src/components/targets/TargetOverview.tsx` · tests in `__tests__/TargetOverview.test.tsx`

Four panels, answering four questions in this order.

### Health

Five states that a single "status" chip would flatten, and each one sends an
operator somewhere different:

- **unreachable** — the add-on did not answer.
- **circuit open** — Syndra is refusing its own calls after repeated failures.
  *Not* the target being down, and an operator who reads it as that looks at the
  wrong machine.
- **draining / read-only** — a deliberate maintenance state somebody set, with
  their reason. Must not read as a fault.
- **backlogged** — `in_flight > 0` while draining. This is what an operator
  waits on before pulling a credential out from under a call.
- **serving from a stale mirror** — reads are answered from a copy, labelled
  with its age. Data with an age, not an error.

And one more, which arrived after this document was first written and does not
belong with the five above because it is not a state of the target's health:

- **`log_anchor` with a `violation_reason`** — the target's mutation log is no
  longer an extension of the one Syndra remembers: records that existed are gone,
  or the same number of them now hash to something else. This is the strongest
  evidence the system produces and it is a SECURITY finding, not a health
  degradation — it appears whether or not the add-on is currently answering, and
  it must not be rendered as another kind of amber. `violation_reason` is
  `records_decreased` or `head_rewritten`, `violation_at` is when it was seen,
  and `head`/`records` are where the anchor stopped. The anchor deliberately does
  not advance past it, so the finding stays until somebody resolves it.

### Accounts Syndra did not create — the unmanaged inventory

A real NAS holds `root`, service accounts and whatever an admin made by hand.
**These are never drift and must never be rendered as drift**: classifying them
as untraced access would bury the triage queue on the first sweep after
deployment, and trust in a triage queue is set on the day it first fills.

Adoption is the one action, and it is heavy: adopting the wrong account hands a
member somebody else's home directory, their shares and their group memberships,
and the next convergence makes that look intended. There is no undo. The
affordance is **disabled while the read is stale** — you cannot adopt from a
list that may have moved.

Copy that must survive: *"Nothing on the account changes now; the next
convergence applies their entitlements to it."* The natural reading of "adopted"
is that something was applied, and nothing was.

### Capabilities

Rendered **from the manifest**, never from a list in the frontend. An operation
removed from an add-on's manifest disappears here with no frontend change, and
that is asserted by a test. An operation the target cannot perform is shown
**disabled with its reason** rather than omitted — omitted, an operator wonders
whether the feature exists at all.

`secret_params` names the parameters whose values are never logged, stored or
echoed. A form built from this should mark them; there is nowhere in the payload
for a value.

### Maintenance

Three buttons and a mandatory reason. `draining` and `read_only` differ in one
way that matters during a credential rotation: draining lets calls already
issued settle. The explanation above the buttons is the whole of how an operator
picks the right one.

---

## Screen 3 — Advanced › Review › Withdrawn access (`/governance/unconfirmed-revocations`)

`ui/src/components/review/WithdrawnAccess.tsx`

**Two populations, rendered apart.** Merged into one count, a healthy queue of
five-minute-old rows hides a revocation that failed permanently three days ago.

- **Not going to happen** (`spent`) — terminal. Nothing will dispatch it again;
  waiting produces nothing and somebody has to act. Danger tone.
- **Still draining** (`queued`) — being retried. The only content of the signal
  is how long it has waited. Accent tone.

Each row carries the reason it failed, because a terminal row an operator can
see and not act on is the whole difference between a finding and a mystery.

The badge escalates on `revocations_escalated`, not on the count: any spent row
escalates immediately, and a queued one escalates after 24 hours. A count cannot
carry that difference and an operator reading "3" cannot tell.

---

## Not built, and deliberately left for this pass

These have backend endpoints and no screen yet. Each is a real gap, listed with
what it needs so it is not rediscovered.

| Task | What it needs | Endpoint |
|---|---|---|
| **9.3–9.6** Entitlement plan-then-apply UI | The `rehearse* → apply*` pattern already used by bulk grants and drift triage. Apply must carry the **plan id**, never the original submission. Stale-plan recovery re-plans and names which subjects moved. | `POST /targets/{t}/entitlements/rehearse`, `.../apply` |
| **9.7–9.8** Mapping management with version history | Rollback, and the plan shown before any edit or delete lands. The blast-radius acknowledgement is enforced by the backend; the UI has to show the number. | `POST /targets/mappings/{id}/rehearse-edit`, `rehearse-delete`, `PATCH`/`DELETE` with `plan_id` |
| **9.11–9.12** Dormant-account housekeeping | Reason, age, individual and bulk action, plan before apply. Accounts held by an active role are excluded. | Needs a listing endpoint; not built |
| **9.22** Allowance authoring | Both bounded forms — an expiry, **or** no expiry with a mandatory review date. A denial with neither is refused by the backend. | `POST /allowances` |
| **9.25** Review-date surfacing | An indefinite suspension appears in governance once its review date passes, and stays in force until decided. | `GET /governance/allowances/review-due` |
| **10.8** Connection instructions | The add-on-reported account name is already on the member page; the SMB path / mount instructions are not. They must list **only** resources current entitlements reach. | `GET /me/targets` |

The revocation composition (6.17) has a backend endpoint —
`POST /targets/{t}/users/{id}/revoke-access` — and no button anywhere. Its copy
is fixed by the backend and must be shown verbatim: *"Sessions already
established end when they next reconnect — this target has no way to close
one."* This target cannot end a session, and a UI that implies otherwise is the
failure that endpoint's whole design is arranged to prevent.

---

## Contracts the design must not quietly change

- **Queued is not succeeded.** Every apply response carries a `summary` whose
  `succeeded` is always zero, present precisely so a client cannot default it.
  Nothing an operator does on these screens reaches a target directly — the rows
  wait for the drain.
- **Provisional is not applied.** A plan computed while the target was
  unreachable carries `provisional: true` and `state_read_at`. It must be
  labelled with that age; "computed against last-known state" with no number is
  a label nobody can act on.
- **Truncated is not empty.** A capped read reports what it saw; absence proves
  nothing.
- **A confirmation is a backend refusal**, not a dialog. `account.adopt`,
  `account.purge` and the revocation composition are refused without one — a
  dialog that only the frontend enforces is a suggestion.

Three more, added after the platform was deployed and run (§19 of `tasks.md`).
Each one is a sentence the backend now composes from what actually happened, and
each replaced one that was wrong:

- **An adoption answers three ways, and only one of them is "adopted."** `200`
  with `status: adopted` means the target confirmed it and the binding is
  written. `409 ADOPTION_REFUSED` carries the target's own words — already bound
  to somebody else, no such account — and nothing was recorded. `202` with
  `status: unconfirmed` means the target did not answer, nothing was recorded,
  and the operator should look at the inventory before trying again. Rendering
  all three as success is what this endpoint used to do.
- **A partial revocation says which half is outstanding and what to do about
  it.** `status: partially_revoked` always means the suspension is recorded and
  the credential is NOT replaced. The `detail` differs by outcome and must be
  shown verbatim: a refusal names the reason, an unreachable target says to try
  again, and an unconfirmed one deliberately does NOT — a second rotation on an
  account that did rotate locks the member out of it.
- **A drain reports one pass per target.** `passes[]` carries each target's own
  counts and its own halt; `halted_target` names whose reason the top-level
  `reason` is. A combined "halted" cannot say which target halted, and that is
  the whole of what an operator does next — a Zitadel outage no longer holds a
  reachable NAS's queue, so "halted" beside `applied: 9` is now an ordinary
  result rather than a contradiction. `POST /targets/{t}/propagations/drain`
  resumes one target alone.
