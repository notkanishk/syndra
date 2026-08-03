> **Status:** ui-capability-gap-closure delta — the vault gains its only surface | [< Index](../../../../INDEX.md)

# Requirement: Shadow Password Vault — Self-Service Surface (delta)

## ADDED Requirements

### Requirement: A member MUST be able to set, rotate and remove their own shadow credential

The vault's four user-facing endpoints are all self-only — the backend refuses
any `{uid}` that is not the authenticated subject, for reads as well as writes.
Its surface therefore lives on the one screen a person owns: Member · My access.
It is NOT on `System › Hardware sync`, which the design brief named: that page
is operator-facing, and an operator can only ever set their own credential
there, which is not what an operator goes to a System page to do.

The card MUST state three things, none of them optional:

1. **That this is not the institutional login.** A second password field with no
   explanation invites exactly one reading — that the person's sign-in has
   changed. It has not.
2. **That nothing reads it yet.** The hardware bridge is unbuilt (see the same
   statement in the operator's register on `System › Hardware sync`). A password
   set today is stored and waiting. Omitting this would be worse than omitting
   the whole card: somebody sets one, tries a door, and concludes the product is
   broken.
3. **That it cannot be read back** — not by a lab manager and not by the page.

The console MUST NOT re-implement the complexity rules. `ValidatePasswordComplexity`
composes the failing requirements into one message; the dialog renders that
message verbatim. A second, drifting opinion about what counts as strong enough
is worse than a round trip.

### Requirement: The console proxy MUST permit exactly the vault routes, for the caller only

The proxy is the OUTER of two locks. A route the backend guards correctly is still unreachable if
the proxy's member allowlist has not been told about it, and the failure is silent — a 403 the
console renders as an absent feature.

`GET`, `PUT` and `DELETE` on `/users/{self}/shadow-credential` and its `/status` and `/audit`
children MUST be permitted for the caller's own id. `PUT`/`DELETE` remain refused everywhere else
under `/users/{self}/…`, including on `grants` and on the credential's read-only children.

The proxy MUST NOT add fields to a member's request body outside `POST /requests`, which is the
one member write that carries a requester. `decodeJSONStrict` rejects unknown fields, so a
blanket injection turns a correct password into a 400.

#### Scenario: The vault is reachable
- **WHEN** a member reads their status or audit, or writes or clears their credential
- **THEN** the proxy forwards each call to the backend

#### Scenario: Somebody else's credential
- **WHEN** a member targets another user's `shadow-credential` by any method
- **THEN** the proxy refuses it without contacting the backend

#### Scenario: The password body is forwarded intact
- **WHEN** a member submits `{ "password": "…" }`
- **THEN** exactly that object reaches the backend

### Requirement: A vault read that fails MUST say so rather than render nothing

The card previously returned nothing on a failed status read. That silence hid a real fault: the
proxy did not permit these routes, so the card was absent for every member and read as a design
decision. A failed read renders the card in an unavailable state, stating that the member's access
is unaffected.

#### Scenario: The status read fails
- **WHEN** `GET /users/{self}/shadow-credential/status` errors
- **THEN** the card renders, says it could not load, and says access is unaffected

#### Scenario: Setting a first credential
- **WHEN** a member with no credential submits a password twice, matching
- **THEN** `PUT /users/{uid}/shadow-credential` is called with it
- **AND** the status card reads Set

#### Scenario: The two fields disagree
- **WHEN** the confirmation field does not match
- **THEN** nothing is submitted, and the mismatch is stated in the form

#### Scenario: The server rejects the password
- **WHEN** the backend returns a complexity failure
- **THEN** the server's own sentence is displayed in the dialog
- **AND** the dialog stays open, because a rejected password must not look saved

#### Scenario: Removal
- **WHEN** a member removes their credential
- **THEN** the dialog states that machines asking for it stop letting them in
- **AND** that their access itself is unchanged
