# Writing a Syndra add-on

For an agent building an add-on for an app Syndra has never seen, with this
guide, `addons/contract/` and the conformance kit (`addons/conformance/`) as
its only context.

The key words "MUST", "MUST NOT", "REQUIRED", "SHALL", "SHALL NOT", "SHOULD",
"SHOULD NOT", "RECOMMENDED", "NOT RECOMMENDED", "MAY", and "OPTIONAL" in this
document are to be interpreted as described in BCP 14 (RFC 2119, RFC 8174)
when, and only when, they appear in all capitals, as shown here.

**Where the truth is.** The contract is the files in `addons/contract/`: one
JSON Schema 2020-12 document per message (`v2/schema/`), every route in
OpenAPI 3.1 (`v2/openapi.yaml`), the closed vocabularies and constants
(`v2/registry/`), a golden document per message (`v2/fixtures/`) and the
transport vector (`transport_derivation.json`). This guide gives the rules
those files cannot express and says where each shape lives; for a field list,
read the schema. Every JSON block below is a fixture copied byte for byte, its
path in the comment above it, and `addons/contract/guide_test.go` fails when
one drifts. Paths that begin `v2/` or name a file directly in
`addons/contract/` are relative to that directory; every other path is from
the repository root.

**What is live.** Syndra and the TrueNAS add-on speak contract 2 today. Four
features are in the schemas but not yet used by Syndra: `results`,
`freshness`, `calls` and `stats`. Every rule about them is marked
**Designed, not yet served by Syndra.** Declare a feature only once you serve
all of it, and only once Syndra serves it is it worth building.

## 1. What an add-on is and is not

**The boundary rule.** The add-on alone holds the app's address and
credential, and the add-on alone talks to the app. Syndra talks only to
add-ons, and only to ask: for state, a plan, an apply, an operation, a read,
or what the add-on has collected. Syndra never calls an app.

- The add-on MUST be the only component configured with the app's address and
  credential, and the only one that calls the app.
- The add-on MUST read the app on its own schedule, within the app's rate
  limits and availability; nothing Syndra sends dictates how or when the app
  is read.
- Syndra guards its side with an allowlist of the HTTP clients its backend
  constructs (`backend/internal/addons/no_second_client_test.go`). The kit
  cannot see Syndra's side, and Syndra cannot see yours: this rule is yours
  to keep.

**What an add-on is.**

- **A translator.** Syndra decides who holds what and sends it as desired
  state; the add-on converges the app to it and reports what the app then
  holds. It decides nothing about who gets access. Syndra is the single
  mutation authority.
- **The least trusted component in the deployment.** It holds a third-party
  credential and talks to a machine Syndra does not control. Its manifest is a
  ceiling, never a grant: Syndra intersects every declared operation with its
  own policy, and the policy wins (chapter 7).
- **One container on its own network**, shared with the backend and nothing
  else, with no published port, behind a Compose profile so that a deployment
  without the app never runs a container holding its credential.
- **A store, never a queue.** It keeps a record of each write's reply (so a
  replay is safe), a mutation log (so a write is accountable) and, if it
  wants, a mirror of the app (so a read can answer while the app is down).
  Work waits in Syndra's queue only, the one that knows why a change exists.

**The two planes.** If a thing has a steady state, it is a **field** of
desired state, converged every time from whatever is there (chapter 3). Only
if it cannot have one is it an **operation**, a one-shot event (chapter 7).
A lock is a field: as an operation it once left an account dark after access
was restored (`addons/README.md`).

### Words used in this guide

| Word | Meaning |
|---|---|
| subject | A person, by Syndra's id for them (`subject` on the wire). Never the app's name for them |
| account | The subject's account on the app, found by its `account_id`: the app's own stable, opaque id (a uid, a UUID) |
| desired state | What Syndra wants true for one subject, keyed by entitlement field. Level-triggered: sent again, it changes nothing |
| observed | What the app reports after a write, read back, never what was sent |
| role | What a field is: one of `account`, `channel`, `membership`, `flag`, `limit` (`v2/registry/roles.json`, chapter 3) |
| managed set | The values of a `membership` field Syndra can remove: what `GET /values/{field}` lists, or, for a field that is not enumerable, the values Syndra's mappings name |
| withdraw-only | A desired state carrying only the `account` and `channel` fields, all `false`. It switches an account off and never gives or creates anything |
| standing | Whether an existing account can be used yet, and who is waited on (`v2/schema/common/standing.json`). Observed, never desired |
| fingerprint | A digest of the subject's state on the app that a plan was computed against, verified again immediately before the write |
| `call_id` | The deduplication token every write carries in its body. One `call_id`, at most one effect |
| request identity | SHA-256 over the RFC 8785 canonical JSON of a write's identity object; what makes a same-`call_id` request a replay |
| current | `current: true`: this answer comes from a read of the app that began after this request arrived. Nothing else |
| stale | Why a reply is not current (`v2/schema/common/stale.json`); present exactly when `current` is `false` |
| kind | What sort of system the add-on fronts: one word of a closed list (chapter 11) |
| feature | A capability beyond the core, declared in the manifest (`v2/registry/features.json`, chapter 15) |
| mutation log | The add-on's append-only record of every write it sends the app, its head anchored by Syndra (chapter 9) |

## 2. Before you write a line

Four steps, in order. Each ends on something you can check.

