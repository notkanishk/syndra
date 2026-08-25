# addons

One container per system Syndra provisions into.

Syndra decides **who** should have access and **what** that means. An add-on
decides **how** — it is the only component that knows a target product's API,
its vocabulary, and its refusals. Nothing above it does, deliberately: the day
`internal/services` learns what an SMB share is, the second target needs a
second copy of everything.

| Path | Contents |
|---|---|
| [`contract/`](contract/) | The wire contract, as artifacts both ends are tested against — plus what a real target answered, recorded rather than written |
| [`truenas/`](truenas/) | The first and so far only add-on: TrueNAS SCALE, SMB storage |

## The trust model

**An add-on is the least trusted component in the deployment.** It holds a
third-party API key and talks to a machine Syndra does not control. Everything
below follows from arranging things so that being the least trusted component
is survivable.

- **The manifest is a ceiling, never a grant.** `GET /capabilities` says what
  this add-on *can* do; the backend intersects that with its own policy and the
  policy wins every disagreement. An operation absent from the backend's policy
  is unavailable no matter what the add-on declares, so a compromised add-on
  claiming `scope: member` on a destructive operation buys nothing.
- **Its own network, no exposed ports.** Compose puts each add-on on a network
  shared with the backend and with nothing else. The UI cannot reach one; the
  datastores cannot; another add-on cannot.
- **The channel is authenticated in both directions.** One configured secret per
  target, from which both ends derive both keys with HKDF — the Ed25519 key the
  add-on serves and the backend pins, and the HMAC key that signs the backend's
  requests. There is no certificate to mint, distribute or renew. A bare shared
  secret is deliberately not an option: it identifies a caller and binds
  nothing, so an intercepted call replays verbatim, forever.
- **The backend is still the single mutation authority.** An add-on never
  decides that something should happen. It is handed an approved plan and
  applies it.

## The contract

Eight routes. Every one is authenticated — there is no unauthenticated health
check, because a target's reachability is itself information.

| Endpoint | What it is for |
|---|---|
| `GET /capabilities` | The manifest — contract version, product, entitlement schema, operation set, and how a *member* reaches the target |
| `GET /health` | The add-on's own view of itself and of the target |
| `GET /subjects` | The target's current state, in entitlement vocabulary. The read every plan and every reconciliation starts from |
| `GET /values/{field}` | Enumerates the legal values of one entitlement field, so an operator authoring a mapping picks a group that exists instead of typing one |
| `POST /plan` → `POST /apply` | Rehearse, then converge. The apply carries a plan id and never the original submission |
| `POST /operations/{name}` | The one-shot half — the operation named in the path, the deduplication token in the body |
| `POST /lifecycle` | `active` · `draining` · `read_only`, at runtime |

### The two halves, and why the line is where it is

**Entitlement fields are desired state.** The add-on is told what should be true
and converges to it, every time, from whatever is there now.

**Operations are one-shot and event-shaped.** A password set. A purge. Things
with no steady state to converge to.

Getting this line wrong has a scar behind it. An early draft had `account.lock`
as an operation, which made it edge-triggered: deprovisioning left an account
locked, and regaining a role could not bring it back, because a
create-if-absent path sees an existing account and does nothing. The account
stayed dark while Syndra believed access was restored. As an entitlement field
it converges like any other, and nothing special-cases restoration because
nothing special-cased suspension.

**If it has a steady state, it is a field.** Only if it genuinely cannot is it
an operation.

### Availability is per operation

An operation the target cannot perform is declared `available: false` **with a
reason**, and rendered disabled. Not omitted — an operator then wonders whether
it exists — and not left to fail on use. Product methods move between releases;
a supported major is not a promise that every method is present.

## What an add-on owes

- **Strict decoding, both ways.** Unknown fields are refused rather than
  ignored, at every boundary. The backend and the add-on are separately
  deployed binaries and the first real deployment found envelopes carrying
  fields the other end's decoders never declared.
- **A contract version**, declared and checked at registration. The two ends
  never share a Go module: a shared constant would hide exactly the version skew
  the contract version exists to surface. Constants that must match are written
  out on both sides and asserted against `contract/`.
- **A mutation log** — append-only, `0600`, fsynced *before* the operation it
  describes. The backend anchors the head, so a truncation or a rewrite is
  detected rather than believed.
- **A result cache, and no queue.** Two durable queues would disagree about what
  is still pending, and Syndra's is the one that knows why an operation exists.
  The add-on's store makes a replayed call safe and lets a read answer while the
  target is down. It never holds work.
- **Honest degradation.** "I could not look" and "there is nothing" must never
  render as the same answer. Every read that can partially fail says which part
  is missing.

## Writing the second one

The platform half is target-neutral and is asserted to stay that way —
`backend/internal/services/merge` fails its own tests if the word `truenas`
appears in it. So a second add-on writes a binary and nothing else: no backend
changes beyond a policy entry, no new dispatch path, no second classifier.

1. **Serve the routes above.** Start from `truenas/server.go` and
   `truenas/transport.go`; the transport is generic and the derivation is
   specified in [`contract/transport_derivation.json`](contract/transport_derivation.json).
2. **Declare an entitlement schema** in the target's own vocabulary. Syndra
   never learns what a value means — it fills the field, the add-on translates.
3. **Record what the target actually answers** before writing a decoder against
   it. Every serious defect this platform has shipped came from a fixture
   somebody wrote by hand, agreeing with the code that read it and disagreeing
   with the target. See [`contract/README.md`](contract/README.md).
4. **Add the target to the backend's operation policy** and to `ADDON_TARGETS`,
   and give it a Compose service on its own network behind a profile — an
   add-on holds a credential, so a deployment that does not use that target must
   not be running its container at all.

Full design: [`openspec/changes/addon-platform/design.md`](../openspec/changes/addon-platform/design.md).
The defect ledger — read the header first — is
[`openspec/changes/addon-platform/tasks.md`](../openspec/changes/addon-platform/tasks.md).
