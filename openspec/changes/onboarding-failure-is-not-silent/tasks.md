# Tasks

## 1. The webhook stops reporting a fault as success

- [x] 1.1 `processUserCreated` propagates any onboarding error that is not `db.ErrNoWelcomeBundleConfigured`, so a real fault marks the webhook event `failed` and Zitadel retries. It still acks `nil` for the named sentinel — retrying a missing bundle fixes nothing — but no longer swallows everything else the same way
- [x] 1.2 Tests: a missing bundle still acks 200 with the webhook event completed; a real fault (DB/cascade) returns 500 with the webhook event failed and dbCompleteWebhookEvent NOT called

## 2. A missing bundle is not a fault

- [x] 2.1 Migration `000046` adds `onboarding_triggers.status = 'unconfigured'`, same shape as `000013`'s `dropped_enrichment_incomplete` on `webhook_events`
- [x] 2.2 `db.MarkOnboardingTriggerUnconfigured` + `db.OnboardingStatusUnconfigured`, migration-coherence guard test (`onboarding_unconfigured_migration_test.go`, mirrors the webhook one)
- [x] 2.3 `TriggerOnboarding` marks `unconfigured` for `ErrNoWelcomeBundleConfigured` and reserves `FailOnboardingTrigger` for a real DB fault reading the bundle. Tests for both branches, plus the existing assignment-failure test
- [x] 2.4 `ui/src/lib/event-outcome.ts`: `unconfigured` buckets with `dropped` ("not acted on"), never `failed` — the operations page's existing "no default bundle for new members is set" sentence was already written for this and unreachable until now

## 3. A reconciler answers from state

- [x] 3.1 `services.FindMissedOnboarding` — Zitadel's active roster vs. who holds the welcome bundle, computed on read (justified in the function's own comment: deployment scale, same trade as `HolderCounts`/the People index)
- [x] 3.2 `MissedOnboardingReport{WelcomeBundleConfigured, Missed}` — the config gap is carried explicitly rather than inferred from an empty list, so "nothing configured" and "everybody has it" cannot collapse into the same shape
- [x] 3.3 `GET /api/v1/onboarding/missed` + tests (empty-array-not-null, found people, fault propagation)
- [x] 3.4 Reconciler tests: silent miss with no trigger row at all, inactive people excluded, no-bundle-configured returns the gap not an error, a directory fault propagates rather than reading as "nobody missed"

## 4. Home names it

- [x] 4.1 `useMissedOnboarding` query hook, polled like the rest of `useOperations.ts`
- [x] 4.2 `OnboardingGaps` block on Home, same shape as every other queue block (a count, a link into the thing it counts); counted in both Basic and Advanced headlines
- [x] 4.3 Tests: block absent when nothing is missing, config gap rendered as a fact with a link to Bundles, a named miss rendered as a person, the gap counted in the headline
- [x] 4.4 `bun run lint`, `bunx eslint` on touched files, `bun run test` (976 passing), `bun run build`, plain-language guard

## 5. Federation event check

- [x] 5.1 Traced Zitadel's own source for what a first-time Google Workspace (external IdP) login actually emits — see proposal.md. Confirmed: JIT auto-registration fires `user.human.selfregistered` (already mapped) alongside `user.human.externalidp.added` (not onboarding-relevant). Auto-LINKING a pre-existing account is a different, unchecked path — the reconciler (§3) is the backstop for it either way
