# FINAL · what to build, as of 23 Oct 2026

This is the index. It says which choice won everywhere there was more than one, which board and frame to build each screen from, and which motion each surface uses. Where any older file disagrees with this one, this one wins.

## 0 · How to read the package

1. **`CLAUDE.md`** (project root): the standing rules. Build to them wherever a board is silent.
2. **This file.**
3. **The answers, newest first.** Each one overrides the ones before it:
   - `from-design/A-07-makerspace-settings.md` (23 Oct)
   - `A-06` (17 Oct)
   - `A-05` motion (17 Oct)
   - `A-04` (17 Oct)
   - `A-03` (16 Oct)
   - `A-02`
   - `A-01` (14 Oct)
4. **`BUILD-LIST.md`, `00-CONSENSUS.md`, `verdicts.md`**: the Phase 3 ledger. Where they disagree with an answer, the answer is later.
5. **The boards** (§3). Open one with `?frame=<name>` for a single frame at true width.

**Precedence.**
- A dated answer beats an undated board. A later date beats an earlier one.
- Every current board's header now ends **"Final, 23 Oct 2026"**. Every board you must not build from ends, in red, **"Do not build: superseded by …"**.
- Where a board holds several turns or options, only the one marked *chosen* (or, on a "newest turn first" board, the top turn) is final.

**Where the boards live.**
- The project root holds every board.
- `handoff/2026-10-reconcile/design/` holds a copy of every current board, beside its `_ds/` and `support.js`.
- The two sets are identical.

## 1 · The final choices

Each line is the choice that stands, the date it was settled, and where it's drawn.

**Shell and navigation**
- **Rail, Advanced (N18 3a, 13 Oct; A-07, 23 Oct):**
  - Home · People · Access (Projects, Roles, Apps) · Requests · Bundles · Automatic rules · Insights
  - Changes: Pending changes · Change history · Roles that wait
  - Review: Drift · Unfinished revocations · Expiring access · Holds due · Audit
  - System: Zitadel · Connected systems · {each target} · **Service accounts** · Incoming events
  - **Makerspace settings**, last of all.
  - This is `ui/src/lib/nav.ts` plus Service accounts (A-07 §26). The boards' rail is the design system's Sidebar; its bundle copy in this project carries a local patch with exactly this tree, so every frame now shows it. Republishing the design system from `ui` replaces the patch.
- **Rail, Basic:** Home · People · Access · Requests. **Member:** My access · Requests · Network storage.
- **The drift sound** lives in the Account sheet, "On this device". Roles that wait no longer mentions it (A-07 §1).
- **Waits-for-you mark (N18 2c):** the half-filled dot, with the words "Waits for you" where a row has room.

