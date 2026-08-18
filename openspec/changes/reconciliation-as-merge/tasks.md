# Tasks

## 0. The prerequisite this change owns

The design assumed `apply` already read back what it wrote. It does so on the
create path only; `convergeExisting` fingerprints a projection built from the
REQUESTED values. Nothing downstream can be built on that, so it is fixed here
and first.

**Shipped as its own change: `apply-reads-back-what-it-wrote`.** 0.1–0.3 are done and
mutation-checked there; they are left here because this change's foundation is
what they are, and a prerequisite that vanishes once ticked is one nobody can
audit later.

- [x] 0.1 **Every successful mutation is followed by a read of the account as the target then holds it.** `convergeExisting` calls `readBack` after `user.update` and fingerprints the READ, not `applied := *current` with the requested fields written over it. The create path's own comment already states the rule this breaks — "a fingerprint computed from a state this add-on invented is a fingerprint the next plan verifies against nothing"
- [x] 0.2 **The observed managed-field values travel in the apply response** as an `observed` map beside the fingerprint. The one defect class this boundary has produced twice is a field one side sends and the other's decoder refuses, so the backend's own decoder is TESTED against an outcome carrying it rather than assumed to tolerate it
- [x] 0.2a A response-direction contract artifact (`addons/contract/apply_response.json`), held from both ends like the other three and running the other way, because the add-on is the producer on the reply leg. Landed ahead of the consumer after all: the safety of adding `observed` rested on "the backend decodes leniently", which was true and was written in a comment — the form an assumption takes right up until it is wrong. Pins the successful shape only; the refusal, conflict and unverified shapes are mutually exclusive with it and are asserted in the suites that produce them
- [x] 0.3 **A failed read-back produces no base, and never a base from intent.** The outcome stays `applied` — the write happened — with no `observed`; the backend records nothing and the subject converges two-way on the next pass, exactly as one that has never been applied. An add-on too old to send `observed` lands in the same state
- [x] 0.4 Test: an update whose read-back fails records NO base — the backend half, which needs the base store to exist. The add-on half (no fingerprint, no observed, one write) is covered in `apply-reads-back-what-it-wrote` 2.4. This is the failure mode the whole change exists to prevent, arriving through the error path, and it is the one a green suite would most plausibly miss

## 1. The base

- [x] 1.1 A `merge_base` per `(subject, target)`, written from the observed values of 0.2. Intent MUST NOT be recorded as the base: a base equal to `OURS` by construction can never produce a conflict, which is today's behaviour with more machinery
- [x] 1.2 Migration adds the column; every existing binding has none, and a missing base converges exactly as today. Inventing one either fabricates agreement or raises a conflict for every managed subject on the first pass
- [x] 1.3 The base is recorded on `already merged` too, or a hand-made change that matched Syndra's intent is re-detected forever
- [x] 1.4 A missing base selects two-way convergence only for a PRESENT account. An absent one is `deleted upstream` with or without a base, and is never queued for creation — that guard is what stopped stub-era bindings recreating accounts on a production NAS, and "no base, converge" hands it back for exactly the bindings that predate the base. Tests for both halves of the pair

## 2. Classification

- [x] 2.1 Per-FIELD classification into the six outcomes, over managed fields only. A whole-account comparison manufactures conflicts between a group change and a password change, which touched nothing of each other's
- [x] 2.2 `already merged` is evaluated before `theirs-only`. Somebody who made the change Syndra was going to make has not drifted, and telling them so is how a system trains people to ignore it
- [x] 2.3 A sweep applies `fast-forward` and records `already merged`, and resolves nothing else
- [x] 2.4 Tests for all six, including the two that are indistinguishable without a base — `theirs-only` versus `conflict` — which is the pair that justifies the whole change

## 3. Findings and resolution

- [x] 3.1 `conflict`, `deleted upstream` AND `theirs-only` become durable operator findings carrying all three values, because "what did it used to be" is the question an operator asks first and cannot currently answer
- [x] 3.1a `theirs-only` is durable in its own right, deduplicated per `(subject, target, field)`, with the same lifecycle as any other finding: raised once, carried until resolved, resolved by an audited choice. It was the state most likely to be left as sweep output — the design called it triage while this list created findings only for the other two — and it is the single most common real one, because it is what a hand edit on the NAS looks like. Ephemeral, it is visible to whoever ran the pass and to nobody else; six-hourly, it would raise four a day for one unresolved edit
- [x] 3.2 Resolutions bounded by where desired state actually lives, each recording the base afterwards. Keep-ours always; take-theirs ONLY where the target's value is expressible for one subject; change-the-policy for everything else
- [x] 3.3 Take-theirs changes the DESIRED state, not an ignore flag. An ignore re-raises next sweep; adopting agrees
- [x] 3.3a **Take-theirs on a lifecycle field whose target value is restrictive is written as a deny allowance** — the mechanism that layer was built for. Adopting through it is a strict improvement on the hand edit: the schema refuses a deny without an actor, a reason, and an expiry or review date, so the suspension acquires all three
- [x] 3.3b **Take-theirs is NOT offered for a group value, or for a lifecycle field whose target value is permissive.** `group` comes only from `target_role_mappings`, which has no subject column — its DDL says editing it "silently changes what every holder of that role can reach" — and the lifecycle fields are refused as mapping fields at three layers because they are derived. There is nothing that can hold either value for one subject
- [x] 3.3c **Change-the-policy replaces the unowned half.** The finding names the mapping that produces the value and how many people hold that role (`GET /targets/mappings/{id}/holders` already answers it), and the operator edits that mapping seeing the blast radius. An operator who wants one person changed is told the truth: in this model that is a role question
- [x] 3.3d **No per-subject additive override is introduced.** `allowances` has the column and the code refuses the arm on purpose (`ErrAllowanceAdditiveUnsupported`); 000030 defers it until a second consumer makes the abstraction real. Building it so a conflict dialog has a convenient button would be the worst available reason, and it would put a grant where no access review looks
- [x] 3.3e A finding whose only honest resolution is "change the policy" is NOT dismissible. Dismissing it is the ignore flag 3.4 refuses, wearing a different label
- [x] 3.4 No automatic resolution, and no flag for one — a flag would be set on the deployment where being wrong costs most

