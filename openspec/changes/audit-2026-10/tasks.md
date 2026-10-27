# Tasks

## 1. Batch 1 (branch fix/audit-batch-1)
- [x] 1.1 Claims are recompiled, never just invalidated (`cache.RefreshUser`): drain, expiry, webhooks, and a grant for one project no longer wiping the others. The simulator computes without writing Redis. Warm at start and every 12h, which never degrades a good entry. Refreshes are serialized per user.
- [x] 1.2 Operator-only: `POST /roles`, simulate, `/users`, `/lookup`, bundle impact, onboarding logs, webhook events. A source guard classifies every `withUserAuth` route.
- [x] 1.3 Live config refuses an empty audience or the dev API key. The deploy probe requires `ZITADEL_DOMAIN` and a 401/403 from the unsigned Action. Server header/idle timeouts. The JWKS client is bounded, with an unknown-kid refetch limited and failure backoff. Dead `PORT` and `NEXT_PUBLIC_API_URL` are removed.
- [x] 1.4 A failed (or crash-stranded) webhook or onboarding row is retaken on redelivery. The internal-shape key carries `event_id`.
- [x] 1.5 Target revocation is retryable on a permanent hold already in force, with no duplicate convergence.
- [x] 1.6 UI proxy:
  - PATCH and DELETE bodies.
  - 204, empty and text pass through with their status.
  - `no-store` and `Retry-After`.
  - A timeout and a 1 MB cap.
  - Encoded dot-segments are refused.
  - A foreign Origin is refused on writes, in the proxy and the auth routes.
  - Method parity with the router is tested.
- [x] 1.7 `safeReturnPath` keeps a return on this origin, and the query survives sign-in.
- [x] 1.8 TrueNAS add-on:
  - Only a 2xx is remembered for replay, and the backend rejects a blocked effect on a 2xx.
  - `BindIfFree`.
  - A torn mutation-log tail is truncated.
  - `account.release` is available when the target is down.
  - Purge refuses an unknown self, and `readSelf` is retried.
  - Shutdown waits for in-flight work (100s; Compose grace 120s).
  - Purge reports a leftover binding.
  - An in-range empty orphan group is adopted.
  - `activity.get` says when it truncated.
- [x] 1.9 The screen never claims what it didn't read:
  - Unread grant ends.
  - The rail's placeholder zeros.
  - Home's service-accounts error and its "last checked".
  - The project error.
  - The apps freshness.
  - MyStorage "couldn't read".
  - The reach card on a failed live read.
  - Zero health times.
- [x] 1.10 Owner decisions: Home's calm panel and blocks open and close by Unfold. Text-styled buttons use `link-tint`, which is colour only, no underline.

## 2. Batch 2: reads and one answer
- [ ] 2.1 `rows.Err()` in the 16 loops that lack it.
- [ ] 2.2 Mapping holders and `ExpectedViaRule` follow rule chains (recursive).
- [ ] 2.3 A queued revoke is not "held" (Zitadel-side remove, `replace`).
- [ ] 2.4 Mapping edits re-read inside the access lock.
- [ ] 2.5 A truncated add-on read draws no absence.
- [ ] 2.6 Insights "first seen" without the grants-index arm.
- [ ] 2.7 Review §3.10, one answer per fact.
- [ ] 2.8 `PutSetting` leaves a trace like every other target write.
- [ ] 2.9 The operator view of unresolved add-on operations (addon-platform 2.15/2.16 are ticked but have no reader).

## 3. Batch 3: keyboard and contrast
- [ ] 3.1 Review §3.11. Contrast waits on A-09.

## 4. Batch 4: scale and less code
- [ ] 4.1 Review §3.12 and §3.13.
