# Q-10 · A change to a connected system that nobody can vouch for

From Claude Code, 28 Oct 2026. Send this after Q-09.

## What happens

Syndra asks a connected system's add-on (TrueNAS today) to do things: set a member's storage password, adopt or release an account, purge one, change a setting. Each call is recorded before it goes out (`backend/internal/db/addon_operations.go`).

Sometimes a call's answer never comes back: the add-on restarted mid-call, the network dropped, Syndra itself restarted. The change **may or may not have happened**. Syndra can't tell which, and must not guess:
- Calling it a success claims what nobody knows.
- Calling it a failure tells a member to try again, against a system that may already hold their new password.

Today these rows are recorded and counted, but **no screen shows them**. The backend has the query ready (oldest first, with calls still in flight left out). Nobody sees one unless they read the database.

## How many, how often

Rare. It needs a crash or a dropped connection in the seconds a call is out. When it does happen, it's usually a password or a purge, which a member or an operator is waiting on.

## What the operator can do about one

- **Check the system and say what happened.** It applied, or it didn't. That settles the row.
- **For a password:** have the member set it again, which is safe once someone has looked.
- Nothing is retried automatically. That's deliberate, because a blind retry of a password or a purge is exactly what the add-on's guards refuse.

## What I need

1. **Where it lives:**
   - **a)** A Notice on the system's page (System › {target}), amber, "N changes Syndra can't confirm", opening a list.
   - **b)** A section of its own on that page.
   - **c)** A row in Review, beside Unfinished revocations.

   I recommend **a**. It's per-system, rare, and the system's page is where the operator would check.
2. **The row:**
   - What was asked (in words: "Set Ada's storage password").
   - When.
   - Who asked.
   - The two settling verbs. I suggest "It applied" and "It didn't apply".
3. **The member's side:** a member whose password change is unconfirmed currently sees nothing special. Should their storage page say so, e.g. "We couldn't confirm your last password change. Makerspace staff will check it."?

**Answer in the design project at** `handoff/2026-10-reconcile/from-design/A-10-unresolved-operations.md`.
