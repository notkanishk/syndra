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

## 2. Batch 2 (branch fix/audit-batch-2, stacked on batch 1)
- [x] 2.1 `rows.Err()` after every db row loop.
- [x] 2.2 Mapping holders (recursive CTE) and `ExpectedViaRule` follow rule chains, with cycles ending the walk.
- [x] 2.3 A queued revoke, or a queued `replace` that omits the role, is not "held".
- [x] 2.4 Mapping edit/delete re-read the mapping and its holders inside the access lock (409 on a moved value, citation required for new holders).
- [x] 2.5 A truncated add-on read draws no absence.
- [x] 2.6 Insights "first seen" drops the grants-index arm.
- [x] 2.7 One answer per fact:
  - Indicators invalidate with summary.
  - The role page counts observed holders only.
  - Assign bundle, create role and renames refresh their lists and the name caches.
  - The member's held set includes observed roles.
  - Withdrawn access links service accounts to their own page and counts from the endpoint's summary.
  - Source pills count people.
  - The People chip says "within 30 days".
  - One `expiryLabel()`.
  - The drift chime plays from the rail only.
  - The target roster refreshes every 30s.
  - A person's role names come from the catalog.
- [x] 2.8 `PutSetting` writes an operation row and audit line before the call and settles it. A timeout is "may have applied".
- [ ] 2.9 The operator view of unresolved add-on operations: waits on Design, `to-design/Q-10-unresolved-operations.md`.

## 3. Batch 3: keyboard and contrast
- [x] 3.1 Keyboard and small UX:
  - A skip link.
  - FilterPanel focus in and back.
  - Modal named from its header, the app root inert behind it, and Esc stopped.
  - Segmented/FilterPills as radio groups with one tab stop and arrows.
  - Tabs as a labelled group.
  - The theme script follows the OS.
  - People: Esc in a dialog keeps the selection, and search re-syncs from the URL.
  - The phone sheet's links replace history.
  - Zitadel grant dialogs keep their outcome.
  - Deleting an app's claim asks first, with one verb on the button and the confirm.
  - Audit export says it exports every match, and CSV cells can't run as formulas.
- [ ] 3.2 Contrast: waits on A-09.

## 4. Batch 4: scale and less code
- [x] 4.1 Scale:
  - The governance summary uses one holder-count query.
  - The directory rebuild is shared by concurrent misses (singleflight), forgotten on invalidate, and cancellable per caller.
  - Home's onboarding poll is 60s.
  - The logs list the newest 200, with indexes (migration 68) and a 90-day prune.
  - `/zitadel/grants` observes on page 1 only; reconciliation observes when the last complete reading is over 5 minutes old or on a refresh.
  - Add-on read-back filters by username.
- [x] 4.2 Less code:
  - Dead code removed (`lib/api.ts`, dead types and helpers, unused db functions).
  - One `succeeded()` and `EXTEND_DAYS`.
  - The smoke scripts share the env loader.
  - StatCard via `next/link`.
  - Four heavy dialogs load on demand.
- [ ] 4.3 Not done, by choice: mega-component splits, a query-key registry, the arbitrary-utility codemod (review §2).
