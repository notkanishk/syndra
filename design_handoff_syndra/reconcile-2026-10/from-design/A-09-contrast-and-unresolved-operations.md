# A-09 · Quiet text, and changes nobody can vouch for

Answer to `Q-09-contrast-and-unresolved-operations.md`, 3 Nov 2026. Board: `a09-quiet-text-and-unconfirmed.dc.html` (Final, 3 Nov 2026).

Part 1 measures ink at each alpha over the real grounds in `ui/src/app/globals.css`, using WCAG 2 relative luminance. "Every ground" here means `--ground`, `--canvas`, `--surface-0/1/2` and `--rail`, plus the three tints that sit under text over each of them: `--tint-3` (a pressed row), `--accent-soft` (a chosen row or selection) and `--warn-soft` (an amber Notice). The board prints every ratio.

## 1 · Quiet text: A, with new values and B's merge

**Dark: `--muted` 0.70, `--faint` 0.56, `--label: var(--faint)`. Light: `--muted` 0.78, `--faint` 0.66, `--label: var(--faint)`.**

| | Dark, plain grounds | Dark, worst tint | Light, canvas · white | Light, worst tint |
|---|---|---|---|---|
| `--muted` | 8.09–8.94 | 6.72 | 7.89 · 8.43 | 7.17 |
| `--faint` / `--label` | 5.66–5.97 | 4.90 | 5.28 · 5.53 | 4.94 |

- **Why not A's values.** They clear the plain grounds but fail on a row under the pointer. Dark `--faint` 0.50 measures 4.23 on a pressed `--surface-2` row. Light 0.62 and 0.61 measure 4.38 and 4.26 on a pressed canvas row. The lowest alpha that clears every ground, tints included, is 0.53 in dark and 0.63 in light. These values sit a little above that.
- **Why not C.** It has the same failures on tinted rows, and it leaves two nearly equal steps in light.
- **B's merge, without a decoration token.**
  - `--label` is only ever used for text: `type-label`, `type-nav-group` and 23 `text-label` uses.
  - Quiet things that aren't text already use `--line` and `--line-strong` (seat rings, dividers), or `--faint` and `--muted` (neutral dots, the Waits-for-you mark). After the lift, all of them clear 3:1.
  - So nothing needs the old 0.34/0.40 step. A new token would have no user.
- **Keep the name `--label`, set to `var(--faint)`** in both themes. The job keeps its name, the call sites don't change, and the two can split later if a reason ever turns up.
- **Muted moves furthest.** In dark it gets brighter (L* 64 → 72); in light it gets darker (L* 50 → 33). Ledes read closer to ink. That's the price of keeping a clear step above faint. Muted is still 23 L* below ink.
- **Stale data dimmed to 0.62 can't keep the rule.** Under it, `--faint` drops to 3.0 in dark and 2.6 in light, and `--muted` to 4.0 and 3.2. Built today in two places: `TargetOverview` (the card) and `MappingManagement`.
  - **Owner's call** (it changes a standing rule). Recommend: **stale data stops dimming.** The amber Notice and the dated read line already say the data is old. It stays readable at full strength, as the rule intends.
  - The alternative is to keep the dim and accept text that fails AA under every stale Notice. Not recommended.
- **NotOpenYet's 0.45 preview stays.** It's inert and not there to be read, which is the inactive-component exception.

## 2 · Hierarchy when the steps get closer

**Two steps of colour, about 12 L* apart in both themes. Case and weight carry the label. Nothing shrinks.**

- The steps: dark ink 96 · muted 72 · faint 61. Light ink 10 · muted 33 · faint 44.
- **`--muted`**: sentences. Ledes, notes and secondary lines, 13.5–15px, regular weight.
- **`--faint`**: times, ends and captions, 12.5–13.5px, regular weight. Faint text is never bold; weight belongs to the label.
- **`--label`**: 12.5px, weight 600, capitals, 0.1em tracking (0.13em for `type-nav-group`), as built. It is now as strong as faint text, but its case and tracking make it unmistakable. It reads as structure, never as a timestamp.
- **Floor: 12.5px**, as built. Size never does the work colour used to do.
- **Text drawn with its own alpha** (`text-ink/NN`, opacity on a block that holds text) follows the same floors: at least 0.56 in dark and 0.66 in light, checked against the tinted grounds.

## 3 · Where it lives: Waiting on you, on the system's page

**Not a, b or c as asked. The rows go in the region the system page already has for "a person must decide": Waiting on you.** Drawn: `?frame=waiting`, with option a beside it for comparison (`?frame=option-a`).

- **Why not a, a Notice.**
  - A Notice says that what you're looking at is unreliable (stale, failed, offline), and an amber one breathes. Here the page is current and TrueNAS is answering.
  - A Notice that opens a list would also be a second place for decisions, beside a region built for them.
- **Why not b, a section of its own.** The page has four regions in a fixed order. Waiting on you already exists, keeps its seat, has its chip on the read line and is counted in the lede.
- **Why not c, Review.** That's a new leaf for something rare. The operator goes to the system to check it, and a Review leaf would only send them there.
- **In the region:**
  - Unconfirmed changes come first, above findings, because a person may be waiting on one.
  - Newest first, by the standing default.
  - The region's lede gains one clause: "…, or a change TrueNAS never answered."
  - The chip ("Waiting on you 3", amber dot) and the page lede ("Three things are waiting on you.") count them with everything else.
