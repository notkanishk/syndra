# The add-on wire contract

> The model these artifacts serve is in [`../README.md`](../README.md);
> the add-on held to them is [`../truenas/`](../truenas/README.md). The change
> that built this directory, and the contract 2 that follows it, is
> `openspec/changes/addon-contract/`.

This directory is **contract 1 exactly as it stands**: twelve routes, pinned
before anything about them changes. A schema written after the wire moves pins
whatever the code happened to do, which is the defect these files exist to stop.
Where the schema and the code ever disagree, the schema follows the code and the
disagreement is written down; nothing here describes how the wire ought to be.

The backend and the add-on are separately compiled modules with no shared type,
and the failure that follows is not theoretical: the backend's envelopes carried
`contract_version` (and `operation`, `plan_id`, `fingerprint` on the operation
leg) while the add-on's structs declared none of them. The add-on decodes
strictly, so **every real `/apply` and `/operations/*` call would have been
answered `400 BAD_REQUEST`**, and neither side's tests could see it, because each
was exercised against its own fake and the two fakes agreed with each other. So
the contract is these files, and each module is held to them in its own suite.

## Two majors, two directories

`v1/` is contract 1, pinned by slice 0 and still what the code speaks and is tested
against. `v2/` is contract 2 as written by slice 1a (schemas only; nothing sends it
yet). A test loads exactly one major, so a v1 fixture cannot validate against a v2
schema by accident. When slice 1 ships, `v1/` is deleted. `transport_derivation.json`,
`truenas_observed.json` and [`CHANGELOG.md`](CHANGELOG.md) stay here at the top. Every
path below that begins `schema/`, `fixtures/`, `registry/` or `openapi.yaml` is inside
`v1/` (or `v2/`, which has the same layout plus `schema/identity/`, `schema/common/` additions
and `registry/{roles,features,kinds,units,constants}.json`).

Go's test cache does not see these files, which sit outside each module's root: after
editing one, run `go test -count=1`, or a stale pass is reported.

## What is here

| Path | What it pins |
|---|---|
| [`openapi.yaml`](openapi.yaml) | OpenAPI 3.1: every route, method, status and body, by `$ref` into `schema/`; the HMAC signature header as the security scheme |
| [`schema/`](schema/) | One JSON Schema 2020-12 per message, `error.json` for `{"error": "<CODE>", ...}`, `common/` for shapes shared across messages, `profiles/storage/` for the two routes only a storage target serves (`/reach`, `/homes`). Every file has a stable `$id`; `$ref`s are relative |
| [`fixtures/`](fixtures/) | A golden document per message, every field populated. Ring names and `example.org` only; addresses from RFC 5737 |
| [`registry/errors.json`](registry/errors.json) | Every `error` code the add-on writes: its statuses and its retry class |
| [`registry/operations.json`](registry/operations.json) | Every operation id in the backend's `operationPolicy`: scope ceiling, confirm, params, secret params |
| [`transport_derivation.json`](transport_derivation.json) | How both ends derive their keys and sign requests (below) |
| [`truenas_observed.json`](truenas_observed.json) | What a real TrueNAS answered, recorded rather than written (below) |

Request schemas are closed (`additionalProperties: false`), mirroring the
add-on's strict decode. Reply schemas describe the fields sent today and invent
no constraint the code does not enforce; the one reply that is closed is the
manifest, because the backend decodes it strictly. `format: date-time` is
asserted, not annotated: the validator runs with format assertion on.

## How each end is held to it

Every message has a producer and a consumer, and each suite asserts the half it
owns. The comparison is over the WHOLE document rather than over a field list,
because a list is a third place to forget something.

| Route | Sent by | Fixtures |
|---|---|---|
| `GET /capabilities` | add-on | `manifest.json` |
| `GET /health` | add-on | `health.json` |
| `GET /subjects` | add-on | `subjects.json` |
| `GET /values/{field}` | add-on | `values.json` |
| `GET /reach` | add-on | `reach.json` |
| `GET /settings` | add-on | `settings.json` |
| `PUT /settings/{name}` | backend | `setting_put_request.json`; reply `setting.json` |
| `GET /homes` | add-on | `homes.json` |
| `POST /plan` | backend | `plan_request.json`; reply `plan_response.json` |
| `POST /apply` | backend | `apply_request.json`; reply `apply_response.json` |
| `POST /operations/{name}` | backend | `operation_request.json`; reply `operation_response.json` plus one per pocket: `_activity`, `_storage`, `_health` |
| `POST /lifecycle` | backend | `lifecycle_request.json`; reply `lifecycle_response.json` |
| any refusal | add-on | `error.json` |

- **What a side produces**: the Go value it builds from the fixture marshals
  byte-equal to the fixture after normalisation, and satisfies the schema. The
  backend's inline bodies (setting, lifecycle, the three read operations) are
  captured as they leave the real function, not rebuilt in the test.
- **What a side consumes**: the fixture decodes (strictly, where the code is
  strict) and every field that side reads lands. The backend runs the real
  client function against a server answering with the fixture.
- **Every fixture validates against its schema**, in both modules
  (`santhosh-tekuri/jsonschema/v6`, test scope; schemas load from disk by
  `$id`, nothing is fetched). A fixture on disk that no test lists fails.
- **The registries are compared with the code.** An error code the add-on writes
  and `errors.json` lacks fails the add-on's suite; an add-on source file that
  writes a code as an inline literal fails it too. An operation in the backend's
  `operationPolicy` that `operations.json` lacks, or lists differently, fails the
  backend's suite.

Changing one of these files is a wire-contract change. It fails test suites in
two modules until both ends move, which is the point.

## What the fixtures deliberately do not cover

`apply_response.json` pins the SUCCESSFUL shape. A refusal, a binding conflict and
an unverified apply are mutually exclusive with it and with each other, so a
single fully populated document cannot represent them; each is asserted in the
suite that produces it. `plan_response.json` carries one of each kind of outcome
for the same reason. `operation_response.json` populates the scalar fields of the
reply (`rotated` and `account_uid` belong to different operations in practice) so
that the whole document is held; the three pocket fixtures are realistic.

The backend does NOT decode replies strictly, except the manifest. That is a
choice: an add-on ahead of its backend must not break it by reporting more than
it is asked about. It is asserted, not assumed (`contract_test.go`).

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

