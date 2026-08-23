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

## 7a. Three more places where the copy names a fact nobody holds

§7 is not the only one. The design reply is written as though the product
knows more than it does, and in each case the honest version is the same move:
say what is true, offer what works, and do not invent the missing half.

- **B14 · the empty member landing.** The reply: *"Nobody has given you access
  yet. Ask Kabir Rao, who looks after Fabrication."* Nothing in Syndra records
  who looks after a project — there is no owner field on a project, in the
  models or in the API — so the name would be fabricated, on the one screen
  whose whole job is telling somebody with no access what to do next. It
  carries the action instead, and does not claim to know who.
- **B8 · the request form's free-text escape.** The reply: *"Can't find it?
  Describe what you need and we'll route it."* A request is a project and a
  role; there is no field for prose and nothing that would route it. A form
  that accepted the sentence would be dropping it. What is true is that Why
  reaches the decider verbatim, so the escape points there.
- **A4 · the count in every bar verb.** The reply puts it in each button —
  *"Rehearse removal for 9 people"*. In a bar with five sibling verbs that is
  the same number five times beside a sentence that already states it. The
  count is stated once, in the sentence; the verbs carry the step. Where a
  verb applies on tap rather than opening a plan — Automatic rules is the only
  such surface — it does state its own count, because there is no plan coming
  to state it.

## 7b. Two more deviations, and one thing the design got backwards

- **B15 · Today keeps no hollow seats.** The reply asks all six blocks to hold
  their places at zero. They do not. Today's blocks are not one-liners —
  Pending changes carries the drain control and its own outcome, Targets
  Syndra can't vouch for lists each target with its age and its reason — so
  six hollow cards would be a page of empty furniture rather than a stable
  structure. The specific failure the seats guard against is already answered
  here in words: a failed load says *"Couldn't check."* rather than *"Nothing
  needs you"*, and the unvouched block states outright that nothing found
  means nothing was looked at.
- **B3 · the rule-delete copy is wrong for this product.** The reply asks for
  *"This stops the rule causing anything new. The access it already caused
  stays."* In Syndra deleting a rule **cascades revokes** — the access it
  caused does not stay — so that sentence would be the most consequential lie
  in the product, told at the exact moment it matters. The existing dialog
  already says the true thing: how many people hold the triggering role, that
  they lose the produced role unless a bundle or a direct grant also supplies
  it, and whether the revokes go immediately or wait under Pending changes.
- **B7 · the reopened row cannot be written yet.** *"Acknowledged on 12 Aug,
  then the expiry moved to 30 Sep — this is back because it is no longer the
  grant you acknowledged"* requires knowing which expiry was acknowledged.
  `ExpiryAcknowledgement` records `by`, `at` and `note`. Until it records the
  expiry it was about, the sentence is uncomputable, and guessing it from the
  acknowledgement date would produce exactly the confident wrong claim the
  sentence exists to prevent. Owed as a backend field.

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

## 9. Three things the branch shipped broken, and what fixed them

Found by an audit of the finished branch rather than by the work itself. All
three had the same shape: a rule the design states, implemented everywhere it
was thought about and missed where it was not.

### The five mutations that reported nothing

§4 says a result never travels and the surface that ran the action reports it.
Five surfaces held an outcome in state and rendered no `ActionOutcome`:
setting and unsetting the welcome bundle, dropping a role from a bundle's
working copy, the bulk confirmation-mode verbs, creating or renaming a role
upstream, and removing an upstream grant. Every one of them had had a
`toast.error` before this branch, so the replacement removed the only signal
and put nothing in its place — the exact sequencing hazard §4 closes with.

`@typescript-eslint/no-unused-vars` reported all five as warnings and the
branch shipped anyway. Warnings are not a gate here; the follow-up worth doing
is promoting the rule to an error, which needs four unrelated dead bindings
cleared first.

Automatic rules is the sharpest of the five: §7a already records those two
verbs as the only ones in the product that apply on tap with no plan to read
first, which makes the report afterwards the whole of what the operator gets.

The upstream role dialog needed one more change than a render. It set an
`applied` outcome and then called `onClose()` on the next line, so the success
report was written into a component already unmounting — code saying one thing
and doing another, which is the same crack blocker 1 came through. Every other
dialog in the product stays open and becomes its result, with its secondary
button relabelled *Done*; `CreateRoleDialog` states the reason in a comment,
and it applies harder here. The sentence this dialog has to deliver is *"Syndra
has no record of this change beyond the audit line"* — about the least undoable
write in the product — and a dialog that closes itself takes that sentence with
it. It now stays open, disables its own primary so the write cannot be repeated,
and offers Done.

