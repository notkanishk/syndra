> **Status:** ia-screen-completion delta — the People index carries what needs attention | [< Index](../../../../INDEX.md)

# Requirement: User Management (delta)

## ADDED Requirements

### Requirement: The People index MUST carry what needs attention, not only what people hold

Each row MUST surface the one thing about that person that might need an operator today: unexplained access, an approaching expiry, or an open request — each rendered in its own semantic colour and each stated in words as well as colour.

Where more than one applies the row MUST show the most serious, in that order. A row with none MUST render a faint dash rather than nothing at all.

Without this column the screen is a directory, and a directory does not warrant a top-level destination.

#### Scenario: The most serious signal wins

- **GIVEN** a person with one expiring grant, one open request and one unexplained item
- **WHEN** the People index renders
- **THEN** the row MUST report the unexplained item
- **AND** it MUST NOT also report the request

#### Scenario: A clear row says so

- **GIVEN** a person with nothing needing attention
- **WHEN** the row renders
- **THEN** it MUST render a dash rather than an empty cell

### Requirement: People search MUST match role keys

Search MUST match a person's name, email and the keys of roles they hold. "Who has `trained` in the laser lab" is typed on this screen before anyone thinks to go to Roles, and a search that silently ignores role keys reads as broken.

#### Scenario: Searching a role key returns its holders

- **GIVEN** one person holding `trained` and one holding nothing
- **WHEN** the operator searches `trained`
- **THEN** only the holder MUST be returned

### Requirement: A departed account MUST stay visible at reduced contrast

An account that is no longer active MUST remain in the list and render at reduced contrast. It is frequently exactly who somebody came looking for, and filtering it out turns an answer into a dead end — but it must never be mistaken for a live person.

#### Scenario: A departed row is dimmed, not hidden

- **GIVEN** a departed account matching the current filters
- **WHEN** the index renders
- **THEN** the row MUST be present and visually reduced

### Requirement: The People index and Expiring access MUST work to a 30-day window

Review › Expiring access MUST read its own endpoint with its own window, defaulting to 30 days, and the People index MUST count against the same window. Today MUST keep its narrower window: its job is a queue short enough to finish, and a 30-day list is a review rather than a queue.

#### Scenario: The two screens do not contradict each other

- **GIVEN** a grant expiring in 24 days
- **THEN** it MUST appear in Review › Expiring access and in the People index count
- **AND** it MUST NOT appear in Today's expiring block

### Requirement: Expiring access MUST offer exactly one action, and emphasise only the soonest row

The screen MUST offer extend and nothing else. It MUST NOT offer a dismiss, acknowledge or let-it-lapse control: such a control would submit nothing while making an operator believe they had recorded a decision.

Only the soonest row may carry the deadline emphasis. Amber is a deadline signal, not a decoration for the whole table.

Each row MUST state who granted the access and when, so extending is a decision rather than a guess.

#### Scenario: Only the first row is emphasised

- **GIVEN** four grants expiring in 2, 2, 11 and 24 days
- **WHEN** the screen renders
- **THEN** only the soonest row MUST carry the amber treatment
