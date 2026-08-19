# Reply to the Claude Design prompt pack

Answers to `CLAUDE-DESIGN-PROMPTS.md`, received 20 Aug 2026. Recorded verbatim
below the line — this is the design authority for the mobile work, and it
supersedes `MOBILE.md` wherever the two disagree.

## Provenance and corrections

The reply arrived as a message, not as a bundle. `design_handoff_syndra 4/` is a
byte-identical copy of bundle 3 (same MD5 on every file, same `19 Aug 02:46:49`
mtimes) and contains no design output — do not read it expecting answers.

**Drawn:** A1 and A2 only (`Syndra Mobile - Block A.dc.html`, figures A1a–A1h and
A2a–A2f). A3–A7, Block B, Block C and Block D are answered in prose here and not
yet drawn.

**Two claims in the reply that were already stale when it was written**, verified
against `feat/addon-platform` on 20 Aug:

| Reply says | Reality |
| --- | --- |
| `DELETE /api/v1/users/{id}/grants/{grantId}` is missing | **Exists** — `handlers/router.go:62`. And it already returns the residual set the reply asks for: `revoked_roles` / `retained_roles`, `services/role_members.go:390-393`. |
| `GET /api/v1/targets/{target}/accounts/dormant` is missing | **Exists** — `handlers/router.go:274`. And it already returns a per-row **reason** rather than a boolean: `DormantAccount.Reason`, `services/dormant.go:53`, alongside `subject_still_member` (the field the surface makes unselectable on), `state_read_at` and `truncated`. |

So point 4 of *Code findings* is satisfied by the backend as it stands, and no Go
work is required. The mobile change lives entirely in `ui/`.

**One constraint the reply does not know about.** `DormantAccount.BytesHeld` is a
pointer and **nothing fills it yet** — it needs a per-account usage read the add-on
does not perform. A5's rung-2 sheet therefore cannot state a size, and must say the
size is unknown rather than implying zero. The pointer exists precisely so that
"we do not know" and "nothing" stay distinguishable in a sentence an operator ticks.

---

# Reply to the Claude Design prompt pack

Every question the pack asks, answered. Rulings are decisions, not options: where I
disagreed with the pack I have said so and why, under **Pushback**.

**Status of the drawing.** A1 and A2 are drawn — `Syndra Mobile - Block A.dc.html`,
figures A1a–A1h and A2a–A2f. A3–A7, Block B, Block C and Block D are answered here
but not yet drawn. The answers are the expensive part; with them settled the figures
are mechanical, and none of them can now contradict each other.

Two things in the earlier handoff were **wrong against the code and are now fixed**:
the nav model (M01) and the login arch scale (M28). Details under *Code findings*.

---

## The seven pre-taken decisions

All seven stand. One amendment.

| Decision | Reply |
| --- | --- |
| The codebase wins | **Agreed, and applied.** `lib/nav.ts` is the structure authority. A1 redraws the tree from it and supersedes M01. |
| Toasts removed everywhere | **Agreed.** A2 is the replacement vocabulary. `sonner`, `lib/toast.ts`, `lib/drain-toast.ts` come out; `lib/drain-outcome.ts` stays. |
| Honest clipboard fallback | **Agreed.** A3 ruling below, with one amendment about the degraded page. |
| Bulk on all five surfaces | **Agreed.** Nobody loses a capability by picking up a phone. |
| Light theme derived, not drawn | **Agreed** — with the exception already in the board: M13b draws the access map in light, because that screen's only content is hairlines and a glow, and a glow is the one thing that does not survive a token swap. Do not extend the exception. |
| Tablet: eight figures at 744 | **Agreed.** The three columns are named per screen under Block C, which is the deliverable rather than the figure. |
| Platform behaviour in scope | **Agreed.** Block D answers below; D4's answer is a rule, not a layout. |

---

## Block A

### A1 — Navigation. Drawn.