One more thing the audit caught, in the test rather than the code: the case
asserting the hook answers during its first render passes against the broken
hook too, because `renderHook` wraps in `act` and effects have flushed before
`result.current` is readable. The frame in question is not observable from
outside the component. It now records the value from inside the render, and
`autoFocus` remains the case that proves the behaviour end to end.

### `autoFocus` could not be switched off

Two dialogs asked `useIsTouch()` and passed `autoFocus={!touch}` to keep the
keyboard shut on a phone. The hook answered from state corrected by an effect,
and React applies `autoFocus` while it commits the node — so the first render
always said "not touch", the field always took focus, and flipping the prop a
frame later changed nothing. The keyboard opened on every phone.

The hook's own docstring had waved this away: *"a frame of the desktop answer
costs nothing — unlike a layout that would visibly reflow."* For `autoFocus`
the frame is the whole decision.

`useMediaQuery` now reads through `useSyncExternalStore`, which answers during
render from outside React and pins the server snapshot to `false` so hydration
still matches the HTML. Every caller that mounts after hydration — which is all
of them, since they are dialogs and sheets — gets the true answer on its first
render.

### The nav sheet leaked a history entry every 30 seconds

The sheet pushes one history entry so the system back gesture closes it before
it leaves the screen. The effect listed `onClose` in its dependencies, and
`onClose` is a fresh closure each render; `useIndicators` refetches every 30
seconds, so the whole bar re-rendered on a timer and the effect re-ran and
pushed again. A sheet left open for five minutes buried the screen behind it
under ten entries.

Independently, dismissing by grabber or scrim called `onClose` directly and
left the pushed entry unspent, so back became a no-op once per sheet opened.

`onClose` now lives in a ref — the same fix `useDialogFocusTrap` already
carries, and for the same reason — and dismissing goes through
`history.back()` so the entry is spent rather than abandoned. Picking a
destination is the one exception and still closes directly: the navigation
pushes its own entry on top, and racing `history.back()` against a Next.js push
would undo the tap.

## 10. The eight the audit left standing

Named in the same audit as §9 and deliberately deferred at the time, then
asked for. Six are the design's own rules broken in one place each; two are
the platform.

### The nav sheet had a second set of manners

`Modal`'s grabber is `h-11` with `tabIndex={-1}`, and `focusableIn` carries
four lines explaining why: as the panel's first focusable element it took the
focus every dialog gives on open, so every dialog opened with the cursor on
"close it". The nav sheet's grabber was written without either — 22px, in the
tab order, and answering to "Close" where Modal's answers to "Dismiss". It now
matches on all three. The negative margins are arithmetic, not taste: the box
grew 22px and the bar has not moved a pixel.

### `--touch-nav-height` measured a bar that does not exist

The token said 76px. The tab-shaped bar is 8 + 52 + 8 = 68px **plus**
`env(safe-area-inset-bottom)`, which the bar itself carries and the token
ignored; the sheet-shaped bar is 60px + inset. `ConvergeEntitlements` puts the
freshness dock at `calc(var(--touch-nav-height) + 24px)`, so on a notched
iPhone the dock sat 2px inside the tab bar — the comment beside the token says
reading it "stops this dock and the tab bar disagreeing", and they disagreed.
Now `calc(68px + env(safe-area-inset-bottom))`, the taller of the two shapes so
one number clears both.

### The access source kept half its answer in a tooltip

`AccessSourceList` collapses everything past the strongest source behind a
`+N more` chip, and the names of those sources lived in a `title` attribute and
nowhere else. A hover tooltip, on the component whose entire job is to answer
"why does this person have this" — unreachable by touch, by keyboard and by a
screen reader. It is a button now: the collapsed state is still the default,
because "never a wall of chips" is the rule this component is built around, but
the wall is one tap away rather than behind a pointer.

### Two banners cannot both be the top one

Offline and degraded were siblings, both `sticky top-0 z-40`. At the same
offset and the same z-index the later one simply paints over the earlier, so
degraded covered offline — inverting the ordering AppShell's own comment argues
for. They share one sticky slot now. Degraded also carried `px-[26px]` with no
`tablet:` split, so it had desktop gutters at 390px.

### iOS had no icon to install

