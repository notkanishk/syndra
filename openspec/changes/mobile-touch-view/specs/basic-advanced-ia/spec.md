# basic-advanced-ia — touch delta

## ADDED Requirements

### Requirement: The application serves three device states from one tree

The UI MUST serve phone, tablet and desktop from the same routes and the same
components. It MUST NOT introduce a phone-only route group, a phone-only
component tree, or a second implementation of any behaviour that exists on
desktop.

Breakpoints MUST be declared as design tokens in `globals.css` and named for
devices: `--breakpoint-tablet` at 45rem and `--breakpoint-desktop` at 67.5rem.
Phone is the unprefixed base.

#### Scenario: A fix reaches both surfaces

- **WHEN** a defect is fixed on a screen at desktop width
- **THEN** the same component serves the phone, so the fix reaches it without a
  second edit
- **AND** no parallel implementation exists that could retain the defect

#### Scenario: Structural decisions are observable

- **WHEN** the shape of navigation or the dialog/sheet decision is made
- **THEN** it is decided in JavaScript from a media query result, so it can be
  asserted by a test
- **AND** presentation-only reflow is left to CSS, which jsdom cannot observe
  and which is verified in a browser instead

### Requirement: Touch targets and type have floors that drawings cannot lower

Every interactive element MUST present a hit area of at least 44px, 50px where
the action is destructive, and 52px for a copy row. The whole row MUST be the
hit area, never a nested chevron or checkbox alone. No rendered text may fall
below 12.5px.

Where a design drawing specifies a smaller value, the floor MUST win. Drawings
are authoritative about structure and unreliable about measurement.

A destructive control MUST be separated from a benign one by at least 12px, with
the benign control nearer the screen edge.

#### Scenario: The view switch is reachable

- **WHEN** an operator taps the Basic/Advanced pill on a phone
- **THEN** the control presents at least 44px of hit area, though the board
  draws it at 34px

#### Scenario: A row grows rather than clipping

- **WHEN** a row's content or the platform's dynamic type setting needs more
  height than the stated minimum
- **THEN** the row grows, because the stated height is a floor and not a fixed
  height

### Requirement: Navigation shape follows the destination count

Navigation MUST be built from `lib/nav.ts` without a second declaration of
structure. Where the destination count allows it, navigation is a bottom tab
bar; where it does not, it is a sheet rising from the bottom edge.

There MUST NOT be a "More" tab. Where a nav group's children cannot each hold a
tab, the tab MUST land on the group's first child and offer its siblings as a
segmented control in the page header, keeping the group's own label.

#### Scenario: Six indicators, four slots

- **WHEN** more counted indicators exist than the tab bar has room for
- **THEN** the entry to the nav sheet shows one dot in the highest tone present
  and a count of destinations wanting attention
- **AND** it MUST NOT sum the items across destinations into one number

### Requirement: An action is reported by the surface that ran it

A mutation's outcome MUST be rendered by the surface that initiated it — the row
in the row, a sheet as its own result, a plan on its result step. The product
MUST NOT report outcomes through a transient notification.

Outcomes MUST use one vocabulary: `Apply`, `Applied`, `No change`, `Refused`,
`Failed`, `Queued`. `Queued` MUST NOT take the tone or tense of `Applied`.
`Refused` and `Failed` MUST both state that nothing was changed, and MUST carry
the `request_id` as a labelled copy row when the backend sent one.

#### Scenario: A queued write is not reported as done

- **WHEN** an action is recorded by Syndra and not yet dispatched to a target
- **THEN** the outcome reads `Queued`, in the warn tone and the present tense
- **AND** it names what has not been told yet

### Requirement: Capability does not depend on the input device

Every action reachable with a mouse MUST be reachable by touch. Bulk selection
MUST be entered through a named control rather than a long-press, and MUST NOT
depend on drag-painting or on a bare-letter keyboard shortcut.

Information MUST NOT be carried by a `title` attribute or any hover-only
affordance, since neither exists on touch.

#### Scenario: Selecting a cohort on a phone

- **WHEN** an operator needs to act on several rows on any of the five bulk
  surfaces
- **THEN** a named control in the header turns rows into targets and raises a
  count bar naming the next step
- **AND** rows that cannot be selected keep their seat and state the reason
