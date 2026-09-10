# A failed welcome is not a success, and a missing one is not silent

## What is wrong

Five real accounts (4 Aug, 6 Aug, 18 Aug, 19 Aug, 9 Sep) hit
`ErrNoWelcomeBundleConfigured` on account creation. Every one was recorded in
`onboarding_triggers` as `status = 'failed'`. Every one was also reported to
Zitadel as a successful webhook delivery, because `processUserCreated`
(`backend/internal/handlers/webhook.go`) logged the error and returned `nil`
unconditionally — not just for the no-bundle case, for every onboarding
failure. Two records of one outcome, disagreeing, and neither of them
reachable from a screen: `onboarding_triggers.status` appeared on no operator
surface, and nothing ever asked "who joined and got nothing" from the state
of the world rather than from a webhook having arrived.

This is `one-truth-many-checks` applied to a trigger instead of a state read:
the system had no way to distinguish "nothing needs retrying" from "nothing
went wrong", and rendered the first as the second.

## What changed

1. **The webhook no longer lies about the outcome.** `processUserCreated`
   still tells Zitadel not to retry a missing welcome bundle (retrying fixes
   nothing), but a real fault — a DB error, a cascade failure — now
   propagates, marks the webhook event `failed`, and gets retried.
2. **A missing bundle is not a fault.** Migration `000046` adds
   `onboarding_triggers.status = 'unconfigured'`, distinct from `'failed'`.
   `TriggerOnboarding` uses it for `ErrNoWelcomeBundleConfigured` and reserves
   `'failed'` for actual defects (a DB fault reading the bundle, a cascade
   failure assigning it). The distinction is explicit in the data, not
   inferred from the error text.
3. **A reconciler answers from state.** `services.FindMissedOnboarding`
   checks Zitadel's roster against who actually holds the welcome bundle —
   computed on read (see design note in that file for why not a sweep), not
   from the onboarding trigger log, which only knows what a webhook happened
   to report. It also carries `welcome_bundle_configured` explicitly, so "no
   default is set" (a fact, not a person) is never confused with "N people
   got nothing" (named people).
4. **Home names it.** A new queue block, `OnboardingGaps`, shows both: the
   config gap as a system fact linking to Bundles, and named people the
   reconciler found holding nothing, following the same shape as every other
   Home queue block (a count, a link into the thing it counts).

## What was checked and not changed

The federation question: does a person joining via Google Workspace (the
sole IdP) actually emit `user.human.added` or `user.human.selfregistered` —
the two events `translateEventName` maps to `user_created` — or does
auto-provisioning via an external IdP fire something else entirely
(`externalidp`/`externallogin`) that onboarding never sees?

Checked against Zitadel's own source
(`internal/command/user_human.go`, `internal/auth/repository/eventsourcing/eventstore/auth_request.go`,
`zitadel/zitadel` main branch): `AutoRegisterExternalUser` — the JIT
auto-provisioning path invoked on first login via an external IdP with no
matching local account — calls `AddHumanFromDomain` with `Register: true`,
which pushes `user.NewHumanRegisteredEvent` (`user.human.selfregistered`) in
the same command as `user.NewUserIDPLinkAddedEvent`
(`user.human.externalidp.added`) for the IdP link. The first event is one
`translateEventName` already handles; the second is a distinct event type
that carries no onboarding meaning and is correctly left unmapped.

So the primary joining path (auto-provision on first Google Workspace login)
does fire `user_created` today. Not checked, because it is a different flow:
whether this deployment's Zitadel is configured to auto-LINK a pre-existing
account instead of auto-registering one (matching by email to a human user an
admin already created) — that path emits only `human.externalidp.added`, no
`human.added`/`selfregistered`, and onboarding would not fire for it. The
missed-onboarding reconciler (item 3) is the safety net for that case
regardless of which webhook path was supposed to catch it, because it reads
state rather than trusting either.

## Impact

- `backend/internal/handlers/webhook.go` — `processUserCreated`
- `backend/internal/services/onboarding.go` — `TriggerOnboarding`
- `backend/internal/services/onboarding_reconcile.go` — new
- `backend/internal/db/onboarding.go` — `MarkOnboardingTriggerUnconfigured`,
  `OnboardingStatusUnconfigured`
- `backend/db/migrations/000046_a_missing_bundle_is_not_a_fault.{up,down}.sql`
- `backend/internal/handlers/onboarding.go`, `deps.go`, `router.go` — new
  `GET /api/v1/onboarding/missed`
- `ui/src/lib/event-outcome.ts`, `ui/src/lib/queries/useOperations.ts`
- `ui/src/components/home/Home.tsx` — `OnboardingGaps`