`appleWebApp: { capable: true }` was set and there was no `apple-touch-icon`
anywhere — the repository holds two SVGs and no raster. iOS ignores manifest
icons entirely and rasterises nothing, so the installed app that members were
given as the whole point of installability launched with a blank tile.
`app/apple-icon.png` is 180×180, the mark at 150 centred on the dark ground the
manifest already declares: iOS applies its own rounded-rect mask and the ring
needs to clear the corners it cuts.

### The type floor was stated and not enforced

§2 rules "never below 12.5px anywhere" and calls it a legibility floor for
somebody reading at arm's length in a workshop. `type-nav-group` sat at 10.5px
— the smallest type in the product, and it carries the heading inside the
mobile nav sheet. `type-label` sat at 11.5px across 29 sites. Both are at the
floor now, and a test reads the sizes out of `globals.css` so the rule is
enforced at the layer that owns it.

**Still breaching, and not swept:** 22 inline `text-[11px]` / `text-[11.5px]` /
`text-[12px]` across 17 components, including two in `ActionOutcome` and one in
`WithheldPill`. Raising those is a layout change on seventeen screens with no
browser pass behind it, which is 8.2's work rather than this one's. Recorded as
10.7 rather than quietly left.

### A destructive control at 30px

The kebab in `PersonAccess` is the only route into the removal flow for a role,
and it was a 30px box. `touch-targets.test.tsx` covers `Button` and
`ButtonLink`, which is exactly why a raw `<button>` slipped past it. The ring
is still 30px; the target around it is 44.

The guard for this needed one more thing than a regex. Matching `<button` to
the first `>` is wrong in JSX — `onClick={() => …}` closes the tag early, the
className is never read, and the sweep reports a clean repository. The
brace-aware extractor catches both the 30px kebab and the 22px grabber.

### A NUL byte, hiding a line from every sweep

`useRowSelection.ts:99` held `ids.join(…)` with a literal NUL rather than
`\u0000`. `file` reports the source as `data` and `grep` skips it
silently, so that line drops out of every text search of the repository —
including the ones looking for bugs in it. Same byte, written as an escape.

## 11. The floor, enforced everywhere, and the ceiling wired where it bites

### The guard covered the layer that was easy to check

§10 raised the two named type roles and added a test reading sizes out of
`globals.css`. That is the layer the design system owns, and it was half the
rule: a component can write `text-[11.5px]` into a className and never touch
`globals.css`. Fourteen readable breaches were living there, the outcome pill
among them — on every result the product reports.

All fourteen are at 12.5px now. The nine that remain are decoration and the
guard can prove it: every one sits on an `aria-hidden` element — a bold "i" in
a 20px note badge, initials on a gradient the name is printed beside. Nothing
there is read, so a legibility floor for reading does not govern it. The one
case outside a tag, `Avatar`'s size map, states `type-floor-exempt` and says
why; a marker covers the eight lines under it.

### Two queues could select past what the server accepts

`services.BulkMaxUsers` is one constant capping three bulk endpoints — grants,
requests and drift. Five surfaces render `SelectAllRow`, and only People passed
the bar a `ceiling`. So on Requests and Unexplained access an operator
select-alled, tapped, and met the 4xx afterwards — which is exactly the
sequence `SelectionBar` says it exists to prevent. Both now state the number
before the tap and offer the same narrowing move People does.

The other two select-all surfaces need nothing: Automatic rules' bulk
confirmation-mode route has no cap, and expiring-access acknowledges one grant
per request. Converge is capped but takes a cohort rather than a row selection,
so it has no select-all to bound.

**Still People-only:** `visibleCount` / `onSelectVisibleOnly` / `wholeScope`.
The other four surfaces cannot say "select only the 12 shown" or distinguish
"all 214" from "214 selected" — the ambiguity `SelectAllRow`'s docstring is
built around. Recorded as 11.3 rather than swept in behind the ceiling work.

## 12. The ceiling that counts something else, and the boundary below the boundary

### Expiring access was gated on the wrong noun

§11 said this surface needed nothing because acknowledging is one grant per
request. That read the per-row control and not the bar. The bar's only verb is
*Rehearse an extension*, which opens `BulkDialog op="extend"` and reaches
`useBulkGrants` — `services/bulk.go:186`, capped at 500. So it was the third
select-all surface with a capped endpoint and no ceiling, not the first of two
that did not need one.

It is also the only one where copying the prop from People would have been
wrong. The screen selects **grants**; the endpoint caps **distinct people**.
600 grants held by 300 people is legal and would have been blocked; 500 grants
held by 500 people is already at the limit and would have sailed through. The
bar derived `overCeiling` from its own `count`, so there was no way to say
this.

