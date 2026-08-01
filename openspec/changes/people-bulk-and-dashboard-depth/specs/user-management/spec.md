> **Status:** people-bulk-and-dashboard-depth delta — identity rendering, bulk operations, person surfaces | [< Index](../../../../INDEX.md)

# Requirement: User Management (delta)

## ADDED Requirements

### Requirement: An identifier MUST NEVER be rendered where a name belongs

No surface may display a Zitadel subject id, or any other opaque identifier, as a person's display name. This holds for the session's own identity as much as for anyone else's.

The session display name MUST be resolved by layering sources in descending order of authority, and MUST NOT fall through to the subject id at any point:

1. `name` or `preferred_username` from the token claims, when present and non-blank
2. `name` from `GET /api/v1/me/profile`
3. The local-part of the email address, title-cased
4. Empty

When the result is empty, consuming surfaces MUST render something else true — the email address, or nothing — rather than the id.

#### Scenario: A Zitadel access token carries no profile claims
- **WHEN** the access token has a `sub` but no `name` and no `preferred_username`
- **THEN** `extractSessionFields` returns an empty name
- **AND** the session falls back to the directory profile, then the email local-part
- **AND** neither the shell header nor the Today greeting renders the subject id

#### Scenario: The directory knows the name the token doesn't
- **WHEN** `/me/profile` returns a name for the authenticated principal
- **THEN** that name is carried into the session cookie rather than discarded

#### Scenario: Nothing knows a name
- **WHEN** every source is empty
- **THEN** the header renders the email address
- **AND** the greeting omits the address entirely rather than greeting an id

### Requirement: A catalog miss MUST be re-asked of the backend before being called unknown

`/catalog` describes the directory as it currently stands, so it cannot contain a deleted account, one created since the last fetch, or a machine principal. The resolver MUST attempt `POST /api/v1/lookup` for any id the catalog does not carry, batching concurrent misses into a single request, and MUST cache the outcome.

An id the backend also cannot place MUST NOT be re-requested for the remainder of the session, and MUST render a stated label ("Unknown account") with the raw id reachable only through `title`.

Misses beyond the backend's per-request batch ceiling MUST still be resolved: settled ids leave the queue so the next batch can proceed, and answers MUST accumulate across batches rather than being read off the most recent request.

#### Scenario: More misses than fit in one batch
- **GIVEN** more unresolved ids than the lookup batch ceiling
- **WHEN** the first batch settles
- **THEN** the remaining ids are requested in a subsequent batch
- **AND** the first batch's names are still resolved after the second lands

#### Scenario: An id the catalog doesn't carry
- **WHEN** a surface resolves a user id absent from the catalog
- **THEN** the resolver reports "still resolving" while the lookup is in flight
- **AND** renders the resolved name if the backend can place it

#### Scenario: An id nothing can place
- **WHEN** both the catalog and the lookup miss
- **THEN** the surface renders "Unknown account", not the id
- **AND** no further lookup is issued for that id

### Requirement: A bulk access change MUST be rehearsed against live state before it is applied

`POST /api/v1/grants/bulk` MUST default to a rehearsal that writes nothing. `?apply=true` executes.

The rehearsal MUST return one row per selected person stating what would happen and what that person is left holding. Rows MUST be classified as: will change, already in that state, or refused — with a reason. No selected id may be omitted from the response.

Apply MUST recompute the plan server-side and act on that. A plan supplied by the client MUST NOT be executed, and rows the recomputed plan classifies as refused or already-done MUST NOT be acted on.

Every mutation MUST route through the existing transactional enqueue or cascade services. No bulk path may call the Zitadel Management API from a handler.

A non-blank `reason` MUST be required by the endpoint, not merely by the dialog. A bulk change writes one audit row per person, so an unexplained one is an unaccountable change multiplied by the size of the selection.

#### Scenario: A request with no reason
- **WHEN** `reason` is absent, empty, or whitespace only
- **THEN** the request is rejected with a field-level validation error
- **AND** nothing is rehearsed and nothing is written, whether or not `?apply=true` was set

#### Scenario: A departed account is in the selection
- **GIVEN** a cohort assembled from a filter that a departed account also matches
- **WHEN** an additive operation is rehearsed
- **THEN** that person's row is refused, naming the account status
- **AND** applying the plan does not grant them access