- **The Access group.** The tab lands on **Projects** and carries Roles and Apps as a
  three-way segment in the page header. Not a sheet: the desktop rail shows all three
  expanded because they are three views of one question, and a sheet would hide a set
  the rail deliberately reveals while taxing the commonest operator loop with a tap
  that shows nothing new.
- **The group label survives** as the page eyebrow, so `crumbsFor`'s `Access › Roles`
  is legible with no breadcrumb bar. The tab keeps the group's name, not the child's —
  renaming it *Projects* makes the tree's own word unreachable.
- **Six indicators, four slots.** The Go-to bar carries **one dot in the highest tone
  present** and a count of **destinations that want attention, not items**. Three
  findings plus eleven expiries plus three holds is seventeen of nothing. A collapsed
  group carrying live counts shows a dot in its own tone, never a rolled-up number.
- **The view switch** is a two-state pill, both labels always legible: header on Basic,
  top of the sheet on Advanced, absent for members. It never navigates and never
  re-sorts.
- **Account and sign-out.** 44px initials in the header on *every* surface open one
  Account sheet, so account never depends on having a nav sheet. Sign-out lives inside
  it below a hairline, states that it clears every tab's place, and is two taps deep.
  It is never a nav row.
- **Nav loading.** A badge whose count has not landed is a **hollow ring in the badge's
  own seat** — not a blank, which shifts the row when the number arrives, and not `0`,
  which is a claim.
- **Reveal in place.** Same URL, view pill goes accent, revealed panel scrolled to, and
  a one-line note reads *Advanced view · revealed below, nothing navigated*. The return
  control names both the view and the row, because returning to Basic without returning
  to the row leaves the operator at an arbitrary scroll position on a page that just
  changed shape.

### A2 — Confirmation. Drawn.

- **The one rule:** the surface that ran the action reports it and never hands the
  report to another surface. Row → the row. Sheet → the sheet becomes its result. Plan
  → the result step. Three surfaces, three homes, no fourth — that is what makes it a
  replacement rather than a scattering of the toast.
- **Six words, everywhere, no screen invents a seventh:** `Apply · Applied · No change ·
  Refused · Failed · Queued`.
- **Queued is amber and present-tense** and names *who has not been told*. Never a tick,
  never accent, never the past tense.
- **A resolved row keeps its seat**, dims, and states its effect in its own corner. It
  leaves on the next read, not on the tap.
- **Every refusal ends "Nothing was changed."** The `request_id` is a copy row with a
  label saying what it is for.
- **Rows needing nothing are counted, not hidden** — they were in the plan the operator
  approved, and dropping them changes the arithmetic they agreed to.
- **No spinners.** One breathing dot, on the row or control that is working, which keeps
  its label.

### A3 — The copy row

1. **At rest / just copied** — as specified: 52px, mono 13px, `Copy` at 12px faint;
   confirmed in place for 900ms.
2. **Clipboard unavailable.** The row knows before it is tapped, because
   `navigator.clipboard` is either there or not. It carries **`Select`** instead of
   `Copy`; tapping selects the value and a line under it reads *"Your browser can't copy
   on this connection — the value is selected, hold to copy."* Voice matters here: the
   value is fine, the browser is the limitation, and the row must not look like an error.
3. **Long values** wrap to as many lines as they need and the row grows. Never truncate:
   an operator reading a path aloud needs all of it.
4. **Copy inside a degraded page — amendment to the pack.** Not simply "copy is a read,
   so it stays live". On a **demo-data** degradation the values in the list are
   fabricated, and copying a fabricated id into a support message is worse than being
   unable to copy. So: **copy goes inert with everything else inside the dimmed
   content**, and stays live **only in the banner itself**, where the command is real and
   is the fix. On an **identity-provider-unreachable** degradation the list values are
   genuine, so copy stays live throughout. The rule is *copy is live when the value is
   true*, which is one sentence a developer can apply.
5. **Multi-line command block** — same row grammar, mono 13px, wraps, one `Copy`
   affordance for the whole block, never per line.

