> **Status:** ui-capability-gap-closure delta — a queue must say when it is not done | [< Index](../../../../INDEX.md)

# Requirement: Queue Outcomes and Filters (delta)

## MODIFIED Requirements

### Requirement: A drain pass MUST report every outcome that means "do this again"

`DrainResult` distinguishes five outcomes. `applied` and `failed` are terminal.
`requeued` and `errored` are not: a requeued row hit a transient error and will
be retried, and an errored row had its Zitadel outcome decided but could not
have that outcome persisted, so it stays `in_flight` until the next drain
reclaims it. `halted` means the pass stopped early and every row behind that
point is untouched.

Any surface that triggers a drain MUST report all of them, MUST state that
another pass is needed when `requeued` or `errored` is non-zero, and MUST NOT
present such a pass as a success.

#### Scenario: A pass that requeued everything is not a success

- **GIVEN** a drain returns `{applied: 0, failed: 0, requeued: 8, errored: 0}` with HTTP 200
- **THEN** the operator MUST be told 8 are still queued and to resume again
- **AND** the outcome MUST NOT render in the success tone

#### Scenario: A write whose outcome could not be recorded is named as such

- **GIVEN** a drain returns `errored: 1`
- **THEN** the operator MUST be told the write reached the identity provider but MkAuth could not record the outcome, and that resuming settles it

#### Scenario: Each halt reason is a different sentence

- **WHEN** a drain halts
- **THEN** `drain_in_progress`, `zitadel_offline` and `max_retries_exceeded` MUST each produce distinct copy

`drain_in_progress` is not an error — nothing was sent twice.

### Requirement: A filtered list MUST distinguish "empty" from "nothing matches"

Any list with a filter MUST render a different empty state when a filter is
active, and MUST offer a way to clear it.

On the triage queue this is not cosmetic: "everything is explained" and "nothing
matches these filters" differ by whether unexplained access exists that nobody
is currently looking at.

#### Scenario: A filtered triage queue does not claim the queue is clear

- **GIVEN** unexplained access exists in Metal Shop
- **WHEN** an operator filters to Printing Lab and no rows match
- **THEN** the screen MUST say nothing matches those filters, not that everything is explained

## ADDED Requirements

### Requirement: The audit log MUST be walkable past one page

`GET /api/v1/audit` MUST accept a keyset cursor as `before_at` + `before_id`,
and MUST order by `(created_at DESC, id DESC)` to match it. `limit` bounds one
page, not the readable history.

The cursor MUST be the tuple. `created_at` is the transaction timestamp, so a
cascade that writes several audit rows writes them at the identical instant; a
timestamp-only cursor would skip the remainder of that batch or return it
forever.

The audit screen MUST state which case it is in — more further back, or the end
of the log — rather than letting a "Load more" control quietly stop appearing.

#### Scenario: A cascade's audit rows all page correctly

- **GIVEN** one cascade wrote 8 audit rows in a single transaction, sharing a `created_at`
- **AND** a page ends in the middle of them
- **WHEN** the next page is requested with that row's `(created_at, id)`
- **THEN** the remaining rows of that batch MUST be returned exactly once

#### Scenario: The end of the log is stated

- **WHEN** a page returns fewer rows than the requested limit
- **THEN** the screen MUST say that is the whole log

### Requirement: An access request MUST carry the duration that was asked for, and show it to the decider

The request form MUST let a member choose how long they need access, in units a
member thinks in, and MUST show the resolved date. A fixed `duration_days` sent
regardless of the ask records something nobody asked for.

Every surface where an operator decides a request MUST display the requested
duration. A duration of zero or absent MUST render as "no end date" rather than
blank — the backend reads it as a grant that never lapses.

#### Scenario: A week is recorded as a week

- **GIVEN** a member asks for a week of laser access
- **THEN** the stored request MUST carry 7 days, not a default quarter
- **AND** the operator queue MUST show "for 7 days" on the row

### Requirement: A member MUST be able to see what exists before asking for it

The member view MUST list every project and the roles inside it, marking the
ones the member already holds rather than hiding them, and MUST offer a direct
route to ask for each of the others with the ask pre-filled.

This is the `projects` slice of `/catalog`, not `applications`: an application
is a token consumer, and nobody requests one.

#### Scenario: Asking does not require knowing the name first

- **GIVEN** a member has never used the laser cutter
- **WHEN** they open their access page
- **THEN** the laser cutter's project and roles MUST be listed with a way to ask for each
- **AND** following it MUST open the request form already naming that project and role
