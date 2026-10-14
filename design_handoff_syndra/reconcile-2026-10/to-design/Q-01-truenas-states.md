# Q-01 · TrueNAS: states the reconcile boards don't draw

From Claude Code, 14 Oct 2026, while building Storage (B1, B2) and Connected systems (B3). Read `BUILD-NOTES.md` beside this file first: it records what the code does where the package assumed otherwise.

**Answer in the design project at** `handoff/2026-10-reconcile/from-design/A-01-truenas-states.md`, with any boards beside it in `handoff/2026-10-reconcile/design/`. Claude Code reads that path directly; there's nothing to download.

## What code is building (Locked)

- **Offline means 3 consecutive failed checks**, 30s apart. "Offline since 09:40" is the time of the first one. One slow answer stays green. This is the board's rule, and there was no threshold in code until now.
- **The same rule now applies to the operator's pages.** Today `route-system-targets-id` says "Not answering" on a single failed read, and `route-systems` says it per row.
- **After a restart, Syndra can't tell until its first checks finish** (up to ~90s). The member page shows no status then, as BUILD-LIST Storage 2 says.
- **The group picker never offers TrueNAS's built-in groups** (`builtin_administrators`, `wheel`, `builtin_users` …). Mapping a role to one would give NAS administration to everyone holding the role.

## What needs drawing or deciding

1. **The operator's system header and Connected systems row, between "answering" and "offline".**
   - After the first failed check and before the third, the member page stays green. What does the operator see?
     - Green, as the member sees it.
     - Something quieter that admits a check failed.
   - What does the operator see in the "can't tell yet" window after a restart?
   - Recommend one, and draw the header and the row in each state: answering, one or two checks missed, offline since, can't tell yet.

2. **Two meanings of "paused" on one product.**
   - The write path's circuit breaker shows "Paused after failures" (`ui/src/app/system/targets/page.tsx:154`). It means Syndra stopped sending for 30s after 5 failures in a row.
   - Lifecycle "paused" (draining or read-only) means the operator stopped account changes.
   - The member page now says "account changes are paused" for the second. The same word for an operator's choice and an automatic back-off will be misread.
   - Rename one, and say where else the word appears.

3. **The group picker when TrueNAS has no groups of its own.** With the built-in groups excluded, a fresh NAS lists nothing. The board's empty case is "TrueNAS hasn't listed its groups yet", which means the list was never read. This case is different: the NAS was read, and there was nothing to offer. It needs its own line, and possibly a pointer to where groups are made in TrueNAS.

Nothing else is blocked. Code builds everything else in the package to the boards and `RULES.md`.
