# Build notes — reading this bundle against the Syndra codebase

Written after reading the repo. **This file overrides the design README where they
disagree**, because the README was written before the code was visible.

Everything here is about *how to land these designs in this codebase*. The design
reasoning stays in `README.md`; the board stays in `design/Syndra IA.dc.html`.

---

## 1. Never paste a hex from the board — it will fail the build

`ui/src/__tests__/design-system.test.ts` is a canary that fails on "a raw hex
colour pasted from the design board instead of the token that carries it in both
themes". The board is authored in hex because it is a static document; the
application is authored in tokens, and **only `ui/src/app/globals.css` may define
them**.

Translate as you build:

| Board hex | Token |
| --- | --- |
| `#7f5af0` violet fill | `--color-accent` |
| `#9b7bff` accent text on dark | `--color-accent-text` |
| `#f7f4ff` label on accent | `--color-accent-ink` |
| `rgba(155,123,255,.15)` accent tint | `--color-accent-soft` |
| `rgba(155,123,255,.3-.55)` accent hairline | `--color-accent-line` |
| `#a3e635` healthy | `--color-healthy` |
| `#f5a524` amber | `--color-warn` / `-text` / `-ink` / `-soft` / `-line` |
| `#ff5c4d` red | `--color-danger` / `-text` / `-ink` / `-soft` / `-line` |
| `#f3f5ef` primary text | `--color-ink` |
| `rgba(243,245,239,.62)` | `--color-muted` |
| `rgba(243,245,239,.42)` | `--color-faint` |
| eyebrow / label text | `--color-label` |
| `#080906`, `#101210`, `#141612`, `#0b0c0a` | the `--ground` / `--canvas` / `--surface-0..2` / `--rail` scale — **read the theme block and pick by role, do not match by eye** |
| row hover `rgba(255,255,255,.035)` | `--color-tint-1..3` |
| hairlines and dividers | `--color-line`, `--color-line-strong`, `--color-divider` |

**Two things globals.css already solves that the design README worried about:**

- **`--accent-dense` exists** specifically "for when the label riding on it is
  small". The README's note about small text on `#7f5af0` failing AA and needing a
  darker fill is already handled — use `--accent-dense`, do not invent a value.
- **`--color-healthy` is deliberately a single token** with no `-soft`, `-ink` or
  `-line` sibling, because there is no such thing as a healthy button. That is the
  same rule the board states in words; here it is enforced by the token set giving
  you nothing to build one out of.

The comment block at the top of `globals.css` already states the five semantic
roles in the same terms as the design README. They agree. Trust the CSS.

---

## 2. §23 was wrong about which way the reconciliation goes

**I previously said §23 was canonical and the existing screens should migrate onto
it. That was wrong — I had not seen the code.**

`ui/src/components/ui/RehearsalDialog.tsx` already exists and is described in its
own header as *"Rehearse, then apply — the shape every bulk operation in the
product wears."* It is used by bulk grants (`people/BulkDialog.tsx`), request
decisions (`requests/RequestsScreen.tsx`), drift resolution
(`review/UnexplainedAccess.tsx`), bundle publishing
(`bundles/PublishVersionDialog.tsx`) and holder moves (`bundles/BundleVersions.tsx`).

**Build entitlement plan-then-apply by adding a caller to `RehearsalDialog`, not
by building the screen in §23.** `useRehearseEntitlements` in
`lib/queries/useEntitlements.ts` already calls
`/targets/{target}/entitlements/rehearse`.

Where the real component is better than the board, follow the component:

- **Blast radius is a backend refusal, not a checkbox drawn upfront.** The dialog
  rehearses with `acknowledge_scope: false`, and a refusal produces a dedicated
  **scope step** — *"This would change access for N people… Anything above {limit}
  is confirmed separately."* Then it re-rehearses with `true`. That is a better
  design than §24's pre-drawn checkbox because the threshold lives in one place
  and the operator only meets the ceremony when it is warranted.
  **Apply §24's mapping-management acknowledgement the same way.**
- **Stale-plan handling is richer than §23 draws it.** There is a `PLAN_STALE`
  code set, an `alert` (not `status`) live region, and it names which rows moved
  via `movedLabels`. §23's "Plan again" is the right idea; the implementation is
  already there and more careful.
- **The steps are `compose → scope → review → done`**, with ledes already written.
  §23's four figures map onto `review` and `done`.

What §23 contributes that is worth keeping: rows needing nothing are counted
rather than hidden; the button names the count; the provisional-vs-blocked rule
in §31 A.

---

## 3. The lost-card question is answered — the backend can drain inline

Open item 13 in the design README asked whether the backend can dispatch from the
mark-lost call. **It can.** Evidence in the repo:

- `services/planapply/apply_test.go` — `TestTheResponseSaysWhetherItDrainsOnItsOwn`
- `handlers/access_flow_test.go` — `TestHandleUpsertUserDirectGrant_ApplyNowDrains`
- `handlers/revocation_test.go` — `TestARevocationSaysItDrainsWithoutAnOperator`
- `handlers/drift_test.go` — `TestHandleRevokeDrift_EnqueuesRevokeAtomicallyThenDrains`
- Routes: `POST /api/v1/propagations/drain` and
  `POST /api/v1/targets/{target}/propagations/drain` (`handlers/router.go`)

