# The add-on wire contract

> The model these artifacts serve is in [`../README.md`](../README.md);
> the add-on held to them is [`../truenas/`](../truenas/README.md). The change
> that built this directory, and the contract 2 that follows it, is
> `openspec/changes/addon-contract/`.

This directory is **the wire contract**: contract 2 (`v2/`), what both ends
speak since slice 1 of `openspec/changes/addon-contract`. Contract 1 was pinned
here first (slice 0) exactly as the code spoke it, before anything about it
changed, because a schema written after the wire moves pins whatever the code
happened to do; it was deleted when contract 2 shipped (design §1: one supported
major). Where the schema and the code ever disagree, the disagreement is a
defect in one of them and is written down, never papered over.

The backend and the add-on are separately compiled modules with no shared type,
and the failure that follows is not theoretical: the backend's envelopes carried
`contract_version` (and `operation`, `plan_id`, `fingerprint` on the operation
leg) while the add-on's structs declared none of them. The add-on decodes
strictly, so **every real `/apply` and `/operations/*` call would have been
answered `400 BAD_REQUEST`**, and neither side's tests could see it, because each
was exercised against its own fake and the two fakes agreed with each other. So
the contract is these files, and each module is held to them in its own suite.

## One major, one directory

`v2/` is contract 2. A future major 3 is added beside it as `v3/`, and `v2/` is
deleted the day it ships, so a test loads exactly one major and cannot validate a
fixture against the wrong one by accident. `transport_derivation.json`,
`truenas_observed.json` and [`CHANGELOG.md`](CHANGELOG.md) stay here at the top.
Every path below that begins `schema/`, `fixtures/`, `registry/` or
`openapi.yaml` is inside `v2/`.

Go's test cache does not see these files, which sit outside each module's root: after
editing one, run `go test -count=1`, or a stale pass is reported.

## What is here

