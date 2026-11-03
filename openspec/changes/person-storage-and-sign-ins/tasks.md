# Tasks

## 1. Network storage on a person's page (A-10 Part 1)
- [x] 1.1 `GET /users/{id}/targets`, operator-only, one view per registered system from `describeMyTarget` (`handlers/person_storage.go`)
- [x] 1.2 Adoption from the `account.adopt` that made the binding (within 10 min of `bound_at`)
- [x] 1.3 Home limit (roles' limit and source, own limit, own ended) shared with `/targets/{t}/homes` (`roleLimits`, `ownHomeLimit`, `ownEndedIfOver`)
- [x] 1.4 Holds in force (denies), last known read while offline, setup (`no_project` / `no_mapping`), preview for no account yet
- [x] 1.5 `addons.MyStorage` takes the actor; policy and add-on comments state the operator case
- [x] 1.6 Card: rows Account · Password · Home folder · Folders · Hold; links to the one write place; states adopted, new password, not set, re-enrolment, no account yet, no role, not mapped, offline (Notice, last known, card recedes per A-13 d), one slow answer, being set up, loading, error, held; Advanced folder paths
- [x] 1.7 Read line gains one chip per system with no count (`NavSection.count` optional)
- [x] 1.8 TrueNAS's page: each person row anchored; `?person=<id>#accounts` opens the list far enough, glides and flashes once; HoldDialog comment corrected
- [x] 1.9 Pending changes seeds its search from `?q=` ("Waiting for you to send →" filtered to the person)
- [ ] 1.10 `needs-info` The `norole` second sentence naming the roles that do reach the system

## 2. Sign-ins (A-10 Part 2; owner, 3 Nov "1a with statistics")
- [x] 2.1 Migration 000069: `usage_last_seen.app_id` in the key; `sign_in_daily`
- [x] 2.2 `recordSignIn` keeps the app (`resolveSignInApp` / `matchSignInApp`); daily total counted once per event (strictly-later rule)
- [x] 2.3 `GET /users/{id}/usage` per app with `receiving`, `receiving_since`
- [x] 2.4 `GET /applications/{id}/usage`: people in 7 and 30 days (service accounts left out), last sign-in, daily totals
- [x] 2.5 `POST /sign-ins/lookup`: strict body, reason required, audited first, `zitadel.SearchEvents`, filtered by the webhook's translation; tested with a fake
- [x] 2.6 Person page: "Last sign-in 10 min ago, to Fab queue" / "No sign-in to its app in 90 days" / "since 3 Sept" in each project heading
- [x] 2.7 App page: "9 people in 7 days · last sign-in 10 min ago" in the meta; the 30-day Sign-ins strip, naming nobody
- [x] 2.8 Look up sign-ins: Activity tab and app page; reason and range; ends on its own result with Close; audit vocabulary `sign_ins.looked_up`
- [x] 2.9 Insights "Held but not used" unchanged (any app of the role's project counts)
- [ ] 2.10 `ready-for-human` First contact with a live Zitadel: confirm `events/_search` accepts `eventTypes` + `range` and returns `payload` as the tests assume (`zitadel/events.go`, UNOBSERVED like `GrantOriginByID`)
- [ ] 2.11 `ready-for-human` Run `db/usage_live_test.go` against the dev box Postgres (migration 69, daily totals)
