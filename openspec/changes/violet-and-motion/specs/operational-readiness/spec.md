> **Status:** violet-and-motion delta — the authenticated surface | [< Index](../../../../INDEX.md)

# Requirement: Operational Readiness (delta)

## ADDED Requirements

### Requirement: Colour MUST carry exactly one meaning each

The interface has five colour roles and no decorative colour. The accent means "you can act on
this"; it MUST NOT be used to mean "good" or "safe". The healthy role means "nothing needed here";
it MUST NOT appear as a button, and MUST NOT be used as a fill behind text.

The healthy role MUST be expressible only as a dot, a word, or a hairline. Its token set MUST NOT
provide an ink, soft or line sibling, so that a filled healthy surface cannot be assembled from
tokens at all.

#### Scenario: A resolved state is not an action

- **GIVEN** a surface reporting that nothing needs the operator — an empty queue, a reachable
  provider, or no expiring access
- **WHEN** it is rendered
- **THEN** it MUST NOT take the accent
- **AND** the healthy signal MUST be a dot, a word, or a hairline rather than a filled field
- **AND** the value being reported MUST remain in the primary text colour

#### Scenario: An empty list distinguishes resolved from absent

- **GIVEN** a list view with no rows
- **WHEN** the emptiness means work existed and none of it needs the operator
- **THEN** the state MAY carry the healthy signal
- **AND** when the emptiness means the thing has simply never existed, it MUST NOT
- **AND** the distinction MUST be declared by the calling view, never inferred from a count

### Requirement: Filled accent surfaces MUST clear AA for the text they carry

A label on an accent fill MUST meet WCAG AA contrast at the size it is actually rendered. The
primary accent fill does not clear AA below large text, so any fill carrying text smaller than
large text MUST use the dense accent instead.

#### Scenario: A small label never rides the bright accent

- **GIVEN** a filled control whose label is below the WCAG large-text threshold
- **WHEN** it is rendered
- **THEN** its fill MUST be the dense accent
- **AND** a fill carrying no text MAY keep the primary accent

### Requirement: Motion MUST be spent through named roles

Every transition and animation MUST come from the motion vocabulary, which pairs each duration with
its easing. A view MUST NOT declare a raw duration or a bare colour transition of its own.

Motion MUST NOT be used where no motion is specified: route transitions, table sorting and
filtering, counts and timestamps, and the contents of a dialog after it has opened all hold still.

#### Scenario: A row does not move when it is only hovered

- **GIVEN** a table or navigation row
- **WHEN** the pointer enters it
- **THEN** only colour and opacity MUST change
- **AND** the row MUST NOT lift, gain a shadow, or change its border

#### Scenario: A list arrives without punishing its length

- **GIVEN** a list view receiving its first data
- **WHEN** the rows are painted
- **THEN** they MUST rise in sequence
- **AND** the stagger MUST stop after the sixth row, so that a list of any length resolves in the
  same time as a short one

### Requirement: Only a state that is still changing MAY loop

A repeating animation means "this is still happening". Exactly two states are licensed to loop: the
degraded state, and a surface that is still loading. Nothing healthy and nothing decorative may
loop, and a healthy state MUST be the stillest thing on the screen.

#### Scenario: A pending control does not invent a second idiom

- **GIVEN** a control awaiting a response
- **WHEN** it signals that the work is still in flight
- **THEN** it MUST use the same looping treatment as the other licensed loops

### Requirement: A changed value MUST be marked once, and never on arrival

When a polled value changes while the operator is on the page, the row around it MUST be marked
once. The value itself MUST NOT roll, count, or tick.

The mark MUST NOT fire on first paint, and MUST NOT fire when a poll returns an unchanged value.

#### Scenario: A poll returning the same number says nothing

- **GIVEN** a polled count displayed in the navigation
- **WHEN** a refetch returns the value it already held
- **THEN** nothing on the row MUST change

#### Scenario: A placeholder is not a reading

- **GIVEN** a polled value backed by placeholder data, so that a fabricated zero is displayed before
  the first real payload
- **WHEN** the first real payload arrives carrying a different number
- **THEN** it MUST be treated as an arrival rather than a change
- **AND** no mark, and no audible signal, MUST fire

#### Scenario: A second change is marked in its own right

- **GIVEN** a value that changed and is still being marked
- **WHEN** it changes again before the mark has finished
- **THEN** the second change MUST be marked from the beginning
- **AND** it MUST NOT merely extend the first mark

### Requirement: A dialog scrim MUST fade without moving

The scrim is the full viewport and is the ancestor of the dialog card, so it MUST animate opacity
only. A transform on it displaces a viewport-fixed element beyond its own edges and compounds with
the card's own entrance, so that the dialog travels further than the system's maximum.

#### Scenario: The dialog rises by its own distance and no more

- **WHEN** a dialog opens
- **THEN** the scrim MUST NOT translate or scale
- **AND** the card's total travel MUST NOT exceed the system's maximum of 10px

### Requirement: Reduced motion MUST remove waiting as well as movement

When a visitor has asked for reduced motion, every duration, every delay and every loop MUST
collapse. A staggered entrance MUST NOT survive as a delay before an instant appearance.

No state may be conveyed by movement alone, so nothing is lost when it collapses.

#### Scenario: A staggered list does not become a staggered wait

- **GIVEN** a visitor whose system reports a preference for reduced motion
- **WHEN** a list view paints
- **THEN** no row MUST be held back before appearing