1. **Record what the app answers.** Before writing a decoder, call each read
   method you will use and each write you will send, against a throwaway
   account on an instance the operator lets you touch, and write what came
   back to `addons/contract/<product>_observed.json`. Record keys and JSON
   types at every level a decoder reads, never values (a value there is a
   member's name). Record every refusal as the request field path it named,
   never the app's prose, which can carry a submitted password. A recorded
   read's field selection is the union of every selection your add-on issues
   against that method; a subset cannot catch the decode that reads the
   field it left out. The shipped example is `truenas_observed.json`, made by
   `scripts/record-truenas-fixtures.sh`; its README section says what each
   omission cost. *Done when:* the file names the app release it was recorded
   from, and every method your decoders read and every write you send is in
   it.
2. **Gate on what you recorded.** Read the app's version at start and on
   every health check. Against a version you have not recorded, the add-on
   MUST refuse every write with 422 `TARGET_VERSION_UNSUPPORTED`, report
   `version_tested: false` with a `version_note` in `/health`, and go on
   answering reads (`addons/truenas/nas.go:679`, `MajorSupported`). An app
   with no version endpoint is gated on what you can probe, and the probe is
   recorded like any read. *Done when:* a test starts the add-on against an
   unrecorded version and sees an apply refused and `/subjects` answered.
3. **Pick the stable id.** Find the app's own id for an account that
   survives a rename. It travels as `account_id`, an opaque non-empty string
   (`v2/schema/common/field.json#/$defs/AccountId`); an integer id is
   formatted as a string. A name is display, never identity. If the app has a
   field you can write at create to hold the Syndra subject id (UniFi Access
   `employee_number`), note it: it is the second way to find an account after
   a crash (`POST /apply`, chapter 4). *Done when:* you can name the id field
   and your evidence that a rename keeps it, or have listed that as
   unverified.
4. **Decide what the credential can do, and have it issued without the
   rest.** List the app scopes or roles your recorded reads and writes need,
   and nothing beyond them. The credential MUST NOT hold a scope that acts on
   the world (unlocking a door, starting a machine) or destroys data
   (deleting an account, editing stock, writing off a loss) wherever the app
   offers a narrower one. The operator issues it, so this, not your code, is
   the guard; `/health` reports the scopes as `credential_scopes`
   (self-reported, so advisory), and the kit warns on your kind's denylist in
   `v2/registry/kinds.json` (`access`: `edit:space`). Where the app offers
   nothing narrower, say so, as `addons/truenas/README.md` does for TrueNAS's
   `ACCOUNT_WRITE`, which can delete. *Done when:* your README lists every
   scope with its reason, none is on your kind's denylist, and `/health`
   reports the same list.

## 3. The manifest: `GET /capabilities`

Schema: `v2/schema/manifest.json`. This fixture declares every feature so
that every key appears; the TrueNAS add-on's real manifest is
`v2/fixtures/manifest_truenas.json`, and the reference add-on builds its own
at `addons/reference/reference.go:173`.

<!-- fixture: v2/fixtures/manifest.json -->
```json
{
  "contract_version": 2,
  "product": "TrueNAS",
  "product_version": "TrueNAS-25.10.5",
  "kind": "storage",
  "features": [
    "operations",
    "settings",
    "storage.reach",
    "storage.homes",
    "calls",
    "freshness",
    "stats",
    "results"
  ],
  "entitlement_schema": [
    {
      "name": "group",
      "role": "membership",
      "label": "Group",
      "x-truenas-pool": "tank"
    },
    {
      "name": "home",
      "role": "flag",
      "label": "Gives a home folder"
    },
    {
      "name": "home_quota",
      "role": "limit",
      "label": "Home folder limit",
      "unit": "bytes",
      "combine": "max",
      "unlimited": "0",
      "per_person": true
    },
    {
      "name": "enabled",
      "role": "account"
    },
    {
      "name": "smb_enabled",
      "role": "channel",
      "label": "SMB"
    }
  ],
  "operations": [
    {
      "id": "password.set",
      "scope": "member",
      "confirm": false,
      "secret_params": [
        "password"
      ],
      "available": true,
      "params": {
        "type": "object",
        "properties": {
          "password": {
            "type": "string",
            "title": "New password"
          }
        }
      }
    },
    {
      "id": "password.rotate",
      "scope": "admin",
      "confirm": false,
      "available": true
    },
    {
      "id": "account.adopt",
      "scope": "admin",
      "confirm": true,
      "available": true
    },
    {
      "id": "storage.status",
      "scope": "member",
      "confirm": false,
      "available": true
    },
    {
      "id": "account.release",
      "scope": "admin",
      "confirm": true,
      "available": true
    },
    {
      "id": "account.purge",
      "scope": "admin",
      "confirm": true,
      "secret_params": [
        "elevated_key"
      ],
      "available": false,
      "unavailable_reason": "This TrueNAS release lacks the method the operation needs."
    },
    {
      "id": "activity.get",
      "scope": "admin",
      "confirm": false,
      "available": true
    },
    {
      "id": "health.get",
      "scope": "admin",
      "confirm": false,
      "available": true
    }
  ],
  "connection": {
    "protocol": "smb",
    "host": "nas.example.org"
  },
  "metrics": [
    {
      "name": "storage.used_bytes",
      "instrument": "gauge",
      "unit": "bytes",
      "label": "Storage used",
      "per_subject": true,
      "dimensions": [
        {
          "name": "share",
          "label": "Share"
        }
      ],
      "against": "home_quota"
    },
    {
      "name": "storage.sessions",
      "instrument": "counter",
      "unit": "count",
      "label": "Storage sessions",
      "per_subject": true,
      "dimensions": [
        {
          "name": "share",
          "label": "Share"
        }
      ],
      "x-truenas-source": "audit.query"
    }
  ],
  "limits": {
    "subjects_per_plan": 200,
    "fresh_read_cooldown": "PT30S",
    "stats_buffer": "P90D"
  },
  "x-truenas": {
    "api": "websocket"
  }
}
```

Rules the schema cannot express:

- `contract_version` is `2`. Syndra compares it by equality at registration
  and calls nothing on an add-on that disagrees, naming both numbers in the
  refusal.
- `product` is what people call the product ("TrueNAS"); Syndra shows it as
  the system's name until an operator names the system. `product_version` is
  the app's own version string.
- `kind` is one word of `v2/registry/kinds.json` (chapter 11). Syndra refuses
  a manifest with any other kind.
- `features` lists only what the add-on serves in full (chapter 15).
- `entitlement_schema` is one entry per field of desired state, each with
  exactly one `role`, and exactly one field with role `account`. Pick the
  role by what the field does on the app:

  | Role | Use it for | Wire value | Rule the schema cannot state |
  |---|---|---|---|
  | `account` | The account's on/off switch | `true` / `false` | Exactly one. Set by Syndra's resolver (on when any role reaches the app, unless an allowance denies it); never mapped |
  | `channel` | A way of using the account (SMB, remote printing) | `true` / `false` | Set like `account`, and `false` whenever the account is `false`, whatever was asked |
  | `membership` | A set of app-defined values a role confers (groups, door policies) | sorted, de-duplicated strings | Roles combine by union. Only values in the managed set are ever removed |
  | `flag` | One thing a role gives or does not (a home folder, a materials right) | `["true"]` or `[]` | Roles combine by "any" |
  | `limit` | A numeric ceiling in a `unit` (`v2/registry/units.json`) | decimal strings | Roles combine by `max`, and the `unlimited` value dominates: max(`"0"`, 5 GiB) is no limit when `"0"` is unlimited. The add-on combines, once; Syndra reads the effective limit back from observed state |

- A field's `name` is the add-on's and is stored by Syndra in snapshots and
  bindings; its role is fixed for ever. Syndra persists the role map at
  registration and refuses a manifest that gives an existing field another
  role or names another field `account`
  (`backend/internal/addons/registry.go`, `acceptRoles`). A new meaning is a
  new name.
- `per_person: true` lets an operator set the field for one person over what
  roles give.
- `operations[]` is chapter 7; its `params` is a hint for the UI only.
- `connection` is how a member reaches the app: `host` is the name a member
  types, never the API address. Leave it out until the deployment says.
- `limits.subjects_per_plan` is the most subjects one `/plan` carries.
  `limits.fresh_read_cooldown` belongs to `freshness` (chapter 4) and
  `limits.stats_buffer` to `stats` (chapter 8). Every declared duration is
  ISO 8601 (`PT30S`, `P90D`). Syndra's own caps live in
  `v2/registry/constants.json`; an add-on never declares them.
- `metrics[]` is chapter 8.
- A key that begins `x-`, at the top or inside a field, operation, metric or
  dimension object, is vendor data: Syndra ignores it for every decision and
  shows it quoted on the system's page. Any other key the schema does not
  list fails the kit (`unevaluatedProperties: false`), so a misspelt
  `unlimted` is a kit failure, never a quota of zero.
- The manifest MUST validate against its schema with format assertion on,
  and MUST be served while the app is unreachable.

## 4. The core routes

Every add-on serves all seven core routes, and the kit does not pass without
them: `GET /capabilities` (chapter 3) and the six below. The rules in this
first part hold on every route.

### Transport

Specified, with vectors, in `transport_derivation.json`; the reference copy
is `addons/reference/transport.go`.

- One per-target secret, as its trimmed UTF-8 bytes (never hex-decoded),
  gives two keys through HKDF-SHA-256 (RFC 5869) with the target name as
  salt: an Ed25519 key (info `syndra/addon-tls/v1`) and an HMAC key (info
  `syndra/addon-hmac/v1`). The target name is the deployment's name for your
  add-on (`ADDON_TARGETS`, chapter 14).
- The add-on MUST serve HTTPS only, TLS 1.3, with a certificate carrying the
  derived Ed25519 key. Syndra pins that key and trusts nothing else.
- The add-on MUST verify `X-Syndra-Addon-Signature: t=<unix>,v1=<hex>` on
  every route, `/capabilities` and `/health` included: HMAC-SHA-256 under the
  HMAC key over `<unix>.<METHOD>.<target>.<body>`, where `target` is the
  escaped path exactly as sent, plus `?` and the raw query when the query is
  not empty. That is RFC 9421's `@query` as sent, except that an empty query
  adds nothing. The add-on MUST NOT sort, decode or re-encode the query
  before verifying it: a reordered query is a different query and fails.
- The add-on MUST refuse a timestamp more than two minutes from its clock
  before checking the MAC, compare MACs in constant time, and act on the very
  bytes it verified.
- Every failure MUST be 401 `UNAUTHENTICATED`, the same whichever half
  failed. Log the reason; never return it.
- The add-on MUST NOT answer with a redirect. Syndra refuses to follow one.
- Your own tests MUST reproduce every vector in `transport_derivation.json`
  (`derivations`, `signature`, `raw_query_signatures`).

### Every request and every reply

- **Strict requests.** Decode every request body strictly. A field the route
  does not know is 400 `UNKNOWN_REQUEST_FIELD`, never ignored; a body that is
  not the route's JSON is 400 `BAD_REQUEST`. Syndra sends a request field
  only to an add-on that declared the feature introducing it
  (`v2/registry/features.json`), so a strict add-on is never surprised.
- **`contract_version` in every request body.** Absent or not `2` is 422
  `CONTRACT_VERSION_MISMATCH` with `caller_version` (0 when absent) and
  `addon_version` (fixture in chapter 6).
- **Absence is never a value.** Every reply carries every field its schema
  lists in `required`. Syndra reads a missing `current`, `truncated`,
  `taken_at`, `standing.usable` or `effect` as "could not look", never as
  `false` or empty, so an add-on that forgets one has made its reply
  unusable, not quietly different.
- **Replies MAY carry more.** Outside the manifest's field, operation and
  metric objects, a reply MAY carry keys its schema does not list, and Syndra
  ignores them; name yours `x-<product>-…`.
- **Size.** A reply MUST fit in `constants.max_reply_bytes` (1 MiB); Syndra
  reads no further.
- **Unknown routes.** Every other method and path, a feature route your
  manifest does not declare included, is 404 `UNKNOWN_ROUTE`, authenticated
  like the rest.

### Every write: once per `call_id`

`POST /apply`, `POST /operations/{name}`, `POST /lifecycle` and
`PUT /settings/{name}` carry `call_id` in the body, inside the signature. A
write's **request identity** is the SHA-256 of the RFC 8785 (JCS) canonical
JSON of its identity object: `v2/schema/identity/apply.json`,
`operation.json`, `lifecycle.json`, `setting.json`. Your tests MUST
reproduce the four vectors `v2/fixtures/identity_{apply,operation,lifecycle,setting}.json`.
Canonicalise decoded values, so that two spellings of the same JSON are one
identity (`addons/reference/identity_test.go:51`). The rules, from
draft-ietf-httpapi-idempotency-key-header where it has one:

1. Same `call_id`, same identity: the add-on MUST answer the recorded reply,
   byte for byte, without acting again, before the lifecycle gate and before
   the fingerprint comparison. Syndra re-plans before it re-sends, so the
   fingerprint will have moved, and an apply that already happened is not
   refused for it.
2. The add-on MUST record only a 2xx reply. A refusal is evaluated afresh
   when the same `call_id` comes again.
3. Same `call_id`, another identity or another kind of write: 422
   `CALL_ID_REUSED`.
4. Same `call_id` while a request carrying it executes in this process: 409
   `CALL_IN_PROGRESS` with `Retry-After`. Hold this as an in-memory lease,
   taken before the mutation-log intent and released when the reply is
   written. Never infer it from an intent with no result, and never carry it
   across a restart: a re-send after a crash relies on lookup before create
   (`POST /apply`), and would otherwise be refused for ever.
5. The add-on MUST keep call records on disk for at least
   `constants.call_memory` (`P30D`).

### Freshness: `taken_at`, `current`, `stale`

Which replies carry `taken_at`, `current` and `truncated` is in each reply's
schema. `/values` carries none of them, because a list of legal values is not
an observation; `/stats` carries a watermark instead (chapter 8). The rules
the schemas cannot express:

- `current: true` MUST mean exactly: this answer comes from a read of the app
  that began after this request arrived. A read shared by concurrent requests
  counts, if it began after this one arrived. An answer from a mirror, a
  cache, or an earlier read is `current: false`, however recent. Syndra's
  drift sweep compares only against current reads; a mirror marked current
  would make every change since the mirror a false finding.
- `current: false` if and only if `stale` is present
  (`v2/schema/common/stale.json#/$defs/Invariant`). `stale.reason` says why;
  `retry_after` is in integer seconds, like `Retry-After`; `note` is a
  sentence for a person, in your words.
- `taken_at` (RFC 3339) is when the app was read, not when you answered.
- `truncated: true` MUST be set whenever a read stopped short of everything
  (a cap, a page limit). A truncated read cannot support a conclusion about
  absence, and Syndra draws none from it. Never cap silently.
- **A slow or failing app yields `stale`, never an error.** When the app is
  slow or down, a read that carries `current` MUST answer the last value it
  read, with `current: false`, `stale.reason: unreachable` and that read's
  `taken_at`. Bound your own wait on the app well inside Syndra's 30-second
  timeout per call (`callTimeout`, `backend/internal/addons/deps.go`; a
  Syndra setting, not a contract constant), so the answer arrives. Only when
  you have nothing at all to serve does the read answer an error: 503
  `NO_CURRENT_READ`, as the TrueNAS add-on does
  (`addons/truenas/server.go:424`). `/values` has no `current`, so it answers
  503 `TARGET_UNREADABLE` when it cannot read (429 when rate-limited, 502
  when the app answered with a failure).
- **`fresh` (feature `freshness`). Designed, not yet served by Syndra.**
  Syndra will send `fresh=true` on `GET /subjects`, `GET /values/{field}`,
  `GET /reach` and `GET /homes`, and `fresh: true` in the body of an
  `effect: read` operation, to ask for a read of the app now; never on
  `/plan`, `/apply` or `/stats`. Its drift sweep's `/subjects` read will
  always send it and treat a `cooldown` answer as a skipped pass; display
  reads will send it only when a person presses Refresh. An add-on that
  declares `freshness`:
  - MAY answer a read without `fresh` from its mirror, as `current: false`
    with `stale.reason: cached`;
  - MUST, given `fresh: true`, read the app now, or answer the last value it
    read with `stale.reason: cooldown` and `retry_after`;
  - MUST coalesce concurrent fresh reads of the same thing into one read of
    the app, and hold a per-target and a per-subject cooldown of
    `limits.fresh_read_cooldown` (default `constants.cooldown_default`,
    `PT30S`);
  - MUST report `health.target_reads`, a count of its reads of the app that
    only ever goes up, so that the kit can prove the three rules above;
  - answers 429 `FRESH_READ_COOLDOWN` or 503 `TARGET_UNREACHABLE`, each with
    `Retry-After`, only when it has nothing at all to serve. Syndra reads
    `TARGET_UNREACHABLE` as "not reached", never as a lifecycle refusal.

  Without the feature `stale` still applies: an add-on answering from its
  mirror on purpose says `cached`, as the TrueNAS add-on's `/reach` does.

### `GET /health`

Schema: `v2/schema/health.json`.

<!-- fixture: v2/fixtures/health.json -->
```json
{
  "reachable": true,
  "product": "TrueNAS",
  "product_version": "TrueNAS-25.10.5",
  "version_tested": true,
  "version_note": "25.10 is a tested release.",
  "last_read_at": "2026-11-10T09:29:41Z",
  "credential_scopes": [
    "account.write",
    "audit.read"
  ],
  "credential_expiry": "set",
  "credential_expires_at": "2027-02-01T00:00:00Z",
  "unaudited_shares": [
    "scratch"
  ],
  "shares_readable": true,
  "circuit_open": false,
  "lifecycle": "read_only",
  "lifecycle_note": "Maintenance window.",
  "in_flight": 0,
  "drained": true,
  "log_head": "sha256:7be1c0a94d2f",
  "log_records": 1284,
  "target_reads": 412,
  "snapshot_taken_at": "2026-11-10T09:25:00Z",
  "checked_at": "2026-11-10T09:30:00Z"
}
```

- The add-on MUST serve it while the app is unreachable. `reachable` says
  whether a real call to the app just succeeded, not whether the add-on is
  up.
- `log_head` and `log_records` are the mutation log's head digest and its
  length (chapter 9). Syndra anchors the head, so a truncated or rewritten
  log is detected rather than believed. `log_records` MUST advance on every
  write the add-on sends the app, and MUST NOT move on a read, a replay, a
  refusal or a preview.
- `lifecycle`, `lifecycle_note`, `in_flight` and `drained` are chapter 10;
  `version_tested` and `version_note` are chapter 2, step 2;
  `credential_scopes` is chapter 2, step 4. `credential_expiry` is `set`
  (with `credential_expires_at`), `none` or `unrecorded`.
- `unaudited_shares` and `shares_readable` are the storage add-on's; leave
  them out unless they mean something for your app. `target_reads` comes
  with `freshness` only.

### `GET /subjects`

Schema: `v2/schema/subjects.json`. The same reply answered from a mirror is
`v2/fixtures/subjects_stale.json`.

<!-- fixture: v2/fixtures/subjects.json -->
```json
{
  "subjects": [
    {
      "id": 17,
      "username": "rey_mysterio",
      "account_id": "3051",
      "groups": [
        "fabrication",
        "lab_makers"
      ],
      "home": "/mnt/tank/homes/rey_mysterio",
      "enabled": true,
      "smb_enabled": true,
      "state": {
        "group": [
          "fabrication",
          "lab_makers"
        ],
        "home": [
          "true"
        ],
        "home_quota": [
          "0",
          "5368709120"
        ],
        "enabled": true,
        "smb_enabled": true
      },
      "standing": {
        "usable": false,
        "waiting_on": "member",
        "note": "No password set yet",
        "next": "Set a storage password on your page"
      }
    },
    {
      "id": "4",
      "username": "syndra_service",
      "account_id": "3001",
      "groups": [],
      "enabled": true,
      "smb_enabled": false,
      "self": true,
      "state": {
        "group": [],
        "home": [],
        "enabled": true,
        "smb_enabled": false
      },
      "standing": {
        "usable": true,
        "waiting_on": "none"
      }
    }
  ],
  "current": true,
  "taken_at": "2026-11-10T09:30:00Z",
  "truncated": false
}
```

The `home_quota` pair in this fixture's `state` shows the shape of a list,
not the rule: a `limit` in `state` is the one effective value the app holds
(`["5368709120"]`, or `["0"]` when unlimited). The fixture is flagged for
correction in `openspec/changes/addon-contract/tasks.md` (5.1).

- List every account the app holds, managed or not, in one reply. Syndra
  matches accounts to people itself; an account nobody manages is listed and
  left alone. Mark the add-on's own service account `self: true`.
- `id` is the app's record key, `account_id` the stable id (chapter 2),
  `username` display only. `groups`, `home`, `enabled` and `smb_enabled` on
  a subject are the storage add-on's own; put every value in `state`.
- `state` holds the account's current value for every field of your
  entitlement schema, by field name, in its role's wire shape: `account` and
  `channel` as JSON booleans, every other role as a sorted, de-duplicated
  list of strings, `[]` and never `null` for none. A `limit` is the one
  effective value; leave it out when it could not be read, rather than
  guess.
- `standing` is on every subject, because every listed subject has an
  account. It is observed: why this account can or cannot be used yet, and
  who is waited on (`member`, `operator`, `target` or `none`). `note` and
  `next` are shown to people as you wrote them, so write them as sentences
  with no internal words. When two things wait on the same person, write one
  `note` naming both and one `next`. A permanent difference the app imposes
  (an invitation not yet accepted) is reported as `standing` with
  `waiting_on: target` and the `account` field as the app holds it, so that
  the apply never loops trying to change what it cannot.
- One reply, no paging: `subjects.paged` is deferred until a target needs it
  (about 300 members fit well inside 1 MiB). If you cap the read, say
  `truncated: true`.

### `GET /values/{field}`

Schema: `v2/schema/values.json`.

<!-- fixture: v2/fixtures/values.json -->
```json
{
  "field": "group",
  "values": [
    "fabrication",
    "lab_makers"
  ],
  "enumerable": true,
  "labels": {
    "fabrication": "Fabrication",
    "lab_makers": "Lab makers"
  },
  "builtin_count": 34,
  "builtin_examples": [
    "builtin_users",
    "builtin_administrators"
  ]
}
```

- Every field of the entitlement schema answers; any other name is 404
  `UNKNOWN_FIELD`.
- `account` and `channel` answer `["false","true"]`, a `flag` answers
  `["true"]`, a `limit` answers `enumerable: false` with `values: []`.
- For a `membership` field, `values` is the managed set: the values that
  exist now and that Syndra gives and takes away, sorted, with `labels`
  for people. Values the app owns that Syndra never manages (built-in
  groups) are left out, counted in `builtin_count` and sampled in
  `builtin_examples`.
- A field whose values cannot be listed (thousands of items) answers
  `enumerable: false`; its managed set is then the values Syndra's mappings
  name.
- Read it live: values are the app's runtime state.

### `POST /plan`

Schemas: `v2/schema/plan_request.json`, `v2/schema/plan_response.json`.

<!-- fixture: v2/fixtures/plan_request.json -->
```json
{
  "contract_version": 2,
  "subjects": [
    {
      "subject": "289471021834760193",
      "email": "bianca.belair@example.org",
      "desired": {
        "group": [
          "lab_makers",
          "fabrication"
        ],
        "home": [
          "true"
        ],
        "home_quota": [
          "5368709120"
        ],
        "enabled": true,
        "smb_enabled": true
      }
    }
  ]
}
```

<!-- fixture: v2/fixtures/plan_response.json -->
```json
{
  "outcomes": [
    {
      "subject": "289471021834760193",
      "effect": "apply",
      "detail": "Would add fabrication.",
      "consequence": "In lab_makers, fabrication. Enabled, SMB on.",
      "username": "rey_mysterio",
      "account_id": "3051",
      "standing": {
        "usable": false,
        "waiting_on": "member",
        "note": "No password set yet",
        "next": "Set a storage password on your page"
      },
      "fingerprint": "sha256:9d1c4f0b7e2a"
    },
    {
      "subject": "289471021834760194",
      "effect": "blocked",
      "detail": "An account named seth_rollins already exists and is not bound to anyone.",
      "consequence": "Nothing would be changed.",
      "conflict": {
        "username": "seth_rollins",
        "account_id": "3077",
        "adoptable": false,
        "bound_to": "289471021834760195"
      }
    }
  ],
  "current": true,
  "taken_at": "2026-11-10T09:30:00Z",
  "truncated": false
}
```

- A plan MUST change nothing: not the app, the mutation log or the call
  record. It carries no `call_id`.
- Above `limits.subjects_per_plan` subjects, answer 422 `PLAN_TOO_LARGE` with
  `subjects` and `limit`. Syndra splits its plans below your limit.
- One outcome per subject, in the shape an apply answers
  (`v2/schema/apply_response.json`), computed by the same decision as the
  apply (chapter 5): `effect` is `apply` (it would change), `no_change` or
  `blocked`. Every outcome that is not `blocked` MUST carry the
  `fingerprint` the apply will send back. `detail` says what changes;
  `consequence` says what the subject holds afterwards.
- For a subject with no account and a desired `account: true`, the outcome is
  `apply` with the `username` the create would derive; nothing is created.
- `current: true` means computed against a read of the app that began after
  this request arrived. Planning from a mirror is `current: false` with
  `stale`, and Syndra applies neither a non-current nor a truncated plan.
  A plan is never cooled down and never takes `fresh`.

### `POST /apply`

Schemas: `v2/schema/apply_request.json`; the 200 body
`v2/schema/apply_ok.json`. A blocked apply is the problem
`v2/fixtures/error_apply_blocked.json`; a binding conflict is the last
fixture here.

<!-- fixture: v2/fixtures/apply_request.json -->
```json
{
  "contract_version": 2,
  "call_id": "0f8b7a1c-6d2e-4c53-9a10-2b7c9d4e5f60",
  "subject": "289471021834760193",
  "email": "bianca.belair@example.org",
  "fingerprint": "sha256:9d1c4f0b7e2a",
  "plan_id": "3c9e2f81-44ab-4d0e-8f21-77aa10bb33cd",
  "actor": "operator-289471021834760199",
  "desired": {
    "group": [
      "lab_makers",
      "fabrication"
    ],
    "home": [
      "true"
    ],
    "home_quota": [
      "5368709120"
    ],
    "enabled": true,
    "smb_enabled": true
  }
}
```

<!-- fixture: v2/fixtures/apply_response.json -->
```json
{
  "subject": "289471021834760193",
  "effect": "applied",
  "detail": "Added fabrication. Unlocked the account.",
  "consequence": "In lab_makers, fabrication. Enabled, SMB on.",
  "username": "bianca.belair",
  "account_id": "3042",
  "standing": {
    "usable": false,
    "waiting_on": "member",
    "note": "No password set yet",
    "next": "Set a storage password on your page"
  },
  "fingerprint": "sha256:9d1c4f0b7e2a",
  "observed": {
    "group": [
      "fabrication",
      "lab_makers"
    ],
    "home": [
      "true"
    ],
    "home_quota": [
      "5368709120"
    ],
    "enabled": true,
    "smb_enabled": true
  }
}
```

<!-- fixture: v2/fixtures/error_binding_conflict.json -->
```json
{
  "type": "urn:syndra:addon:error:BINDING_CONFLICT",
  "title": "Binding conflict",
  "status": 409,
  "detail": "An account named seth_rollins already exists and is not bound to anyone.",
  "code": "BINDING_CONFLICT",
  "retry": "never",
  "subject": "289471021834760194",
  "effect": "blocked",
  "consequence": "Nothing was changed.",
  "conflict": {
    "username": "seth_rollins",
    "account_id": "3077",
    "adoptable": false,
    "bound_to": "289471021834760195"
  }
}
```

In the order the add-on works:

1. Decode strictly and check `contract_version`. `call_id`, `subject` and
   `desired` absent or blank is 400 `BAD_REQUEST`; `fingerprint` absent is
   400 `FINGERPRINT_REQUIRED` (here or after step 2).
2. Answer a replay (rules above), identity `{subject, desired}`. `email`,
   `plan_id` and `fingerprint` are outside the identity, because a re-plan
   changes them for the same call.
3. Refuse a write while the lifecycle is not `active` (503
   `LIFECYCLE_REFUSAL`, chapter 10) or against an unrecorded app version
   (422 `TARGET_VERSION_UNSUPPORTED`, chapter 2).
4. Find the subject's account: by its recorded `account_id` first.
5. Read that account on the app now, decide (chapter 5), and compare the
   fingerprint of what you read with the request's. A difference is 409
   `PLAN_STALE` and nothing is written. The comparison MUST happen
   immediately before the write, from that read.
6. Append the intent to the mutation log, synced to disk, before the write
   (chapter 9).
7. Write, read the account back, and answer `observed` from the read-back:
   what the app reports, never what you sent. When the read-back fails, set
   `unverified: true` and leave `observed` out.

What `desired` means, all MUST:

- **Absent is unmanaged.** A field absent from `desired` is not touched. A
  field present and empty means "make it empty".
- **Withdraw-only never creates.** A desired state with no `account: true`
  for a subject with no account is `no_change`: nothing is created, and the
  outcome carries no `account_id` and no `standing`. For an existing account
  it switches the account and every channel off and leaves every other field
  as it was.
- **A channel follows the account.** Every `channel` field is `false`
  whenever the `account` field ends `false`, whatever `desired` said.
- **Unlisted values are preserved.** On a `membership` field, a value the app
  holds outside the managed set is never removed. Where the app's API
  replaces a subject's whole set, read, merge, then write. Where it replaces a
  set shared by many subjects (a group's member list), the add-on MUST
  serialise read, merge and write per shared set, and preserve the members
  Syndra does not manage as well as the values, so that two applies for two
  subjects never lose each other's change.
- **A limit is combined once, here.** By `max`, the `unlimited` value
  dominating; `observed` carries the effective limit, one value, or leaves it
  out when it could not be read.
- **Every value is checked against its role** (a `flag` value is `"true"`, a
  `limit` value a non-negative integer, a `membership` value one the app
  has). A desired state the app cannot hold is 422 `APPLY_BLOCKED`, its
  outcome as extension members, `consequence` saying what was left as it was.

**Identity and the one create** (all MUST; the three studied apps have no
idempotent create):

- Look up before you create. An **unbound** account that matches by the
  app's external-reference field or by the derived name is adopted **only**
  when the add-on's own mutation log holds a create intent for this subject
  with no recorded id: the crash between a create and its record. Any other
  match, or two candidates, is 409 `BINDING_CONFLICT` with `conflict`
  (`username`, `account_id`, `adoptable`, `bound_to` when another subject
  holds it), and an operator decides through the confirmed `account.adopt`
  operation. Anyone with console access can type an external reference, and
  a wrong adoption hands one person's data to another.
- Write the mutation-log intent before the create, record the returned id
  under the `call_id` before you answer, and answer a replay from that
  record.
- Derive a new account's name once, at create (`addons/truenas/username.go`
  is the storage derivation); the binding is authoritative from then on. A
  rename on the app is recognised by `account_id`, reported, and never taken
  for a missing account.

**The reply.** A 200 carries `effect: applied` or `no_change`. `account_id`
and `standing` travel together and only when an account exists
(`dependentRequired` in the schema). A blocked or conflicting outcome is
never a 200: it is the problem `APPLY_BLOCKED` (422) or `BINDING_CONFLICT`
(409) with the outcome's fields as extension members (RFC 9457 §3.2).

**Failure modes.** 400 `BAD_REQUEST`, `FINGERPRINT_REQUIRED`,
`UNKNOWN_REQUEST_FIELD`; 409 `PLAN_STALE` (re-plan), `BINDING_CONFLICT`,
`CALL_IN_PROGRESS`; 422 `APPLY_BLOCKED`, `CALL_ID_REUSED`,
`CONTRACT_VERSION_MISMATCH`, `TARGET_VERSION_UNSUPPORTED`; 503
`LIFECYCLE_REFUSAL`; and `APPLY_FAILED` for anything else, its status saying
whether retrying can help (chapter 6 says how Syndra reads each status).

### `POST /lifecycle`

Schemas: `v2/schema/lifecycle_request.json`,
`v2/schema/lifecycle_response.json`.

<!-- fixture: v2/fixtures/lifecycle_request.json -->
```json
{
  "contract_version": 2,
  "call_id": "7c1e5b92-3a08-4d6f-b1c4-90e2d8a7f314",
  "state": "read_only",
  "reason": "Moving the pool to new disks."
}
```

<!-- fixture: v2/fixtures/lifecycle_response.json -->
```json
{
  "state": "read_only",
  "reason": "Moving the pool to new disks.",
  "in_flight": 0,
  "drained": true
}
```

- The operator's stop switch, so every add-on serves it. `state` is
  `active`, `draining` or `read_only` (chapter 10); any other is 422
  `UNKNOWN_LIFECYCLE_STATE`. A blank `reason` is 400 `REASON_REQUIRED`: a
  state nobody explained looks like a fault.
- It is a write, deduplicated by `call_id` with identity `{state, reason}`,
  and it is accepted in every state.
- The reply carries `in_flight` and `drained` as `/health` does.

### Settings: `GET /settings` and `PUT /settings/{name}` (feature `settings`)

A system-wide choice (where home folders go), with one write place in
Syndra. Schemas: `v2/schema/settings.json` (a list of
`v2/schema/setting.json`), `v2/schema/setting_put_request.json`. The `GET`
reply is `v2/fixtures/settings.json`.

<!-- fixture: v2/fixtures/setting_put_request.json -->
```json
{
  "contract_version": 2,
  "call_id": "b3f0c6d4-8e17-4a52-9d6b-1f4e7a2c5d80",
  "value": "/mnt/tank/homes",
  "actor": "operator-289471021834760199"
}
```

<!-- fixture: v2/fixtures/setting.json -->
```json
{
  "name": "home_root",
  "label": "Where home folders go",
  "value": "/mnt/tank/homes",
  "set_by": "operator-289471021834760199",
  "set_at": "2026-11-09T14:05:00Z",
  "values": [
    "/mnt/tank/homes",
    "/mnt/tank/members"
  ],
  "labels": {
    "/mnt/tank/homes": "tank › homes",
    "/mnt/tank/members": "tank › members"
  },
  "values_read": true
}
```

- `value` is the current choice, `""` when none is made. `values` are what
  the app offers now, `values_read: false` when they could not be read.
- A `PUT` is a write: deduplicated by `call_id` with identity
  `{name, value}`, refused while the lifecycle is not `active`. An unknown
  name is 404 `NO_SUCH_SETTING`; a value not offered now (and not `""`) is
  refused, 400 `INVALID_BODY` in the reference add-on and 422
  `NOT_A_HOME_PLACE` in the storage add-on. The reply is the setting as now
  held, with `set_by` (the `actor`) and `set_at`.

## 5. Plan and apply agree

- The add-on MUST compute a plan's outcome and an apply's write with one pure
  decision: from the subject's account as just read, and `desired`, to the
  effect, the consequence and the writes. The plan serialises the decision;
  the apply reads again, decides again, compares fingerprints and executes
  that decision. There is no second implementation for the plan.
- The fingerprint is a digest of exactly the state the decision read, and
  "no account" has a value of its own, distinct from "an account holding
  nothing" (`addons/reference/reference.go:508`).
- The kit's `plan-equals-apply` check holds you to it on every convergence
  it makes: a plan's `apply` is the apply's `applied`, with the same
  `consequence`.
- In code: the reference's `decide` (`addons/reference/reference.go:342`) is
  called by its plan (`:292`) and its apply (`:331`); the TrueNAS add-on's
  `decide` (`addons/truenas/apply.go:679`) by `plan.go:253` and
  `apply.go:726`.

## 6. Errors

Schema: `v2/schema/error.json`; every code: `v2/registry/errors.json`.

<!-- fixture: v2/fixtures/error.json -->
```json
{
  "type": "urn:syndra:addon:error:CONTRACT_VERSION_MISMATCH",
  "title": "Contract version mismatch",
  "status": 422,
  "detail": "This add-on speaks contract 2; the request declared 1.",
  "code": "CONTRACT_VERSION_MISMATCH",
  "retry": "never",
  "caller_version": 1,
  "addon_version": 2
}
```

- Every non-2xx MUST be `application/problem+json` (RFC 9457) and validate
  against `error.json`, with no exception: `type` is
  `urn:syndra:addon:error:<CODE>`, `title` is the registry's title for the
  code, `status` is the HTTP status, and `code` is a registered code with
  that status among the registry's.
- `retry` is the code's class: `never`, `backoff`, `retry_after` or
  `replan`. A `by_status` code resolves per reply; the TrueNAS add-on writes
  `backoff` for 429 and 5xx and `never` otherwise
  (`addons/truenas/errors.go`). A `retry_after` code MUST send a
  `Retry-After` header (RFC 9110), in seconds.
- Anything else a refusal carries is an extension member (RFC 9457 §3.2):
  the outcome of a blocked apply, `caller_version` and `addon_version`,
  `subjects` and `limit`, `lifecycle`, `subject`.
- `detail` is a sentence for an operator, in the add-on's own words. It
  MUST NOT carry the app's own error text (it can echo a submitted
  password), a request body, or any secret. Name a bad input by its field
  path (`desired.printers`), never by quoting its value.