- **Home.** What needs you gets a block like Merge findings:
  - Card title "Not confirmed", with its count, in amber.
  - Caption: "Changes sent to a connected system that never answered. They are kept with each system."
  - One row per system: "TrueNAS · 2 changes · Open TrueNAS →", linking to `#waiting`.
  - It counts toward "N things need you". "Nothing needs you" can't show while one is open.
- **No rows to confirm: nothing shows.** No card, no zero, no line (`?frame=empty`). The region shows whatever else is waiting. With nothing at all, it reads "Nothing is waiting on you.", as today.
- **Only rows a person must settle appear here.**
  - A record that was never claimed was never sent. Syndra knows it didn't happen and settles it itself.
  - Adopt, release and the home-folder setting are visible in the next read of TrueNAS. Syndra should settle those itself (For code).
  - In practice the operator sees passwords and purges, which matches what the question expects.

## 4 · The row, and how it's settled

**What was asked, as a sentence, then an amber "Not confirmed". When it was sent, who asked, and one verb: Say what happened.**

**The row** (`?frame=waiting`), in the region's row form:
- Line 1, 15px/600: what was asked, with the person linked. "Set **Charlotte Flair**'s storage password". Beside it, an amber dot and "Not confirmed" (13.5px, 600, `--warn-text`), so the sentence can't be read as a to-do.
- Line 2, `--muted`: "Sent to TrueNAS at 09:41 today, 2 hours ago. No answer came back, so it may or may not have happened." Older ones read "on 2 Nov at 18:05".
- Line 3, `--faint`: "Asked by Charlotte Flair". When Syndra asked by itself: "Asked by Syndra, bringing accounts in line".
- Seat: **Say what happened**, outline sm, nowrap, right. Its aria-label repeats the sentence.
- No id on the row. The id belongs in Audit.

