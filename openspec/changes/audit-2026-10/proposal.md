# Audit remediation, 27 Oct 2026

## Why
An outside audit (`docs/from another side/AUDIT-2026-10-27.md`) was checked claim by claim against the code and the recorded decisions (`docs/from another side/AUDIT-2026-10-27-REVIEW.md`). Most findings held. The worst was the Actions v2 claims cache: every applied change emptied it and nothing rebuilt it, so a token after any grant, revoke or expiry carried no Syndra claims. Production's cache was empty when checked. No app reads Actions v2 claims yet, so nobody saw it.

## What
Batch 1 (this change, `tasks.md` §1): the claims cache, backend route gates, live-config guards, retryable webhooks, onboarding and target revocation, the UI proxy's contract, the sign-in return path, the TrueNAS add-on's integrity (replay, binding, mutation log, shutdown), and the screens that claimed what they hadn't read.

Later batches (`tasks.md` §2–§4): data reads that pass a partial answer off as whole, one answer per fact across screens, keyboard and contrast (Q-09 to Design, with the unconfirmed add-on changes), scale, and dead code.

## Not changing
- **Network:** the LXC's published ports (owner, 27 Oct: leave for now).
- **Logout:** it ends only Syndra's session, not Zitadel's (owner, 27 Oct: communal sign-in is unlikely).
- **Recorded decisions the audit questioned:** demo mode, the global access lock, the plan cohort, the read legs not gated as operations. See the review, §2.
