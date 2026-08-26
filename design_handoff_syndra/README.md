# Handoff: Syndra

## Overview

**Syndra** is an access-management tool for a single academic makerspace. It sits
on top of [Zitadel](https://zitadel.com) as the identity provider and answers one
question for the person running the shop: *who may enter, why, and until when.*

It is a personal, open-source project, not a commercial product. There is one
makerspace, a handful of operators, and one maintainer. Design decisions
throughout favour **being honest about what the software knows** over looking
capable.

The name comes from **Syn**, the Norse goddess who keeps the door of Frigg's hall,
bars it against those who should not enter, and is invoked as a defence at
trials. Those are the product's two halves: the access list, and the record of
why it looks the way it does. The tagline is *"Syn keeps the door. Syndra keeps
the list."*

This bundle covers the **whole application**: 41 screens across three audiences,
plus the login screen and the repo banner images.

Syndra also runs an **add-on platform**. An add-on ("target") is a system that
holds accounts of its own — a NAS, a file server — which Syndra provisions into.
Nothing Syndra does reaches a target directly: changes are queued and an operator
resumes a drain. That single fact shapes three of the screens below, and every
piece of copy on them. §19–§30 are the add-on platform; they were added after the
first pass and are the newest work in the bundle.

---

## Commission 2 is open — `addon-2/`

The target page grew from §21's four panels to eleven, and six surfaces built
since appear on no board at all. `addon-2/README.md` says exactly which, and
`addon-2/CLAUDE-DESIGN-PROMPTS.md` is the prompt pack that commissions them.
Prompt A is the one that matters: the target page's *composition*, which was
never designed even though four of its panels were.

## Read BUILD-NOTES.md first

`BUILD-NOTES.md` was written after reading the Syndra repo and **overrides this
file wherever they disagree.** Three things in particular:

- **Never paste a hex from the board** — `ui/src/__tests__/design-system.test.ts`
  fails the build on raw hex. BUILD-NOTES maps every board colour to its token.
- **§23 is not canonical after all.** `components/ui/RehearsalDialog.tsx` already
  is the rehearse-then-apply shape the product wears; entitlement plan-then-apply
  becomes a caller of it. Its scope step is a better blast-radius design than the
  checkbox drawn in §24 — build that as a backend refusal.
- **The lost-card question is answered.** The backend drains inline, so §28 ships
  as drawn and §31's fallback copy is not needed.

## About the design files

The files in `design/` are **design references created in HTML**. They are
prototypes that show intended look and behaviour. They are **not production code
to lift wholesale**.

The task is to **recreate these designs in the target codebase's existing
environment** — React, Vue, Svelte, a server-rendered template, whatever the
project already uses — following its established patterns, component library and
conventions. If no frontend exists yet, choose the most appropriate framework for
the project and implement the designs there.

### How to read the design files

`design/Syndra IA.dc.html` is the master document. Open it in a browser (it needs
`support.js` beside it, which is included). It is a single wide canvas divided
into numbered sections, §01 through §32. Each section contains one or more
`<figure>` elements; **every `<figcaption>` names the screen id and its API
endpoint**, e.g.:

> E3 · `GET /api/v1/users/{id}/access` …

Those captions are the authoritative screen-to-endpoint mapping. Read them.

The captions also carry the *reasoning* behind each decision. When you have to
make a judgement call the README does not cover, the caption for that figure is
usually the answer.

`design/Sidebar.dc.html` and `design/Source.dc.html` are two real components,
implemented rather than mocked, and used repeatedly throughout the board. Their
logic classes are the specification for their behaviour.

---

## Fidelity

**Fidelity is not uniform across the board.** The header chips in
`Syndra IA.dc.html` declare the split, and it matters — building a mid-fidelity
mock pixel-perfectly wastes your time and bakes in numbers nobody chose.

| Surface | Screens | Fidelity | How to treat it |
| --- | --- | --- | --- |
| **Basic** | E1–E7, Today, People, the four list states (§04–§12) | **High** | Colours, typography, spacing, radii, easings and durations are final and exact. Recreate pixel-perfectly. |
| **Unexplained access** | S6 + revoke dialog + reconciliation (§14, §15) | **High** | Same. The highest-stakes screen in the product, designed to the pixel. |
| **Member** | My access, member Requests (§10) | **High** | Same. |
| **Advanced** | S1–S5, S7–S11 (§15–§18) — Expiring access, Bundles, Confirmation policy, Automatic rules, Pending changes, Change history, Audit, Identity provider, Intents, Event activity | **Mid** | Layout and information architecture are settled and should be followed. Treat spacing and type sizes as **directional**: apply the token set in [Design Tokens](#design-tokens) rather than measuring the mock. |
| **Light theme** | §13 | **Directional** | Proves the palette inverts. Not a spec for every screen. |
| **Add-on platform** | §19–§22 — nav delta, member TrueNAS, target overview, withdrawn access | **High** | Exact values. |
| **Add-on platform, second pass** | §23–§26 — plan-then-apply, mapping management, holds, the member side with two add-ons | **High** | Exact values. **§23 is canonical** — the existing bulk-grant and drift-triage screens migrate onto it. |
| **Add-on platform, third pass** | §27–§30 — revocation composition, door cards, dormant accounts, connection instructions | **High** | Exact values. Newest work. §29 needs a listing endpoint that does not exist; its shape is specified in the section and below. |
| **Cross-cutting patterns** | §31 — read freshness, the acknowledgement ladder, urgency | **Normative** | Not screens. Three rules everything else obeys; build these as shared components and reach for them before inventing a fourth variant. |
| **Motion** | §32 | **High** | Durations and easings are exact. |

The Advanced surface is 10 of the 41 screens. Its *structure* — what is on each
screen, in what order, linked to what — is decided. Its *measurements* are not.
Where the mock and the token table disagree on Advanced screens, **the token table
wins**.

Two further exceptions, marked as such in the board:

- **S10 (Intents)** is parked — designed but not to be built yet.
- **Bulk revoke** is designed in full but shipped disabled, so the copy can be
  reviewed. The button carries its own reason for being disabled.

---

## The three audiences

The same application serves three people, and the navigation is the mechanism.

| View | Who | Destinations |
| --- | --- | --- |
| **Basic** | An operator assigning access | Today, People, Access (Projects / Roles / Apps), Requests |
| **Advanced** | The maintainer changing the machinery | All of Basic, then Bundles, Automation, Review, System appended |
| **Member** | Someone who just wants in | My access, Requests |

**Advanced appends to Basic.** Same items, same order, nothing renamed, nothing
inserted above. Switching views never navigates and never re-sorts — it reveals
in place. A Basic user who switches to Advanced stays on the row they were
reading, now expanded with lineage and ids.

The switch is a **two-state pill**, not a dropdown, sitting top-right beside the
account. Both labels stay legible so the current state is unmistakable. Members
do not see it.

See §01 and §02.

### The add-on platform's nav delta

Six changes, in `ui/src/lib/nav.ts`. See §19.

| Change | Where | Why |
| --- | --- | --- |
| **`Hardware sync` removed** | Advanced › System | It named the LLDAP bridge, which is deleted. A nav row for a subsystem that no longer exists is worse than a missing one — an operator clicks it before they read anything. |
| **One row per registered add-on** | Advanced › System, between `Identity provider` and `Event activity` | Built by `targetNav(targets)` from `GET /api/v1/targets`, which is **deployment configuration**. A row appears because the deployment registered an add-on — never because this operator can see data. |
| **`Withdrawn access` added** | Advanced › Review, after `Unexplained access` | Its own destination, never a tab inside drift. Drift is access that appeared and cannot be explained; this is access somebody decided to take away that is still there. |
| **`TrueNAS` added** | Member nav, third row | Present for **every** member, whatever they can reach. Named after the add-on, not after its function, so a second storage add-on adds a row rather than overloading one. |
| **`Holds` added** | Advanced › Review, after `Withdrawn access` | Every hold in force, whatever its shape. No badge — a hold in force is not work outstanding. |
| **`Review due` added** | Advanced › Review, after `Holds` | Holds whose review date has passed. Amber badge: the date is a deadline and it has gone by. |
| **Unifi Access gets no member row** | — | A door is something you walk through, not a place to visit. It lives under the role that opens it. See §26. |

Two of these must survive a visual pass:

- **The member storage row is ungated.** Gating it on entitlement would make the
  rail move as somebody's roles change, and it answers the wrong question: a
  member without access is asking whether they can get it, and a missing row does
  not answer that.
- **Target rows carry no badge.** A count on them would be data driving structure.

---

## Screen inventory

41 screens. `E` = everyday (Basic), `S` = system (Advanced). Section numbers refer
to `Syndra IA.dc.html`.

### Basic surface

**All high fidelity — exact values.**

| § | Screen | Route / endpoint | Purpose |
| --- | --- | --- | --- |
| 04 | **Today** (E-Today) | `GET /api/v1/governance/summary` | Landing. Every block is something you can finish — no counts you cannot act on, no charts. |
| 05 | **People index** | `/users` | Search. Highest-traffic screen in the product. |
| 05 | **Person access** (E3) | `GET /api/v1/users/{id}/access` | One person's access, grouped by project, granted above automatic, every row carrying its source. |
| 06 | **Role members** (E2) | `/projects/{id}/roles/{key}` | "Who can currently use the laser cutter?" Every row carries its source. |
| 07 | **Projects index** (E1) | `/projects` | A project is a boundary that owns roles. Apps-served is a column, not a value. |
| 07 | **Project detail** (E1) | project → roles | Role descriptions shown in full, never truncated to a tooltip. |
| 07 | **Roles index** (E2 index) | `/roles` | Project column is first and never collapses — the same role key means different things in different projects. |
| 07 | **Apps index** | `/applications` | An app is a thing that receives a token. Not the same as a project. |
| 08 | **Assign bundle** (E4) | `POST /api/v1/users/{id}/bundles` | The one place a bundle is assigned or unassigned. Preview is the point, not a footnote. |
| 08 | **Set expiry** (E5) | — | Presets before the picker, because "end of term" is what people mean. Operator-only. |
| 09 | **Token simulator** (E6) | `GET /api/v1/applications/{id}/simulate` | "My app isn't seeing the roles it expects." The most technical thing in Basic; it earns its place. |
| 10 | **Requests, operator** (E7) | `GET /api/v1/requests` | One request expanded at a time, with a decision panel so nobody has to leave. |
| 10 | **Requests, member** (E7) | `POST /api/v1/requests` | Asks in verbs, not role keys. |
| 10 | **My access** (member) | same route as E3, scoped to self | Access source becomes a sentence rather than a chip. |
| 11 | **Four list states** | every list view | Loading, empty, error, degraded. No exceptions. |
| 12 | **Access map** (S5) | `GET /api/v1/topology` | Pick one node; the map draws its neighbourhood. Never everything at once. |
| 20 | **TrueNAS** (member) | `/storage/[target]` · `GET /me/targets` | Three states — no entitlement, entitled-with-no-account, account present. **All three must stay.** |
| 26 | **My access** — doors and card | `/me` · `GET /me/targets` | Unifi Access folds in here: one read-only card-status line, and doors nested under the role that opens them. |
| 30 | **Connection instructions** | `/storage/[target]` · `GET /me/targets` | Bottom of the member TrueNAS page. Three platforms, copyable strings, reachable resources only. Absent in the other two states. |

### Advanced surface

**Mid-fidelity except the three drift screens in §14–§15** (Unexplained access,
its revoke dialog, and Reconciliation), which are high-fidelity. For the mid ones,
follow the structure and take measurements from the token table.

| § | Screen | Route / endpoint | Fidelity | Purpose |
| --- | --- | --- | --- | --- |
| 14 | **Unexplained access** (S6) | `GET /api/v1/governance/drift` | High | Highest stakes in the product. Access that exists upstream which Syndra cannot explain. |
| 14 | **Revoke dialog** | `DELETE …` | High | The only place a solid red fill appears. |
| 15 | **Reconciliation** | (relocated from a retired route) | High | Drift in the other direction. Agreeing rows stay visible at reduced contrast. |
| 15 | **Expiring access** (S7) | `/review/expiring-access` | Mid | Its own destination, not an audit tab. Time-boxed work, one action. |
| 16 | **Bundles** (S1) | `GET/POST /api/v1/bundles` | Mid | Editing what a bundle *contains* changes access for everyone holding it. |
| 16 | **Confirmation policy** (S2b) | `GET/PUT /api/v1/config/confirmation` | Mid | Which rules queue for review and which fire unattended. |
| 17 | **Automatic rules** (S2) | `/rules/mapping` | Mid | A rule fires → a write queues → the write is recorded. Read left to right. |
| 17 | **Pending changes** (S3) | `GET /api/v1/propagations` | Mid | The queue rules fill. |
| 17 | **Change history** (S4) | `GET /api/v1/propagations/cascades` | Mid | The record of what they did. Threaded by cascade id. |
| 18 | **Audit** (S8) | `GET /api/v1/audit` | Mid | Every line names a human or a named machine. |
| 18 | **Identity provider** (S9) | `/zitadel/{health,projects,users,grants}` | Mid | Provider health and raw upstream state. |
| 18 | **Intents** (S10) | `GET /api/v1/intents` | Parked | **Parked and now without a host.** Its old nav row named the deleted LLDAP bridge and was removed; the shadow-credential routes went with it. Do not build a page — where it lives is undecided. |
| 18 | **Event activity** (S11) | `GET /api/v1/webhook/events` + `/onboarding/triggers` | Mid | Merged into one time-ordered stream, each row drillable to its raw payload. |
| 21 | **Target overview** | `/system/targets/[target]` · `GET /api/v1/targets` | High | Four panels: health, unmanaged inventory, capabilities, maintenance. One row per registered add-on. |
| 22 | **Withdrawn access** | `/governance/unconfirmed-revocations` | High | Two populations rendered apart: terminal (`spent`) and retrying (`queued`). |
| 23 | **Entitlement plan-then-apply** | `POST /targets/{t}/entitlements/rehearse` · `.../apply` | High | Two steps in place. Apply carries `plan_id`. Provisional, stale-plan and post-apply states all drawn. |
| 24 | **Mapping management** | `POST /targets/mappings/{id}/rehearse-edit` · `rehearse-delete` · `PATCH`/`DELETE` | High | Version history with rollback; blast-radius acknowledgement. |
| 25 | **Holds** | `/governance/holds` · `POST /allowances` | High | Every hold in force. Authored from the access row it holds. |
| 25 | **Review due** | `GET /governance/allowances/review-due` | High | Its own queue. A hold stays in force until decided. |
| 27 | **Take away access on a target** | `POST /targets/{t}/users/{id}/revoke-access` | High | Dialog reached from three places. Verbatim session copy, type-to-confirm, solid red. |
| 28 | **Door cards** | person page + Unifi Access page | High | Operator enrols, replaces, marks lost. Every card issued listed on the target. |
| 29 | **Dormant accounts** | **listing endpoint not built** — shape specified in §29 | High | Grouped by cause. The one bulk action in the product. |

### Not in this table

- **Login** — fully specified separately in `login/LOGIN.md`, with a working
  standalone reference in `login/login-reference.html`. It is the only
  unauthenticated route.
- **Settings** — a nav destination inside Automation; not separately drawn.

---

## The add-on platform

### Nothing reaches a target directly

Every change Syndra makes to an add-on is **queued**, and dispatched when an
operator resumes the drain. This is not an implementation detail to be smoothed
over — it is the dominant fact of these three screens.

The consequence a UI must not hide: **queued is not succeeded.** Every apply
response carries a `summary` whose `succeeded` is always zero, present precisely
so a client cannot default it.

### Member › TrueNAS — three states, and the middle one is ordinary

`/storage/[target]`, from `ui/src/components/storage/MyStorage.tsx`. See §20.

The nav row is named **after the add-on**, not after its function, so a second
storage add-on adds a row instead of overloading one. Not every add-on earns a
member destination — see "The member side with two add-ons" below.

A two-state "access / no access" page lies to the member in the middle. Because
add-on changes wait for a person, "recorded, not created yet" is the **ordinary
experience of every new member** until an operator resumes the drain — not an edge
case, and not a spinner.

| State | Renders | Must NOT render |
| --- | --- | --- |
| **No entitlement** — no role of theirs is mapped here | An explanation that access comes with a role, and the way to ask | No credential form, no account name, no connection instructions |
| **Entitled, no account yet** | "Recorded, not created yet, nothing needed from you", with the age of the record | **No credential form.** Setting one dispatches at an account that does not exist and tells them their password was set |
| **Account present** | Account name, what they can reach, the credential form | — |

Design decisions in §20 worth keeping:

- **One three-node spine opens all three states**, so they read as a progression
  rather than three unrelated panels. In the no-entitlement state the spine is
  **inert** — it explains the mechanism without promising an outcome.
- **The pending state has no spinner and nothing that loops.** Per the motion
  rules a loop means "still happening"; this wait is on a person. It states an age
  instead ("recorded 2 days ago · waits on a person, not a timer"), so a member
  refreshing four times in a day sees an honest number rather than perpetual
  imminence.
- **The account name gets connection-string treatment** — mono, on its own ground,
  copyable — because members paste it into a file manager.
- **The scope sentence is body copy at 15px, not a hint:** *"This password is used
  only for TrueNAS. It is not your Syndra sign-in, and not your Google account."*
  Members reasonably assume one password. This is load-bearing.
- **Withheld access renders its reason.** A member who sees access they expect and
  do not have, with no explanation, asks an operator; one who can read the reason
  does not.
- **Unreachable target replaces the form rather than disabling it.** A credential
  set against an add-on that never answered must never be reported as done, so the
  field does not exist to be submitted.

### Advanced › System › ‹target› — four questions in order

`/system/targets/[target]`, from `ui/src/components/targets/TargetOverview.tsx`.
See §21. Health, then whose accounts are on it, then what it can do, then what
state somebody put it in — in that order, because each answer changes what the
next one means.

**Health — five states a single "status" chip would flatten.** Each sends an
operator to a different machine.

| State | Tone | Means |
| --- | --- | --- |
| `unreachable` | red | The add-on did not answer. Look at the add-on. |
| `circuit_open` | red | **Syndra is refusing its own calls** after repeated failures. *Not* the target being down — an operator who reads it as that looks at the wrong machine. |
| `draining` / `read_only` | **accent** | A maintenance state somebody set, with their reason. **Must not read as a fault** — hence accent, not amber. |
| backlogged (`in_flight > 0` while draining) | amber | Calls issued before the drain have not settled. This is what an operator waits on before pulling a credential out from under a call. |
| stale mirror (`state_read_at`) | amber | Reads answered from a copy, labelled with its age. Data with an age, not an error. |

Accent for *draining* is deliberate and consistent: a still-draining revocation in
§22 is accent for the same reason — somebody chose it.

**Accounts Syndra did not create — the unmanaged inventory.** A real NAS holds
`root`, service accounts and whatever an admin made by hand. **These are never
drift and must never be rendered as drift** — classifying them as untraced access
would bury the triage queue on the first sweep after deployment, and trust in a
triage queue is set the day it first fills.

Adoption is the one action and it is heavy: the wrong choice hands a member
somebody else's home directory, shares and group memberships, and the next
convergence makes that look intended. **There is no undo.** The affordance is
**disabled while the read is stale** — you cannot adopt from a list that may have
moved — and the disabled state states that reason in the row.

Copy that must survive verbatim: *"Nothing on the account changes now; the next
convergence applies their entitlements to it."* The natural reading of "adopted"
is that something was applied, and nothing was.

**Capabilities — rendered from the manifest**, never from a list in the frontend.
An operation removed from an add-on's manifest disappears here with no frontend
change, and a test asserts it. An operation the target cannot perform is shown
**disabled with its reason** rather than omitted — omitted, an operator wonders
whether the feature exists at all. `secret_params` names parameters whose values
are never logged, stored or echoed; a form built from the manifest must mark them,
and there is nowhere in the payload for a value.

**Maintenance — three buttons and a mandatory reason.** `draining` and `read_only`
differ in one way that matters during a credential rotation: draining lets calls
already issued settle. **The explanation above the buttons is the whole of how an
operator picks the right one**, so it carries the weight, not the labels. The
reason is mandatory because the person who reads it is not the person who set it.

### Advanced › Review › Withdrawn access — two populations, never one count

`/governance/unconfirmed-revocations`, from
`ui/src/components/review/WithdrawnAccess.tsx`. See §22.

Access somebody decided to take away that is still there. Merged into one count, a
healthy queue of five-minute-old rows hides a revocation that failed permanently
three days ago — so the two are rendered apart, under headings that say which is
which.

| Population | Tone | Reading |
| --- | --- | --- |
| **Not going to happen** (`spent`) | danger | Terminal. Nothing will dispatch it again; waiting produces nothing and somebody has to act. |
| **Still draining** (`queued`) | accent | Being retried. The only content of the signal is how long it has waited. |

Every row carries the reason it failed and its cascade id — a terminal row an
operator can see and not act on is the whole difference between a finding and a
mystery.

**The badge escalates on `revocations_escalated`, not on the count.** Any spent row
escalates immediately; a queued one escalates after 24 hours. A count cannot carry
that difference, and an operator reading "3" cannot tell.

### Plan, then apply — the pattern (§23)

`POST /targets/{t}/entitlements/rehearse` → `.../apply`.

**This is now the canonical plan-then-apply pattern.** Bulk grants and drift triage
move onto it rather than the other way round, so there is one shape for "show me
what this does before it does it" everywhere in the product. Confirmed with the
product owner — migrate the older screens.

Two steps in place, not a wizard: an operator who has read the plan should not have
to walk forward to reach the button.

- **Apply carries the `plan_id`**, never the original submission, so what gets
  queued is what was on screen.
- **Rows needing nothing are counted, not hidden.** "This changes less than you
  think" is the most useful thing a plan can say.
- **The button names the count** — "Queue these 15 changes", not "Apply".
- **Provisional plans stay applicable**, labelled with the age of the state they
  were computed against. This differs deliberately from the stale unmanaged-
  inventory read in §21, where adoption is *blocked*: adopting binds an identity
  irreversibly, while applying only joins a queue an operator can inspect. Do not
  unify the two behaviours.
- **A stale plan names who moved** — the two subject rows are the difference
  between re-planning with confidence and re-planning blind. There is no "apply
  anyway"; the plan id is gone.
- **After apply, the word *done* never appears and neither does a tick.**
  `summary.succeeded` is always zero. The response shows `queued`, and the cascade
  id threads into §17 and §18.

### Mapping management (§24)

A mapping ties a role to what it reaches on an add-on, so editing one moves access
for everyone holding that role. **Edit, delete and rollback all rehearse first.**

- **Every version carries who changed it and why.** A rollback target with no
  reason attached is a guess.
- The current version is **tinted rather than badged**, so the eye finds it without
  reading.
- **The blast-radius number sits inside the sentence** the operator ticks: *"I
  understand this changes 34 people."* Ticking it means reading it.
- **Not type-to-confirm.** A mapping edit is routine, and copying digits trains an
  operator not to look. The ceremony is sized to the task; the backend enforces the
  acknowledgement regardless. (Type-to-confirm stays reserved for the revoke dialog
  in §14.)

### Holds — one object, three faces (§25)

`POST /allowances`, `GET /governance/allowances/review-due`.

A **hold** takes away access a role still grants, without touching the role. It is
**the same object the member reads as "withheld"** in §20 — authored on the row it
holds, listed under Review, and chased once its review date passes.

**The UI word is "hold", not "allowance".** The API name reads as permission
granted when the object withholds; members already see *withheld*, and *hold* is
the same root as a noun, so the interface teaches one vocabulary instead of two.
Keep the API name in code and out of the interface.

Authoring:

- Framed as **"how it ends"**, not "expiry / review date" — the operator is
  choosing what happens if they never come back, which is the actual decision.
- **Two bounded forms and no third.** Lifts on its own by a date, *or* stays until
  somebody decides with a mandatory review date. A hold with neither is refused by
  the backend, and the UI says so rather than offering it.
- The reason field is labelled **"shown to her"**, because it is — it renders on the
  member's page (§20).
- The copy states that the role is untouched: holding a share does not take the
  role that grants it.

**Review due is its own queue**, beside Expiring access, not a row inside it.
Inaction means the opposite thing in each: an expiring grant lapses if ignored, a
hold **stays in force**. One list would sit "do nothing and access ends" next to
"do nothing and access stays blocked". Extending demands a new date, so a review
can be deferred but not dropped.

### The member side with two add-ons (§26)

**Not every add-on earns a member destination, and there is deliberately no rule.**
The judgement is made per add-on as each one lands.

- **TrueNAS earns one** — there is something to look at and a password to set.
- **Unifi Access does not.** A door is something you walk through, so it lives
  under the role that opens it, inside My access.

How doors read on My access:

- **Nested under the granting role**, so "why can I open this?" is answered by
  where the door sits on the page rather than by a source chip.
- Each door carries its **schedule** ("weekdays, 08:00–20:00"), because a door you
  can only open at some hours is not the same as one you can always open.
- A withheld door shows the **Withheld** pill and its reason inline — same
  treatment as a withheld share in §20.
- **The card status line has no button.** An operator enrols the card, so a member
  cannot act on it; an affordance that only produces a refusal is worse than none.
  It states who enrolled it and when.

### Taking access away on a target (§27) — 6.17

`POST /targets/{t}/users/{id}/revoke-access`. **The backend fixes this copy and it
must be shown verbatim:**

> Sessions already established end when they next reconnect — this target has no
> way to close one.

That sentence is the reason the endpoint is shaped the way it is. A UI implying the
access is gone the moment the button is pressed is the exact failure it was built
to prevent.

- **Dressed as a revocation**, because it is one: type-to-confirm and the one solid
  red fill in the product, matching §14. Muscle memory must not depend on which
  revocation an operator is doing.
- **The session sentence is amber, not red.** It is a broken assumption — *revoked*
  implies immediate and here it is not — rather than the danger itself. Red is the
  confirming button.
- **The button says "Queue the revocation"**, because that is all it does.
- **The label is "Take away", not "Revoke".** It says what happens to the person,
  and *revoke* already carries §14's meaning of undoing a grant Syndra itself made.
- Reached from three places, one dialog: the person's access row beside **Hold**
  (pause it or end it — the two answers to one question), the account row on the
  target page, and the **empty state** of Withdrawn access, which is the only place
  a new withdrawal is offered.

**Confirm before building:** the reason field is marked required to match holds and
§14. Verify the endpoint wants it.

### Door cards (§28)

A card is enrolled by an operator standing next to the person, so the panel lives
on the person's page; every card issued is listed on the Unifi Access page.
Members only read status (§26).

- **The reader is the primary input, typing the fallback** — the operator is at a
  door with the card in their hand.
- The no-card state **names the roles that are waiting on it**: the entitlements
  already exist, and nothing opens until there is plastic.
- **Lost cards stay in the list, struck through.** A card removed from the list is
  a card nobody recognises when it turns up in a drawer.
- The target list surfaces **cards enrolled but never assigned** — the only place
  in the product a card with no person attached is visible.

**The one place a screen dispatches, and it needs a decision.** Everywhere else in
Syndra, queued-not-done is correct and honest. Here it means a lost card keeps
opening doors until the drain runs. §28 therefore offers *"Mark lost and resume the
drain now"* as a single action, and states the exposure plainly if it is not taken.
If the backend cannot dispatch from that call, drop the action — but then the copy
has to say **how long** the card stays live, not merely that it does.

### Dormant accounts (§29) — 9.11–9.12, blocked on an endpoint

Accounts Syndra created whose reason for existing has gone. **Anything an active
role still grants never appears here.**

**This is the only bulk action in the product, and the exception is principled.**
§14 deliberately has no bulk revoke because every revoke removes real access from a
real person. No active role grants any of these accounts, so removing them takes
access from nobody: a bulk revoke would be guessing at forty people, this is
tidying forty things that already grant nothing. Do not read it as permission to
add bulk actions elsewhere.

- **Grouped by cause, because each cause has a different remedy.** A former
  member's account is housekeeping; *still a member, role deleted* is somebody who
  may be quietly locked out. Same dormancy, opposite action.
- **Only the safe group is selectable.** Rows whose subject is still a member get a
  dashed, unselectable box and a line saying why — removing that account locks the
  person out rather than tidying up. The bulk count never includes them, so the one
  bulk action in the product can only ever touch accounts that grant nobody
  anything. Give the person a role that grants it, or take it away deliberately
  in §27.
- The action runs through the **§23 plan-then-apply pattern** unchanged.
- **The acknowledgement names data, not people:** *"I understand 41.2 GB of their
  files goes with the accounts."* That is what is actually irreversible.

Required listing endpoint — dormancy is a backend judgement, not a frontend filter:

```
GET /api/v1/targets/{target}/accounts/dormant
→ {
    state_read_at, truncated,
    accounts: [{
      account, subject_id | null, display_name,
      reason: "membership_ended" | "role_deleted"
            | "mapping_removed" | "never_assigned",
      subject_still_member: bool,
      last_seen_at, bytes_held
    }]
  }
```

`state_read_at` and `truncated` for the same reasons as everywhere else.
`bytes_held` is what makes the acknowledgement possible. `subject_still_member`
is what separates housekeeping from a lockout.

### Connection instructions (§30) — 10.8

`GET /me/targets`. Sits at the bottom of the member TrueNAS page, below the
credential form.

- **Three platforms, one share, every string copyable.** This is the only screen
  where a member retypes something into another application.
- **Only resources current entitlements reach**, generated from the endpoint and
  never from a template. A path here that does not work teaches a member to
  distrust the whole page — and the next thing they distrust is the part that was
  right.
- **A withheld resource is named as excluded**, not silently dropped. A member who
  knows their folder is absent on purpose does not spend twenty minutes hunting a
  typo.
- **Absent in the other two states**, same rule as the credential form in §20.
- The host name comes from the add-on's registration, like its nav row (§19).
  Moving the NAS must not mean editing a component.
- **No instructions for doors.** Unifi Access has nothing to connect to, which is
  the whole reason it earns no member destination.

### Contracts these screens must not quietly change

Rendered as cards at the end of §22.

1. **Queued is not succeeded.** `summary.succeeded` is always zero. Nothing an
   operator does on these screens reaches a target directly.
2. **Provisional is not applied.** A plan computed while the target was unreachable
   carries `provisional: true` and `state_read_at`. It must be labelled *with that
   age* — "computed against last-known state" with no number is a label nobody can
   act on.
3. **Truncated is not empty.** A capped read reports what it saw. Absence proves
   nothing.
4. **A confirmation is a backend refusal**, not a dialog. `account.adopt`,
   `account.purge` and the revocation composition are refused without one — a
   dialog only the frontend enforces is a suggestion.

### Not built, and deliberately open

These have backend endpoints and no screen. Each is a real gap.

**Everything from the original list is now designed.** 9.3–9.6 in §23, 9.7–9.8 in
§24, 9.22 and 9.25 in §25, 6.17 in §27, card enrolment in §28, 9.11–9.12 in §29,
10.8 in §30. Struck rows below are kept for traceability, not as work.

**One backend item remains:** the dormant listing endpoint does not exist yet. Its
shape is approved as specified in §29 — build it, then the screen works as drawn.

**One question is still with the product owner:** whether the backend can dispatch
from the lost-card call (§28). Until that is answered, build the button as drawn;
if it cannot, swap in the fallback copy in §31 §C and supply the real exposure
window.

**Sequencing:** TrueNAS first. Unifi Access is designed (§26, §28) but is the later
phase — build the door-card and door-listing work after the storage path is done.

| Task | Needs | Endpoint |
| --- | --- | --- |
| **9.11–9.12** Dormant-account housekeeping | **Designed in §29.** The response shape below is **approved — build it as specified.** | `GET /api/v1/targets/{target}/accounts/dormant` — to build |
| ~~**Card enrolment**~~ | **Designed in §28.** | — |
| ~~**10.8** Connection instructions~~ | **Designed in §30.** | `GET /me/targets` |
| ~~**6.17** Revocation composition~~ | **Designed in §27.** Its copy is fixed by the backend and must be shown verbatim: *"Sessions already established end when they next reconnect — this target has no way to close one."* This target cannot end a session, and a UI that implies otherwise is the failure that endpoint's whole design exists to prevent. | `POST /targets/{t}/users/{id}/revoke-access` |

## Three patterns, used everywhere (§31)

These are not screens. Three questions recurred on nearly every add-on screen and
were answered separately as they arose; §31 states them once. **Build these as
shared components** — everything in §19–§30 is retrofitted to them, and anything
new should reach for them rather than invent a fourth variant.

### A. Read freshness

Any screen showing state read from a target carries one freshness strip. **Always
an age, never a word alone** — "recently" is not something an operator can act on.

| State | Tone | Reads |
| --- | --- | --- |
| `live` | lime dot | Read just now |
| `ageing` | neutral | Read 4 minutes ago |
| `stale` | amber | Read 11 minutes ago — too old to act on, with a Refresh |
| `provisional` | amber | Could not read it — this is the last state seen, 14 minutes ago |
| `truncated` | neutral | Showing 200 of more than 200 — this is not the whole list |

`truncated` is a separate flag and can coexist with any of the other four.

**When a stale read blocks the button — the rule is what the action does, not how
old the read is.**

- **Blocked** (`adopt`, `purge`): the action binds or destroys something on the
  strength of that list. Acting on a list that may have moved is how a member gets
  somebody else's home directory.
- **Allowed, labelled** (`plan`, `apply`, `hold`): the action only joins a queue an
  operator can still inspect and cancel. Blocking it would stop work for a risk the
  drain already absorbs.

This is why §21 disables adoption at eleven minutes while §23 applies a
fourteen-minute-old plan. **Do not unify them.**

### B. The acknowledgement ladder

Three rungs. **The rung is set by what cannot be undone, never by how important it
feels.**

| Rung | Gesture | For | Used by |
| --- | --- | --- | --- |
| 1 | **Read it** — the plan states the numbers, the button names them | Work that queues, can be inspected, and can be undone by doing the opposite | Entitlement apply (§23); resume/drain/read-only (§21); lifting a hold (§25) |
| 2 | **Tick the number** — the quantity inside the sentence | Routine work whose reach is larger than it looks | Mapping edit and rollback (§24); dormant sweep (§29); removing a role from a bundle (§16) |
| 3 | **Type the name**, which is what unlocks the solid red fill | Taking real access from a named person, or anything with no undo | Revoke (§14); take away on a target (§27); remove a bundle assignment (§16); purge an account (§29) |

A daily action on rung 3 is a ceremony people stop reading — and once they stop
reading it, it protects nothing on the day it matters. Rung 2 exists precisely so
routine-but-wide work is not pushed up into a ritual. **When in doubt, go down a
rung and make the copy better.**

The backend enforces its own confirmation regardless; these rungs are what a person
meets, not what protects the data.

### C. Urgency — when a queued action may carry the drain

**Default: it waits.** Everything queues and dispatches when an operator resumes
the drain, and saying so plainly is most of what these screens do.

**Exception: only when the gap between queuing and dispatch is itself the danger.**
Then the action *carries* the drain — one button, not a reminder to go and resume
it elsewhere. Today this is earned exactly once, by a lost card (§28). A safety-gated revocation
was considered and rejected: standing somebody down is urgent for the person, but
the drain interval is not what makes it dangerous.

**Never for convenience.** If this appears on ordinary work it becomes the button
everybody presses, the drain stops being a batch, and the one guarantee these
screens make — that nothing reaches a target until somebody says so — quietly stops
being true.

**If the backend cannot dispatch from that call:** drop the button and state the
exposure instead. Never leave "it is queued" as the whole answer on an urgent
action — an operator needs to know whether to go and stand by the door.

> Marked lost. The card keeps opening doors until the next drain — usually within
> the hour. To stop it now, disable it on Unifi Access directly.

---

## The causal chain

Three screens tell one story and must link to each other, left to right:

```
Automatic rules (S2)  →  Pending changes (S3)  →  Change history (S4)
     a rule fires          a write queues          the write is recorded
```

Every write carries a **cascade id** (e.g. `c_8841`). Two writes caused by the
same rule firing share one cascade id. That id is the join key across S2, S3, S4,
S8 and S11, and it must be a link everywhere it appears. This is the single most
important piece of data plumbing in the product — it is what makes "why does this
person have this access?" answerable.

See §17 and §18.

---

## Signature component: Access source

Every row that grants access states **where the access came from**. Three kinds,
fixed vocabulary, **always in this order**:

1. **Direct** — someone granted it to this person
2. **Via bundle** — it came with a bundle they hold
3. **Automatic** — a rule produced it

The dot carries the meaning before the word is read. The order is fixed so the
eye learns one sequence and reads it everywhere.

Expanding a source row answers *who, when, and until when* — and hosts the only
removal that belongs to that specific source. A grant that came from a rule
offers no removal; it offers the two real ways forward (change the rule, or
adopt the grant).

Implemented in `design/Source.dc.html`. See §03.

---

## Interactions & behaviour

### Motion system

Six tokens cover the entire product. §32 contains live, hoverable specimens of
every one.

| Token | Duration | Easing | Used for |
| --- | --- | --- | --- |
| `tint` | 120ms | `cubic-bezier(.4,0,.6,1)` | Colour and opacity only — row hover, link hover, nav hover. **No transform.** |
| `press` | 180ms | `cubic-bezier(.22,.61,.36,1)` | Buttons, tabs, toggles, focus rings. Anything answering a click. |
| `settle` | 260ms | `cubic-bezier(.16,1,.3,1)` | Disclosures, drawers, dialogs — anything changing page height. |
| `arrive` | 420ms | `cubic-bezier(.16,1,.3,1)` | First paint of a screen or list. Staggered 40ms per row, **capped at six rows**. |
| `flash` | 900ms | `ease-out`, once | A value changed while you were on the page. Never on load. |
| `breathe` | 2.4s | `ease-in-out`, alternate, ∞ | Degraded and loading only. |

Three rules govern all of it:

1. **Direction carries meaning.** Things the system produced *rise*. Things you
   opened *scale from where you clicked*. Nothing slides sideways. Nothing moves
   more than 10px.
2. **Only one thing loops.** A repeating animation means *this is still
   happening* — degraded mode breathes, skeletons shimmer. Nothing healthy and
   no decoration ever loops.
3. **Never animate a number.** Counts, dates and ids are read, not watched. What
   animates is the *row around them*, once, when the value arrived while the
   operator was looking elsewhere.

Specific behaviours worth calling out:

- **Row hover is background only** — no lift, no shadow, no border change. A
  forty-row table where each row lifts is a table that flickers as you read it.
- **Button press is a 3% scale-down**, not a translate, so the button stays under
  the finger. Destructive buttons behave identically — muscle memory must not
  depend on what a button does.
- **Tab underlines grow from the left**, they do not travel between tabs. A
  travelling underline implies the tabs are a sequence; they are not.
- **Dialogs**: scrim fades (260ms), card rises 10px from 97% scale, 40ms behind
  the scrim. It never zooms from screen centre — the destination is where the
  dialog will live.
- **Disclosures animate height and opacity together**, so a payload never appears
  before it has room. Rows below are pushed, never overlapped.

### What deliberately does not move

Route transitions (the sidebar and header persist, so crossfading content implies
a bigger change than happened). Table sorting and filtering (animated reordering
makes it impossible to follow one row with the eye). Counts and timestamps.
Anything inside a dialog after it has opened. And the healthy state, which earns
its calm by being the only thing on screen holding perfectly still.

### Reduced motion

`prefers-reduced-motion: reduce` collapses every duration to `.001ms` and stops
every loop. **No state is conveyed by movement alone**, so nothing is lost — the
degraded row still turns amber, the row still tints, the dialog still appears.

### The four list states

Every list view implements all four. No exceptions. See §11.

| State | Treatment |
| --- | --- |
| **Loading** | Row-shaped skeletons at the *real row height*, so nothing jumps when data lands. Ids resolve asynchronously. |
| **Empty** | Names what is absent and offers the next move. Never a shrug illustration, never "you're all caught up!" |
| **Error** | Names the failed thing, confirms nothing changed, carries a request id the operator can paste into a message. |
| **Degraded** | The backend has fallen back to demo data. **Every number on the page is invented and the page says so.** The one moment a semantic colour fills a whole field — amber, because the data is wrong rather than dangerous. |

### Disabled controls

A disabled action **always states its reason in the row**, not only on hover.
Hover does not exist on touch, and a disabled button with no explanation is a
dead end. See §04.

---

## State management

### Global

```ts
type View = 'basic' | 'advanced' | 'member'
```

Persisted per user. Changing it re-renders in place; it never navigates, never
re-sorts, never scrolls.

The member view is not a preference — it is derived from the account's role. An
operator can switch between Basic and Advanced; a member sees only Member.

### Per list view

```ts
type ListState = 'loading' | 'ready' | 'empty' | 'error' | 'degraded'
```

`degraded` is a **server-declared** state, not an inference from an empty
response. The API tells the client it is serving demo data; the client must not
guess.

### Data fetching

Endpoints are named per screen in the inventory above and in each figure's
caption. Notes:

- **Cache age is user-visible.** "cache 4m old" appears in the Today header and
  the provider health row. Whatever caching layer you use must expose its age.
- **Cascade ids join across five screens.** Fetching a cascade must be possible
  from any of S2, S3, S4, S8, S11.
- **`GET /api/v1/applications/{id}/simulate`** is a read-only dry run. It must
  never write.

---

## Design tokens
<a id="design-tokens"></a>

### Colour — five semantic roles, no decoration

| Token | Dark (home) | Light | Meaning |
| --- | --- | --- | --- |
| **Accent / Violet 500** | `#7f5af0` | fill `#7f5af0`, text `#5b3fd6` | The primary action and the current selection. One per screen region. **Never means "good" or "safe".** |
| **Violet 400** | `#9b7bff` | — | Accent text on dark, where 500 is too low-contrast. Links. |
| **Violet 300** | `#c9b6ff` | — | Active nav label, focus outline. |
| **Healthy / Lime** | `#a3e635` | `#4d7c0f` | *Nothing needed here.* Provider answered, queue empty, cache fresh. **Never a button, never a fill behind text.** A dot, a word, or a hairline. |
| **Deadline / Amber** | `#f5a524` | — | Expiry, rotation due, a broken assumption. The degraded field. |
| **Destructive / Red** | `#ff5c4d` | — | Destructive actions only. **Solid fill only on a confirming button inside a dialog, and only once its rung-3 gesture is satisfied** (§31). A destructive action sitting in a row is outlined — `1px solid rgba(255,92,77,.4)` with `#ff8d82` text — never filled. A destructive action that is **blocked** (no endpoint, missing prerequisite) is *dashed* outlined and states its reason as text, never a tooltip — the same dashed language §29 uses for an unselectable row. Three states, visibly distinct: blocked (dashed), awaiting its rung-3 gesture (`rgba(255,92,77,.32)` fill), confirmed (`#ff5c4d` fill). |

### Surfaces

| Token | Value | Use |
| --- | --- | --- |
| Page | `#080906` | App background |
| Login page | `#0a0b08` | Slightly lifted; login only |
| Rail | `#101210` | Sidebar |
| Card | `#141612` | Panels, cards, table containers |
| Card (nested) | `#0b0c0a` | Specimen frames, code blocks |
| Hairline | `rgba(255,255,255,.07)` | Borders |
| Divider | `rgba(255,255,255,.05)` | Row separators |
| Row hover | `rgba(255,255,255,.035)` | Table and nav hover |

### Text

| Token | Value | Use |
| --- | --- | --- |
| Primary | `#f3f5ef` | Body, headings |
| On accent | `#f7f4ff` | Label on a violet fill |
| 82% | `rgba(243,245,239,.82)` | Table cell content |
| 62% | `rgba(243,245,239,.62)` | Section intros |
| 50% | `rgba(243,245,239,.5)` | Captions |
| 42% | `rgba(243,245,239,.42)` | Eyebrows, labels, ids |
| 32% | `rgba(243,245,239,.32)` | Footnotes |

### Typography

Two families. Google Fonts, or self-host if the codebase already self-hosts.

| Role | Family | Size / weight | Tracking |
| --- | --- | --- | --- |
| Page title | Bricolage Grotesque 600 | 48 / 1.02 | −.02em |
| Section title | Bricolage Grotesque 600 | 34 / 1.05 | −.02em |
| Card title | Bricolage Grotesque 600 | 19–20 | 0 |
| Wordmark | Bricolage Grotesque 600 | 18 (rail), 46–62 (login/banner) | −.01 to −.035em |
| Eyebrow | Figtree 600 | 12.5, uppercase | .12em |
| Body | Figtree 400 | 17 / 1.55 | 0 |
| Table cell | Figtree 400 | 14.5 | 0 |
| Caption | Figtree 400 | 13.5 / 1.5 | 0 |
| Badge | Figtree 600 | 11.5 | 0 |
| Ids, endpoints, timestamps | JetBrains Mono 400 | 12.5–13 | 0 |

**Ids, endpoints, counts in monospace; everything else in Figtree.** Monospace is
a signal that a value is a machine identifier the operator may need to copy.

### Geometry

| Token | Value |
| --- | --- |
| Card radius | 20–22px |
| Inner card radius | 16px |
| Row / nav radius | 12–13px |
| Pill / badge radius | 999px |
| Button radius | 999px (pills), 14px (login) |
| Canvas width | 1600px fixed (see note below) |
| Sidebar width | 252px |
| Section gap | 120px |
| Figure gap | 38px |

### Shadows

```
card:         0 20px 44px -28px rgba(0,0,0,.9)
dialog:       0 30px 70px -30px rgba(0,0,0,.95)
button rest:  0 10px 26px -14px rgba(127,90,240,.85)
button hover: 0 16px 34px -14px rgba(127,90,240,.95)
button press: 0 6px 16px -12px rgba(127,90,240,.7)
```

---

## The mark

A **contained orb**: a faint base ring under a brighter arc, with a lit dot at the
centre. It is a miniature of the login's arch-and-orb.

- Base ring `rgba(155,123,255,.22)`, 1.5px
- Lit arc `rgba(206,188,255,.92)` at 50–62% opacity, falling off symmetrically
  from the apex
- Dot `#9b7bff` at 35% of the box, with `box-shadow: 0 0 14px 2px rgba(155,123,255,.6)`
- Rail size 22px; favicon 16px

On light grounds the values darken: base `rgba(91,63,214,.18)`, arc
`rgba(76,48,196,.85)`, dot `#5b3fd6`. The near-white arc is invisible on paper.

Two implementation notes, both learned the hard way:

- The ring's radial fade mask must keep its opaque stop at **88% or beyond**. At
  62% on a 22px box the mask erases the 1.5px border entirely, because the border
  occupies 93.5–100% of the mask radius.
- For **rasterised output** (favicons, the banner), draw rings and arches as
  *filled* SVG bands, not gradient strokes. Gradient strokes flatten to the first
  stop's colour when rasterised and the fade disappears. CSS `mask-image` does not
  rasterise at all.

---

## Voice

Syndra never says "I". It is a tool in a workshop, not a service — it does not
greet you and has no opinions about your day. Warmth comes from **admitting what
the software does not know** and **naming the consequence** of an action.

| Situation | Not this | This |
| --- | --- | --- |
| Role with no members | "Nobody holds this role yet." | "Nobody holds this role yet. Nothing is checking it, and nothing will until someone does." |
| Empty triage queue | "No unexplained access." | "Everything upstream has an explanation. Checked 4 minutes ago — this can change without anyone doing anything." |
| Revoke confirmation | "This action cannot be undone." | "She loses badge entry to the laser bay at the next cache compile. If an integration created this grant, it will be back tomorrow." |
| Degraded mode | "Running on demo data." | "Syndra can't reach the provider, so every number on this page is invented. Don't act on it." |
| Unbuilt feature | "Not connected yet." | "Not connected yet. This is a plan, not a feature — nothing here is wired to a door." |
| Error | "Something went wrong." | "Couldn't load role members. Nothing was changed. Try again, or send someone this: `req_9c14e`." |

Consequences are stated in **physical terms**. "She loses badge entry to the laser
bay", not "grant deleted".

Naming rules:

- Full name: **Makerspace Syndra**
- In-app, everywhere: **Syndra**
- Never: `SYNDRA`, `Syndra™`, "the Syndra platform", or any first-person voice

---

## Design decisions that are deliberate

Do not "improve" these without asking. Each was argued for.

1. **No bulk revoke.** Every revoke removes real access from a real machine, so
   each one is read on its own. The dialog is designed and reachable so its copy
   can be reviewed, but the button ships disabled and says why.
2. **Unexplained access shows both outcomes on the row.** Revoke *and* adopt,
   ranked by risk, so the operator is not forced to guess which is safer.
3. **Reconciliation names drift directions differently.** Access upstream that
   Syndra did not make is not the same problem as access Syndra made that is
   missing upstream. They get different words.
4. **Agreeing rows stay visible at reduced contrast** rather than being filtered
   out. "Nothing wrong here" is information.
5. **The access map draws one neighbourhood, never the whole graph.** The old
   graph was unreadable because it drew everything at once.
6. **Expiring access is its own destination, not an audit tab.** Audit is a record
   you consult; this is time-boxed work you do.
7. **Only the error row is tinted in the event log.** A log where half the lines
   are coloured is a log nobody reads.
8. **Role descriptions are shown, never truncated to a tooltip.** "Can cut
   unsupervised" versus "may enter and watch" is the whole difference.
9. **The project column in the roles index never collapses.** The same role key in
   two projects is two different things.
10. **Presets before the date picker.** "End of term" is what people actually
    mean.

---

## Known constraint: canvas width

The design board is authored at a **fixed 1600px** width because flex children
resolved at `max-content` when it was fluid, causing wrapping bugs in the
specimen tables. This is a property of the *design document*, not of the
application. The real app should be responsive; the board simply is not, and
should not be read as prescribing a fixed-width app.

The login screen has one real minimum: `min-height: max(100vh, 800px)`. Below
~610px of viewport height the sign-in button's overhang collides with the Syn
line. See `login/LOGIN.md`.

---

## Accessibility

- Contrast: `#f7f4ff` on `#7f5af0` is **4.18:1** — passes AA as large text
  (≥18.5px, or ≥14px bold) but **fails for small text**. The 11.5px count badges
  on violet are below AA. Darken the fill to `#6f4ae0` (5.2:1) for any small text
  on accent.
- Every disabled control states its reason as text, not as a tooltip.
- Degraded and error states are conveyed by text and colour, never colour alone.
- Reduced motion is honoured throughout; no state depends on movement.
- Focus rings use `press` timing and `#c9b6ff` at 2px with 4px offset.
- Ids and endpoints are selectable text, never images — operators copy them.

---

## Assets

**No images, no icon fonts, no SVG illustrations anywhere in the application.**
Everything is CSS and inline SVG: borders, radii, gradients, masks, and one
`feTurbulence` noise tile on the login screen.

The only binary assets are the two repo banner PNGs in `banner/`, which are
exports of `design/Syndra Banner.dc.html`.

---

## Files

| Path | What it is |
| --- | --- |
| `design/Syndra IA.dc.html` | **Start here.** The master board: §01–§32, all 41 screens, three cross-cutting patterns, live motion specimens. Needs `support.js` beside it. |
| `design/Sidebar.dc.html` | The sidebar component, implemented. Its logic class is the spec for all three views' nav trees. |
| `design/Source.dc.html` | The Access source component, implemented. |
| `design/support.js` | Runtime for the `.dc.html` files. Not part of the application. |
| `design/Syndra Banner.dc.html` | Source for the two repo images. |
| `login/LOGIN.md` | Full spec for `/login` — layout, three states, animation timings, four rasterisation gotchas. |
| `login/login-reference.html` | Standalone, dependency-free working build of the login screen. Opens directly in a browser. Delete the demo controls in the top-right. |
| `mobile/MOBILE.md` | **The touch form of the whole application.** Two breakpoints, the four movements, count-driven navigation, how the dense tables become rows, and the touch form of `/login`. Says only what changes; this README stays normative. |
| `mobile/Syndra Mobile.dc.html` | The mobile board: M00–M28 and M32, ~60 phone figures at 390px, numbered against this README's `§` sections. Needs `support.js` beside it. |
| `banner/syndra-banner-1280x400.png` | README header image (2× export). |
| `banner/syndra-social-1280x640.png` | GitHub social preview (2× export). |
| `banner/BANNER.md` | Markup snippet and re-export instructions. |
| `ia-2026-08-04/` | **The predecessor bundle, kept whole.** The information-architecture handoff of 2026-08-04, which this one grew out of two days later. Its README carries ~600 lines of reasoning about the Basic/Advanced split that never moved into this file, and its boards are the narrower IA-only cut. Superseded for building from; not superseded for understanding why. |

### Suggested reading order

1. This README, through "The three audiences".
2. `design/Syndra IA.dc.html` §01–§03 — navigation, the view switch, Access source.
3. §04 Today, then §05 People. Those two are 80% of the product's traffic.
4. §11 the four list states, §31 the three cross-cutting patterns, and §32 motion.
   All three apply to everything else.
5. `login/LOGIN.md`.
6. The rest of the sections as you build them.
