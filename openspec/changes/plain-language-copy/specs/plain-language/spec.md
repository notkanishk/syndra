# Plain-language copy

## ADDED Requirements

### Requirement: Every sentence is readable without identity-management background

All user-facing text SHALL be written for makerspace staff and members with no
knowledge of identity management, in the register, sentence shape and
vocabulary fixed by `design.md`. A term from the guide's *never appear on
screen* list SHALL NOT appear in any user-facing string; a term the guide
lists as *glossed* SHALL carry its gloss on its first appearance on each page.

#### Scenario: A mechanism word reaches a string

- **GIVEN** a component or `lib/` module whose copy contains a word from the
  never-on-screen list
- **WHEN** `plain-language.test.ts` runs
- **THEN** it fails naming the file, line, offending text and the plain word
  the guide gives instead

#### Scenario: A glossed term appears on a page for the first time

- **GIVEN** a page that mentions a bundle, a hold, an automatic rule, a
  mapping, Zitadel, or revoking
- **WHEN** the reader meets the term for the first time on that page
- **THEN** the gloss from `design.md` §6 follows it in a parenthesis or a short
  clause, and later mentions on the same page do not repeat it

### Requirement: One name for one thing

The product SHALL use exactly one word for each thing in `design.md` §6:
Zitadel for the sign-in service; *revoke* for ending a person's access;
*preview* then *apply* for the two-step change; *send* for dispatching waiting
changes; *waiting to be sent / sent / failed / given up* for a change's state;
*makerspace staff* for the people a member asks. *Withdraw* SHALL mean only a
member taking back their own request; *remove* only taking a thing out of a
set; *delete* only an object ceasing to exist; *retire* only a bundle closing.

#### Scenario: The same concept on two screens

- **GIVEN** the queue of unsent changes shown on Home, on Pending changes, in a
  bulk outcome, and on a person's page
- **WHEN** each screen describes it
- **THEN** each uses the same noun (*changes waiting to be sent*) and the same
  verb (*send*), and none says drain, resume, queued or write

### Requirement: Every page states its purpose

Every page SHALL carry a lede beneath its title — one or two sentences saying
what the page shows, when the reader would come here, and, for any queue or
review list, what happens if the reader does nothing. The lede lives in
`PageHeader`'s `lede` prop; `meta` SHALL carry only metadata (a count, an
email, an identifier) and never a sentence.

#### Scenario: A review queue is opened

- **GIVEN** Expiring access, Holds due, Pending changes, Unexplained access or
  Unfinished revocations
- **WHEN** the page renders, with or without items
- **THEN** its lede is present and its last sentence says what inaction means
  on this page

#### Scenario: A page lacks a lede

- **GIVEN** a `PageHeader` with no `lede` prop
- **WHEN** `plain-language.test.ts` runs
- **THEN** it fails naming the file and line

### Requirement: Every action states its consequence before the click

Every control that changes something SHALL be accompanied by a sentence
stating what will be true afterwards and for whom. Every destructive action
SHALL carry a titled *What happens* list with one consequence per line. Every
button SHALL name its object in its own label; a bare *OK*, *Confirm*,
*Submit*, *Go* or *Dismiss* SHALL NOT appear. Every preview dialog SHALL open
with the fixed sentence: *"Syndra first shows exactly what would change, person
by person. Nothing changes until you press Apply."*

#### Scenario: A staff member reaches a revoke button

- **GIVEN** any control that revokes access
- **WHEN** it is rendered
- **THEN** a sentence within the same card or dialog names the person, what
  they lose, and when it takes effect, before the button is reachable

### Requirement: Every outcome says what did not happen

Every refusal and failure message SHALL end by stating the state of the world
("Nothing was changed." or the true partial state). Every queued outcome SHALL
say where the change went and what moves it ("Recorded in Syndra and waiting to
be sent to Zitadel. Nothing has changed there yet; send it from Pending
changes."). Error references SHALL be called *reference* and carry the line
"Quote this if you ask for help."

#### Scenario: A connected system does not answer

- **GIVEN** an action against TrueNAS that times out
- **WHEN** the outcome renders
- **THEN** it says which system failed, that Syndra itself is fine, that
  nothing was changed, and what to try next

### Requirement: Members and staff read their own words

Member-facing copy SHALL name a person to ask (*makerspace staff*) and never a
mechanism to understand; it SHALL say *paused* for a hold, *ends* for an
expiry, *because you're in* for a bundle, and *your makerspace sign-in* for
Zitadel. Staff-facing copy MAY name the mechanism after the consequence. The
word *operator* SHALL NOT appear on screen for either audience.

#### Scenario: A member's access is on hold

- **GIVEN** a member viewing access that is under a hold
- **WHEN** the panel renders
- **THEN** it says the access is paused, when staff will look again, the
  reason shown to them, and that they may ask makerspace staff, mentioning
  that reason

### Requirement: Accessible names say the whole action

An icon-only control's `aria-label` SHALL name the action and its object
("Copy your account name"). Visible button text SHALL be the accessible name
wherever text is visible. Counts in links SHALL say what they count. No
placeholder SHALL carry an instruction; instructions go in `FieldHint`.

#### Scenario: Six copy buttons on one page

- **GIVEN** the Network storage page with several copyable values
- **WHEN** a screen reader lists the page's buttons
- **THEN** each is distinguishable by name — none reads only "Copy"