- A new code is a registry entry and a contract change (chapter 15), never
  something an add-on invents.

**How Syndra reads a status** (`classifyStatus`,
`backend/internal/addons/transport.go`), which is why the codes matter:

| Reply | Syndra concludes |
|---|---|
| 2xx | it acted, and said so |
| `LIFECYCLE_REFUSAL` (by code alone) | not reached: a maintenance window; it waits |
| `CONTRACT_VERSION_MISMATCH`, `UNKNOWN_REQUEST_FIELD`, a 404 on a feature route | not reached, and the manifest is read again: the add-on is not the one Syndra thought |
| `CALL_IN_PROGRESS`, `TARGET_UNREACHABLE`, any 429 | not reached; asked again later |
| 408, any 5xx | **possibly acted**: never resent automatically, and shown to a person |
| any other 4xx | it checked the request and did nothing |

So the add-on MUST answer a 4xx only when it wrote nothing, and SHOULD
answer a write with a 5xx only when it cannot tell whether the app acted: a
5xx sent before any write costs an operator a check for nothing.

## 7. Operations

### `POST /operations/{name}` (feature `operations`)

Schemas: `v2/schema/operation_request.json`,
`v2/schema/operation_response.json`.

<!-- fixture: v2/fixtures/operation_request.json -->
```json
{
  "contract_version": 2,
  "call_id": "5a2d9e30-1b44-4f7c-9e08-6d3a2c1b0f95",
  "operation": "storage.status",
  "subject": "289471021834760193",
  "actor": "289471021834760193",
  "plan_id": "3c9e2f81-44ab-4d0e-8f21-77aa10bb33cd",
  "fingerprint": "sha256:9d1c4f0b7e2a",
  "params": {
    "since": "2026-11-01T00:00:00Z"
  },
  "preview": false,
  "fresh": true
}
```