## 4. Surfaces

- [x] 4.1 Conflicts AND theirs-only findings on the target page beside the drift count, with the three values shown. Distinct rows, not one merged count: "somebody changed this on the NAS" and "we each changed it differently" are different sentences and different next actions
- [x] 4.2 The governance summary counts all three kinds, so none can sit unseen behind a landing page that says nothing needs you
- [x] 4.3 The audit record names which side won and who chose it — today it records only that a value changed

## 5. What the review found after the three phases shipped

- [x] 5.1 **`deleted_upstream` was returned and written nowhere.** It lived exactly as long as the HTTP request that carried it — gone on refresh, absent from the decision queue, uncounted by governance — which is the failure this table exists to prevent, reintroduced on the one outcome that names a deleted account. Persisted through the same deduplicated path; a present account closes any standing one, because that slot is keyed on the empty field and no field loop would ever clear it
- [x] 5.2 **Unbinding forgot only Syndra's side.** The add-on kept its binding and went on planning and applying an account every surface here called unmanaged — the split `account.release` exists to close, recreated by the resolution path. It now dispatches the release first and forgets nothing unless the add-on confirms. The call happens BEFORE the transaction: an add-on that takes thirty seconds to answer would otherwise hold the access lock for thirty seconds
- [x] 5.3 **Resolutions are checked against the finding.** The API accepted `unbound` for a value disagreement, which would stop Syndra managing an account sitting right there. Which buttons a surface renders is not validation
- [x] 5.4 **The impossible adoption is no longer offered.** The list says, per finding, whether the value can be adopted at all, and when it cannot, which mapping produces it and how many people hold that role (3.3b/3.3c). Computed on the backend: which fields have a per-subject home is a fact about the entitlement model, and a copy of that rule in a component is a second definition that disagrees the first time the model grows a field
- [x] 5.5 **A finding that could not be written marks the target unreconciled**, as the Zitadel sweep already did with its own. A pass that lost a finding was marking itself clean over findings nobody would ever see
- [x] 5.6 **A decision is not a settlement.** Keeping Syndra's state queues a convergence; the difference is still there when the request returns, and closing the finding then let the next sweep raise a second one about the same field — one decision refilling the queue every six hours until the drain caught up. Decisions are recorded on the standing row, which closes when a pass sees the two sides agree, carrying the decision and its author rather than the anonymous `agreed`. Unbinding settles immediately, because it leaves nothing to observe

## 6. Applied history, so a removal is not a stranger

- [x] 6.1 **A `syndra_only` row carries the history of the grant it is about.** Produced by comparing two sets, it read as an unexplained absence: nothing said Syndra granted this deliberately, who did, why, or that Zitadel was holding it this morning. The row's own sentence called every one of them "a queued write that never landed", which is the wrong half of the story for a grant somebody undid
- [x] 6.2 **Two sources, each answering half.** The ledger says who decided the access, when, why, and whether a person granted it or a rule derived it; the merge base says when a complete read last saw the target HOLDING it. Computed on read and never stored — a copy would be a second account of the same history, free to disagree with the row it came from
- [x] 6.3 **A grant nobody observed is dated with nothing**, rather than with its subject's observation time, which would be a claim about a grant nobody saw. `target_only` rows carry no provenance at all: that is access Syndra has no record of, and any history attached to one would be somebody else's
- [x] 6.4 **Detection moves to the moment it happens.** `grant.removed` raises the finding directly, carrying the editor from the event — the one thing a set comparison can never supply. Self-mutation events are dropped at the door, and the check runs BEFORE the cascade, whose revocations remove derived intent: asking afterwards would answer about the world the cascade made rather than the one the removal happened in
- [x] 6.5 **A decided finding is not mutable.** The surface hid its controls after a decision and that was the whole protection: the read dropped the decision fields, so a second request saw an undecided finding and the write overwrote the first answer — for `unbound`, releasing the account on the target while the first answer's re-provision sat in the outbox. The decision is now an atomic reservation (`decision IS NULL` in the UPDATE), taken BEFORE the target is called and given back if that call fails, so one unreachable add-on cannot wedge a finding as decided-but-never-done. A second answer is refused with 409 naming who decided
- [x] 6.6 **The policy hint names only the mappings this subject's roles produce.** Mappings are per role, so listing every mapping on the field presented unrelated policies as the thing to edit, with holder counts belonging to people the finding is not about — a blast radius read off the wrong mapping is worse than none, because it is a number somebody acts on. An unreadable role graph names NO policy: "we could not tell which" and "all of them" are different answers
- [x] 6.7 **The memory that a write LANDED.** `target_propagations` records the moment a target ACCEPTED a write, per (target, subject, field). It is not a merge base — a base is what the target was seen HOLDING at a read, this is what it accepted at a write — and they disagree in the case that matters: a grant applied at noon and removed at one is invisible to every observation and plain here. Without it, that grant's absence read as a write that never landed and was replayed, restoring access somebody removed on purpose. Only what ADDS access is remembered; a revoke landing is Syndra removing something, and remembering it as applied would make the next pass argue the target should still hold it