### A4 — Selection mode

- **At rest** the header carries a named `Select`. Nothing about the rows suggests
  selectability until it is tapped; the header is where the capability is announced.
  Long-press is forbidden — it is invisible.
- **Checkboxes are 24px glyphs in a 44px row**, and the whole row is the target.
- **Select-all when filtered** says what it will do, in words, with both numbers:
  **"Select these 12"**, and directly beneath it, muted, *"340 match no filter."* Never a
  bare "Select all", which is the ambiguity the pack is right to flag.
- **The count bar names the next step, not the action.** Destructive or wide-reaching →
  **"Rehearse removal for 9 people"**. Never "Remove 9 people".
- **Unselectable rows** keep a dashed left edge and state the reason on the row.
- **The 500 ceiling.** At 640 selected the bar goes amber and reads **"640 selected · 500
  is the most that can run at once."** The action it offers instead is **"Rehearse the
  first 500"**, with the ordering stated (*by last login, oldest first*) — an arbitrary
  500 is not a cohort. It does not silently truncate and it does not disable the only
  control on screen.
- **Leaving selection mode** is a named `Done` where `Select` was. It **keeps** the
  selection if the operator re-enters within the same screen visit, and discards it on
  navigation. Escape's desktop behaviour maps to `Done`; the bare-`a` select-all
  shortcut is **not implemented on touch** — a single letter that selects 340 people
  cannot coexist with a keyboard that appears for search.

### A5 — The ladder with a keyboard in the way

- **Rung 2 with a password** (the dormant sweep). Order top to bottom: consequence
  sentence, ticked sentence, password field, footer button. The sheet's body scrolls;
  **the footer never floats over the field** — the sheet's max height is the space above
  the keyboard, and the footer sits at its bottom edge. The button states its own gate:
  **`Remove · 1 of 2 acknowledged`**, then `Remove · password needed`, then armed.
- **Rung 3 reading order matches keyboard order.** Reason field *above* the confirm
  field, because the operator writes the reason before they commit, and the field nearest
  the keyboard should be the last one filled.
- **What may scroll away.** If something must leave the viewport it is *never the
  consequence sentence*. Rank, most protected first: consequence sentence → the string to
  type → the field → the button. In practice the sheet docks to the keyboard and all four
  fit; the ranking is the rule for when they do not.
- **The unarmed button says what it is waiting for** — `Type the name to confirm` — not a
  greyed label.
- **Armed:** solid `#ff5c4d`, 50px, a line break and 12px clear of Cancel, Cancel nearer
  the screen edge.
- Matching is trimmed and case-insensitive; an empty expectation never arms the button.
- The four rung-3 places keep their four different expectations. **Adopting** carries
  *"There is no undo."* as its own line, not folded into a paragraph.

### A6 — The freshness strip

Four classifications, one component:

| State | Dot | Behaviour |
| --- | --- | --- |
| **Live** | `#a3e635` filled | Under a minute. Outline Refresh pill. Not sticky. |
| **Ageing** | `#a3e635` filled, text muted | Older, **nothing is gated**. Not amber and not sticky — amber means a broken assumption, and an ageing read has not broken one. The distinction is *whether an action is blocked*, never the age alone. |
| **Stale** | `#f5a524` filled | Over ten minutes **and something is gated**. Strip goes sticky, Refresh takes the accent fill, the gated action goes dashed and states its reason. |
| **Provisional** | **outline ring, no fill** | The read answered; the target could not confirm it. Word: **"Reported, not confirmed."** Neither lime nor amber — it is not healthy and nothing is broken. A gated action **cannot** be satisfied by a provisional read, and says so in those words. |

**Refreshing** keeps the previous age legible and puts the breathing dot on the Refresh
control. The old value stays until the new one lands: a strip that blanks while it
refetches tells the operator less than it did before they tapped it.

Placement: **above** a tab set when it governs all tabs; directly under the page title
when it governs one list.

### A7 — Dialogs become sheets

