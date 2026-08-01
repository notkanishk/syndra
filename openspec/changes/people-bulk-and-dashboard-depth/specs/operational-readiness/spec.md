> **Status:** people-bulk-and-dashboard-depth delta — Today's two-zone contract | [< Index](../../../../INDEX.md)

# Requirement: Operational Readiness (delta)

## MODIFIED Requirements

### Requirement: Today MUST put work first and MUST NOT be empty when there is none

Today's previous contract was "actionable work only — no counts you cannot act on, no charts". That rule was correct about the top of the page and incorrect about the rest of it: it assumed a non-empty queue, and on a day with no open requests and nothing expiring it produced a landing page consisting of one sentence. An operator who lands on an empty page goes hunting through the navigation, which is the behaviour this page exists to prevent.

The contract is replaced with two zones in a fixed order.

**The work zone comes first and never moves.** Open requests and expiring access in Basic; pending changes and unexplained access appended in Advanced. Nothing may be inserted above this zone, and nothing below it may displace it. When the queue is empty it MUST collapse to a single line rather than a full-height empty state, because the page continues underneath and a hero-sized empty state reads as the end of it.

**The second zone is always present**, including when the queue is empty. It MUST carry: the two lifecycle gaps (people with no access at all; departed accounts still holding roles), the health of the machine (identity provider reachability, queued writes, unexplained access, expiring grants), where access actually lives (roles by headcount, plus catalogue entries nobody holds), and recent activity with names resolved.

The surviving half of the original rule is binding on this zone: **every number MUST be a link into the thing it counts.** No charts, no trends, and nothing that can only be looked at. A count with no destination is prohibited here exactly as it was before.

A zero MUST be stated in words rather than rendered as a bare "0" where the absence is the reassuring outcome, and a health cell MUST take its semantic colour only when the thing it reports is actually wrong.

#### Scenario: A day with no work
- **WHEN** there are no open requests and nothing expiring
- **THEN** the work zone collapses to one line confirming the queue is clear
- **AND** the second zone still renders in full

#### Scenario: A day with work
- **THEN** the work blocks render above the second zone
- **AND** no element of the second zone appears above them

#### Scenario: A count in the second zone
- **WHEN** any number is rendered
- **THEN** it links to the surface that lists what it counts

#### Scenario: No lifecycle gaps
- **THEN** the absence is stated in words ("Everybody here has something.") rather than as a zero

#### Scenario: The identity provider is reachable
- **THEN** the health cell reports it without taking a semantic colour — only a genuine problem is coloured
