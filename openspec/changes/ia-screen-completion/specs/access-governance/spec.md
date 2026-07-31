> **Status:** ia-screen-completion delta — cascade identity, drift evidence, triage ranking, upstream escape hatches | [< Index](../../../../INDEX.md)

# Requirement: Access Governance (delta)

## ADDED Requirements

### Requirement: Writes produced by one triggering event MUST share a cascade identifier

Every outbox row enqueued by one cascade computation MUST carry the same `cascade_id`. The id is minted once per enqueue call, which is the set of writes one triggering event produced — a chained rule contributes to the same closure diff and therefore the same call, not a second one.

Pending changes MUST group by it and state that grouped writes confirm together. Change history MUST render one entry per cascade rather than one per write.

Rows written before the column existed carry no id; readers MUST fall back to the row's own identifier so history stays complete rather than silently dropping them.

#### Scenario: Two writes from one rule firing are shown as one cascade

- **GIVEN** a rule fires and chains, producing two queued writes
- **WHEN** an operator opens Pending changes
- **THEN** both rows MUST show the same cascade id
- **AND** the queue MUST state that they confirm together or not at all

#### Scenario: Two independent firings are not merged

- **GIVEN** two separate enqueue calls, each producing one write
- **WHEN** the queue renders
- **THEN** neither row MUST be described as sharing a cascade with the other

### Requirement: A drift row MUST carry its upstream evidence, or say it has none

A drift item MUST record `upstream_actor` and `upstream_created_at` when the detector knew them, and MUST leave them absent when it did not. The webhook path knows both from the Zitadel event; the reconciliation sweep compares grant sets and knows neither.

The triage row MUST NOT infer or invent an actor. Where evidence is absent it MUST say what it does know — that the sweep found the grant and cannot see who made the change.

`last_seen_at` MUST be refreshed on every detection, including a deduped re-detection, so "is this still there?" is answerable for an old row. Re-detection MUST NOT overwrite known evidence with an unknown.

#### Scenario: A webhook-detected drift names its author

- **GIVEN** a grant created upstream by `svc-badge-sync` and reported by webhook
- **WHEN** the triage queue renders that row
- **THEN** the row MUST name `svc-badge-sync` and the date it was created

#### Scenario: A sweep-detected drift admits it cannot say who

- **GIVEN** a drift row detected only by the reconciliation sweep
- **WHEN** the triage queue renders that row
- **THEN** the row MUST state that the sweep compares grant lists and cannot see who made the change
- **AND** it MUST NOT name any actor

#### Scenario: A sweep re-detecting an attributed row keeps the actor

- **GIVEN** a pending drift row with `upstream_actor` set by a webhook
- **WHEN** the sweep re-detects the same triple
- **THEN** `upstream_actor` MUST be unchanged
- **AND** `last_seen_at` MUST be updated

### Requirement: The triage queue MUST be ordered by risk then age

The backend MUST order pending drift by risk descending then detection time ascending. Risk has exactly three tiers: a safety-gated role, a role no longer in the catalogue, and everything else. Safety-gated MUST be matched case-insensitively as a substring of the identity provider's own role group, not as an enum.

The UI MUST NOT re-sort the result. The row layout MUST NOT change with risk; only the ordering and a left border on the leading row may.

#### Scenario: A safety-gated role found yesterday outranks a wiki role found last week

- **GIVEN** a wiki-role drift detected seven days ago and a safety-gated drift detected one day ago
- **WHEN** the triage queue is read
- **THEN** the safety-gated row MUST come first

#### Scenario: Within one tier the oldest leads

- **GIVEN** two drift rows of equal risk detected nine and two days ago
- **WHEN** the queue is read
- **THEN** the nine-day-old row MUST come first

### Requirement: Adoption MUST record only the provenance it can create

Adopting drift writes a `direct_role_grants` row: MkAuth records the grant, the operator becomes granter of record, and nothing changes upstream. That is the whole of what the action does, and the recorded source MUST say only that.

`external_backfill` MUST be the only accepted attribution source. `bundle` and `rule` MUST be rejected with 400, and the rejection MUST explain why. The request MUST NOT carry a `source_ref`.

Cascades deliberately never write the ledger — a bundle's or rule's effect lives in the bundle and rule tables — so a `direct_role_grants` row labelled `source='bundle'` had no bundle assignment behind it and no rule-derived relationship. The access survived removal of the very bundle it named, while the ledger claimed that bundle managed it.

Routing adoption through real ownership is explicitly NOT the alternative. Assigning a bundle to explain one drifting role would grant every other role that bundle carries; making a rule produce the role would require granting the person the rule's input role, which is frequently safety-gated. Triage explains or removes access that already exists — it MUST NOT become a way to grant more.