`SelectionBar` now takes `ceilingCount` and `ceilingNoun` — what the ceiling
counts, when that is not what the bar counts. Both default to the selection, so
the three surfaces where the units agree are untouched and cannot grow a clause
about themselves. Where they differ the sentence says both numbers, because the
operator can see one of them and is being refused on the other:

> All 601 grants selected · they cover 520 people, and 500 is the most that can
> run at once.

Narrowing takes **whole people**. Once the cohort is full, the later grants of
somebody already in it still come along: dropping them would extend part of a
person's access and leave the rest to lapse, which is a worse thing to do than
refuse.

### `error.tsx` cannot catch the layout it lives in

A throw in the root layout — or in a provider it mounts — unmounts the layout,
so the boundary inside it never renders and Next falls back to its own default
page: unstyled, unthemed, no identifier, no way back. The blank phone screen
this branch set out to remove, arriving by a different door.

`app/global-error.tsx` answers it, under two constraints that both cost
something:

- **It imports nothing from the app.** Not the copy row, not a button, not a
  token. Whatever threw may be a provider every one of those sits inside, and a
  boundary that re-enters the broken tree is not a boundary. So the colours are
  literal — the one place in this product where that is correct rather than a
  violation, because the document that loads `globals.css` is the thing being
  replaced. The palette guard exempts this file by name, and a second test
  holds the exemption honest by failing if the file ever imports from `@/` or a
  relative path.
- **No `reset()`.** Same reasoning as `error.tsx`. A render that threw on the
  state it was given throws again on the same state.

Both themes are authored in a `<style>` element rather than inline props, since
an inline style cannot hold a media query and the shell that would have chosen
a theme is the thing that is gone.

## 13. One number in two languages, and the route that had no bound

### The constant was never checked against the constant

`BULK_MAX_USERS` in `useBulkGrants.ts` is what four selection bars promise an
operator before they tap. `services.BulkMaxUsers` in `bulk.go` is what three
handlers refuse on afterwards. Nothing compared them: two literals, two
languages, no guard. Drop the Go one to 250 and every bar keeps promising 500,
stops refusing at the right point, and operators are back to meeting the 4xx
after the tap — the exact failure three rounds of ceiling work exist to
prevent, restored silently by an edit in another language.

Worse, a comment in the expiry queue's test said this check existed. It did
not. That is the third comment on this change found asserting a property the
code did not hold — after the nav-height token that "stops this dock and the
tab bar disagreeing" while they disagreed, and the first-render test that could
not observe a first render. A comment claiming a guard is worse than silence,
because it tells the next reader not to look.

`lib/queries/__tests__/bulkCeiling.test.ts` reads the Go constant off disk and
compares it, the same move `design-system.test.ts` already makes against
`globals.css`. It throws rather than skipping if the backend tree is not there:
a guard that quietly passes when it cannot find what it guards is the thing it
was written to replace.

### The least reversible bulk write had no ceiling at all

`handleBulkSetConfirmationMode` checked only that `ids` was non-empty. Every
other bulk route in the product stops at `services.BulkMaxUsers` — and this is
the surface whose two verbs apply on tap rather than opening a plan, and whose
rules cascade revokes. An unbounded set there is one statement flipping every
rule in the product with nothing computed first. The ceiling is the mirror
image of what the front end spent three rounds on: it existed everywhere except
where the write was least reversible.

Bounded now, with the same message shape the other routes use, and tested at
both `BulkMaxUsers` and `BulkMaxUsers + 1` — the boundary included, because a
cap that refuses at exactly the limit makes the number the bar promises a
number the server does not honour.

The front end needs no change: `policies/page.tsx` selects rules, the cap
counts ids, and the units already agree. Wiring `ceiling` there is left for
whoever next opens that screen with the browser pass, since it is now a real
limit rather than a hypothetical one.

## 14. The browser pass, and the two numbers it corrected

8.2, run at a real 390 (Chrome's window floor is ~500px, so this is device
emulation and not `resize_page`), plus 744 and 1280. The app was served with no
backend reachable, so every data read failed and the lists rendered their error
and empty states. That bounds what this pass proves and what it does not, and
the boundary is stated in 8.2 rather than left to be assumed.