<!-- fixture: v2/fixtures/operation_response.json -->
```json
{
  "operation": "account.adopt",
  "subject": "289471021834760193",
  "outcome": "succeeded",
  "detail": "Bound the existing account rey_mysterio.",
  "rotated": true,
  "account_id": "3051"
}
```

**Registry first, policy second, manifest third.** An operation id is
contract vocabulary: `v2/registry/operations.json` names every id Syndra will
ever have a policy for, with its meaning, its broadest scope, whether it
needs confirming, its parameters, its secret parameters and its `effect`.
Syndra's own policy is held equal to the registry by test. A manifest that
declares an id the registry lacks has that operation dropped at
registration and logged (`backend/internal/addons/policy.go`). A new
operation is a registry entry and a policy entry (chapter 15), then a line
in your manifest.

The manifest entry is a ceiling:

- `scope` (`member` or `admin`) is the broadest you offer it at; Syndra uses
  the narrower of yours and its policy's.
- `confirm`: Syndra asks the person to confirm when either side says so.
- `secret_params`: Syndra treats as secret the union of yours and its
  policy's.
- `available: false` with an `unavailable_reason` when the app cannot do it
  right now. Never leave an operation out to mean that.
- `params`: a JSON Schema for the UI's labels and pickers, never used for a
  decision; Syndra validates parameters against its policy alone.