#### Scenario: Bundle attribution is refused

- **GIVEN** a pending drift row
- **WHEN** an operator attributes it to `bundle`
- **THEN** the request MUST be rejected with 400
- **AND** no ledger row MUST be written
- **AND** the response MUST say that adoption cannot create a bundle assignment

#### Scenario: Rule attribution is refused

- **GIVEN** a pending drift row
- **WHEN** an operator attributes it to `rule`
- **THEN** the request MUST be rejected with 400 and nothing MUST be written

#### Scenario: Adoption records a direct grant with no owner reference

- **GIVEN** a pending drift row
- **WHEN** it is adopted
- **THEN** the ledger row MUST record source `external_backfill`
- **AND** it MUST carry no `source_ref`, because there is no owner to point at

### Requirement: Bulk resolution MUST report per-item outcomes

Both bulk endpoints MUST return the number that succeeded, the number that failed, and the identifiers of the failures.

The UI MUST announce the counts the server reported, never the count that was selected, and MUST retain exactly the failed rows in the selection.

A batch can partially fail — a row somebody else triaged a second earlier, a write that did not land. Announcing the selected count regardless tells an operator that twelve items are handled when eleven are, and the twelfth is unexplained access nobody is going back to.

#### Scenario: A partial batch reports what actually happened

- **GIVEN** two rows selected and one of them already resolved by another operator
- **WHEN** the operator bulk-adopts
- **THEN** the response MUST report one attributed, one failed, and name the failed id
- **AND** the screen MUST report one adopted rather than two
- **AND** the failed row MUST remain selected

#### Scenario: A wholly failed batch does not read as success

- **GIVEN** a batch in which every item fails
- **WHEN** the result is reported
- **THEN** the screen MUST state that nothing was resolved

### Requirement: Bulk resolution MUST cover adopt and mark-external, and MUST NOT cover revoke

The triage queue MUST offer bulk adopt and bulk mark-as-external. It MUST NOT offer bulk revoke, and MUST state that absence on the screen rather than leaving it unexplained.

Adopting and marking-as-external are reversible bookkeeping. Revoking removes real access from real machines, and reading a dozen consequences at once is not something anyone does — so revoke stays one row, one dialog, one decision.

#### Scenario: Selecting rows offers two bulk actions

- **GIVEN** four rows selected in the triage queue
- **WHEN** the bulk bar renders
- **THEN** it MUST offer adopt and mark-as-external for all four
- **AND** it MUST NOT offer revoke for all four

#### Scenario: The absence is explained on screen

- **GIVEN** the triage queue rendered
- **THEN** the screen MUST state that bulk revoke is deliberately not offered, and why

### Requirement: Direct writes to the identity provider MUST be disclosed before they are offered

The upstream console MAY expose grant assign/update/remove and project-role create/update/delete. Those controls MUST sit behind an explicit disclosure that, when opened, names all three consequences before any button: that the next cache compile can overwrite the change, that the drift sweep will report it as unexplained access created by an unnamed actor, and that Change history will not record it.

Every such control MUST carry the destructive tone regardless of the verb, and every confirming dialog MUST repeat the warning beside its button.

#### Scenario: The write section is collapsed by default

- **GIVEN** the Identity provider page rendered
- **WHEN** it first loads
- **THEN** no direct-write control MUST be visible until the operator expands the section

#### Scenario: Upstream reads are disabled with a stated reason when unreachable

- **GIVEN** the identity provider is unreachable
- **WHEN** the Identity provider page renders
- **THEN** the inspection actions MUST be disabled
- **AND** the reason MUST appear in visible copy, not only in a title attribute

## MODIFIED Requirements

### Requirement: Provider health MUST be reported as a sentence with a cause

Health MUST be rendered as a sentence carrying a cause and, where the provider is unreachable, what MkAuth is doing in the meantime. A coloured dot alone is not sufficient: it says something is wrong and nothing else.

#### Scenario: An unreachable provider explains itself

- **GIVEN** the identity provider is unreachable
- **WHEN** the Identity provider page renders
- **THEN** it MUST state that it is unreachable, give the reported cause, and say that writes stay queued and nothing is lost

### Requirement: A parked capability MUST read as unbuilt, not as idle

Hardware sync MUST render a dashed, explicitly-unbuilt panel. It MUST NOT render a spinner, an empty table, or a zero count — each of those implies a working feature with nothing to do. The intents ledger MAY be rendered only when it actually holds rows.

#### Scenario: No intents means no table

- **GIVEN** no provisioning intents exist
- **WHEN** Hardware sync renders
- **THEN** no intents table MUST be shown
- **AND** the panel MUST say the integration is not connected yet