**Clean:** zero horizontal overflow on all 20 routes in `nav.ts` plus the 404,
at 390 and at 744. One control under 24px on `/roles` — an inline `<a>` in the
last clause of a sentence, which WCAG 2.5.8 exempts. No rendered text below
12.5px on any route once `aria-hidden` decoration is excluded, which is the
§10/§11 type work confirmed in a browser rather than in a regex. The rail's
group headings at 12.5px measure 100px at their widest inside a 227px rail and
do not wrap. `#app-scroll` contains exactly one `sticky top-0` element — the
banner collision is structurally gone. The nav sheet's grabber is 44px, carries
`tabIndex={-1}`, and the focus the sheet gives on open lands on the view toggle
rather than on the handle.

**Two numbers were wrong, both of them mine, both stated as facts in a commit
message:**

- `--touch-nav-height` was `calc(68px + env(…))`. The bar measures **69**: a
  1px top border, 8px padding, a 52px tab, 8px below. §10 said this token was
  now measured; it was derived, and the derivation dropped the border. The
  freshness dock still cleared the bar — the error ate 1px of a 24px gap — but
  the token claims to be the bar's height and was not.
- The grabber's negative margins were computed as `-mt-[5px] -mb-[7px]` from
  the panel's padding, and the commit said the bar "has not moved a pixel". It
  had moved 6px down: the panel's own border is not in the padding. Measured
  against the original — bar centre 24px from the panel top, 19px of gap below
  it — the correct pair is `-mt-[11px] -mb-[1px]`, which reproduces both
  exactly.

Both were the same mistake the audit kept finding: arithmetic asserted as
measurement. The fix is not better arithmetic, it is that these two numbers are
now the output of a measurement and say so.

## 15. The pass with data, and the bug every previous sweep was blind to

Run against the dev host's backend, authorised for this pass. Real rows: 5
people, 6 projects, 27 roles, 4 applications, 50 audit entries, 1 bundle. Drift,
holds and mapping rules answer 502 from that stack, so those three routes were
still measured in their error state.

### Every long route on a phone could not be scrolled, and had no tab bar

`AppShell` is `flex h-dvh flex-col overflow-hidden`, and inside it the column
and `#app-scroll` were `flex-1` with no `min-h-0`. A flex item defaults to
`min-height: auto` and refuses to shrink below its content — so `flex-1` never
constrained the scroller. It grew to the full height of the page inside it,
`overflow-y-auto` had nothing to scroll, and the shell's `overflow-hidden`
clipped everything past the fold.

On a 390×844 phone, with real data:

| Route | scroller height | tab bar top |
|---|---|---|
| `/users` | 1000 | 1066 |
| `/roles` | 3971 | 4037 |
| `/audit` | 4173 | 4239 |
| `/` | 2368 | 2434 |

The tab bar was below the fold on every one, and the page could not be
scrolled to reach it. The product's entire navigation, absent, on the form
factor this change exists for.

**Every sweep before this one measured horizontal overflow and nothing else** —
including the one this branch recorded as "zero horizontal overflow on every
route", which was true and is still true. Nobody asked whether the page could
scroll. `min-h-0` on both flex items fixes it: the scroller becomes 717px
(844 − 58 top bar − 69 tab bar), scrolls its full height, and the tab bar sits
at the viewport bottom on all 21 routes. 744 and 1280 unchanged.

The guard is static, because jsdom computes no layout and the whole failure was
one missing class.

### One more target under the floor, on the screen the app opens on

The Makerspace card's footer reads *"17 roles nobody holds · 1 empty bundle"* —
two links and a separator, no prose. WCAG 2.5.8's inline exemption is for a
target inside a sentence, and there is no sentence here, so it does not apply.
Both measured 16px tall. They carry their own 44px on touch now, which grows
that row from 45px to 69px on a phone and leaves it unchanged above the desktop
breakpoint.

### Confirmed with data, not argued

Zero horizontal overflow on all 20 routes plus the 404, at 390, 744 and 1280.
Every route scrolls; the tab bar is pinned at the viewport bottom on all of
them. No rendered text below 12.5px outside `aria-hidden` decoration. One
sub-24px control left in the product — the inline `<a>` in the last clause of a
sentence on `/roles`, which the exemption does cover. The selection bar sticks
above the tab bar rather than under it (bar bottom 767, tab bar top 775).

### Not a bug, but worth a decision

With five people selected, the bulk bar is **312px tall** — five stacked
*Rehearse …* verbs filling 43% of a 717px scrollport. It behaves correctly and
nothing overflows. Whether five verbs should stack on a phone is a design call
that wants the screen in front of somebody, so it is recorded rather than
changed.