**Each kind, worded** (the add-on's operation names):

| Operation | What was asked | How to tell, in the dialog |
|---|---|---|
| `password.set` | "Set Charlotte Flair's storage password" | "TrueNAS can't show a password. Ask Charlotte Flair to sign in to storage with the new one: if TrueNAS lets Charlotte Flair in, it happened." Then: "Can't reach Charlotte Flair? Choose It didn't happen. Setting a storage password again is safe once someone has looked." |
| `password.rotate` | "Replace Sami Zayn's storage password, so the old one stops working" | "TrueNAS can't show a password. If you can't tell, choose It didn't happen: the revoke stays unfinished, and Syndra replaces the password again." |
| `account.purge` | "Delete Sami Zayn's account and files on TrueNAS" | "Read TrueNAS again. If sami.zayn is gone from Accounts on TrueNAS, it happened. Syndra won't send a delete again by itself." |
| `account.adopt` | "Assign kiosk to Roman Reigns" | "If kiosk is under Accounts Syndra manages, it happened." (Syndra should settle this itself from a read.) |
| `account.release` | "Let go of Roman Reigns's account on TrueNAS, keeping it" | "If roman.reigns is under Not made by Syndra, it happened." (Syndra should settle this itself.) |
| `setting.put` (`home_root`) | "Change where home folders go to main › homes" | "Home folders, in the setup card above, shows what TrueNAS holds now." (Syndra should settle this itself.) |

**The dialog** (`?frame=settle`), Modal md:
- **Title** as the question: "Did TrueNAS set Charlotte Flair's storage password?"
- **Lede:** "Syndra sent it at 09:41 today, asked by Charlotte Flair, and no answer came back. Until you say, Syndra treats it as neither done nor failed."
- **"How to tell"** (`type-label`), then the sentence for its kind from the table, in ink.
- **Two single-choice ChoiceRows. Nothing is chosen at first.**
  - **It happened**: "TrueNAS has the new password. Charlotte Flair's storage page says it was changed today."
  - **It didn't happen**: "TrueNAS still has the old password. Charlotte Flair's storage page asks for a new one."
  - For other kinds the second line says the state of the world in the same way.
- **Why two choices inside one verb, not two row buttons.** "It applied" and "It didn't apply" side by side on a row are opposite claims one tap apart. The dialog puts "how to tell" in front of the choice, so the operator looks before saying. "Happened" is also plainer than "applied", and keeps "Applied" away from something nobody has confirmed.
- **Footer note** (the before-line): "Nothing is sent to TrueNAS. This records what you found, with your name."
- **Buttons:** Cancel (outline), then the accent confirm, which names what it records:
  - "Record it as done", or "Record it as not done".
  - Before a choice: "Record what happened", disabled.

**The result, in the sheet** (96b):
- **Not done** (`?frame=settled`): neutral dot, appears still. "You recorded that TrueNAS didn't set Charlotte Flair's storage password." Faint: "Charlotte Flair's storage page now asks for a new one. Audit keeps your answer, with your name." Close.
- **Done** (`?frame=settled-done`): lime dot that lifts and blooms once. "TrueNAS has Charlotte Flair's new storage password." Faint: "It leaves Waiting on you. Audit keeps your answer, with your name." Close.
- The row leaves on the next read: it tints out and the gap settles (A-05 §2.6). Reopened, the dialog starts clean.

**States:**
- Degraded, or TrueNAS offline: the rows stay, and Say what happened still works. The answer is about something the operator checked, not something Syndra sends.
- A loading or error frame shows no rows and no count.
- Phone (`?frame=phone`): the row wraps (sentence, Not confirmed, when, who). The verb stays right. The dialog is the md sheet from the bottom edge.

## 5 · The member's side

**Owner's call. Recommend B: tell them, in one line where the password lives** (`?frame=member`).

- **A · Say nothing**, as today. The member tries the new password; if TrueNAS refuses it, they don't know why or which password to use.
- **B · One line in the Storage password row (recommended).**
  - It takes the form's place, with an amber dot and no box.
  - Ink, 14.5px/600: "Syndra couldn't confirm your password change at 09:41 today."
  - `--muted`, 14px: "TrueNAS didn't answer, so it may have your new password or still your old one. Try the new one first. If TrueNAS turns it away, your old one still works. Makerspace staff are checking, and you can change it again once they have."
  - The rest of the page stays as it is. The account and the files are fine, and the status beside TrueNAS stays green.
- **C · An amber Notice in the card** (`?frame=member-notice`). Same words. Not recommended: a box says the account is in trouble, and only one field is in doubt.
- **Why B.** The member learns what to try, and the order to try it in. Trying the new password is also the evidence the operator's "how to tell" asks for.
- **The form while it's unconfirmed.** It's replaced, never disabled, as the Owner ruled on 9 Oct for a form TrueNAS can't take. If code confirms that a second password change is safe before anyone has looked, keep the form open under the line and end the line "…or set it again now."
- **After Not done** (`?frame=member-redo`):
  - The form comes back with: "Your change at 09:41 today may not have gone through. Set your password again, and TrueNAS will have the one you set."
  - The line doesn't promise the old password works, because the operator may have chosen Not done without being sure.
  - It stays until the next change goes through.
- **After Done:** the row reads as usual: "Last changed today."
- **Which kinds reach the member.** Only `password.set`. A rotate belongs to a revoke, and that member has lost access already.

## For code

**Tokens (Part 1)**
- `globals.css`: change the six values in §1, set `--label: var(--faint)` in both themes, then republish the design system.
- Check every place that draws text with its own alpha (`text-ink/NN`, `opacity` on a block that holds text, `--door-label-quiet` on its ground) against 0.56 in dark and 0.66 in light, over the tinted grounds.
- Stale dimming (`TargetOverview` `opacity-[0.62]`, `MappingManagement` 0.62): remove it if the owner takes the recommendation.

**Unconfirmed changes (Part 2)**
- **Read endpoint, per target.** `ListUnresolvedAddonOperations` filtered by target, **newest first** (it's oldest first today). Each row carries:
  - `operation`
  - the subject resolved to a name
  - the asker resolved to a name, or "Syndra"
  - `sent_at` (`claimed_at`, falling back to `created_at`)
  - a new non-secret `about` column: the account name for adopt, release and purge, and the value for `setting.put`. Never a password.
- **Settle what's certain.** A record never claimed was never sent: Syndra settles it as `unreached` itself, and it never appears.
- **Settle from a read where a read can tell.**
  - `account.adopt` and `account.release` write only the add-on's binding, and repeating either is reported as done. The next inventory read shows whether the binding is there.
  - `setting.put` can be settled from the setting's `set_by` and `set_at`.
  - Confirm each, and record "Settled by Syndra from a read of TrueNAS" in Audit.
- **Settle endpoint.** `POST …/unconfirmed/{id}` with `{ "happened": true|false }`, decoded with `decodeJSONStrict`.
  - It records the terminal status, who settled it and when, plus an audit row.
  - `SettleAddonOperation` today moves only from `dispatching`, and refuses `indeterminate`. The operator's answer needs its own path, kept apart from the dispatch path, so a late settle can still never turn `indeterminate` into `succeeded` on no evidence.
  - Choose whether "didn't happen" is stored as `rejected` or as a new value. Audit must read "Kanishk Singh said it didn't happen".
- **`password.rotate` not done.** The revoke's credential half reopens in Unfinished revocations, and Syndra replaces the password again. Confirm.
- **The member's page.** The storage read (`GET /me/targets`) grows a `password_change: { state: "unconfirmed" | "not_done", at }` field.
  - Confirm whether `password.set` is refused while an earlier one is unconfirmed. Refused, the form is replaced, as drawn. Safe, it stays open (§5).
- **Counts.** Governance indicators grow an unconfirmed count per target. It feeds the system page's Waiting on you chip and lede, Home's "Not confirmed" block and the headline count.

**Design system.** The board draws Notice, NextStep (the dot-and-line form), ChoiceRow (single choice) and StatusSeat as stand-ins. All four entered the canvas design system on 3 Nov 2026, so the next edit of this board can mount the real ones.
