# A-13 · Stale data that stays readable, and passwords that settle themselves

Answer to `Q-13-stale-and-self-settling-passwords.md`, 3 Nov 2026. Board: `a13-stale-and-passwords.dc.html` (Final, 3 Nov 2026).

Both parts revise A-09. Part 1 uses WCAG 2 contrast with the tokens in `ui/src/app/globals.css` and A-09's text values (dark `--muted` 0.70, `--faint` 0.56; light 0.78, 0.66). "Worst ground" means the worst of every card surface over the page, with a pressed row (`--tint-3`), a chosen row (`--accent-soft`) and an amber Notice (`--warn-soft`) under the text. The board prints every number. The owner's ruling of 3 Nov on Part 2 (no "Changed" until the add-on confirms; a resend is marked in the record) is folded into §4 and §5.

## 1 · Yes, the accepted set is busy. Keep four, drop one, widen one

**Keep the Notice, the age, the rings and the controls that are off. Drop the amber outline. Say the age once per read, not once per card. Rings keep each dot's own colour. The dim becomes the card receding (§2).**

- **The honest count.** On the stale System › TrueNAS page, the accepted set puts amber in about 15 places:
  - the Notice
  - four amber card outlines
  - four "Last known · 2 hours ago" lines, under two read lines that already say it
  - the rings
  - a reason under every control that's off
  - and a dim over all of it.

  Everything is equally loud, so nothing leads, and the one thing to do ("Look at TrueNAS") gets lost. The board draws it next to the calm version (`?frame=stale-accepted` beside `?frame=stale`).
- **1 · The Notice: keep, as built.** One warn Notice at the top of the content column, `live` (it breathes). It's the only box on the page and the only amber that moves.
  - Title: "TrueNAS isn't answering."
  - Body: "It stopped answering at 09:41, and Syndra keeps checking every 30 seconds. Look at TrueNAS, not at Syndra. Changes for TrueNAS wait until it answers, and nothing is lost. Below is what it held at 09:41."
- **2 · The age: keep, once per read.**
  - The read line says it: ReadFreshness, provisional (amber ring and its sentence).
  - Cards drawn from that read don't repeat it.
  - A card that is its own read says it in its CardHeader note: an amber ring, then "Last known · 09:41", 12.5px, `--warn-text`.
  - On this page that is three places: the page's read line, the account list's read line, and "What TrueNAS reports".
- **3 · The amber outline: drop.**
  - A box means something is wrong, and the Notice is that box: one dysfunction, one Notice. A stale card isn't wrong. It's old.
  - Four amber frames would turn four cards into four warnings, all competing with the Notice.
  - The card's edge still changes, quietly, in §2.
- **4 · Rings: keep, and widen.** Every dot on a card drawn from the last read becomes a ring in its own colour.
  - The colour still says what TrueNAS said: a lime ring for "Pool tank · Online", an amber ring for the disk alert.
  - The ring says it isn't a fresh read (86c).
  - Not every ring turns amber. An amber ring on a healthy pool would claim a problem nobody saw.
  - This is the "dim what isn't text" idea, done with meaning.
- **5 · Controls that need TrueNAS live: off, with one reason per card.**
  - Off: Assign to a person, Reconcile now, Change folder.
  - On: what only queues or records, such as Set a limit, Put on hold, Revoke, Decide, Say what happened, Start draining and Set read-only.
  - Each card says its reason once, in one faint line where its controls are, never under every button:
    - "Assigning waits for a current read of TrueNAS: it ties an account and its files to a person, and there is no undo."
    - "Reconcile now waits too: it reads TrueNAS first."
    - Home folders, as the value's second line: "Change folder waits too: it lists the folders on TrueNAS."
- **6 · The dim:** §2.
- **What stays raised and live.** These are Syndra's own knowledge, current now, so they don't change:
  - Syndra's reach
  - the lifecycle card
  - Waiting on you
  - What it can do
  - Check for differences
  - the setup card

  Because only TrueNAS's cards recede, the reader sees at a glance which parts of the page are memory.

## 2 · The dim: (d), the card recedes and the ink stays

**A card drawn from the last read loses its raised surface. Its ground becomes the page's (`--canvas`), its edge `--line-strong`, with no shadow. Its text stays at full strength. Owner's call (it replaces the 0.80 he leans toward). Recommend (d). (b) is the fallback if he wants the 0.80 itself.**

Worst ground inside a stale card, tints included. Text needs 4.5. Rings and dots need 3:1.