Rules for the route:

- The path and the body name the operation; a disagreement is 400
  `OPERATION_MISMATCH`, never resolved in either direction. An unknown name
  is 404 `OPERATION_FAILED`; a missing parameter is 400 `OPERATION_FAILED`.
- **A member scope binds the subject.** For a member operation Syndra sets
  `subject` to the member acting. The add-on MUST act on the body's
  `subject` and on no subject a parameter names.
- An operation that writes is deduplicated by `call_id` with identity
  `{name, subject, params}` minus every secret parameter, so a re-send with
  the same `call_id` and another password is a replay of the first, and the
  store never holds even a hash of a password. It is refused while the
  lifecycle is not `active` and against an unrecorded app version, and it is
  in the mutation log.
- The reply is thin by design: `outcome: succeeded` and a `detail`. There is
  no field for the app's own error payload.
- The member credential operation is `password.set`. A PIN or a card is
  another credential and gets its own id the day a target needs one.

**`effect`**, in the registry for every id: `read` (writes nothing),
`account` (the subject's account or entitlements), `credential` (a secret
the member holds), `physical` (something in the world: stock leaves a
shelf). There is no destructive effect, because there is no destructive
operation: removal disables, and `account.purge` is an admin act with a
separately issued key. Syndra's own policy does not read `effect` yet; its
retry and confirmation rules by effect are **designed, not yet served by
Syndra**. The add-on's rules below hold today.

**`effect: read`.** The add-on MUST NOT write anything, append to the
mutation log or record the reply for such an operation, and never gates it
on the lifecycle: a read that happens again is a read
(`addons/truenas/operations.go:104`). Its reply is an observation and
carries `taken_at`, `current` and `stale` like any other. Syndra serves
`storage.status` through one read path that is never dispatched as a write
and never logged; serving every `effect: read` operation that way is
**designed, not yet served by Syndra**.

**`result` (feature `results`). Designed, not yet served by Syndra.** An
add-on that declares `results` answers one `result` object, validated by the
schema the registry names for the id: `v2/schema/common/records.json` (things
the subject holds or did), `common/activity.json` (events), or
`common/status.json` (whether the subject can use the app now, and meters),
or an id's own schema, shown raw until something reads it. The fixtures are
`v2/fixtures/records.json`, `activity.json`, `status.json` and
`operation_response_result.json`. Syndra will read `result` only from an add-on that declares
`results`; until then it reads the TrueNAS add-on's typed pockets
(`activity`, `storage`, `health`), which go at the next major. A new add-on
writes `result` and never a pocket.

**`secret_result`. Designed, not yet served by Syndra.** The registry will
mark an id whose reply carries a secret (a generated PIN). The add-on
records such a reply redacted: a replay and `GET /calls` answer `recorded`
without it. No registered operation has a secret reply yet.

### `GET /calls/{call_id}` and the `physical` rules (feature `calls`)

**Designed, not yet served by Syndra.** Schema: `v2/schema/call.json`.

<!-- fixture: v2/fixtures/call.json -->
```json
{
  "call_id": "5a2d9e30-1b44-4f7c-9e08-6d3a2c1b0f95",
  "state": "recorded",
  "reply": {
    "operation": "account.adopt",
    "subject": "289471021834760193",
    "outcome": "succeeded",
    "detail": "Bound the existing account rey_mysterio.",
    "account_id": "3051"
  }
}
```

