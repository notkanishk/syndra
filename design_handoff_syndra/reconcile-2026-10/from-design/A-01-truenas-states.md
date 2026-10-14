<!--
Landed 14 Oct 2026 from the design project, handoff/2026-10-reconcile/from-design/A-01-truenas-states.md, verbatim.
Boards it names live in the design project at handoff/2026-10-reconcile/design/
(route-systems, route-system-targets-id, route-system-targets-id-mappings) and are read from there when each screen is built.
Code's reading of it: BUILD-NOTES.md, "A-01, read against the code".
-->
# A-01 · TrueNAS: the states between answering and offline

Answer to `Q-01-truenas-states.md`, 14 Oct 2026. The boards are in `design/` beside this folder: `route-systems`, `route-system-targets-id` and `route-system-targets-id-mappings`, each with a new group marked **Q-01**. Open one frame with `?frame=<name>`. Your four Locked points are followed as written.

## 1 · Between answering and offline: B, recommended

The operator sees what Syndra knows; the member sees only what they can act on. A missed check is something the operator should know about, but not something they need to act on.

| State | Dot | Word | Line under it (row) · sentence (system page) | Notice |
|---|---|---|---|---|
| Answering | healthy | Answering | Answered just now · "Accepting changes. Nothing is in flight." | none |
| 1 or 2 checks missed | healthy + pips ●●○ | Answering | **Missed 1 check, 09:40** · "Missed its last check, at 09:40. Syndra checks every 30 seconds and calls it offline after 3 in a row. Changes still go out." | none |
| Offline since | warn + pips ○○○ (amber rings) | **Offline** | Since 08:33 · "Since 08:33: 3 checks in a row went unanswered. Changes for TrueNAS wait until it answers." + "Look at TrueNAS." | amber, "TrueNAS isn't answering." |
| Can't tell yet | neutral + pips ○○○ (grey rings) | **Checking** | No check since restart, 09:38 · "Syndra restarted at 09:38 and hasn't heard from TrueNAS since. This clears at its first answer, within about 90 seconds." | none |

- **Check pips.** Three 6px dots after the status word show the last three checks, oldest first: lime filled = answered, amber ring = missed, grey ring = no check yet. They appear only when a check has been missed or none has run yet (missed, Offline, Checking); a healthy row shows none. The ring follows 86c: a ring means not a fresh read. `aria-label` reads them out: "Last 3 checks: answered, answered, missed".
- "Missed 2 checks, 09:40" for the second; the time is always the first miss of the run. The line clears on the next answer.
- **Offline** replaces "Not answering" as the status word on Connected systems and the system page, so the operator and the member read the same word and the same time. "isn't answering" stays in the sentences.
- **Can't tell yet** ends at the first answered check, so it usually lasts seconds. Only a NAS that's down at start-up holds it for the full ~90s, and then it becomes Offline. The rest of the system page comes from its own reads and isn't affected.
- Frames: `route-systems` missed-a, missed (B), missed-c, offline, checking; `route-system-targets-id` missed (B), missed-c, checking, quiet (now Offline since).
- Not chosen: **A**, green as the member sees it, hides a NAS that drops one check in three and tells the operator 60 seconds after Syndra knew. **C**, amber, fires for something that clears itself, and teaches the operator to ignore amber.

**Which reading wins.** When more than one is true, the row and the header show one, in this order:
1. Cannot connect (the signing key)
2. Offline (the monitor: TrueNAS doesn't answer checks; look at TrueNAS)
3. Waiting to retry (the breaker is open while checks still answer; look at the Syndra server)
4. Checking
5. Answering, with or without a missed check

The board's current code lets `circuit_open` win over everything. That sends the operator to the Syndra server even when TrueNAS is down, and it's the one wrong turn this page exists to prevent.

## 2 · Two meanings of "paused": the breaker is renamed **Waiting to retry**

"Paused" now means only that a person stopped account changes to a system (lifecycle draining or read-only). The member page is built on that meaning, and it's an operator's choice. Automatic stops say what's happening instead.

- Breaker, status word: **Waiting to retry** (amber). Notice: **"Syndra is waiting before it calls TrueNAS again."** "5 calls in a row failed, so it stopped calling and tries again by itself every 30 seconds. TrueNAS still answers Syndra's checks, so look at the Syndra server. Changes for TrueNAS wait, and nothing is lost."
- Rejected: "Waiting to retry" is engineering slang, and an operator reads it as a retreat. "Retrying" sounds busy while Syndra is deliberately idle. "Holding off" collides with holds (#79). "Waiting to retry" says both halves: it is idle on purpose, and it will try again by itself.

Where the word appears in `ui/src` today, and what each says instead:
- `app/system/targets/page.tsx:154` "Paused after failures" → "Waiting to retry", with the detail above. Tests: `connected-systems.test.tsx:111, 147`.
- `components/targets/TargetOverview.tsx:604` "Paused after failures." → "Waiting to retry." `:1672` "the 'Paused after failures' reading" → "the 'Waiting to retry' reading".
- `app/governance/pending/page.tsx:117` "Sending is paused." (Zitadel down) → "Sending waits until Zitadel answers." Test: `pending/__tests__/page.test.tsx:101`.
- `lib/drain-outcome.ts:95` "The queue is paused at that change" → "The queue stops at that change". `:174` "so that queue is paused at it" → "so that queue stops at it".
- `components/storage/MyStorage.tsx:258` "Your account is paused." reads as a suspended account. The board says "Account changes are paused …" (route-storage, draining and readonly frames); use that.
- Keep: `MyStorage.tsx` Paused (lifecycle), `TargetOverview.tsx:114` lede ("whether somebody has paused changes to it"), `HoldDialog.tsx` (a hold is "on hold", never "paused").
- Boards: route-systems, route-system-targets-id and route-system-targets-id-mappings are updated to Waiting to retry.

New standing rule, added to the design project's `CLAUDE.md` (`RULES.md` in the package): **"Paused" is only ever a person's choice. Syndra's own stops say what's happening: waiting to retry, waits until it answers, stops at that change.**

## 3 · The group picker when TrueNAS has no groups of its own

Frame `route-system-targets-id-mappings?frame=add-nogroups`, beside `add-unread` for contrast.

- The field stays in its seat, disabled, reading "No groups to offer". Hint: "Read from TrueNAS 2 seconds ago."
- A neutral panel (`.panel`, not warn: a fresh NAS starts here, and nothing is broken):
  - **"TrueNAS has no groups of its own yet."**
  - "It lists 34 groups, all of them built in, like builtin_users and builtin_administrators. Syndra never offers those: a role mapped to one would give everyone who holds it the NAS's administration or its default share access. Make a group on TrueNAS, under Credentials → Groups, then read again."
  - Button: **Read again** (outline, sm). It re-reads the values list.
- Preview is disabled. Footer note: "Nothing to preview until TrueNAS has a group of its own."
- The count (34) and two example names come from the read itself. Name the first two built-in groups the read returns, in its order.
- If the add-on reports the NAS's web address, the instruction can end with a link, "Open TrueNAS →". It isn't drawn, because the manifest doesn't carry the address today.
- **Not listed yet**, the other case: "TrueNAS hasn't listed its groups yet." "Syndra asks for the list when this opens, and TrueNAS hasn't answered since Syndra restarted at 09:38. This usually clears within a minute and a half." Option "Waiting for TrueNAS's list", hint "Not read yet.", no count.
- The ordinary picker's hint now says so too: "Groups made on TrueNAS, read 2 seconds ago. Its built-in groups are never offered."

Nothing else changes.
