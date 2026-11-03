# Q-10 · A person's page: their network storage, and where they sign in

From Claude Code, 3 Nov 2026. Nothing here reopens a verdict.

---

## Part 1 · Network storage on a person's page

### What the owner asked (15 Oct, `openspec/NEXT.md:116`)

> A read-only section for operators: the person's account on TrueNAS, their home folder use against its limit, the folders they can open, and any limit or hold of their own, each with "Set on TrueNAS's page →". Every write stays on the system's page.

### Locked

- **Read-only.** One write place per setting (standing rules). Limits and holds are set on System › TrueNAS (`ui/src/components/targets/PeopleOnTarget.tsx`). The person's page shows them and links there.
- **The data already exists.** The member's own storage page reads it from `describeMyTarget` (`backend/internal/handlers/my_storage.go:172`, behind `GET /me/targets`, self only). Per system, it reads:
  - the binding: account name, adopted or created, conflicts;
  - the credential: set or not, needs re-enrolment, last changed;
  - storage: usable or not, each place (home, shares) with its usage and its quota;
  - the connection state.
  
  The operator's `GET /targets/{t}/homes` (`router.go:310`) adds used, quota, own limit and the roles' limit for each account. Code will add `GET /users/{id}/targets`, which runs the same function for an operator.
- **Pieces to reuse:**
  - The member's `PlaceBlock`, `Folders` and `SystemStatus` (`ui/src/components/storage/MyStorage.tsx:571, 449, 365`).
  - The system page's per-person row (`PersonRow` and `HomeLine` in `PeopleOnTarget.tsx`).
- **Today the person's page** (`ui/src/components/people/PersonAccess.tsx`) shows a hold only as the `Withheld` band, and NAS file events on its Activity tab (`PersonActivity.tsx`). The board is `route-users-id`, which draws no storage.
- **States are the standing ones:** loading, error, degraded (last known with its age, never sample data), offline (a Notice), and the system still being set up (NotOpenYet).
- **Two rulings disagree.** `proposed/Deliberate-later.md` (6 Oct, explore-hold-share option A) drew "Put on hold" per row on the person's page. The read-only ruling of 15 Oct is later.

### What I need

1. **Placement:** a card on the Access tab (below the projects), a tab of its own, or a section of the Activity tab. Basic view, Advanced, or both?
2. **The card:** one per connected system, giving:
   - the rows, with their order and labels;
   - what "own limit" and "roles' limit" look like side by side;
   - how folders read (a list, or a count that links).
3. **The links:** where each "Set on TrueNAS's page →" lands (the person's row on the system page, or the system's setup card).
4. **Empty and edge states:**
   - no account on this system yet;
   - an account Syndra didn't create (adopted);
   - an account that needs a new password;
   - the system offline;
   - the person not mapped to any folder.
5. **The 6 Oct "Put on hold" row:** is it dropped (my reading of the later ruling)?

---

## Part 2 · Where someone signs in

### Locked

- Every sign-in reaches Syndra (Zitadel's session event, `backend/internal/handlers/webhook.go`, `recordSignIn`).
- Today only the **last** sign-in per person and project is kept (`usage_last_seen`). It's read by `GET /users/{id}/usage` (`router.go:337`) and Insights' "Recent use" and "Held but not used".
- A-06 §7 already settled two copy lines: "Recent sign-ins" and "No sign-in to its app in 90 days".
- Code can add a count per person, per app, per day, kept about 13 months. An app belongs to exactly one project (ADR-002), so "per app" is the honest unit. Sign-ins Syndra can't match to an app stay as "app unknown".
- **Whether to keep counts at all is the owner's call** (a university, people's sign-ins). This part draws it assuming yes, so the owner can see what they'd be choosing.

### What I need

6. **Person page:** where the apps they sign in to go, and the form: a last-used line, a count over 30 days, or a small 30-day strip.
7. **App page:** "9 people this week, last sign-in 10 min ago", or another form, and where it sits.
8. **Insights:** does "Held but not used" gain counts, or stay a last-date list?

---

**Answer at** `handoff/2026-10-reconcile/from-design/A-10-person-storage-and-sign-ins.md` on the canvas, numbered 1–8, with the board beside it.
