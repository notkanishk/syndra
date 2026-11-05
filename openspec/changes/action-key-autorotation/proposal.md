# Syndra replaces the action signing key itself

**Why.** Zitadel signs every Actions v2 call to Syndra with a per-target key, and never expires it. Syndra's policy limit is 90 days. Until now, replacing the key was a command the owner ran on the server (#71: "no Rotate now button"). On 5 Nov 2026 the production key was 92 days old, and nothing would have replaced it unless somebody remembered.

The owner decided on 5 Nov 2026: **"Automatic + button".**
- Syndra replaces the key itself a few days before the limit, at a quiet hour.
- It checks that a signed test sign-in passes, records the replacement in the audit log, and tells the owner afterwards.
- "Replace now" does the same on demand.
- There are zero manual steps, and the key never goes overdue.

This reverses #71. Two things made that decision safe to reverse. Since S5 (claims-delivery-integrity) Syndra follows Zitadel's key on its own, so a key change no longer needs a restart. And Syndra already holds the permission it needs, because `rotate.sh` runs as the same service account.

**What.**
- **A rotator in the backend.** It works through the existing Zitadel client and writes its trace before the call (`action_key_rotations`, migration 000077). A Postgres advisory lock keeps it single flight. It refuses when the key was replaced in the last 24h (unless forced) and when Zitadel doesn't answer.
- **No window where a call can't be verified.**
  - The new key goes into the keyring straight from Zitadel's answer, for the backend and the claims container at once.
  - A door that matches no key looks again before it refuses, and while a replacement is in flight it waits up to 1s for the new key.
  - The previous key keeps working for 15 minutes, or 7 days after a replacement the test sign-in did not confirm.
- **The event listener's key** is under the same keyring, so no key change ever needs a restart. The `.env` keys are only the bootstrap.
- **Age is observed.** "Last replaced" comes from Syndra's record, or from the keyring seeing Zitadel's key change. The `.env` date is used only until one of those exists.
- **The schedule.**
  - Syndra replaces the key once its age reaches the threshold minus a lead (default 5 days), inside the quiet window (default 03:00–05:00 local).
  - It runs only when Zitadel answers and L1 passed in the last 5 minutes.
  - There is at most one attempt per window, and a kill switch.
- **The check after a replacement.**
  - L1 runs at once, and L2 too when it is set up.
  - A failure leaves both keys accepted, raises `KEY_ROTATION`, and never retries on its own.
- **Endpoints.**
  - `POST /api/v1/zitadel/action-key/rotate` replaces the keys now.
  - `POST /api/v1/zitadel/action-key/verify` is the by-hand guide's check.
  - The rotation status gains the schedule and the last outcome.
- **On the Zitadel page.**
  - The signing key row says when the key was replaced and when it is next due.
  - "Replace now" opens a Modal.
  - "Do it by hand" opens the designed guide (route-zitadel §2). It watches for the new key and checks it on its own.

**Not in scope.** A per-target schedule, a notification channel of its own, and storing keys anywhere but Redis and Zitadel. A failure is still reported to the owner: it makes `/healthz/claims` degraded, which stops the dead-man ping.