`GET /calls/{call_id}` answers what the add-on recorded for a write:
`recorded` with the first call's reply (left out when redacted), `in_doubt`
(the intent is in the mutation log with no result, and the add-on is still
finding out), or `unknown` (no effect recorded: the request never arrived or
was refused before any write, so it is safe to try again). An add-on with any
`physical` operation MUST declare `calls`. The six `physical` rules (the
add-on's part in each is a MUST):

1. **Confirmed, and the confirmation names the thing.** `confirm: true`
   always. Syndra first sends the operation with `preview: true` and no
   `call_id`; the add-on answers the would-be result (the item's name, the
   due date) with no effect and records nothing. The member confirms that,
   and the confirm carries a fresh `call_id`. A preview kept under an id the
   confirm reused would be replayed as "done" when nothing happened.
2. **At most once.** An `indeterminate` attempt is never executed again by
   anyone: Syndra does not resend, a member's retry carries a new `call_id`
   and is answered 409 `CALL_UNSETTLED` while an earlier call for that
   subject is unsettled, and the add-on MUST NOT execute a physical call
   whose mutation-log intent has no recorded result.
3. **Settled through `/calls`.** Syndra polls `GET /calls/{call_id}`; the
   member sees "we are checking whether it went through" and then the
   outcome. A call `in_doubt` for longer than `constants.in_doubt_bound`
   (`PT1H`) becomes an item for a person at the desk.
4. **Naturally idempotent.** Checking out an item this subject already holds
   changes nothing and answers with the record; one someone else holds is a
   conflict; returning an item already returned changes nothing.
5. **Serialised per subject**, so two concurrent checkouts cannot both pass
   the limit.
6. **Remembered** for at least `constants.call_memory` (`P30D`).

Also: refuse every physical operation unless the lifecycle is `active` and
the app is reachable now (never queued: the member is at the shelf now or
not), and check the subject's limit and categories before calling the app,
so that a refusal is Syndra's words and no half-record exists on the app.

Not yet in the contract's files, so not yet usable: the operation outcome
vocabulary has only `succeeded` (`v2/schema/common/outcome.json`), with no
word for "changed nothing" or "conflict" on an operation; and the codes
`UNKNOWN_RESOURCE` (404), `NOT_ENTITLED` (403) and `LIMIT_REACHED` (409), the
`params[].format: target_id` hint and the ids below are not yet in the
registry. They arrive with slice 6 of `openspec/changes/addon-contract`.

### Worked case: an inventory checkout

**Designed, not yet served by Syndra.** An inventory system where a member
borrows items through Syndra (chapter 13 has the mapping):

- Desired state says only who can borrow: `borrower` (`account`),
  `categories` (`membership`, "May borrow from"), `max_items` (`limit` in
  `items`, `"0"` unlimited). Holdings are observed, never desired: the desk
  lends and takes back without Syndra, and converging "holdings" would undo
  a return the desk recorded.
- `holdings.list` (scope `member`, `effect: read`, no parameters, result
  `common/records.json`) answers what the member holds now, read live, never
  stored by Syndra:

<!-- fixture: v2/fixtures/records.json -->
```json
{
  "records": [
    {
      "id": "item-2207",
      "label": "Soldering iron, Hakko FX-888D",
      "category": "tools",
      "since": "2026-11-03T16:10:00Z",
      "due": "2026-11-17T17:00:00Z",
      "standing": {
        "usable": true,
        "waiting_on": "none"
      },
      "x-inventory-shelf": "B4"
    }
  ],
  "taken_at": "2026-11-10T09:30:00Z",
  "current": true
}
```

- `item.checkout` and `item.return` (scope `member`, `effect: physical`,
  `confirm: true`, one parameter `item_id`, result `common/records.json`: the
  record made or closed). Under a fictional member, Kofi Kingston:
  1. He scans an item. Syndra sends `item.checkout` with `preview: true` and
     no `call_id`. The add-on checks lifecycle, reachability, his
     `categories` and `max_items`, and answers the record it would make, with
     the item's label and due date, writing nothing.
  2. He confirms. Syndra sends the same operation with a fresh `call_id`. The
     add-on takes the per-subject lock, writes the intent to the mutation
     log, calls the app, records the reply under the `call_id`, and answers.
  3. The reply is lost. Syndra marks the call indeterminate, never resends,
     and polls `GET /calls/{call_id}`: `recorded` with the record, or
     `unknown` (safe to try again), or `in_doubt` until the add-on knows. A
     second checkout meanwhile is 409 `CALL_UNSETTLED`.
  4. The item comes back at the desk. Syndra learns it on the next
     `holdings.list`; a return never depends on Syndra.
- Overdue is `standing`: `{usable: true, waiting_on: "member", note: "1
  item overdue", next: "Return it at the desk"}`.

## 8. Statistics: `GET /stats` (feature `stats`)

**Designed, not yet served by Syndra.** Syndra does not collect statistics
yet; build this when Syndra does. Schema: `v2/schema/stats.json`; the
manifest's `metrics[]` and `limits.stats_buffer` are in the chapter 3
fixture.

Syndra keeps aggregates and never copies records. A number over time ("laser
minutes per day") travels here and is kept; a list of things with ids (what
a member holds, what they did) is a live read through an operation
(chapter 7), never stored.

**Declaring metrics.** Each entry of `metrics[]` (at most 50) is a
descriptor, and Syndra stores and draws it with no code for your product:

- `name`: lowercase and dot-namespaced, its first segment your kind or
  product (`laser.job_seconds`). No `_total` suffix; `instrument` says it.
- `instrument`: `counter` (how much happened in the day) or `gauge` (the
  last sample in the day). No histograms: an average is two counters, a
  maximum is a gauge.
- `unit` from `v2/registry/units.json` (`count`, `items`, `bytes`,
  `seconds`, `grams`); a new unit is a registry entry.
- `dimensions`: at most four, each naming a **resource, never a person** (a
  machine, a share, a category; a home folder is the one value `home`, not
  one per person). Syndra drops and logs a dimension value equal to any
  subject's `account_id` or name.
- `per_subject: true` when the metric is also reported per person.
- `against` names a `limit` field of the same unit; Syndra then draws the
  gauge as a meter against that person's observed limit.
- A metric's `instrument` and `unit` are fixed for ever; a new meaning is a
  new name. Syndra will refuse a manifest that changes them.

**The collector is yours.** The add-on reads usage from the app on its own
schedule, with its own backoff, within the app's rate limits, from the app's
own log, and keeps daily points in its own buffer. Syndra never asks the app
for anything and never decides when the app is read; the add-on never calls
Syndra. Where the app pushes events to the add-on, fold them into the same
buffer; Syndra still pulls.

**The buffer.** A point stays in the buffer for `limits.stats_buffer`
(default `P90D`, at most `P400D`) whether or not Syndra has collected it,
under a size ceiling of your choosing that drops the oldest first.
Syndra's first collection of a metric asks from now minus `stats_buffer`.
When Syndra asks from further back than you still hold, answer a gap from
`since` to your oldest point with the reason "the add-on no longer holds this
window", and go on from what you have.

**A point is the whole day.** A point's `start` and `end` (RFC 3339, UTC)
bound one UTC day, `end = start + P1D`. A counter point is the complete total
for that day; a gauge point is the last sample inside it. Points are not
OpenTelemetry deltas, which add up: a point with the same
`(metric, dimensions, subject, start)` replaces the one before, so sending
partials undercounts. They are not cumulative counters either: derive each
day from the app's log, so a restart loses nothing and a day the app missed
can be filled in later. Within a window you observed, an absent counter point
is zero and an absent gauge point is "no new sample"; the add-on SHOULD NOT
send a counter's zeros.

**The watermark, per metric.** `watermark` is the instant through which this
metric is final, absence included.

- The add-on MUST NOT move a metric's watermark backwards, or past a window
  it can still fill: while the laser PC is off, `laser.job_seconds` stays at
  the last day it could read, while another metric moves on.
- A point that ends at or before the watermark is final and MUST NOT be sent
  again with another value. A point after it is provisional: send it again
  as it changes, and Syndra replaces its provisional window with each reply,
  so a provisional 3 that turns out 0 is corrected by its absence.
- A window you could not observe is a `gap` with a `reason` in a member's
  words, never a zero ("The laser PC was off" is not "nobody used it"). A gap
  is keyed by `(metric, dimensions, start)`; one ending after the watermark
  is provisional and is withdrawn when the window fills, one at or before it
  is final. When the app comes back, read its log for the missed window and
  fill it; the watermark had not passed it, so Syndra collects it.

<!-- fixture: v2/fixtures/stats.json -->
```json
{
  "contract_version": 2,
  "metric": "laser.job_seconds",
  "taken_at": "2026-11-10T07:40:00Z",
  "watermark": "2026-11-08T00:00:00Z",
  "points": [
    {
      "start": "2026-11-07T00:00:00Z",
      "end": "2026-11-08T00:00:00Z",
      "subject": "289471021834760193",
      "dimensions": {
        "machine": "speedy-400"
      },
      "value": 2520
    },
    {
      "start": "2026-11-07T00:00:00Z",
      "end": "2026-11-08T00:00:00Z",
      "dimensions": {
        "machine": "speedy-400"
      },
      "value": 3660
    }
  ],
  "unbound": {
    "accounts": 2
  },
  "gaps": [
    {
      "start": "2026-11-08T00:00:00Z",
      "end": "2026-11-10T00:00:00Z",
      "dimensions": {
        "machine": "speedy-400"
      },
      "reason": "The laser PC was off"
    }
  ],
  "next_cursor": "eyJzIjoiMjAyNi0xMS0wN1QwMDowMDowMFoiLCJ1Ijoic3ViLTdmM2EifQ"
}
```

**The reply, all MUST:**

- One metric per call: `GET /stats?metric=<name>&since=<that metric's
  watermark>[&cursor=…]`. An undeclared metric is 404 `UNKNOWN_METRIC`.
  `stats` takes no `fresh`.
- No point with `end <= since`.
- A point without `subject` is the total for the whole app (or for its
  dimensions) and includes everyone, managed or not. On a `per_subject`
  metric every day that has any per-subject point MUST also have that total,
  so that Syndra can delete per-person points at their retention and keep the
  total.
- A point with `subject` is on a `per_subject` metric and carries the
  Syndra subject id, found through the binding the add-on recorded at apply,
  never a local name.
- Usage by an account you cannot bind (a local account, a shared desk login)
  counts in the totals and in `unbound.accounts`, and never as a per-person
  point. No name of an unbound account crosses the wire.
- Pages are keyed (`next_cursor` encodes the last `(start, subject,
  dimensions)` sent; `null` on the last page), each under
  `constants.max_reply_bytes`, with `taken_at` and `watermark` from the first
  page. `taken_at` is when your collector last read the app, not when you
  answered. There is no `current`: the watermark says what is final.

**What Syndra will do with it.** Collect hourly, per metric, since that
metric's stored watermark, and write the watermark last, after the final
page; upsert points by key and replace the provisional window; drop and count
a point for a subject with no binding on your app, a `subject` on a metric
that is not `per_subject`, and anything beyond `constants.series_per_metric`
(500) dimension sets; keep totals five years and per-person points 400 days.
Admins see the totals and each person's numbers; a member sees their own.

## 9. Security rules

- **Secrets.** The add-on MUST NOT store or log a declared secret anywhere:
  not in a table, the mutation log, the mirror, a call record (secret
  parameters are outside the request identity, chapter 4), an error, a
  panic, or request-logging middleware. A secret parameter never appears in
  any later byte of any reply; the kit plants one and looks.
- **The app's credential** holds no scope that acts on the world or destroys
  data wherever the app offers a narrower one (chapter 2, step 4), and lives
  only in the add-on's container, preferably read from a file.
- **No destructive default.** Removal disables, never deletes. Deleting an
  account is an admin operation with a separately issued key
  (`account.purge`), never a side effect of convergence.
- **Read only what you need.** Name every column a read selects. The TrueNAS
  add-on names its columns because `user.query` otherwise returns password
  hashes, and a hash that is never read cannot leak
  (`addons/truenas/subjects.go:138`).
- **The transport** is chapter 4: pinned TLS 1.3, every route signed, HTTPS
  only, no redirects, a refusal that never says which half failed.
- **The mutation log** is append-only, mode `0600`, and synced to disk
  before the write it describes; a record whose sync failed never happened.
  Each record carries the digest of the one before, and `/health` serves the
  head digest and the count, so a truncation or rewrite is detected
  (`addons/truenas/mutationlog.go`).
- **What crosses the wire about people.** Every person is named by Syndra's
  subject id; no local name of an account Syndra does not manage leaves the
  add-on (chapter 8); a dimension names a resource, never a person; a gap is
  never a zero.
- **Text is quoted, never trusted.** Syndra renders your `label`, `note`,
  `next`, `reason`, `detail` and `x-` content escaped and length-capped.
  Write them for people; never put a secret, a token or the app's error text
  in them.
- **Fixtures and examples** use WWE ring names, `example.org` and RFC 5737
  addresses, never a real member.

## 10. Lifecycle and degradation

- `active` accepts work. `draining` finishes what is running and refuses new
  writes. `read_only` serves reads only. Every write refused for the
  lifecycle is 503 `LIFECYCLE_REFUSAL` with `Retry-After`, the `lifecycle`
  extension member saying which; Syndra detects it by that code alone.
- **Reads are never gated.** `/capabilities`, `/health`, `/subjects`,
  `/values`, `/plan` and every `effect: read` operation answer in every
  state, and so does `/lifecycle` itself.
- `in_flight` counts writes executing now, lifecycle changes not included;
  `drained` says whether a `draining` or `read_only` add-on has finished
  what was running.
- **"Could not look" is never "nothing".** Every read that can partly fail
  says which part is missing: `truncated` for a short read, `stale` for an
  old one, `unverified` for a write that could not be read back, an absent
  `limit` in `state` when the quota could not be read, `values_read: false`
  for a setting's options, and in the storage profile `quota_read: false`
  (chapter 11). An empty list means the app holds none.
- **When the app is down**: `/health` says `reachable: false`; reads answer
  their last value as `stale` (chapter 4); writes fail before they start and
  say so. Nothing waits in the add-on: Syndra's queue holds the change until
  the app answers.

## 11. Kinds

Every add-on declares one `kind` from `v2/registry/kinds.json`: `storage`
(a file or object store), `access` (doors), `inventory` (stock and loans),
`machine` (a tool with a job log, a laser, a print queue). Pick the word for
what the app is to a member. The kind:

- keys the credential-scope denylist the kit warns on (`access`:
  `edit:space`; `inventory` gets its entry when its product is researched);
- decides where a member meets the system: a `storage` system folds into
  Network storage; any other kind gets its own row in the member's
  navigation and its own page, `/systems/{target}`;
- selects a **profile**, routes and shapes only that kind has. Only
  `storage` has one. `access`, `inventory` and `machine` declare the kind
  and use the core routes alone until a member page needs doors, items or
  materials. Syndra's generic pages (mappings, drift, plans, people on a
  system) read only the core.