So **build §28's "Mark lost and resume the drain now" as drawn.** The §31 §C
fallback copy is not needed.

One refinement the code makes possible: the apply response already reports whether
it drained on its own, so the confirmation can say which happened rather than
guessing — *"Marked lost and dispatched"* versus *"Marked lost — queued, the drain
did not run"*. Prefer that over a fixed string.

Note also `lib/drain-outcome.ts` and `lib/drain-toast.ts` already exist for
rendering drain results. Reuse them.

---

## 4. Where each design section lands

| Board | Route / component in the repo |
| --- | --- |
| §01–§02 nav, view switch | `lib/nav.ts`, `lib/ui-view.tsx`, `components/shell/` |
| §03 Access source | `components/access/` |
| §04 Today | `app/page.tsx`, `components/home/` |
| §05 People, person access | `app/users/`, `components/people/` |
| §06 Role members | `app/roles/`, `components/roles/` |
| §07 Projects / Roles / Apps | `app/projects/`, `app/roles/`, `app/applications/`, `components/apps/` |
| §08 Bundles, expiry | `app/bundles/`, `components/bundles/` |
| §10 Requests | `app/requests/`, `components/requests/` |
| §11 Four list states | `components/states/` |
| §14–§15 drift, reconciliation, expiring | `app/review/`, `components/review/` |
| §16 Bundles, confirmation policy | `app/bundles/`, `app/policies/` |
| §17 Automation | `app/automation/`, `app/operations/` |
| §18 Audit, IdP, events | `app/audit/`, `app/zitadel/`, `components/upstream/`, `components/audit/` |
| §19 nav delta | `lib/nav.ts` — **navigation structure lives only here** |
| §20, §30 member TrueNAS + connecting | `app/storage/`, `components/storage/MyStorage.tsx` |
| §21 target overview | `app/system/`, `components/targets/TargetOverview.tsx` |
| §22 withdrawn access | `app/governance/`, `components/review/WithdrawnAccess.tsx` |
| §23 plan-then-apply | **`components/ui/RehearsalDialog.tsx` + `lib/queries/useEntitlements.ts`** |
| §24 mapping management | new, under `components/targets/`, using `RehearsalDialog` |
| §25 holds, review due | `app/governance/`, `components/review/` |
| §26 My access, doors, card status | `components/member/` |
| §27 take away on a target | new dialog; follow `RehearsalDialog`'s modal shell |
| §28 door cards | `components/people/` (person page) + `components/targets/` (Unifi page) |
| §29 dormant accounts | `components/targets/`; **listing endpoint still to build** |
| §31 three patterns | shared — see §5 below |
| §32 motion | tokens in `globals.css`; `lib/useFlashOnChange.ts` already exists for the flash token |

---

## 5. §31's patterns, and what already exists for them

**Read freshness.** No shared component found. Build one — it is used by §21
health and inventory, §23 plans, §29 dormant. Five states, always an age.

**The acknowledgement ladder.** Rung 2 is `RehearsalDialog`'s scope step (see §2 —
build it as a refusal, not a checkbox). Rung 3 (type-the-name) needs a shared
confirm control; the board now uses it in four dialogs (§14, §16, §27, and §06
once its endpoint exists). Build it once.

**Urgency.** See §3 — the inline drain exists. `lib/drain-outcome.ts` and
`drain-toast.ts` are the existing rendering path.

**Blocked destructive controls.** The board's dashed treatment (§06, §03) marks
"no endpoint yet". Two of these are waiting on
`DELETE /api/v1/users/{id}/grants/{grantId}`. A disabled control **states its
reason as text, never a `title` tooltip** — the board has zero tooltips left, keep
it that way.

---

## 6. Conventions this bundle must not break

From `CLAUDE.md`, and all of them constrain the UI work:

- Design tokens **only** in `ui/src/app/globals.css`; both themes authored in full.
- Navigation structure **only** in `ui/src/lib/nav.ts`.
- **Structure never moves in response to data** — a section with nothing in it
  keeps its seat and shows a hollow zero. This is why target rows carry no badge
  (§19) and why the member storage row is ungated.
- Backend is the single mutation authority. Every screen in §19–§30 signals; the
  backend decides. `summary.succeeded` is always zero on apply responses.
- Strict JSON decoding on mutation endpoints — a UI sending an unexpected field
  gets a refusal, not a silent ignore.

**Workflow, per `CLAUDE.md`:** consult `openspec/` before starting, and after any
change update the relevant `openspec/changes/**` proposal, tasks, design and specs,
then run `bun run test && bun run lint && bun run build` in `ui/`. Treat these
designs as input to a change proposal, not as a licence to skip one.

Existing specs worth reading first: `openspec/changes/basic-advanced-ia/design.md`
(the navigation contract and where the token shape lives) and `openspec/NEXT.md`
(the live gap list).

---

## 7. Still genuinely open

1. **`GET /api/v1/targets/{target}/accounts/dormant`** does not exist. Shape is
   specified in §29 and in the design README; approved by the product owner.
2. **`DELETE /api/v1/users/{id}/grants/{grantId}`** does not exist. Two blocked
   controls are drawn waiting for it (§03, §06).
3. **Sequencing: TrueNAS first.** Unifi Access is designed (§26 doors and card
   line, §28 cards) but is the later phase.
