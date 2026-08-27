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

### Requirement: `dangerConfirm` MUST follow what the act does, not how final it is

A solid destructive fill MUST be used only for a confirm that takes access away
or destroys data. A confirm that GRANTS — an adoption, a binding, a provision —
MUST be `accent`, however irreversible it is. Irreversibility is carried by the
confirmation rung and by the copy, never by the colour.

"`accent` or `dangerConfirm`" left the choice to whoever wrote the screen, and
the adoption form took the red one: adopting an account gives a person a home
directory, and it was drawn in the product's word for taking one away. Spending
red on a grant is not a harmless over-warning — it is what makes the next real
revocation read as routine.

#### Scenario: An irreversible grant is confirmed

- **WHEN** a confirm binds, adopts or provisions, and cannot be undone
- **THEN** it MUST be `accent`
- **AND** the irreversibility MUST be stated in the copy and gated by its rung

#### Scenario: A confirm removes access

- **WHEN** a confirm revokes, removes, purges, deletes, or discards evidence
- **THEN** it MUST be `dangerConfirm`

### Requirement: A panel opened from a row MUST render under that row

A form or an outcome belonging to one item in a list MUST render attached to
that item, through `CardRow`'s disclosure, and MUST NOT be rendered after the
list. Where the row's own control opens it, `onToggle` MUST be omitted so the
row does not also become a button — a button inside a button is two overlapping
targets and invalid markup.

The adopt form rendered after the whole inventory: clicking Adopt on the first
account opened a form under the LAST one, naming an account the operator had not
pointed at. The same treatment carries the product's one disclosure motion, so a
panel that opts out of it is also the only thing on the screen that appears
without settling into place.

#### Scenario: A row's control opens a form

- **WHEN** a control inside a row opens a form about that row's item
- **THEN** the form MUST render directly beneath that row
- **AND** it MUST use the shared disclosure treatment, including `settle-in`
- **AND** the row itself MUST NOT become a button

#### Scenario: An action reports what it did to one item

- **WHEN** an action on a single row completes
- **THEN** its outcome MUST render beneath that row rather than at the foot of
  the card

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

### Requirement: A control MUST carry the touch floor, whoever wrote it

Every pressable control MUST meet the 44px floor through the tablet range,
including one composed by hand from `PILL` or from classes. A control that
restates the pill without the floor MUST NOT ship.

The first guard on this rule looked for a pill that ALSO claimed the floor —
the careful copy. It could never catch the careless one, which is the copy that
matters: five hand-rolled pills had simply omitted the floor, among them the ✕
that clears a filter (a 20px target, on the only way out of a view somebody
reached by link) and the "how long" choice on a grant.

#### Scenario: A control is composed by hand

- **WHEN** a component builds a pressable pill outside `components/ui`
- **THEN** it MUST import `PILL` or use `Button`
- **AND** the rendered control MUST meet the 44px floor below the desktop
  breakpoint

### Requirement: One count MUST have one rendering, including at zero

`CountChip` and `CardHeader` both draw a count. They MUST agree at zero: hollow,
never a filled badge. A solid badge is an alarm, and a solid `0` beside "Not
going to happen" is an alarm about nothing happening.

#### Scenario: A section holds nothing

- **WHEN** a card or region heading renders a count of zero
- **THEN** the count MUST render hollow, keeping its seat

### Requirement: A page's own sections MUST keep their seats

A page composed of classification buckets MUST render every bucket at every
count, in one order, with a sentence saying what an empty one means. A bucket
that appears with its first row inserts itself above whatever the operator was
reading.

This is the nav rule applied where it also holds. On the withdrawn-access page
the two buckets are the page: a revocation going terminal inserted a red card
above the queue somebody was reading, and "nothing has given up" is the good
news that its absence could not state.

#### Scenario: A bucket is empty

- **WHEN** one of a page's buckets holds nothing
- **THEN** it MUST keep its position and its heading
- **AND** it MUST say what its emptiness means, rather than being absent