- **420 → a short sheet** sized to its content (a rename field is not a full-height
  event). **520 → a sheet stopping 96px short of the top.** **760 → full-height with a
  sticky footer.**
- **Which sheets stop 96px short:** any sheet acting on **one named subject** — a person,
  a card, a mapping — so the subject stays visible behind it and the operator cannot lose
  track of who they are acting on. Full-height is for sheets whose content *is* the
  subject: plans, payloads, pickers.
- **Sheet from a sheet: push, never stack.** The second sheet replaces the first's
  content in place; the first's title becomes a back-line at the top. Reasons: two
  grabbers are two dismiss gestures with different meanings, and a stacked sheet on a
  390px screen leaves ~8px of its parent showing, which reads as a rendering fault. Back
  goes **up one level inside the sheet**, then closes it — a sheet is a level of history,
  and so is each level within it.
  - Consequence: **a rung-2 confirmation raised from a plan is not a second sheet.** It
    is the plan's *review* step. The four steps already exist for this.
  - The filter sheet over the search overlay is the one legitimate two-surface case,
    because the overlay is a full screen and not a sheet.
- **A busy sheet cannot be dismissed** and says so: the grabber is replaced by a 38 × 4px
  bar in accent at 40% with the line *"Working — this can't be closed yet."* Silently
  ignoring a dismissal reads as a frozen app.
