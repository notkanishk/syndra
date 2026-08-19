# Design — Basic / Advanced IA and real claim shaping

**Roadmap phase:** 5 (operator experience).
**Related:** [core architecture](../syndra-core-architecture/design.md) · [application-claims spec](../syndra-core-architecture/specs/application-claims/spec.md) · [zitadel-actions-v2-deployment](../zitadel-actions-v2-deployment/design.md) · supersedes the visual layer of [obsidian-clarity-redesign](../obsidian-clarity-redesign/proposal.md).

## 1. The two views

`ui_view: basic | advanced`, persisted per browser in `localStorage`, never called `mode` (`GET /api/v1/system/mode` already means demo-vs-live backend state, and two "modes" in one product is how the previous IA drifted).

Three rules the switch keeps:

1. **The URL does not change.** Switching reveals panels in place. Losing your place is the fastest way to make a view switch feel punitive.
2. **No dead ends.** Where something can only be resolved in Advanced, Basic names the cause and offers one scoped jump that sets the view, stays on the URL, and scrolls the revealed panel into view — `UiViewProvider.revealInAdvanced`, which scrolls the `#app-scroll` container rather than calling `scrollIntoView` (that would also move the page sideways in a narrow viewport).
3. **Basic is not Advanced-minus-features.** It is a smaller job done completely.

Advanced is operator-only and is not rendered for members at all — not greyed, not present-but-403. `middleware.ts` enforces the same rule with an allowlist rather than a denylist, so a new operator route is protected the day it is added rather than the day somebody remembers it.

The allowlist is **`nav.ts`'s `MEMBER_ROUTES`, read through `memberMayVisit`** — not a copy of it. It was a copy, holding `/`, `/requests` and `/login`, and it drifted the day `addon-platform` gave members a third row: the rail offered every member `Network storage` and middleware redirected them off it. A route a member may reach is navigation structure, and this change's own rule is that navigation structure lives in one file. `/login` needs no seat, because a valid session is already sent away from it by an earlier guard and an absent one never reaches the member check. Sub-paths belong to their parent, matching `leafMatches`, so `/storage/{target}` is reachable the day it exists; `/` matches exactly, or it would admit everything. `middleware.test.ts` asserts a member reaches every row `MEMBER_NAV` renders, and a source guard fails if middleware names a member route itself — a second list agreeing with the first passes every behavioural test and then drifts again.

### Structure never moves

`ui/src/lib/nav.ts` is the single navigation contract, consumed by both the rail and the breadcrumb so the two cannot disagree. Both trees are static arrays; counts are looked up by key. A count going 0 → 12 therefore cannot reorder anything, and a section with nothing in it renders a hollow `0`. This is asserted directly: the rail is rendered twice, with all-zero and all-non-zero indicators, and the row order must be identical.

`/projects/{id}` belongs to Projects while `/projects/{id}/roles/{key}` belongs to Roles, which a prefix match gets wrong — nav entries therefore carry a `pattern` regex rather than a prefix list.

## 2. Claim shaping — where the shape lives

### The defect

| | Before | After |
|---|---|---|
| Compiler writes | a finished claim map | facts (`roles`, ids, profile attributes) |
| Actions v2 emits | the raw cache map, keys namespaced `syndra.<projectID>.` on multi-project | `claims.Shape(profiles, facts)` |
| Simulator emits | `{iss, sub, aud, source, project, <claim_name>: …}` | `claims.Shape(profiles, facts)` — the same call |
| `claim_name` / `format_type` | read only by the simulator | read by both |

### Shape on read, not on compile

Shaping at compile time would mean a format edit applied only to users whose cache happened to be rebuilt afterwards — an edit that takes effect per-user at random is worse than no edit. Shaping on read costs one Redis-cached profile lookup (`claim_shape:<projectID>`, invalidated on every save so an edit is never one TTL late) and keeps the Actions v2 latency budget intact. Profile attributes (email, name, title, team) are captured at compile time because a directory call is affordable there and is not affordable inside the token path.

Every failure path in `claimProfilesRead` returns the built-in default profile rather than an empty set: emitting roles under the default key is a degraded token; emitting nothing is a locked door.

### Per-application overrides, honestly

Verified against the documented `preaccesstoken` payload (`function`, `userinfo`, `user`, `user_metadata`, `org`, `user_grants`): **there is no client or application identifier.** An override therefore cannot be resolved by "which app asked".

The model that is actually true: a token issued for a project carries the project default **and** every override key on that project; each application reads its own key. Claim keys are validated unique across every project (a user with grants in several projects receives one flat token), and the UI states the arrangement rather than letting an operator discover a sibling's key by decoding a production token — `emitted_keys` names the owner of every key, and the preview dims the ones this app does not read.

### The collision the old namespacing hid

Two projects nobody has configured both default to `roles`. The old code prefixed every multi-project key with `syndra.<projectID>.`, which prevented the collision and simultaneously guaranteed no application ever received the key it asked for. Now: keys are explicit and validated, and `mergeProjectClaims` namespaces only keys that two projects both emit — logging loudly and telling the operator to name the claims. Configured keys are never rewritten.

## 3. Removal is source-specific

There is never a generic "Revoke role". A person can hold one role three ways, so a generic action is ambiguous at best and destructive at worst. The action is named after the thing being removed and the confirmation states the residual outcome — what they are left holding. `DELETE /users/{id}/grants/{grantId}` deletes the ledger row and enqueues the delta in one transaction; it is deliberately not the Zitadel-side grant delete, which removes a different object and leaves this row to restore the access at the next compile.

**The delta, not a revoke.** The first implementation enqueued an unconditional revoke after deleting the row — which made the dialog lie. "They will still hold this role via Lab Tech" was rendered on screen while the queued write removed the role from Zitadel anyway; the access came back only at the next compile. The fix reuses the closure machinery every other cascade already uses (`userBaseHoldings` → `effectiveClosure` → `closureDelta`), with one new simulation helper, `userBaseHoldingsExcludingGrant`, which excludes by **grant id** rather than by (project, role) — excluding by pair would blind the base to a role a bundle still carries and reintroduce the same bug from the other side.

The response reports `revoked_roles` and `retained_roles` so the promise the dialog made is checkable rather than assumed.

## 4. Verification

```bash
cd backend && go test ./... && go vet ./...
cd ui && bun run test && bun run lint && bun run build
```

Manually verified in a browser (dev server, backend down): both themes, the Basic and Advanced rails, breadcrumbs, and the error state. Hydration was checked and one real mismatch fixed — locale-dependent dates rendered on both server and client; dates now format in a fixed locale and anything derived from "now" renders client-side only (`components/ui/Time.tsx`).

The delta behaviour is pinned by a mutation check: reintroducing the unconditional revoke fails four of the six new tests.

**Not verified against a live backend**: the claim editor's write path, the simulator's payload, and the role→members and delete-grant endpoints have unit tests but have not been exercised against Postgres + Redis + Zitadel.