**Home**
- Layout **1c** (2 Oct). The headline counts blocks (#31).
- **Every block holds at most 5 rows (3 on a phone)**, newest or soonest first, then "and N more →" to the full list, filtered (A-07 **32a**).
- "New people without a bundle" opens People filtered to no bundle, where Select's verb is Give a bundle.
- New block in When you're ready: **"Service accounts to look at"**, for one with no owner, or one not used in 90 days (A-07 §31).
- The Makerspace strip has three tiles. People reads "12 whose access ends in 14 days".

**One window for ending access:** **14 days, with FilterPills 14 days | 30 days** (A-04 **4a**). This holds on Expiring access, Home's tile, the rail and Insights' Ending soon (now two columns, 14 and 30). Never 7, 60 or 90.

**Asking and giving (A-04, A-06)**
- One RolePicker in single choice. The note seat says what the person already has.
- **Ask to keep it** extends an end. Approving it runs Extend.
- Every dialog ends on its own result (96b): a dot, one word, one sentence, a faint line, Close.

**Words**
- **Revoke / Revoked** (never "taken away").
- **Put on hold** (never "Place a hold").
- **Check for differences** (never "Reconciliation").
- **Ends 2 Aug / Ended 2 Aug / until 2 Aug** (never "expires").
- **Waiting for you to send →** (never "waiting to be sent").
- **Will wait for you to send**, for a plan row.
- **People** (never holders, members or accounts as a count).
- **Retire**, for a bundle or a rule.
- **When Zitadel isn't answering** (A-07 §8): "Anything you give or revoke now waits until Zitadel answers."
  - "Paused" is only a person's choice.
  - Syndra's own breaker is "Waiting to retry".
  - Offline means 3 missed checks.

**Degraded (N23, A-07 §9)**
- The last known data, with its age; never sample data.
- Drift's decisions are off.
- A person's page, Requests and Expiring stay on, and say before acting that the change will wait.

**A name Syndra can't read (A-07 §7):** "Unknown account" or "Unknown project", with the id beside it in faint mono (CopyableValue), **in both views**.

**Makerspace settings (A-07 §1–3)**
- `/settings`: one card, Term dates.
- Edit is the dialog **2b**: Modal lg, one row per term, with First day and Last day.
- Defaults: **Monsoon 1 Aug–31 Dec, Spring 1 Jan–31 May, Summer 1 Jun–31 Jul**.
- Trends shows the term it's reading, with "Set in Makerspace settings →".

**Service accounts (A-07 §26–31)**
- Their own leaf, **26b**: System › Service accounts.
- An operator marks a human-type account once, in its page's Kind row. Machine accounts are service accounts automatically.
- They leave every people flow and stay wherever access is checked.

**Lists**
- Sort arrow **↑** for the column's natural order, turned over on reverse.
- Section chips stay visible while selecting (A-07 §16).
- Revocations are newest first.
- A person's page row that ends within 14 days offers **Extend** (A-07 §19).

**What the API can't carry yet (A-07 §21–25)**
- Revocations: "Tried 3 times · Tries again by itself".
- Systems: "Last answered" (the API grows this). No "Configured" and no product kind.
- TrueNAS accounts: sort by Person and Since, with no group.
- Events: "Syndra chose not to act".
- Zitadel: "Answering, and sees 5 projects", two checks only (Reachable, Projects).

**Motion:** A-05 and the **Motion v2** board. Six movements, the 10px cap, one loop. §4 maps every surface.

**Benched, not built**
- `route-additions` (N24–N27).
- The Zitadel key guide dialog (2c, A-07 §35).

## 2 · What A-07 adds to the build

Frontend:
1. **Makerspace settings**: as built, with these changes:
   - Drop the header note.
   - Show "· This term" or "· Next term" faint after the dates.
   - The dialog becomes rows (2b) in Modal lg, with "First day" and "Last day" and an outline Cancel.
   - Phone: Name, then First day and Last day side by side, then Remove right.
2. **Service accounts**:
   - Nav leaf `/system/service-accounts` and its list: Account (with kind) · Purpose · Owner · Last used · Roles 110.
   - A page: This account (Purpose, Owner, Kind, Last used), then What it holds.
   - The Mark as a service account dialog, and Mark as a person to undo it.
   - Removed from People, New people without a bundle, onboarding gaps, the welcome bundle, asking, and People counts.
   - "Service account" in the description seat on Drift, Expiring, Holds and the TrueNAS accounts table.
3. **Home**:
   - The 5 / 3 row cap and "and N more →" on every block.
   - The Service accounts to look at block.
   - The People tile reads "in 14 days".
4. **People**: honours `?bundle=none` as an in-force FilterChip, sorted by Joined, newest first. Select's verb is Give a bundle.
5. **Degraded copy**: the one sentence everywhere.
   - Remove the `disabled` from a person's page, Requests, Expiring and Home's row buttons.
   - Their dialogs' before-line says the change will wait.
6. **Unknown account or project**: show the id in Basic too.
7. **A person's page**: Extend on rows ending within 14 days.
8. **Expiring**:
   - FilterPills 14 | 30 in the FilterBar's pill seat, replacing the Filters panel.
   - Chips stay visible in Select mode.
   - Lapse dialog: Cancel outline, first; Close on the result.
9. **Insights**:
   - Ending soon shows two columns, 14 days and 30 days ↑.
   - Trends reads "revoked", with the term line and its link.
10. **Copy fixes**:
    - "Put on hold" and "Check for differences".
    - "Revoked" for the member.
    - "Syndra is waiting to retry TrueNAS." on Holds.
    - "Tried N times · Tries again by itself" on Revocations.
    - "No answer yet" for a system that hasn't answered.
    - Zitadel's two checks.

Backend: grow "Last answered" for each connected system (A-07 §22). Optional, later: the next retry time on Revocations; Zitadel's last-used time for keys and sign-ins, which the 90-day stale check needs.

## 3 · Board by board

What to build from on each board, and the answers that last touched it. Frames are named as on the board (`?frame=`).

**Across the product**
- `Controls`: Checkbox and Radio, ChoiceRow, AnchoredPanel, **4a**, **7.1a**, the verbs, the waiting words, the arrows, destructive size. A-04.
- `Cohesion`: the 2 Oct walkthrough. Its "Expires" examples now read "Ends".
- `route-states`: the four states, every list. Degraded is last known (N23).
- `route-mobile`: the phone and tablet rules, 87c (N20).
- `route-login`: the login. Its two ambient loops are allowed, outside the product.
- `Motion`: the picks 1b, 2a, 3a, 4a, 5b, 6a, 7a and 8a–8d. Read with Motion v2.
- `Motion v2`: **the motion spec**, S1–S18 and the token table. A-05.
- `shell-account`: the Account sheet, proposed-both (A-03): Network storage, My access, Appearance and the drift sound on this device.

**Operator**
- `route-home`: default (1c), empty, loading, error, degraded, phone, plus **long-a** and long-phone (32a). 1a, 1b, 32b and 32c are history.
- `route-users`: every frame; bulk-a is the Give a bundle flow that "and N more →" lands in.
- `route-users-id`: default (`person` opens it), activity, loading, error, empty, degraded, phone.
- `route-projects`, `route-projects-id`, `route-roles`, `route-applications`, `route-applications-id`: every frame.
- `route-bundles-v2`: the top turn first; earlier turns only where the top turn doesn't redraw them. VersionBand per A-06 §6.
- `route-requests`: every frame.
- `route-asking`: every frame, including keep-sent, keep-mine and op-keep-done. Asking and giving everywhere use it.
- `flow-cancel-queued`: every frame. The pushed undo step.
- `route-drift`, `route-expiring` (now with window-30), `route-holds`, `route-revocations`, `route-pending`: every frame.
- `route-automation-settings-v5`: the Roles that wait page (`/changes/roles-that-wait`), the frames under Turn 2's picks.
- `route-insights`: every frame. Ending soon is 14 | 30, Trends says revoked and links to Makerspace settings, and the what-if thread is A-06 §7.
- `route-systems`, `route-system-targets-id`, `route-system-targets-id-mappings`: every frame. A-01, A-02 and A-03 for TrueNAS states, places, home folders and limits.
- `route-zitadel`: every band. Two checks.
- `route-events`: every frame. Rows whose reason isn't recorded say "Syndra chose not to act".
- **`route-settings`** (new): default, never-set, loading, error, **2b** (edit) and its states edit-error, edit-refused, edit-twelve, edit-saved; phone, phone-edit, phone-error, phone-saved. 2a (edit-groups) is history.
- **`route-service-accounts`** (new): **26b** (sa-list), sa-page, sa-mark, sa-marked, sa-phone. 26a (sa-tab) is history.

**Member**
- `route-me`: the top turn. "Ask to keep it" is the row button; Recent changes says "Revoked".
- `route-storage`: every frame.

**Do not build from**
- `route-bundles` → `route-bundles-v2`.
- `route-graph` and `route-graph-v2` → `route-insights`.
- `route-automation-settings` v1–v4 → v5.
- `explore-*` → their route boards.
- `route-additions`: benched.
- The August boards (`Syndra IA`, `Syndra Mobile*`, `Syndra Brand`, `Syndra Target*`, `Syndra Mapping Screen`, `Syndra Member Storage`, `Syndra Contradictions`, `Syndra Handback`, `Source`): history only.

**Proposed components** (`proposed/`, and `handoff/2026-10-reconcile/proposed/`), each built once and used everywhere:
- Notice, NextStep, SelectionSeat, DegradedBanner, NotOpenYet
- AnchoredPanel, ChoiceRow, CancelQueuedStep
- ActionOutcome-seat (with its block form)
- StatCard-href, VersionBand, ConnectionGuide, Expiry-sweep-run, Deliberate-later
- Implementation notes: Mapping-implementation-notes, Zitadel-code-notes.
- WaitsMark and RolePicker are named in BUILD-LIST.

## 4 · Motion, everywhere the same

The spec is `A-05-motion.md` and the `Motion v2` board (S1–S18 with still forms). The earlier picks are on `Motion` (5b, 6a, 8a–8d). **Every place that moves uses one of the patterns below, and only these.**

### Tokens (in `_ds_bundle.css`; the last five are new, for /design-sync)
- **Tint:** 120ms, `--ease-tint`. Colour, opacity, words crossfading.
- **Press:** 180ms, `--ease-press`. The 3% press on controls only.
- **Settle:** 260ms, `--ease-settle`. Height, position, an indicator gliding, a sheet rising.
- **Arrive:** 420ms, `--ease-arrive`, 8px. `--arrive-step` 40ms between rows, capped at 5 (row 6 onwards arrives with row 5).
- **Flash:** 900ms, `--ease-flash`. The bloom, a 9px ring of the dot's own colour, once.
- **Breathe:** 2400ms, `--ease-breathe`. **The only loop in the product.**
- **Glide:** `--duration-glide` 520ms, on settle's curve. Scrolling only.
- **`--travel-cap: 10px`.** Nothing in the page travels further. A phone sheet from its edge and a control's own indicator are outside the cap.

Light leads shape by 40ms. Closing fades first, then folds, and is quicker. Counts never animate. Hover is colour only. Rows and cards never scale.

### Patterns, and where each one appears

| Pattern | Motion | Where it appears |
|---|---|---|
| A screen's first paint | **S11**: the shell never moves; the header tints in, then cards arrive (stagger 40ms, cap 5). Only when you navigate there; never on back, refresh, filter or sort. No route transitions. | Every route |
| Loading → content | **S7**: skeleton only after 200ms; skeleton fades, then rows arrive. No crossfade. | Every list and card that reads |
| Hover, press, focus | **S8** and **S9**: colour only on hover; 3% press on buttons, chips, nav items and segments; rows and cards darken one step (`--surface-2`) when pressed; the focus ring appears at once and fades on tint | Everywhere |
| Refresh, filter, search, sort | **A-05 §2.6, §2.10**: rows keyed by id; rows that stay hold still; new rows tint in; leaving rows tint out, then the gap settles. A re-sort fades the list out and back in (120ms each way); rows never travel. | People, Projects, Roles, Requests, Drift, Expiring, Holds, Revocations, Pending, Events, Service accounts, TrueNAS accounts, Bundles, Insights lists |
| Folds and long lists | **Motion 8a–8d** with A-06 §5: fold opens on settle with contents fading in 120ms behind; closing fades first. Show 25 more: only new rows arrive. Back to 10: glide to the heading, trim out of sight. | Drift's two directions, Expiring sections, Pending batches, a person's projects, a bundle's versions, the RolePicker's folds |
| Jump to a section | **8d**: glide (520ms), then the landing heading flashes once. A folded section opens on the way. | Read-line chips on Drift, Expiring, a person's page, a bundle's People |
| Floating section pill | **8d**: drops in 10px on settle once the read line scrolls away; leaves on tint | Same pages as the chips |
| A status changing in place | **S4**: words crossfade on tint in a seat sized to the taller word; the dot changes colour where it sits; **blooms only for something that went well, because of you, on screen** | Requests' decision seat, Expiring (Extended, Recorded), Drift decisions, Holds (Lifted), a person's rows, Revocations, Pending rows, Service accounts' owner seat |
| A row leaving | **S5**: changed by someone else, the row keeps its seat and says so; on the next read it fades, then its gap settles; the gap waits while the pointer or focus is in the list | Pending changes, Requests, Home blocks, Expiring, Drift, People (a person marked as a service account leaves on the next read) |
| A Home block emptied while you're there | **Motion 5b**: its outcome shows, then it folds. The next row's buttons work during the beat. | Home |
| A dialog | **S1**: scrim tints, then 40ms later the sheet rises 8px from 98% on settle; closing fades the sheet and sinks it 4px, then the scrim | Every Modal: asking, giving, keep, extend, lapse, rehearsal, publish, retire, term dates, mark as a service account, phone Filters |
| A step pushed inside a sheet | **S2**: the form fades, the sheet's height settles to the step; the sheet doesn't move; Keep it reverses | Cancel a queued change (flow-cancel-queued), any confirm inside a dialog |
| A phone sheet | **S3**: from the bottom edge on settle; follows the finger 1:1; let go short of a third and it settles back; while busy, 7a (15% resistance, capped at 10px, never closes) | Every dialog below tablet, Filters sheet, the AnchoredPanel you act in |
| A dialog's result | **S6**: success rises 6px and its dot blooms once (`role="status"`); failure appears still on tint, steady danger dot (`role="alert"`) | Every dialog's end: Asked, Given, Extended, Saved, Marked, Not sent, Didn't save |
| A field error | Failure: appears still on tint under its field; goes on tint when the field is edited | Term dates, Ask's Why, every form |
| Adding or removing a part in a form | New row tints in, then its height settles; a removed row fades first, then its gap settles | Term dates' rows, bundle roles, mapping groups |
| Tabs, Segmented, Select | **S10**: the fill or underline glides on settle and labels tint; tab content swaps with no slide; Select is native and still | A person's page tabs, Insights tabs, Period on Trends, How long, Home on/off |
| Popover | AnchoredPanel: opens on settle rising 4px, closes on tint. Tooltips (the WaitsMark) tint only. | Filters panel (desktop), WaitsMark, the account menu |
| Role picker | **S15**: the seat fills on tint; its dot or tick grows from 60% on settle, 40ms behind; the chip tints into its seat, rising 4px; the fold count tints | route-asking, Give a role, bundle editor, every RolePicker |
| Typing the name to confirm | **S16**: the border warms toward danger, letter by letter, to 70%; a wrong letter returns it to neutral, with no shake; the confirm tints live on the last letter | Retire a bundle or rule, ConfirmByTyping everywhere |
| Refreshed | **S17**: Refresh presses and shows its busy state; the old age stays until the new read lands, then "just now" crossfades in and the freshness dot blooms once. Background reads never bloom. | Every ReadFreshness with Refresh; Check again on Systems; Check Zitadel again |
| Copied | **S18**: crossfade to lime dot and "Copied" in its own seat, bloom once, back to Copy after 900ms | CopyableValue: support references, the Zitadel key, an unknown id, a mapping's group |
| Everything sent | **S13**: sent rows fade, the list folds, one line rises 6px ("Everything has reached Zitadel"), its dot blooms, Send fades out | Pending changes |
| Daylight | **S14**: every colour token crossfades over 260ms with tint's curve, and nothing moves; the toggle's thumb glides | The theme toggle in the Account sheet |
| Breathe | The amber dot of a Notice for something stale or waiting (Zitadel isn't answering, a system waiting to retry, last known data), and Insights' node while an answer is computed. Nothing else loops. | Every amber Notice; Insights |
| Insights answer | Lights travel once along the answer, then rest. Hovering or focusing a named person or role lights its line (tint). No halo, no travelling lights at rest. | route-insights |

**Never:** a shake; a bounce; a count that ticks; a row that scales; a route transition; a loop other than breathe; a bloom for something the reader didn't do; arrive replaying on refresh, filter or sort; anything travelling more than 10px in the page.

**Reduced motion:**
- Every movement has a still form (Motion v2's token table).
- Press survives. Glides become jumps.
- Flash becomes colour only, with no bloom.
- Breathe rests at full strength.
- Sheets and dialogs appear and leave together with their scrim.
- 7a doesn't move; its sentence tints.