- **Content shorter than the keyboard:** the sheet's height is `min(content, space above
  keyboard)`. The footer is pinned to the sheet's own bottom edge, never to the viewport,
  so it can never cover the field.

---

## Block B

**B1 · Projects.** Apps-served is a fact, rendered as a count with no chevron; the row's
tap goes to the project. Role descriptions are shown **in full, wrapped** — a description
that must be hovered is a role nobody can safely grant. Zero-role project keeps its row
and reads *"No roles yet — nothing in this project can be granted."*

**B2 · Token shape.** **Explanation-first does not invert here.** The token debug screen
inverts because the operator arrives with a question; this screen has them *composing*,
so the editor leads and the preview follows it. The preview is mono, wraps, and is one
copy block. **Stale preview** carries a dashed top border and *"Behind your edits —
recomputing."* It must never show a shape that is not the one being edited. Override
state names what is inherited and what this app changed, with one control to drop back to
the project's shape. Saved says *"Applied — the next token this app issues carries this
shape."*

**B3 · Automatic rules.** Long option lists become a **stepped sheet**, one choice per
step, four steps, each step a searchable list. Not four nested sheets, which is four
dismissals deep, and not four inline expanders, which puts four long lists on one column.
The step *after* the four is **the rule stated as prose** — *"When somebody is given
Studio Access · enter, give them Studio shares · read, write."* That sentence is where an
operator sees whether they built the rule they meant. Validation refusal names which of
the two problems it is, marks the offending step and returns to it. **Deleting** states
plainly: *"This stops the rule causing anything new. The access it already caused stays."*
— the copy has to say it or an operator will assume the opposite.

**B4 · Automation settings.** Two settings, and the page should look like two settings —
no padding, no filler cards. The global default states *"Existing rules keep their current
mode."* The **chime**, when enabled but unplayable (no user gesture yet, or the device is
silenced), reads **"On · can't play yet"** with the reason. A toggle that says on while
nothing can be heard is a lie the page is in a position to catch. Off under
`prefers-reduced-motion`, stated.

**B5 · Bundles.** Create is the lightest sheet in the product: one field, one button.
Delete states the holder count and what happens to them. The picker is a full-height
sheet, search, sticky group headers by project, matches highlighted
`rgba(155,123,255,.24)` not bolded. **The welcome bundle** is set from a single control
that reads as deployment configuration, not as a bundle property: *"New members receive
this bundle."* **Versions are the spine, one dot per version, current filled — and there
is no one-tap revert.** Restoring goes through rehearsal like any edit. Publishing shows
how many people it reaches before it runs. Moving holders groups the plan by what each
holder gains and loses.

**B6 · Unexplained access.** The three answers are **not three equal buttons**. Attribute
and Mark external are stacked full-width controls; **Revoke sits below a hairline**, is
the only solid red in the product, and each of the three carries its consequence in a
sentence. **Unknown actor** reads *"Syndra can't say who did this — the sweep compares
grant sets, it doesn't read the target's log."* The field is never left blank. **Bulk
revoke is deliberately absent**, and the count bar says so where an operator would look
for it: *"Revoking is one row at a time."* Filtered per-person counts are facts about the
person, not the filter, and say *"2 more items for this person."*

**B7 · Expiring access.** *Let this lapse?* is a sheet with a required note and a plain
statement of the date and who is affected. **An acknowledged row stays visible in its
group**, dimmed, note quoted, with a control to clear it. **A reopened row says what
happened**: *"Acknowledged on 12 Aug, then the expiry moved to 30 Sep — this is back
because it is no longer the grant you acknowledged."* Without that sentence an operator
reads it as a bug. Bulk acknowledge via the A4 bar.

**B8 · Requests.** The member's form asks **in verbs** — *Use the laser cutter* — never
`laser.operate`. When the thing they want is not listed, the form says so and offers
free text: *"Can't find it? Describe what you need and we'll route it."* **Withdrawn is
not declined**: withdrawn reads *"You withdrew this"* in neutral tone, declined carries
the operator's reason **verbatim**, quoted, never paraphrased or truncated. Bulk decide
groups approvable and not-approvable, and the button names only what it will do.

**B9 · One person.** All four removal cases are four different sheets, and every one
states the **residual outcome** — what this person still holds when it is done. The
bundle case names which roles go and which stay because something else supplies them.
The bundle-supplied case's whole job is to say **where it can be removed**. The automatic
case says changing the rule reaches everyone, and offers the rule, not the removal.
Expiry uses **presets before pickers** — *End of term · End of year · 90 days · Pick a
date* — because "end of term" is what people mean. Operator-only. A removal whose
endpoint does not exist is dashed with its reason in the row.

**B10 · A target.** Eight panels become a horizontally scrolling tab set with counts
inline and the freshness strip **above** it, because it governs all eight. **Lifecycle** is
the one control measured in what stops happening, so draining states what is still
outstanding: *"Draining — 7 writes still in flight."* Quiesced states that nothing is
being sent and what will happen when it resumes. **Merge findings** read as a sentence
with both sides named; resolution requires a reason. **Binding conflicts** are rung 3 on
the account's username; **log integrity** is rung 3 on the target's name and says plainly
what re-baselining gives up: *"Syndra will stop looking for what happened between the old
anchor and now."* **Adopt** is rung 3, gated on a fresh read, and carries *"There is no
undo."* **Reconcile now** drains per item. **Creating a mapping** is a stepped sheet
ending in a rehearsal.

**B11 · Identity provider consoles. I agree with you, and would go further.** Direct
upstream writes deserve **rung 2**, and the desktop asking for nothing is a defect, not a
precedent. The ladder's own rule is *set by what cannot be undone* — these writes have no
rehearsal, no cascade preview and no ledger entry, which makes them the least undoable
actions in the product. The ticked sentence carries **what is missing**, not a quantity:
*"I understand this writes to Zitadel now, with no plan and no record in Syndra."* Beyond
that, the four consoles carry a **persistent header line**, not just per-action ceremony:
*"Changes here go straight to Zitadel."* Per-action ceremony teaches an operator to tick;
a standing line tells them where they are. **Zitadel unreachable** is not the ordinary
degraded banner and must not reuse it — it reads *"Zitadel isn't answering. This screen
has nothing to show, and Syndra itself is fine."* Everywhere else the degraded banner
means Syndra is impaired; here the subject is impaired and Syndra is the messenger.

**B12 · Audit and events.** `Load more` states what it has: *"Showing 50 · load 50
more"*, then *"Loading…"*, then *"That's all of it."* Never infinite scroll. The window is
a sheet with the result count in the apply button. A trace id is truncated on the line and
**a full copy row inside the disclosure** — that is where the desktop's hover tooltip
goes. **Live event rows do not insert themselves.** A counted bar appears at the top —
*"4 new since you opened this"* — and inserting happens on tap. Rows arriving under a
reading thumb is the same failure as a reflowing list. Raw payload is a full-height sheet,
mono, wrapped, one copy block, nothing prettified or elided: it is evidence.

**B13 · Member storage.** Password rules are stated **before** typing, never as errors
after. In flight is the breathing dot on the control. Confirmation says **when it will
work**, not that it works: *"Set. It will work on the file server within a few minutes."*
**Re-setting** after the recent change needs its own copy — factual, not alarming:
*"Everyone who set a password before 2 August needs to set a new one. Nothing is wrong
with your account."* **Connection instructions** appear only in the ready state — absent,
not disabled — with the platform as a segment, numbered steps in the member's language,
and the connection string as a copy row **with the A3 fallback**, because this is exactly
where a member on http taps Copy and gets nothing.

**B14 · Member landing.** Grouped by project. Source in a **sentence**, not a chip —
*"You have this because you are in Fabrication."* Doors nested under the role that opens
them, because a door is a fact about a role. Card as one read-only line. **Withheld leads
the list**, names the person and the date, and offers the one action that resolves it. The
empty member is told how to get access **by name**: *"Nobody has given you access yet. Ask
Kabir Rao, who looks after Fabrication."* A member landing with no next step and no name
is the screen that generates the support message.

**B15 · Today.** Two blocks in Basic, six in Advanced — the sixth is merge findings, and
it is not decoration. Advanced blocks are one line with a subtitle carrying the fact that
makes the count actionable: *"14 queued · oldest 3 days"*. **All six keep their seats at
zero**, each a hollow zero; a good day reads as one without becoming a celebration
screen. The queued-writes block's *Resume now* drains inline, per A2, on the landing page.

**B16 · When the app fails.** The render-error card **does not offer "Try again" if trying
again does the same thing** — it offers Home and the `request_id`. **403** says which it
was, when it can be known: an old link or a typed URL. **404** gets two different
sentences — *"This existed and no longer does"* and *"There's nothing at this address"* —
because they mean different things to an operator chasing a link. Both degraded variants
are pinned under the status bar, not dismissible, with the content dimmed to .55 and the
amber frame, drawn **over a working list** so the rule is visible in context.

**B17 · Access map controls.** Node search is a full-screen overlay, results grouped by
what they are. Depth is a segment — *1 hop · 2 hops · Everything* — and all three states
get a figure, because the difference is the screen's value. Re-centring pushes onto the
thread, and **back walks back out of the map**, one centre at a time; a map you can walk
into needs a way out that is not the browser's. A capped node shows 12 and a row reading
*"28 more point at this role"* which opens a full list — not an expander, which would put
40 rows inside a section.

**B18 · Token simulator.** Empty is not a dead end: it states what it will tell them —
*"Pick an app and a person, and this will show the token that app would issue for them
right now."* One chosen leaves the run control disabled **with its reason in place**.
Running keeps both choices visible and changeable. **No access at all through this app**
gets its own sentence — *"This app would issue a token with no roles for Meera. That is
not an error: nothing she holds is in this app's project."*

---

## Block C — the eight tablet figures, and which three columns

| Screen | The three columns | What became of the rest |
| --- | --- | --- |
| **A target's page** | Panels stay **stacked in one column**; the accounts table keeps *account · person · state* | Last login and adoption date go to the row's disclosure. Two panel columns would put a gated action beside an ungated one at the same height, which is exactly the adjacency rule's prohibition. |
| **Unexplained access** | *principal · what Syndra intends · what the target reports* | The comparison **is** the three columns; nothing else earns one. Kind-of-drift stays a word at the end of the first column. |
| **Rehearsal plan** | Centred dialog, 560px, single column | The sticky footer stays at the **dialog's** bottom edge with its counts and the plan's age. A dialog whose footer scrolls is a sheet with worse manners. |
| **Access map** | **Two** columns, not three: *points at it* beside *it reaches*, centre as the page title | The desktop's third column is the centre itself, which at this width is better as a title than a node. |
| **Dormant + selection** | *checkbox · account · why it is dormant* | Last login joins the reason in one column. The count bar spans the **content** column and stops at the rail, so the rail never sits under a bar that does not apply to it. |
| **A person's access** | **Tabs, not side panels** | Only one is right, per the pack: side panels at 744 give lineage 300px, which truncates the sentences that are the whole point of lineage. Tabs keep them full width. |
| **People index** | *name · project · what needs attention* | Role count folds into the attention column. This is the highest-traffic screen and three columns is the difference between scanning and reading. |
| **Bundles** | Contents **beside** versions, two columns | Versions are a narrow spine and contents is a list; stacking them buries the spine under a long list on the screen where the spine is the safety feature. |

---

## Block D

**D1 · Coming back.** A two-day-old session states the age **before** the operator acts,
not after: the freshness strip is already the component for this and it reads stale on
open. An expired session on a read-only screen lands on sign-in and **returns to that
screen**, not the landing — a member who tapped their storage link wants storage.
**Expiring mid-apply** says both things and loses neither: *"Your session ended before
this ran. Nothing was changed, and the plan is still here."* Sign in, come back, the plan
is on screen. That state decides whether an operator trusts the app after a bad week.
Sign-out is confirmed, per A1f, because on a phone it is a mis-tap.

**D2 · Offline.** A **different banner** from degraded, and it must not be reused:
degraded means the API answered badly, offline means nothing answered.
*"No network. What's on screen is what already arrived."* Everything already loaded stays
readable; every action goes inert. **Cold open** says what it would show and why it
cannot. **A mutation attempted offline is refused before it is sent** — *"Nothing was
changed"* — and the control stays armed for when the network returns. **No client-side
queue**, and I agree with the reasoning: the product's argument is that Syndra decides and
records, and a queue in the browser would be a second, invisible ledger. **Reconnected**:
banner leaves, strip goes stale-and-amber, Refresh takes the accent, because the read on
screen is now old by definition.

**D3 · Installed.** Icon from the arch and orb at 512 / 192 / 180, plus a maskable
variant with its safe zone marked — the arch's fade must survive the mask, so the safe
zone is drawn against the fade, not the stroke. **The splash is the sign-in composition at
rest and does not animate**: a splash that starts an animation the app interrupts reads as
a stutter. Standalone chrome gets the status-bar tint and the correct top inset, and the
bottom bar the home-indicator inset. **The install prompt is not a first-load banner** —
it lives in the Account sheet as *Install on this phone* (already drawn in A1f), so it can
be found on purpose.

**D4 · Landscape.** 844 × 390.
- **The bottom bar does not survive.** It costs a fifth of the viewport. In landscape the
  tab bar becomes a **left-edge rail at 64px**, which is the tablet rule arriving early —
  one fewer shape to specify, and the same component.
- **Sheets become centred dialogs** at this height, for the same reason the tablet range
  does: a full-height sheet at 390px tall is a dialog with one rounded edge.
- **Rung 3 in landscape is unavailable, and that is the rule.** The sheet states it:
  *"Turn your phone upright to confirm this."* A consequence sentence, a field and an
  armed red button do not fit in ~150px, and the design's answer to not fitting is not to
  squeeze the consequence out of view — the consequence is the most protected element in
  A5's ranking. Rungs 1 and 2 work in landscape.
- **Sign-in** scales rather than compresses, so at 390px of height the stage's existing
  `min-height: max(100vh, 800px)` takes over and **the composition scrolls**. That is
  already the shipped behaviour and it is the right one: the arch is a fixed-ratio
  composition and cropping it is worse than scrolling it.

---

## Pushback — where I would change Prompt 0

1. **"No tooltips" is right and costs nothing. "No horizontal scroll" costs one screen.**
   A target's page has eight panels; its *tab set* has to scroll horizontally or the tabs
   become a sheet, and a sheet for tabs is a menu for switching panels. I have kept
   horizontally scrolling **tabs** and refused horizontally scrolling **tables**. Worth
   making that distinction explicit in Prompt 0, because they are not the same
   affordance: scrolling tabs hide navigation, scrolling tables hide data.
2. **"No charts, gauges or sparklines" is right, and provider latency is the test case.**
   A round-trip figure in milliseconds is a number an operator acts on; a latency
   sparkline is one they watch. Keep the ban, keep the number.
3. **The `press` movement on destructive controls.** A 50px red button that scales to
   `.97` on touch-down feels responsive and slightly encouraging. I would keep `press`
   everywhere for consistency but drop the scale on the **armed** rung-3 button, leaving
   only the tint. Consistency is worth more than any one control, except on the control
   with no undo.
4. **"Structure never moves in response to data" collides with the add-on rows once.**
   `targetNav` injects a row per registered add-on from deployment configuration, which is
   correct — but a deployment that registers four add-ons makes System six rows, and the
   nav sheet's one-section-open rule starts to strain. Not a change now; a thing to
   revisit at four.

---

## Code findings — replies

1. **Member storage route.** Acknowledged as fixed (`e5a95ec`, `257fb31`). `MEMBER_ROUTES`
   derived from `MEMBER_NAV` is the right shape — the derivation is what stops the next
   row drifting. Nothing needed from design.
2. **Two arch scales — resolved in the design's files, in the code's favour.**
   `globals.css` wins. `mobile/MOBILE.md` and the board's M28 now carry
   `320 / 292 / 141`, inset 24, wordmark 40, stage inset 24, and **only** those six
   tokens — M28 had also moved the orb and the group offset, which that breakpoint does
   not touch. The orb stays 46px at `top: 54`; the group keeps `margin-top: -52px`.
3. **Person page: three tabs, not four.** Draw the three that exist and give Cards **a
   stated seat**, not a drawn tab: a line at the foot of the Access tab reading *"Door
   cards will appear here."* A seat says the shape is known; a fourth tab says it is
   built. The board's M06b is corrected to three tabs when Block B is drawn.
4. **The two missing endpoints.** Agreed the dashed, reason-stated rendering is right, and
   agreed the screens cannot close until they land. Two things design needs from them,
   which are cheap to get right now and expensive later: the dormant listing must return
   **the reason a row cannot be removed**, per row, not a boolean; and per-grant removal
   must return **the residual set** — what the person still holds — because every one of
   B9's four removal sheets states that, and computing it client-side would be a second
   opinion about access.
5. **`sonner` out.** Agreed, with `lib/toast.ts` and `lib/drain-toast.ts`.
   `lib/drain-outcome.ts` stays and only its presentation changes — A2f and A2e are that
   presentation.
6. **The five `title` attributes.** Agreed, they move into row disclosures. The Zitadel-read
   one is the important one: it is the only explanation for a failed read, and per B11 that
   failure now gets its own sentence rather than a hover.

### Two more, found while answering

7. **`MEMBER_NAV`'s doc comment contradicts its own array.** It opens *"Member — two
   destinations, and that is deliberate"* and then defines three leaves. The array is
   right and the comment is stale — it predates the storage row. Worth fixing in the same
   pass as anything else in that file, because the comment is the thing a developer reads
   before trusting the array, and Prompt 0 quotes the array's count.
8. **Nothing in the code names the six indicators as a set the UI must summarise.** A1's
   Go-to bar needs *"how many indicator keys are non-zero"*, which is derivable from
   `GET /api/v1/governance/indicators` on the client. Fine as it stands — flagging it so
   nobody adds a `total` field to that endpoint, which would be the seventh number A1
   exists to avoid.
