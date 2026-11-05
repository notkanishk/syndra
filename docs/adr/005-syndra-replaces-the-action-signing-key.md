## ADR-005: Syndra replaces the action signing key itself

**Date:** 2026-11-05
**Status:** Accepted (owner, "Automatic + button"). Supersedes #71 ("no Rotate now button") and the application-claims requirement "Signing Key Rotation is Policy-Driven, Not Scheduled".

### Context

Zitadel signs every Actions v2 call with a per-target key and never expires it. Syndra's policy limit is 90 days. Replacing the key used to be `make zitadel-actions-rotate-key` on the server, followed by an `.env` edit and a restart. #71 kept it that way deliberately: the operator saw the exit code and the new keys, and a button could not.

Two things changed that.
- **S5** (ADR-004, claims-delivery-integrity) made Syndra follow the claims target's key from Zitadel, so a new claims key needs no restart.
- **The 92-day-old key.** On 5 Nov 2026 the production key was 92 days old. Nothing would have replaced it unless somebody remembered.

The backend's service account is the one rotate.sh uses, so Syndra already holds the permission.

### Decision

- Syndra replaces the keys of both of its targets, the claim injector and the event listener. It does this itself a few days before the limit (default 5) and inside a quiet window (default 03:00–05:00 local). It acts only when Zitadel answers and the L1 probe passed in the last 5 minutes. `SYNDRA_ACTION_KEY_AUTOROTATE=off` turns it off.
- "Replace now" on the Zitadel page does the same on demand, and may override the 24h guard.
- **A replacement is a Syndra-mediated Zitadel mutation.**
  - It holds a Postgres advisory lock.
  - It writes its trace (`action_key_rotations`) before the call.
  - It records the outcome and an audit row afterwards.
- **The new key enters the keyring straight from Zitadel's answer.**
  - It goes into Redis, so both processes have it.
  - A door that matches no key looks again before refusing, and waits up to 1s while a replacement is in flight.
  - The previous key keeps verifying 15 minutes, or 7 days when the check did not pass.
- **The event listener's key** is under the same keyring. The `.env` keys are only the bootstrap.
- **Keys persist in Redis only.** Redis's append-only file is on its volume, on the internal network, which is the same exposure as `.env`. Keys are never stored in Postgres, where every dump and backup would carry them.
- **Age is observed:** from Syndra's record, or from the keyring seeing Zitadel's key change. The `.env` date counts only until one of those exists.
- **The check after a replacement** is L1 at once, and L2 when it is set up.
  - A failure keeps both keys accepted and raises `KEY_ROTATION`. That makes `/healthz/claims` degraded and stops the dead-man ping, which tells the owner.
  - Nothing retries on its own.

### Consequences

- The key never goes overdue while the schedule is on and Zitadel accepts the write. No step of a replacement is manual.
- The backend's service account must keep `action.target.write`. Revoking the instance permissions between script runs (DEPLOY.md's old advice) now turns every automatic attempt into `KEY_ROTATION:failed`.
- The quiet window is read in the container's `TZ`. Unset, that is UTC.
- rotate.sh stays as the manual fallback. Syndra notices a manual replacement through the keyring and dates the key from it. The by-hand guide on the Zitadel page watches for that and checks the key on its own.
- A call Zitadel signs with the new key before its answer reaches Syndra is covered by the in-flight wait. A call that arrives after the 1s wait, with a key no keyring has, is refused as before: interrupt-on-error stays off, so the sign-in goes ahead without Syndra's claims, and telemetry counts it.
