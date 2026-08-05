> **Status:** login-doorway delta — the unauthenticated surface | [< Index](../../../../INDEX.md)

# Requirement: Operational Readiness (delta)

## ADDED Requirements

### Requirement: The unauthenticated surface MUST offer exactly one action

`/login` is the only route reachable without a session. In a live deployment it MUST present one
control and no others: no email field, no password field, no alternative provider, no sign-up and no
password reset. Zitadel is the sole identity provider and owns all of those.

That control MUST be a link to the authorization route, not a scripted button, so the flow completes
with JavaScript disabled. It MUST be keyboard reachable and MUST carry a visible focus indicator.

#### Scenario: A live deployment renders one control

- **GIVEN** `ZITADEL_DOMAIN` is configured
- **WHEN** an unauthenticated visitor loads `/login`
- **THEN** the page MUST expose exactly one activatable control
- **AND** that control MUST be a link whose target is the authorization route
- **AND** the page MUST expose no text input of any kind

#### Scenario: The unauthenticated surface asks the backend for nothing

- **GIVEN** a visitor with no session loads `/login`
- **WHEN** the page has finished rendering
- **THEN** it MUST have issued no request to an authenticated endpoint
- **AND** any shared provider that warms a cache MUST be gated on the presence of a session rather
  than relying on the backend to refuse it

#### Scenario: The flow survives without scripting

- **GIVEN** JavaScript is unavailable
- **WHEN** the visitor activates the control
- **THEN** the browser MUST navigate to the authorization route unaided

### Requirement: The doorway MUST tell its state through the composition, not a banner

`/login` has three observable states — resting, opening, and refused — and each MUST be conveyed by
the screen itself rather than by a toast, an alert bar, or a colour swap on the button alone. A
refusal MUST render as a closed door: the arch's dissolve removed so the stroke reads as a complete
line, and the accent moved to the amber that this system reserves for a broken assumption. Red is
destructive-only and MUST NOT appear here.

The refused state MUST be present in the server-rendered markup when the visitor arrives carrying an
error, so there is no frame in which the door is shown open before it shuts.

Retrying MUST return the page to its resting state and MUST clear the error from the URL, so a
reload does not re-raise a refusal the visitor has already moved past.

#### Scenario: A refusal arrives already refused

- **GIVEN** a visitor is redirected to `/login` with an error parameter
- **WHEN** the document is served
- **THEN** the markup MUST already carry the refused state
- **AND** the arch MUST render as a complete closed line in the amber attention colour
- **AND** no red MUST be used

#### Scenario: Retry reopens the door and forgets the error

- **GIVEN** the page is in its refused state
- **WHEN** the visitor retries
- **THEN** the page MUST return to its resting state
- **AND** the error parameter MUST be removed from the address
- **AND** a reload MUST NOT show the refusal again

### Requirement: A failed sign-in MUST name what failed without echoing the provider's code

Every failure path redirects to `/login` with a machine code. The page MUST NOT render that code:
it arrives in a URL that anyone can type, and it is not a sentence a member can act on.

The message MUST distinguish, at minimum, a refusal by the provider from a provider that did not
answer, because telling a member the provider was silent when it refused them by name is false. Every
variant MUST state that nothing was signed in, and MUST offer a next step the visitor can actually
take.

The failure MUST be announced to assistive technology.

#### Scenario: A refusal is not reported as silence

- **GIVEN** the provider returns `access_denied`
- **WHEN** `/login` renders the failure
- **THEN** the message MUST say the provider refused, not that it failed to answer

#### Scenario: An unrecognised code still produces a sentence

- **GIVEN** an error code the page does not classify
- **WHEN** `/login` renders the failure
- **THEN** the page MUST render a complete message stating nothing was signed in
- **AND** the raw code MUST NOT appear anywhere in the rendered document

### Requirement: Sign-in motion MUST be cover for the redirect, never a gate in front of it

The transition that plays on sign-in exists to cover the authorization redirect's latency, which is
otherwise a frozen page. The navigation MUST be issued at the start of that transition, and MUST NOT
be delayed until it completes — a visitor on a fast connection MUST NOT wait for an animation.

A modified click (a request for a new tab) MUST leave the page in the state it was in.

Where the visitor has asked for reduced motion, the entrance and the pointer-tracked lighting MUST
be skipped, but the sign-in and refused states MUST still be conveyed, as instant state changes if
necessary — they carry meaning, and meaning is not decoration.

#### Scenario: The redirect is not held behind the animation

- **GIVEN** a visitor activates the sign-in control
- **WHEN** the transition begins
- **THEN** the navigation MUST already be in flight

#### Scenario: Reduced motion keeps the meaning

- **GIVEN** the visitor has requested reduced motion
- **WHEN** the page enters its refused state
- **THEN** the closed arch and the message MUST still be presented
- **AND** the entrance animation and the pointer-tracked lighting MUST NOT run

### Requirement: Development identities MUST NOT be reachable from a live deployment, or before they are offered

The local-development identity picker on `/login` MUST be gated on the absence of a configured
identity provider, at the source as well as at the call site — see the Demo Catalog spec, which
forbids serialising demo entities into a production payload.

Where the picker is offered, its controls MUST NOT exist in the document until the visitor has asked
for them. A control that is present but invisible is still reachable by keyboard and still read by a
screen reader, which would put five sign-in buttons on a page that shows one.

#### Scenario: A live deployment carries no identities

- **GIVEN** `ZITADEL_DOMAIN` is configured
- **WHEN** `/login` is served
- **THEN** no demo identity MUST appear in the response payload

#### Scenario: The picker is absent until asked for

- **GIVEN** local development with no identity provider configured
- **WHEN** `/login` first renders
- **THEN** no identity control MUST be present in the document
- **AND** after the visitor activates the single control, each identity MUST be a real form post to
  the session route
