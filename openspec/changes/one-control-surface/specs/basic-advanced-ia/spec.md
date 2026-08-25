## ADDED Requirements

### Requirement: A control's variant MUST follow its role, not its author

The one action a row or a finding offers MUST be the `outline` variant. A
destructive row action MUST be `danger`. The `ghost` variant MUST be used only
for the quieter half of a pair — a Cancel or a Dismiss beside a primary. A
dialog's confirming button MUST be `accent` or `dangerConfirm`.

A borderless control is invisible as a control in a table row: it reads as text
until hovered, and hover does not exist on a touch device and does not survive a
screenshot. Two adjacent actions of equal weight drawn as two different kinds of
control state a difference in weight that is not there.

#### Scenario: A row offers one action

- **WHEN** a row's only affordance opens a dialog or performs its action
- **THEN** it MUST be rendered as `outline`, or `danger` when it destroys access

#### Scenario: A dialog offers a confirm and a way out

- **WHEN** a dialog renders both
- **THEN** the confirm MUST be `accent` or `dangerConfirm`
- **AND** the way out MUST be `ghost`

### Requirement: A control's surface MUST have exactly one definition

`Button`, `ButtonLink`, `Tabs`, `FilterPills` and `Badge` are the surfaces.
Outside `components/ui`, no component may reconstruct one from its classes. A
control that is the pill box without being `Button` — a switch, a radiogroup, a
pressed choice — MUST import the shared `PILL` metric rather than restate it.

A hand-rolled copy is invisible in review: it renders almost right, and the
"almost" is a padding value or an animation that does not fire. It is found
later, on a screen, after it has been copied again. Three copies of one "load
more" pill and two of one tab row had each drifted separately.

This MUST be enforced against the source rather than by convention, since
nothing about a copy announces itself.

#### Scenario: A pill control is written by hand

- **WHEN** a file outside `components/ui` combines a pill radius with the touch
  floor on one element
- **THEN** the check MUST fail and name the file and line

#### Scenario: A status pill is written by hand

- **WHEN** a file outside `components/ui` restates `Badge`'s box
- **THEN** the check MUST fail and name the file and line

### Requirement: A reading MUST NOT be carried by colour alone

A status word MUST be preceded by its tone dot. The dot and the word MUST come
from the shared `STATUS_TONE` map.

Green and amber are the same word to a reader who cannot separate them, and the
same word in a greyscale print or screenshot. The dot adds presence and
position, which survive both.

#### Scenario: A target's reachability is reported in a list

- **WHEN** the add-on index renders a target's reading
- **THEN** it MUST render a tone dot beside the word

### Requirement: An identifier MUST be typeset by what it is, not by where it is

An identifier appearing inside a sentence MUST use the `Mono` component. An
identifier that is a row's title keeps the row-title size, which every row title
already shares.

The same username was being rendered at four sizes depending on which paragraph
it fell in, so the type reported the surrounding prose rather than the kind of
thing being named.

#### Scenario: An account name appears in explanatory prose

- **WHEN** a sentence names an account, a role key, a chain head or a grant id
- **THEN** it MUST be rendered with `Mono`
