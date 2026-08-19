# Design — the touch form of the whole application

## 1. One application at a different width

The strategy decision, taken before any code: this is **not a separate app, and
is not to be treated as one**. Same routes, same components, same copy. CSS
performs the reflow; JavaScript decides only what CSS cannot observe — the shape
of navigation, and whether a dialog renders as a dialog or a sheet.

The alternatives were considered and rejected:

- **Component-level branching** (phone and desktop variants of dense
  components, chosen by a hook) gives freedom per screen and costs two
  implementations of the same behaviour, kept in step by discipline. The
  product already has one drift bug of exactly that shape in its history — the
  member allowlist that existed twice — and the lesson generalises.
- **A parallel route group** is what the handoff explicitly forbids. It
  guarantees the two surfaces diverge, because nothing forces a fix on one to
  reach the other.

The consequence to hold onto: **a screen cannot be fixed on one device and left
broken on the other**, because there is only one of it.

## 2. The board is a drawing. The rules are the spec.

`Syndra Mobile.dc.html` states its values are "final and exact". They are not
self-consistent, and where they conflict with the stated rules the **rules
win**. Extracted contradictions, worst first:

| Drawn | Stated | Ruling |
| --- | --- | --- |
| Type below 12.5px on **every figure** — 10px carets, 10.5px badges, 11px eyebrows, 11.5px tab labels | "Never below 12.5px anywhere" | **Rules win.** The floor is a legibility floor for somebody reading at arm's length in a workshop. MOBILE.md's own type table breaches it by listing an 11px eyebrow; the table is wrong, and the eyebrow becomes 12.5px. |
| Row min-heights of 46/48/52/56/58/60/64/66 | "Row min-height 60px · copy row 52px" | **Rules win as floors, not as fixed heights.** MOBILE.md says so itself: "Rows grow; the 44px floor is a floor, not a fixed height." A two-line fact row may exceed 60; nothing may fall under it. |
| Copy rows at 46px (M10a) and 48px (M19a); M12c has no Copy affordance at all despite its caption naming one | "52px copy rows" | **Rules win.** These are the rows a member taps while holding a phone in one hand and a laptop lid in the other. |
| Nav-sheet rows at 42px, indented children at **40px**; segments at 38px; the header **view pill at 34px** | "44px minimum" | **Rules win, and this one is not close.** The view pill is a primary control that changes what the whole application shows. 34px is below every published touch guideline. |
| Two-button action bars at `gap:10px` — including the figure whose own caption claims 12px | "12px between a destructive control and a benign one" | **Rules win.** The rule exists so a thumb travelling to Cancel cannot land on Revoke. |

### The tablet rail keeps its labels

MOBILE.md's tablet rule collapses the rail to **64px, icons and badges only,
labels dropped**. That instruction assumes an icon set this product does not
have and has deliberately avoided: the README's assets section is
"**no images, no icon fonts, no SVG illustrations anywhere**", and the rail's
own affordance is a 6px dot beside a word. A 64px rail here would be fourteen
identical dots.

The deeper reason is the product's voice. This is an application that refuses to
truncate a role description because "can cut unsupervised" versus "may enter and
watch" is the whole difference, and that forbids tooltips because a label that
has to be hovered is a label nobody can act on. Replacing eight words with eight
glyphs contradicts the argument the rest of the interface is making.

So at 720–1080 the rail stays at 252px with its labels. A 744px tablet gives the
content column 492px, which is more than a phone and enough for the three
columns the same rule asks for. Revisit if a real icon language is ever
introduced — but introducing one to satisfy a rail is the tail wagging the dog.

Where the board shows **structure** the rules do not cover — a full-height sheet
with no grabber and a `Close` control, a sheet docked above the keyboard with
borders on all sides — the board is followed. A drawing is authoritative about
shape and unreliable about measurement, and this is the line between the two.

Precedent for the ruling is in the handoff itself: the README calls its own
1600px canvas "a property of the *design document*, not of the application",
and says so specifically to stop a builder reading measurements off a static
board as a prescription.

**Where the board and `nav.ts` disagree on a label** — `Today` vs `Home`,
`Storage` vs `Network storage`, `Drift` vs `Unexplained access` — `nav.ts`
wins, per the standing decision that the codebase is the structure authority.

## 3. Breakpoints named for devices

`--breakpoint-tablet: 45rem` (720px) and `--breakpoint-desktop: 67.5rem`
(1080px), declared in `@theme`. Phone is the unprefixed base: mobile-first is
what stops a phone paying for a desktop layout it then has to override.