#### Scenario: Removing a role somebody holds two ways
- **GIVEN** a person holds a role both directly and through a bundle
- **WHEN** `remove_role` is rehearsed
- **THEN** their row states that the direct grant is removed
- **AND** states that they keep the role via the bundle, and that their access does not go away

#### Scenario: Removing a role somebody holds only through a rule
- **WHEN** `remove_role` is rehearsed for a person with no direct grant
- **THEN** their row is "no change", pointing at the source to remove instead
- **AND** carries no grant for the apply pass to act on

#### Scenario: Extending access that does not expire
- **WHEN** `extend` is rehearsed against a permanent direct grant
- **THEN** that grant is excluded — extension never converts "no expiry" into a deadline

#### Scenario: One person's write fails mid-batch
- **WHEN** a write fails for one person during apply
- **THEN** that row is marked failed with its cause
- **AND** the remaining rows are still attempted
- **AND** the summary counts successes and failures separately

#### Scenario: A selection larger than the ceiling
- **WHEN** more than `BulkMaxUsers` people are submitted
- **THEN** the request is rejected with a field-level validation error

### Requirement: People filters MUST live in the URL and address projects by id

The People index MUST read `q`, `project`, `role`, `bundle`, `attention` and `bulk` from the query string, and MUST match projects on id rather than display name so a link survives a rename. `UserListItem` MUST therefore carry `key_project_ids` alongside `key_projects`.

Role membership MUST come from the role-members endpoint rather than being inferred from the people list, and rows MUST be left unfiltered while that request is in flight.

An unrecognised filter value MUST degrade to "no filter" rather than to an empty result.

#### Scenario: Arriving from a link while role membership loads
- **WHEN** People is opened with `?project=…&role=…` and membership has not arrived
- **THEN** every row is shown
- **AND** the list is narrowed once membership resolves — an empty list would claim nobody holds the role

#### Scenario: A stale or hand-edited attention value
- **WHEN** `?attention=` carries a value the model does not define
- **THEN** the filter is treated as unset

### Requirement: Bulk selection MUST be opt-in, scoped in words, and dropped when its meaning changes

Checkboxes, the selection bar, and every bulk verb MUST be absent until bulk mode is enabled. Enabling it MUST NOT change the information architecture of the list.

Select-all MUST cover every row matching the current filter, MUST state the count and the filter in words, and MUST offer to narrow to the rendered page instead.

The selection MUST be cleared when bulk mode is left or when the active filter changes, because a filter change re-aims a pending action at a different set of people.

#### Scenario: Bulk mode is off
- **THEN** no checkbox is rendered anywhere on the page
- **AND** each row remains a link to the person

#### Scenario: Select-all across a filter wider than the page
- **WHEN** select-all is used with 60 matching rows and 50 rendered
- **THEN** all 60 are selected
- **AND** the bar states "All 60 people selected" with the filter named
- **AND** offers to select only the 50 shown

#### Scenario: The filter changes while a selection is live
- **THEN** the selection is dropped

### Requirement: A person's Requests and Activity MUST be surfaces, not signposts

Neither tab may consist solely of a link elsewhere.

**Requests** MUST show that person's full request history including decided requests, because the shared queue holds only pending work and therefore cannot answer what was decided. An operator MUST be able to approve or deny from the tab without navigating.

**Activity** MUST be filtered server-side via `GET /api/v1/audit?user_id=`, matching entries where the person is the actor **or** the target, and MUST distinguish the two directions in the row so the person is never implied to have acted when they were acted upon. When the response reaches the backend's cap, the tab MUST say the history is partial rather than presenting it as complete.

The audit vocabulary MUST be shared with the audit log, so one event cannot read differently on two screens.

Because this route also serves a member reading their own record, and the audit endpoint is operator-gated, the Activity tab and any link into the audit log MUST NOT be rendered for a non-operator. A member's page MUST NOT issue the audit request at all. Requests remains available to both audiences, because that endpoint accepts self-reads.

#### Scenario: A member opens their own record
- **THEN** Access and Requests are offered
- **AND** the Activity tab and the audit-trail link are absent
- **AND** no request is made to the audit endpoint

#### Scenario: A person with a decided request
- **THEN** the Requests tab shows the decision and any review note

#### Scenario: A grant made to the person by an operator
- **THEN** the Activity row names the operator as the actor
- **AND** does not read as though the person performed the change

#### Scenario: A person with more history than the cap
- **WHEN** the response reaches the 200-entry cap
- **THEN** the tab states that this is not their whole history and links to the full log