| Path | What it pins |
|---|---|
| [`openapi.yaml`](v2/openapi.yaml) | OpenAPI 3.1: every route, method, status and body, by `$ref` into `schema/`; the HMAC signature header as the security scheme |
| [`schema/`](v2/schema/) | One JSON Schema 2020-12 per message, `error.json` for the RFC 9457 problem every refusal is, `common/` for shapes shared across messages, `identity/` for the request-identity objects, `profiles/storage/` for the two routes only a storage target serves (`/reach`, `/homes`). Every file has a stable `$id`; `$ref`s are relative |
| [`fixtures/`](v2/fixtures/) | A golden document per message, every field populated. Ring names and `example.org` only; addresses from RFC 5737 |
| [`registry/`](v2/registry/) | `errors.json` (every `code`: title, statuses, retry class), `operations.json` (every operation id in the backend's `operationPolicy`), `roles.json`, `kinds.json`, `features.json`, `units.json`, `constants.json` |
| [`transport_derivation.json`](transport_derivation.json) | How both ends derive their keys and sign requests (below) |
| [`embed.go`](embed.go) | Files only: `v2/` and the transport vector embedded, so the conformance kit carries the contract it was built from. No Go type lives here, ever (design §1); the two ends keep reading these files from their own suites |
| [`truenas_observed.json`](truenas_observed.json) | What a real TrueNAS answered, recorded rather than written (below) |

Request schemas are closed (`additionalProperties: false`), mirroring the
add-on's strict decode. Reply schemas say what a reply carries and which fields
are required; the backend decodes every reply tolerantly, the manifest
included, and reads the safety-relevant fields (`current`, `truncated`,
`taken_at`, `standing.usable`, an outcome's `effect`) as present-or-unusable,
never as Go's zero value. `format: date-time` is asserted, not annotated: the
validator runs with format assertion on.

## How each end is held to it

Every message has a producer and a consumer, and each suite asserts the half it
owns. The comparison is over the WHOLE document rather than over a field list,
because a list is a third place to forget something.

| Route | Sent by | Fixtures |
|---|---|---|
| `GET /capabilities` | add-on | `manifest_truenas.json` (TrueNAS), `manifest.json` (every feature) |
| `GET /health` | add-on | `health.json` |
| `GET /subjects` | add-on | `subjects.json`, `subjects_stale.json` |
| `GET /values/{field}` | add-on | `values.json` |
| `GET /reach` | add-on | `reach.json` |
| `GET /settings` | add-on | `settings.json` |
| `PUT /settings/{name}` | backend | `setting_put_request.json`; reply `setting.json` |
| `GET /homes` | add-on | `homes.json` |
| `POST /plan` | backend | `plan_request.json`; reply `plan_response.json` |
| `POST /apply` | backend | `apply_request.json`; reply `apply_response.json` (a 200), `error_apply_blocked.json`, `error_binding_conflict.json` |
| `POST /operations/{name}` | backend | `operation_request.json`; reply `operation_response.json` plus one per pocket: `_activity`, `_storage`, `_health`, and `_result` (feature results) |
| `POST /lifecycle` | backend | `lifecycle_request.json`; reply `lifecycle_response.json` |
| any refusal | add-on | `error.json` |
| request identity | both | `identity_{apply,operation,lifecycle,setting}.json` (RFC 8785 vectors) |

- **What a side produces**: the Go value it builds from the fixture marshals
  byte-equal to the fixture after normalisation, and satisfies the schema. The
  backend's inline bodies (setting, lifecycle, the three read operations) are
  captured as they leave the real function, not rebuilt in the test.
- **What a side consumes**: the fixture decodes (strictly, where the code is
  strict) and every field that side reads lands. The backend runs the real
  client function against a server answering with the fixture. A request
  fixture's keys from a feature an add-on does not declare (`preview`, `fresh`)
  are refused by that add-on (`UNKNOWN_REQUEST_FIELD`) and never sent by a
  backend that does not know the feature.
- **Every fixture validates against its schema**, in both modules
  (`santhosh-tekuri/jsonschema/v6`, test scope; schemas load from disk by
  `$id`, nothing is fetched). A fixture on disk that no test lists fails.
- **The registries are compared with the code.** An error code the add-on writes
  and `errors.json` lacks, or with another title or retry class, fails the
  add-on's suite; so does a refusal written anywhere but through its one
  `writeError`. An operation in the backend's
  `operationPolicy` that `operations.json` lacks, or lists differently, fails the
  backend's suite.

Changing one of these files is a wire-contract change. It fails test suites in
two modules until both ends move, which is the point.

## What the fixtures deliberately do not cover

`apply_response.json` pins the SUCCESSFUL shape; a blocked and a conflicting
apply are the two problem fixtures. `plan_response.json` carries one of each
kind of outcome. `operation_response.json` populates the scalar fields of the
reply (`rotated` and `account_id` belong to different operations in practice) so
that the whole document is held; the pocket fixtures are realistic. The fixtures
are illustrative documents, not one add-on's recording: a producer is held to
the SHAPE by marshalling a value built from a fixture's own values, and its real
output to the schema.

## `transport_derivation.json` — the same idea, one layer down

The files above pin what the two ends *say*. This one pins how they *authenticate*
before either is read, for the transport described in
`openspec/changes/addon-transport-derived-keys/`.

Both keys come from one configured per-target secret — an Ed25519 TLS key the
backend pins, and the HMAC key that signs each request — so the derivation is a
new place where two separately compiled modules must agree byte for byte and
nothing otherwise makes them. A disagreement about the hash, the salt, the
domain-separation string, or whether a hex-looking secret is decoded before use
presents identically to a wrong secret: `the key is not the one derived`, with
each side internally consistent and certain. That is the failure mode the
signing key already produced once, when one end HMAC'd the file's contents and
the other the literal path string.

| Field group | What it pins |
|---|---|
| `secret_utf8` + `secret_bytes_hex` | the input encoding, stated twice so it cannot be misread from the artifact — trimmed UTF-8 bytes, **never** hex-decoded |
| `hash`, `kdf`, `tls_info`, `hmac_info` | the derivation parameters and their domain separation |
| `derivations[]` | seed, Ed25519 public key and HMAC key, for **two** targets |
| `signature` | one complete MAC over a known request, so the vector covers the existing signing construction and not only the new derivation |

Two targets rather than one on purpose: an implementation that ignored the
target salt would pass every assertion while only one add-on exists, and fail
the day UniFi is deployed.

`signature.mac_hex` was verified against the shipped `computeMAC`, not against a
second implementation of it — a vector generated by a reimplementation pins the
reimplementation.

## `truenas_observed.json` — what a real target answered

Not a fixture anybody wrote. `scripts/record-truenas-fixtures.sh --write` asks a
live TrueNAS and writes down what it said, including its **refusals** and the
schema field each one named.

That is the half no hand-written fixture has ever contained, and it is the half
that matters. Every serious defect this add-on shipped had one shape — a fixture
written by hand, agreeing with the code that read it, disagreeing with the
target:

- `system.version` answers `TrueNAS-25.10.5`; the fixture said `25.04.2.1`, so
  the version gate refused every mutation on a release that is supported.
- `user.create` **refuses** a payload with no password decision; the fixture
  answered that payload with a success, so account creation had never worked
  against any release and both suites were green.

Reads are recorded as **keys and types, at every level a decoder touches** —
values never, at any level, because a value here is a member's name or a group's.
Types were added after `audit.query` answered `message_timestamp` as an integer
into a string field, which failed the whole response and left `activity.get`
broken for its entire life while a recording of key names agreed with the code.
Nesting was added after the same thing happened one level down: `audit` was
recorded as `dict`, which tells a decoder it needs a struct there and nothing
about what belongs in it, and the add-on read `enable` out of it while
`watch_list` narrowed the auditing to groups the member was not in.

The `select` a probe sends must be the **union of every select the add-on
issues** against that method. A subset is a recording those decodes cannot be
checked against — `user.query` was missing `groups` for as long as the file
existed, and `groups` is read on every inventory pass.

`addons/truenas/truenas_rules_test.go` derives its assertions from this file
rather than from anybody's memory: each one fires only because the target
refused something, and stops firing if a later release stops refusing it.

**Re-record on a major upgrade.** A recording names the release it came from,
and a recording from one major says nothing about another — pretending otherwise
is how this went wrong twice. The probe creates one throwaway account and
deletes it again; it refuses to touch a name that already exists, and records
whether it cleaned up.

Refusals are recorded as field **paths** and never as the middleware's prose.
The prose is free text built from a call whose parameters can include a member's
password; a path is a key of the request schema and cannot carry a value. Same
rule the add-on itself follows (`nas.go`, `validationFields`).