| | Dark: faint · muted · amber words | Light: faint · muted · amber words | Rings |
|---|---|---|---|
| Fresh card, and (a) no dim | 4.90 · 6.72 · 7.52 | 5.18 · 7.67 · 5.27 | dark amber 8.92 · light amber 4.44 |
| 0.80 over the whole card, as the owner leans | **3.72 ✗** · 4.96 · 5.49 | **3.45 ✗** · 4.60 · **3.58 ✗** | |
| (b) marks at 0.80, text full | as (a) | as (a) | dark 6.09 · light amber 3.17, lime 3.42 |
| (c) lifted scope, then 0.80 | 4.76 · 6.45 · 5.49 | 4.83 · 6.59 · 4.57 | |
| **(d) recede** | **5.54 · 7.86 · 9.27** | **4.94 · 7.17 · 4.73** | dark lime 13.0, amber 9.61 · light lime 4.48, amber 3.98 |

- **A plain 0.80 fails three ways.** `--faint` fails in both themes. In light, so do the amber "Last known" words, the very line that says the data is old.
- **(a) No dim.** Clean and accessible, but the owner asked for a visible change, and only words and rings would carry it.
- **(b) Marks at 0.80.**
  - It passes, but it barely shows: a 7px ring at 0.80 is still a 7px ring.
  - Light rings fall to 3.2–3.4, just over the 3:1 floor.
  - The line between text and not-text runs through components (an avatar's initials, a chip's dot, a badge's fill), so code would have to chase every mark.
- **(c) Lifted scope.** If chosen, the exact values are:
  - Dark: `--muted` 0.84, `--faint` 0.68.
  - Light: `--muted` 0.92, `--faint` 0.80, and a darker light `--warn-text`, `color-mix(in srgb, var(--warn-text) 70%, var(--ink))`.

  Not recommended. After the dim, muted and faint land almost where they started (dark L* 69 · 58 against today's 72 · 61), so all the dim visibly does is grey the ink. In light it puts ink at L* 30 next to muted at 36, so titles and sentences read alike. That's a second set of tokens to keep in step, for a loss.
- **(d) Recede (recommended).**
  - It uses only grounds A-09 already measured, so every pair passes in both themes.
  - The change is visible. In light, the white card becomes the lilac page, held by its edge. In dark, the raised grey drops to the near-black page. It's the same step that tells a card from the page today (1.12 light, 1.08 dark).
  - It reads as "no longer raised". That's the right meaning: the card is a record now.
- **Not NotOpenYet.** NotOpenYet is dashed, previewed at 0.45 and inert. A receded card has a solid edge, full-strength text, and controls that still work if they only queue.
- **Motion.** Receding and rising back are a tint of surface and edge (settle, 260ms, the soft curve). Nothing moves. The still form is instant.
  - When TrueNAS answers, the cards rise back by tint, the rings fill, the ages go back to faint, and the Notice fades, then folds.
- **The standing rule changes.**
  - Old: "Stale data is not disabled: it stays readable at 0.62 under its Notice."
  - New: "Stale data is never disabled or dimmed. A card drawn from the last read recedes to the page (canvas ground, line-strong edge), its dots become rings of their own colour, and its age is said once per read, under its Notice."
- **Mappings.** The "Opens …" lines stop dimming. The card isn't receded, because it is mostly Syndra's own mappings. Its note turns amber with the ring: "What each group opens, as last read from TrueNAS's folder permissions at 09:41."

## 3 · Drawn

- **One card, fresh beside stale:** `?frame=card` (dark) and `?frame=card-light`. Both show "What TrueNAS reports". Fresh: raised, filled dots, "All read". Stale: receded, rings, an amber "Last known · 09:41".
- **The whole page:**
  - `?frame=stale`: the calm set, recommended.
  - `?frame=stale-accepted`: the accepted set, for comparison.
  - `?frame=stale-light`: the calm set in light.
- **Phone 390:** `?frame=stale-phone`. The Notice comes first. Cards stack and recede the same way. Row verbs wrap right.
- The specimen on the board draws (a)–(d) and the plain 0.80 in both themes, each with its measured numbers.

## 4 · The member's password change

**Step 1: one quiet resend. The change shows as in progress until the add-on confirms, and only then "Changed". Step 2: an amber line, and the form stays open with what you typed. Step 3: one line in the Storage password row until your next confirmed change, which clears it. This replaces A-09 §5. Its Owner's call (B) no longer applies: nobody needs to check.**

All in second person. TrueNAS is the actor. Each message says what's true and what to do next, and never suggests the member did something wrong.

**Step 1 · The reply is lost, and the page sends again** (`?frame=pw-wait`)
- Change password shows the change as in progress, per A-08: the pending dot breathes in the button's left padding, and the label and width don't change. The field keeps what you typed, masked.
- When the reply is lost, the page sends the same password once more, at once. Nothing on screen changes.
- **No "Changed" until Syndra has a confirmed done from the add-on** (owner, 3 Nov). A lost reply is never shown as success, early or otherwise.
- If no confirmed answer has come 4 seconds after the press, one line opens under the field (Unfold, then tint), muted, with no dot: "Still waiting for TrueNAS. Keep this page open until it answers." Only this page holds the password, so only this page can send it again.
- **Confirmed** (`?frame=pw-cleared`), after either send:
  - a lime dot that lifts and blooms once
  - "Changed" (600), then "TrueNAS has your new password." (muted)
  - the field clears, and the caption reads "Last changed just now. Only for TrueNAS: not your Syndra sign-in or your Google account."
- **Refused** (a clear no from TrueNAS): as built, in red, ending "Nothing was changed." No resend.

**Step 2 · The second send gets no answer either** (`?frame=pw-noanswer`, phone `?frame=pw-phone`)
- The button comes back. The field still holds what you typed, masked and in memory only, so one press sends it again.
- Under the field, appearing still (a failure doesn't lift), with an amber dot and no box:
  - Ink, 14.5px/600: "TrueNAS didn't answer, so your password may not have changed."
  - `--muted`, 14px: "Try again in a minute: press Change password. Sending the same password twice is safe, and TrueNAS then has the one you set."
- "Last changed 3 weeks ago" leaves the caption while this is up, because it may no longer be true. The scope sentence stays.
- If Syndra then calls TrueNAS offline (3 missed checks), the page becomes the offline state already drawn (route-storage `quiet`), and this row reads as in step 3, offline.

**Step 3 · You leave and come back** (`?frame=pw-return`)
- In the Storage password row, above the form, with an amber dot and no box:
  - Ink, 14.5px/600: "Your password change at 09:41 today may not have reached TrueNAS."
  - `--muted`, 14px: "If your new password doesn't work, your old one still does. Set it again below to be sure."
  - Older changes read "on 2 Nov at 18:05".
- The second sentence is true either way. If the new password is turned away, the change never landed, so the old one still works.
- The form is open under it. The caption is the scope sentence only.
- **It clears itself.** Your next change that TrueNAS confirms removes the line: it fades, then its space folds (A-05 §2.6), and "Changed" shows as usual. Nothing else clears it, and nobody else needs to.
- **TrueNAS offline when you come back** (`?frame=pw-offline`). The blocked sentence and the line become one:
  - Ink: "Your password change at 09:41 today may not have reached TrueNAS."
  - Muted: "You can set it again here once TrueNAS is back. If your new password doesn't work then, your old one still does."
  - The form waits, as the offline state rules.
- Never "Makerspace staff are checking". Nobody is, and nobody needs to.
- The status beside TrueNAS keeps its own colour. The account and the files are fine.
- The first visit's Set password (the violet step panel) follows the same three steps, in the panel.

**In the record** (owner, 3 Nov; `?frame=activity`)
- Members have no history page. Their record is the password row itself.
- The operator sees each change on the person's **Activity** tab, and the same lines in **Audit**. One row per change the member made. A resend is never a second change.
- The row shows what happened, who did it, and the time. A faint line under it lists the sends:
  - Confirmed on the second send: "Storage password changed on TrueNAS — by Ada Lovelace". Faint: "Sent at 18:05:03 with no answer. Sent again automatically at 18:05:15, and TrueNAS confirmed that one."
  - Never confirmed: "Storage password change on TrueNAS, not confirmed — by Ada Lovelace". Faint: "Sent at 09:41:02 with no answer. Sent again automatically at 09:41:14 with no answer. Settled by her change at 11:52, which TrueNAS confirmed."
  - The change that settles it: "Storage password changed on TrueNAS — by Ada Lovelace". Faint: "TrueNAS confirmed it at once. It also settles her change at 09:41."
- Times show seconds only on these send lines, where the gap between two sends is the point.
- Audit also offers the operation reference for each send, one tap away, as references always are.

## 5 · The operator's side: confirmed

**Waiting on you keeps only changes nobody else can settle. In practice, that means deletes (`account.purge`).** Drawn: `?frame=waiting`.

- **`account.purge`** stays exactly as A-09 §3–4 drew it: the row, Say what happened, the dialog and both results.
  - The dialog title for this kind is "Did TrueNAS delete Grace Hopper's account and files?"
- **`password.set`** is dropped. It never reaches Waiting on you, Home's "Not confirmed" block, the chip or the lede. The member settles it by setting the password again (§4).
  - If the member never comes back, it stays unconfirmed in Activity and Audit. Nothing waits on anyone.
- **`password.rotate`** (part of a revoke) is dropped from Waiting on you.
  - Syndra generated that password, so it sends a newly generated one by itself until TrueNAS confirms. Either way, the old password stops working.
  - Until then, the revoke stays in Unfinished revocations, where unfinished revokes already wait.
  - Each send is marked in the record the same way as §4.
- **`account.adopt`, `account.release` and `setting.put`** are still settled by Syndra from a read (A-09 §3).
- The "how to tell" table in A-09 §4 loses both password rows.
- Home's "Not confirmed" block, the chip and the lede count deletes only.

## For code

**Part 1 · Stale**
- Remove `opacity-[0.62]` from `SystemHealth` in `TargetOverview.tsx`, and `opacity: dim ? 0.62 : 1` from `MappingManagement.tsx`.
- **`Card` gains `stale`:** `bg-canvas`, `border-line-strong`, no shadow, and the surface changes by tint. One place, for every receded card.
- **`StatusDot` gains `ring`:** a 1.5px border in the tone's colour, no fill. A stale card's dots pass it. ReadFreshness already draws its own ring.
- **Card notes for a card that is its own read**, when it isn't current: an amber ring plus "Last known · 09:41" (`SystemHealth`). `OpensNote` turns `--warn-text` with the ring.
- **Cards that recede on System › TrueNAS:** What TrueNAS reports, Accounts Syndra manages, Not made by Syndra, Lost their reason. Nothing else does.
- One reason line per card for controls that are off. Use Button `reason` only where a card has a single control.
- Apply the same three marks to any other card shown as last known, such as pages under the DegradedBanner. Confirm the list.
- Change the standing rule in the canvas `CLAUDE.md` and in `RULES.md` (§2).

**Part 2 · Passwords**
- **`PasswordForm`:**
  - Keep the value in memory until a final answer.
  - On a lost reply (network error, timeout, or `202` with an indeterminate status), send again once, at once, with the same value.
  - Clear the field only on a confirmed `set`.
  - Show "Changed" only on a confirmed `set` (owner).
  - Keep the button pending, label and width unchanged, across both sends.
  - Open the slow line at 4 seconds (one constant).
  - Confirm the total wait is bounded by the add-on call's timeout.
- Tell a refusal (a clear answer, no resend) apart from a lost reply (resend). Confirm the add-on's outcomes map that way.
- **Rate limit.** The automatic resend must not count against "You have changed this recently", or the limiter would answer the resend.
- The handler comment "an operation carrying a secret is never auto-retried" changes. The page sends the member's own value again, once. The server still never retries a member's password by itself, because it holds no copy.
- **Fields on the operation and its audit row:**
  - `attempt` (1, 2)
  - `retry_of` (the first send's operation id)
  - `retried_by` (`"automatic"`; a member's own later change is not a retry)
  - each send's `sent_at` and outcome
  - `settled_by` (the later confirmed operation that settled it) and `settled_at`

  Activity and Audit read one change from these, with both sends and which one TrueNAS confirmed.
- **Superseding.** A confirmed `password.set` settles every earlier unconfirmed `password.set` for the same person and system, records `settled_by`, and writes the "settles her change at 09:41" line.
- **`GET /me/targets`.** Add `password_change: { state: "unconfirmed", at }`, present only while the person's latest `password.set` on that system is unconfirmed. This replaces A-09's `"unconfirmed" | "not_done"`.
- Exclude `password.set` and `password.rotate` from Waiting on you, Home's "Not confirmed" block and the governance counts.
- **`password.rotate`.** Syndra sends a newly generated value again until TrueNAS confirms, with the same `attempt` / `retry_of` fields. While it is unconfirmed, the revoke shows in Unfinished revocations. Confirm the retry schedule.

**Design system.** `Card stale` and `StatusDot ring` enter the design system. Everything else on the board is already in it (Notice, ReadFreshness, PaginationRow, Button pending, Input, CopyableValue, Tabs, CardRow).