A kind not in the list is refused at registration. A new word is one
registry entry and a contract release (chapter 15); a profile and a UI
module come only when a page needs them.

### The storage profile: `GET /reach` and `GET /homes`

Features `storage.reach` and `storage.homes`, declared only by an add-on of
`kind: storage`. Schemas: `v2/schema/profiles/storage/reach.json` and
`v2/schema/profiles/storage/homes.json`; fixtures `v2/fixtures/reach.json`
and `v2/fixtures/homes.json`. Both are observations and follow the freshness
rules of chapter 4. `/reach` answers what each group opens (shares,
folders, access) and what everyone can reach; the TrueNAS add-on serves it
from a five-minute cache as `stale.reason: cached`, and 503
`REACH_NEVER_READ` before its first read. `/homes` answers every home folder
and its use against its limit, Syndra's "how much now"; `quota_read: false`
says the limits could not be read while the folders were, a partial read
and not a stale one.

## 12. Worked example: the reference add-on

`addons/reference` is a complete contract-2 add-on over an in-memory print
queue (`kind: machine`): accounts, one field of each role, a built-in
`lobby` printer as the value Syndra never manages, `password.set` and one
setting. CI runs the kit against it on every push, so it is conformant by
construction. Read it in this order; each row says what to take from it.

| Where | What it shows |
|---|---|
| `reference.go:45-58` | One field per role, and the managed set (`printers`) beside the app's own value (`lobby`) |
| `reference.go:81-91` | `standing`, observed: off, or waiting on the member for a password, or usable |
| `reference.go:146-169` | Every route behind the signature check, and every other route 404 `UNKNOWN_ROUTE` |
| `reference.go:173-182` | The manifest |
| `reference.go:184-194` | Health: lifecycle, `in_flight`, `drained`, the log's head and count |
| `reference.go:204-215` | `/subjects`: `current: true` because it reads now, `truncated: false` because it never caps |
| `reference.go:217-234` | `/values` by role: booleans, the managed set with `builtin_count`, a limit not enumerable |
| `reference.go:276-297` | `/plan`: strict, `PLAN_TOO_LARGE`, the same `decide` as the apply, nothing written |
| `reference.go:310-337` | `/apply` in the contract's order: replay before lifecycle and fingerprint, `PLAN_STALE` before any write |
| `reference.go:339-384` | `decide`: withdraw-only never creates, a name collision is a conflict, a create, `no_change` |
| `reference.go:386-429` | Convergence by role: absent is untouched, the app's own values kept, channel follows account, unlimited dominates |
| `reference.go:431-468` | Commit: conflict and blocked as problems with the outcome as extension members, the log written, `observed` and `standing` with `account_id` |
| `reference.go:470-504` | Each desired value checked against its field's role |
| `reference.go:506-530` | A fingerprint with its own value for "no account"; a name derived once, at create |
| `reference.go:545-586` | An operation: path and body agree, the secret outside the identity, the lifecycle gate |
| `reference.go:595-659` | Lifecycle and the one setting, both through the call record; `LIFECYCLE_REFUSAL` |
| `reference.go:669-719` | `once`: the four call rules and the in-flight lease, only a 2xx recorded |
| `reference.go:721-754` | Request identity: SHA-256 over canonical JSON of decoded values |
| `reference.go:758-835` | Strict decoding, the version check with both numbers, the RFC 9457 body with its retry class and `Retry-After` |
| `transport.go:48-99` | Key derivation, the pinned certificate, the MAC over the raw query |
| `transport.go:106-152` | Verification: body cap, the time window before the MAC, constant-time compare, a 401 that says nothing |
| `cmd/reference-addon/main.go:25-57` | Serving it: TLS 1.3, the secret from a file |
| `identity_test.go`, `transport_test.go` | Your tests: the four identity vectors and every transport vector |

`defects.go` is not an example. It switches on deliberate violations so the
kit's own tests can watch each check fail; never copy from it.

What the reference simplifies, and a real add-on MUST do in full:

- Call records live in memory for the life of the process
  (`reference.go:680`); keep them on disk for `constants.call_memory`.
- The mutation log is a hash chain in memory (`reference.go:661`); write it
  to a `0600` file, synced before each write.
- The target is in memory, so there is no crash between a create and its
  record, no read-back that can fail and no mirror that can be stale. A real
  add-on needs lookup before create, `unverified` and `stale` (chapter 4).
- `Retry-After` is a constant 30; send what your app's state says.
- It declares no `results`, `freshness`, `calls` or `stats`; those halves
  are slice 6 work.

## 13. Mapping sketches

Four apps, mapped onto the contract. Only TrueNAS is built. Each sketch
names what nobody has verified yet; verify each before relying on it.

### TrueNAS SCALE (`kind: storage`, shipped)

`addons/truenas`. The worked production add-on; its README has the full map.

| Field | Role | On TrueNAS |
|---|---|---|
| `group` | `membership` | SMB group membership, by name; built-in groups never listed, never removed |
| `home` | `flag` | A private home folder under the one home location (a setting) |
| `home_quota` | `limit`, `bytes`, `"0"` unlimited, `per_person` | The user quota on the home dataset; `/subjects` reports the effective limit (`server.go:388`) |
| `enabled` | `account` | `locked`, inverted |
| `smb_enabled` | `channel` | SMB on the account; TrueNAS turns it on only together with a password, so it waits for the member's next `password.set`, and `standing` says so |

- Features `operations`, `settings`, `storage.reach`, `storage.homes`
  (`capabilities.go:137`). `account_id` is the uid as a string; a new name is
  derived once from the email's local part (`username.go`).
- Plan and apply share `decide` (`apply.go:679`). `/subjects` answers the
  last read with `stale.reason: unreachable` when the NAS cannot be read,
  503 `NO_CURRENT_READ` with nothing to serve (`server.go:424`).
- Version gate on `TRUENAS_SUPPORTED_MAJORS`, recorded in
  `addons/contract/truenas_observed.json`. Credential: a dedicated user with
  `ACCOUNT_WRITE`, `READONLY_ADMIN`, `SYSTEM_AUDIT_READ`, never `FULL_ADMIN`.
  `ACCOUNT_WRITE` can delete an account and TrueNAS has nothing narrower, so
  the separate purge key is an audit separation, not a capability one.
- Read operations (`storage.status`, `activity.get`, `health.get`) are never
  recorded or gated (`operations.go:168`). Call records last 30 days
  (`store.go:40`).
- Not yet: `freshness`, `calls`, `results`, `stats` (`storage.used_bytes`,
  `storage.sessions` from the audit log, with a gap per share auditing does
  not cover).

### UniFi Access (`kind: access`)

From `openspec/changes/addon-contract/research/unifi-access-api.md` (the
official API reference PDF, read 10 Nov 2026; no lab console).

| Field | Role | On UniFi Access |
|---|---|---|
| `policy` | `membership`, "Access policy" | Access policy ids on the user (`PUT /users/:id/access_policies`); `/values` lists the policies Syndra gives, by name |
| `enabled` | `account` | `status` `ACTIVE` or `DEACTIVATED`, set by Syndra's resolver like any `account` field |

- `account_id` is the user's `id` (a UUID). At create the add-on writes the
  Syndra subject id to `employee_number`, and adopts an unbound user matching
  it only under a logged create intent (chapter 4, `POST /apply`).