Tailwind's stock scale is deliberately not used. Its steps are 40/48/64/80rem
and none of them fall where this shell breaks — the rail is 252px and the widest
fixed table row sums to roughly 950px, so a stock `lg:` collapses columns a
whole device early. Naming them for devices also means a reviewer can tell
whether an author meant a device or a number.

## 4. Confirmation without toasts

Toasts leave the product entirely rather than only the phone, because the
argument against them is a product argument and not a screen-size one: a
notification that removes itself after four seconds is the wrong place to report
an action whose consequence is somebody's access.

The replacing rule is that **a result never travels**. The surface that ran the
action reports it: a row reports in the row, a sheet becomes its own result, a
plan reports on its result step. Three surfaces, three homes, no fourth.

Six words, shared by every surface: `Apply · Applied · No change · Refused ·
Failed · Queued`. Two properties are load-bearing and are enforced by test:

- **Queued is not succeeded**, wears the warn tone and the present tense, and
  never a tick. Everything Syndra sends to an add-on is recorded now and
  dispatched later; the response reports `succeeded: 0` precisely so a client
  cannot default it into success. An accent pill would report a door as locked
  while it still opens.
- **Refused and failed stay apart.** A 4xx is Syndra declining for a reason
  somebody wrote, which an operator can act on. Anything else is the machinery
  not answering, which they can only quote — so the `request_id` is a labelled
  copy row rather than a bare hex string in a chat window.

Sequencing constraint discovered while surveying: about 40 of the 85 toast call
sites are `toast.error`, and are the **only** user-visible signal that a
mutation failed. The replacement therefore lands before any removal, not after.

## 5. What the design reply got wrong about the backend

The reply asks for two endpoints to be built and names what they must return.
Both already exist, and both already return it:

- `DELETE /api/v1/users/{id}/grants/{grantId}` — `handlers/router.go:62` — and
  it returns the residual set as `revoked_roles` / `retained_roles`
  (`services/role_members.go:390`), which is exactly what B9's four removal
  sheets state.
- `GET /api/v1/targets/{target}/accounts/dormant` — `handlers/router.go:274` —
  returning a per-row `reason` rather than a boolean (`services/dormant.go:53`),
  with `subject_still_member` as the field the surface makes unselectable on,
  plus `state_read_at` and `truncated`.

So this change requires **no backend work** and stays inside `ui/`.

## 6. Freshness has five flags, not four

The design reply's A6 gives four states. README §31A gives five, and the fifth
is orthogonal: `truncated` coexists with any of live / ageing / stale /
provisional and reads "Showing 200 of more than 200 — this is not the whole
list". A capped read is not an absence, and a surface that drops the flag turns
"we did not see everything" into "there was nothing".

The blocked-versus-allowed rule stays exactly as §31A states it, and the two
must **not** be unified: a stale read blocks `adopt` and `purge`, because those
bind or destroy on the strength of a list that may have moved, and allows
`plan`, `apply` and `hold`, because those only join a queue an operator can
still inspect and cancel. `blocksIrreversibleAction` already encodes this.

## 7. The dormant sweep cannot state its own acknowledgement

README §29 fixes the rung-2 copy as *"I understand 41.2 GB of their files goes
with the accounts"*, and calls `bytes_held` "what makes the acknowledgement
possible". `DormantAccount.BytesHeld` is a pointer and **nothing fills it** — it
needs a per-account usage read (`pool.dataset.query`) the add-on does not
perform.

The pointer is right: "we do not know" and "nothing" are different answers and
only one of them is safe inside a sentence an operator ticks. So until the
add-on grows that read, the sheet **states that the size is unknown** rather
than implying zero or omitting the clause. This is a copy change to the one bulk
action in the product and should be re-read by design when the field lands.

## 8. What the tests can and cannot see

jsdom loads no stylesheets and computes no layout, so **a CSS breakpoint is not
observable in vitest**: a `tablet:hidden` element is present and visible in
every test regardless of any width set. Only code that *asks* a question can be
asserted.

This is a second reason structural decisions on touch are made in JS rather than
inferred from a class — a `matchMedia` answer is testable, a media query is not.
Everything else is verified in a browser at 390 / 744 / 1280, per the per-route
checklist that `BIA-36` and `ISC-43` already owe.

Two harness gaps were fixed to make even that much possible, and both had been
silently narrowing the suite: `matchMedia` did not exist, so three production
sites had only ever executed their no-preference branch; and `localStorage` was
Node 22's experimental global with no working `setItem`, shadowing jsdom's, so
nothing that remembers a choice — the theme, the view, the chime — could be
tested at all.
