## ADDED Requirements

### Requirement: Every route MUST name its auth gate

Every route registered on the router MUST be wrapped in one of the named auth
gates. The set MUST be enumerated explicitly rather than matched by shape, and
an ungated route MUST be listed by name with its argument.

A route registered without a gate is not a style mistake — it is an endpoint
serving whoever finds it, and the line it is written on looks exactly like the
130 lines around it that are safe. Nothing about the omission is visible.

#### Scenario: A route is added

- **WHEN** a new route is registered
- **THEN** it MUST carry `withUserAuth`, `withOperatorAuth`,
  `withSelfOrOperatorAuth`, `withAPIKeyAuth`, or `withZitadelActionSignature`
- **OR** it MUST appear in the ungated allowlist with the reason

#### Scenario: The health check

- **WHEN** the container runtime probes liveness
- **THEN** `/healthz` MUST answer without a credential, because the runtime
  carries none

### Requirement: A mutation MUST reject unknown fields

Every mutating endpoint that reads a request body MUST decode strictly.
Exceptions MUST be named individually with the reason, never granted by default.

A lenient decode silently drops a field the caller believed it sent: the caller
thinks it asked for something the server never read. On the one endpoint in the
product that removes access, that had actually happened.

#### Scenario: A mutation reads a body

- **WHEN** a POST, PUT, PATCH or DELETE handler decodes a request body
- **THEN** it MUST use `decodeJSONStrict`
- **AND** a handler that only delegates MUST be followed to the body that
  decodes, since a two-line handler handing off to a shared implementation is
  the normal shape

#### Scenario: An external system owns the payload

- **WHEN** the body's shape is owned by a system that may extend it
- **THEN** `decodeJSONLenient` MAY be used, named in the guard with its reason
- **AND** the trailing-token guard MUST still apply — the laxity is on the field
  set only

### Requirement: A dependency seam MUST NOT be bypassed

Where `deps.go` holds a seam for a function, callers in that package MUST go
through the seam.

A handler that calls the service directly is untestable, and it makes the seam
dead code that reads as live: the next person substitutes it, sees no effect,
and goes looking for a bug in their test. Two seams over one function are the
same trap wearing a different hat — substituting one leaves the other's callers
talking to the real thing.

#### Scenario: A seam exists for a service call

- **WHEN** a handler calls a function that `deps.go` already seams
- **THEN** it MUST call the seam
- **AND** a file-local wrapper over the same function MUST delegate to the
  `deps.go` seam rather than to the service

### Requirement: The operator log MUST speak one vocabulary

Every log line MUST be `[SUBSYSTEM] what happened`, with the subsystem drawn
from a fixed set. Severity MUST NOT appear in the tag, and one subsystem MUST
NOT write under two tags. The vocabulary MUST span the backend and the add-ons
together.

The shape is the whole of how an operator finds anything: `grep '\[DRIFT\]'` is
the question "what has the sweep been doing", and it is only answerable while
every line the sweep writes carries that tag and nothing else does. Severity in
the tag fragments the subsystem's own index — grepping `[CACHE]` returned the
lines that went well. An add-on runs in its own container, but its lines are
read beside the backend's, by the same person, during the same incident.

#### Scenario: A line is written

- **WHEN** any component logs
- **THEN** the tag MUST be one of the enumerated subsystems
- **AND** the tag MUST NOT contain WARN, ERROR, INFO, DEBUG, FATAL or CRITICAL
- **AND** severity MUST be carried by the sentence

### Requirement: Only the traced path MAY mutate Zitadel grants

A Syndra-mediated Zitadel grant mutation MUST leave its trace before the
Management API call — a ledger row for a direct grant, an outbox row for a
bundle or rule cascade. Files permitted to call a grant mutation directly MUST
be enumerated with their reason, and the allowlist MUST NOT name a file that
does not exist.

This is the premise the drift sweep reasons from: a Zitadel-side change with no
trace is not trusted, it is triaged. A path that writes a grant with no row
behind it makes the sweep reason about a world it cannot see all of.

#### Scenario: A new caller writes a grant

- **WHEN** code outside the allowlist calls a Zitadel grant mutation
- **THEN** the guard MUST fail
- **AND** the fix MUST be to enqueue, or to add the file with its argument

#### Scenario: An allowlisted file is deleted

- **WHEN** an allowlist entry names a path that no longer exists
- **THEN** the guard MUST fail, because a file later created at that path would
  inherit the exemption silently