- `PUT /users/:id/access_policies` replaces the user's whole set: read,
  merge the policies Syndra does not manage, then write. A console edit
  between the read and the write is lost (the API has no ETag); the next
  drift sweep finds it. This race is accepted and named.
- `standing`: no credential (no card, PIN or Touch Pass) is
  `waiting_on: member`, "Enrol a card at the front desk". `PENDING` (an
  invitation not yet accepted) is observed as `enabled: true` with
  `waiting_on: target`, so the apply never loops on a difference only the
  user can close; the add-on never sets `PENDING`.
- The credential MUST NOT hold `edit:space`, which unlocks doors. Minimum:
  `view:user`, `edit:user`, `view:policy`, `view:space`, `view:credential`,
  `edit:credential`, and `view:system_log` for usage. The registry has no
  door-opening operation, so Syndra cannot be asked to open one.
- Removal deactivates; delete is never sent. Drift is found by polling users
  (there is no webhook for user or policy changes). Usage (door openings per
  day, per door) would come from `system/logs` with `since`, under `stats`.
- There is no version endpoint: the API reference names the app version
  each endpoint needs, so the add-on gates on what it can probe and records
  each probe like any read.
- The API is unavailable once the console moves to UniFi Identity
  Enterprise.
- **Unverified:** whether `employee_number` is unique and filterable (if
  not, the add-on pages every user and matches itself); whether deactivation
  revokes entry at once and keeps policies and credentials; whether the
  duplicate-name error blocks two members who share a name; rate limits and
  the page-size ceiling; the scope name `view:policy` (inferred); the exact
  string the webhook signature covers; how to pin the console's self-signed
  certificate.
- This supersedes the research note's §7 row that made `enabled`
  withdraw-only: `account` is set by the resolver both ways.

### Trotec Ruby (`kind: machine`)

From `docs/research/2026-11-05-trotec-ruby.md` (read-only checks against the
live install, 5 Nov 2026).

| Field | Role | On Ruby |
|---|---|---|
| `active` | `account` | The user's `active` switch |
| `role` | `membership` | Roles `User`, `Admin`; never `Superadmin` or `Hmi`, which `/values` never lists |
| `group` | `membership` | Ruby groups |
| `materials` | `flag` | The `MdbAdmin` role with the `CanModifyMdb` permission |

- Ruby's user API is its internal API (`0.1-InternalApi`), unversioned and
  free to change with any update, and Ruby updates itself with no way to
  defer. So the add-on cannot do without the version gate: `/health` reads the unauthenticated
  `GET /api/Info/GetApiInformationalVersion`, and on a WebApi build with no
  recorded contract the add-on refuses every write (or reports `read_only`)
  and plans nothing until the new build is recorded.
- The credential is a dedicated Ruby admin account that signs in (personal
  access tokens reach no user route), with the EULA accepted; its password
  lives only in the add-on's secret.
- `UpdateUserData` takes the whole user: read, merge, put.
  `ReplaceUsersInGroup` replaces a group's whole member list, a set shared by
  many subjects: serialise read, merge and write per group, and keep the
  members Syndra does not manage.
- Create sets a random password nobody sees; the member sets their own
  through `password.set`, mapped to `ResetUserPassword`. Removal sets
  `active: false`; `DeleteUsers` is never sent (it hands a person's designs
  and jobs to the superadmin).
- The laser PC is sometimes off: `/health` says `reachable: false`, reads
  answer their last value as `stale`, and changes wait in Syndra's queue.
  Usage (laser time per member, per machine) would be `laser.job_seconds`
  under `stats`, collected from the job history when the PC answers.
- **Unverified:** the type and stability of a Ruby user's `id`; whether
  `ResetUserPassword` sets `temporaryPassword` and forces a second change at
  the laser; whether `CreateUser` emails the person; whether a read shows a
  password the member has not yet set (which would make it `standing`
  waiting on the member); whether the job history names the user by id;
  rate limits on the internal API.

### The inventory system (`kind: inventory`)

**Its API is unknown: nothing about it has been researched.** This sketch
tests the contract, not the product (design §10 of
`openspec/changes/addon-contract`). It uses the operations of chapter 7's
worked case, which are designed, not yet served by Syndra.

| Need | Mechanism |
|---|---|
| who can borrow | `borrower`, `account` |
| from which categories | `categories`, `membership`, values from `/values/categories` |
| how many at once | `max_items`, `limit` in `items`, `"0"` unlimited, `per_person` |
| check out, return | `item.checkout`, `item.return`: `member`, `physical`, `confirm: true` |
| what I hold | `holdings.list`: `member`, `read`, result `common/records.json` |
| overdue | `standing`, waiting on the member |
| how much lending is used | `inventory.checkouts` (counter, `count`) and `inventory.items_on_loan` (gauge, `items`, `against: max_items`), both `per_subject`, dimension `category` |

- Features `operations`, `results`, `calls`, `stats`. `account_id` is the
  product's member id, `username` its member name; bind by its
  external-reference field if it has one, else by email at an operator's
  adoption. The first `/subjects` lists today's borrowers as unmanaged.
- The credential lends and returns and holds no scope that deletes items,
  edits stock levels or writes off losses; `v2/registry/kinds.json` gets
  those scope names for `inventory` once the product is known.
- **Unverified, the four questions to answer first:** whether the product
  has a borrower profile at all; whether its loan records carry an id;
  whether an API credential can check an item out on a member's behalf; and
  whether returns are events or state. A product with no borrower profile
  does not fit this sketch: an add-on keeping `borrower`, `categories` and
  `max_items` in its own store would report its own claim as the app's
  observed state, and that is a design question for its own change.

## 14. Validating before you ask to be connected

Steps, in order; each ends on a check.

1. **Pass your own suite.** Your tests reproduce every vector in
   `transport_derivation.json` and the four `identity_*.json` vectors, and
   every reply you produce validates against its schema with format
   assertion on (in Go, `github.com/santhosh-tekuri/jsonschema/v6`, as both
   Syndra ends use). *Done when:* the suite is green.
2. **Build the kit.** From the repository root:
   `go build -C addons/conformance -o "$PWD/syndra-addon-conform" ./cmd/syndra-addon-conform`.
   It embeds the contract it was built from; there is no override. *Done
   when:* `./syndra-addon-conform -h` lists `-url`, `-target`,
   `-secret-file`, `-mutate`.
3. **Run it reads only**, against your add-on in front of a test instance of
   the app (or your own fake of it):
   `./syndra-addon-conform -url https://127.0.0.1:8443 -target <name> -secret-file ./secret`.
   `-target` is the same name your add-on derives its keys with. *Done
   when:* it exits 0, meaning no MUST failed. A `WARN` line is advisory;
   put it in your README with its reason.
4. **Run it with `-mutate`**, against a non-production instance only. It
   creates one subject named `conformance-…`, converges it through every
   write check, and leaves one disabled, labelled account, which its last
   line names; removal disables, so it cannot leave the app as it found it.
   Add `-mutate-settings` if you serve `settings` and the instance can have
   a setting written back. Add `-json report.json` for the report. *Done
   when:* it exits 0 and the last line (`left:`) names the one disabled
   account.
   (Exit 1: a MUST failed. Exit 2: the run could not start or `-mutate` was
   refused, for example because the subject exists. Exit 130: interrupted,
   after it put back what it changed.)
5. **Ask to be connected,** attaching the JSON report, your README (the
   scopes with their reasons, and every unverified item), and for each new
   operation id the registry entry it needs (chapter 15). The operator then
   adds your target to `ADDON_TARGETS` with `ADDON_<TARGET>_BASE_URL` and
   `ADDON_<TARGET>_SECRET_FILE`, gives your add-on a Compose service on its
   own network behind a profile, and mounts the same secret into both
   containers. *Done when:* the system's page in Syndra shows your manifest
   accepted.

What Syndra refuses at registration, and logs as the target's last error: a
`contract_version` that is not its own (it calls nothing on that add-on
until they agree), an unknown `kind`, a schema without exactly one `account`
field, and a field whose role changed. It drops an operation id the registry
lacks and keeps the rest. Against production, the kit runs reads only.

## 15. Evolving

**Features, not versions.** `contract_version` is a major, compared by
equality; everything after it is additive and travels as a feature in
`v2/registry/features.json`, which lists every route, request field,
manifest key, health field and reply field each one introduces.

1. A reply can always carry more, never less. Syndra decodes every reply
   tolerantly and shows what it does not know; the kit validates strictly,
   so a typo fails the kit and never Syndra.
2. A request carries a new field only under a feature: Syndra sends it only
   to an add-on that declared that feature.
3. An add-on ahead of Syndra declares a feature Syndra does not know: nothing
   is sent for it, nothing breaks.
4. An add-on behind Syndra lacks a feature Syndra would use: Syndra does
   without, and the system's page says the add-on does not do it.
5. A major changes the meaning of something that exists. It is rare, is in
   `addons/contract/CHANGELOG.md` with its migration, and Syndra speaks one
   major at a time.
6. Nothing is ever re-purposed. A deprecated feature or field is marked in
   the registry with its replacement and removed only at the next major; a
   new meaning is a new name.

**What a change needs.**

| Change | Lives in | Needs |
|---|---|---|
| a field on your app | your manifest | nothing else: its role says what to do |
| a metric | your manifest's `metrics[]` | nothing else (once `stats` is served) |
| vendor data | `x-<product>` keys | nothing; Syndra ignores them for decisions |
| an operation | `v2/registry/operations.json` and Syndra's policy | two entries; its reply is a common `result` shape or its own |
| a per-person list (holdings, bookings, jobs) | a `read` operation with `result: common/records.json` | the two entries; the renderer exists |
| an error code | `v2/registry/errors.json` | one entry, with title, statuses and retry class |
| a unit | `v2/registry/units.json` | one entry and one formatter case in Syndra's UI |
| a kind word | `v2/registry/kinds.json`, with its denylist | one entry; a profile and a UI module only when a page needs them |
| a new route or request field beside the core | `v2/registry/features.json` and the schema | a feature |
| a field role | `v2/registry/roles.json`, Syndra's resolver and UI | a core change, rare by design |
| an effect class | `v2/registry/operations.json` and Syndra's retry rule | a core change, rare by design (`physical` was the last) |

Every contract change is an entry in `addons/contract/CHANGELOG.md` naming
the feature or major, the schema files touched and the fixtures that changed,
and fails the test suites of both ends until both move. That is the point.
