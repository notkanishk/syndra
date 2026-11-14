# Writing a Syndra add-on

For an agent building an add-on for an app Syndra has never seen. Your
context is this guide, `addons/contract/` and the conformance kit
(`addons/conformance/`), with `addons/reference` and `addons/truenas` as
read-only examples: the first a complete add-on over an in-memory app, the
second the add-on in production.

The key words "MUST", "MUST NOT", "REQUIRED", "SHALL", "SHALL NOT", "SHOULD",
"SHOULD NOT", "RECOMMENDED", "NOT RECOMMENDED", "MAY", and "OPTIONAL" in this
document are to be interpreted as described in BCP 14 (RFC 2119, RFC 8174)
when, and only when, they appear in all capitals, as shown here. Every rule
in this guide carries one of them; a sentence without one is explanation or
advice.

**Where the truth is.** The contract is the files in `addons/contract/`: one
JSON Schema 2020-12 document per message (`v2/schema/`), every route in
OpenAPI 3.1 (`v2/openapi.yaml`), the closed vocabularies and constants
(`v2/registry/`), a golden document per message (`v2/fixtures/`) and the
transport vectors (`transport_derivation.json`). This guide gives the rules
those files cannot express and says where each shape lives. For a field list
or a closed vocabulary, read the schema or the registry: this guide names
them and never copies them. Every JSON block below is a fixture copied byte
for byte, its path in the comment above it, and `addons/contract/guide_test.go`
fails when one drifts. Paths that begin `v2/` or name a file directly in
`addons/contract/` are relative to that directory; every other path is from
the repository root. Code is cited by symbol (`decide` in
`addons/reference/reference.go`), never by line.

**What is live.** Syndra and the TrueNAS add-on speak contract 2 today,
`results` ([chapter 7](#read-operations-and-result-feature-results)),
`freshness` ([Fresh reads](#fresh-reads-feature-freshness)) and `calls`
([Physical operations](#physical-operations-and-get-callscall_id-feature-calls))
included. `stats` ([Statistics](#statistics-get-stats-feature-stats)) is
served by both add-ons and collected by Syndra hourly. Apps nobody has built an add-on for are in
[the appendix](#appendix-designed-not-yet-served-by-syndra); what may be
declared is in [chapter 3](#3-the-manifest-get-capabilities).

## 1. What an add-on is

**The boundary rule.** The add-on alone holds the app's address and
credential, and the add-on alone talks to the app. Syndra talks only to
add-ons, and only to ask: for state, a plan, an apply, an operation or a
read.

- The add-on MUST be the only component configured with the app's address
  and credential, and the only one that calls the app.
- The add-on MUST read the app on its own schedule, within the app's rate
  limits and availability; nothing Syndra sends dictates how or when the app
  is read.
- Syndra keeps its side with an allowlist of the HTTP clients its backend
  constructs (`backend/internal/addons/no_second_client_test.go`). Neither
  the kit nor Syndra can see your side: this rule is yours to keep.

**What an add-on is.**

- **A translator.** Syndra decides who holds what and sends it as desired
  state; the add-on converges the app to it and reports what the app then
  holds. Syndra is the single mutation authority: the add-on decides nothing
  about who gets access.
- **The least trusted component in the deployment.** It holds a third-party
  credential and talks to a machine Syndra does not control. Its manifest is
  a ceiling, never a grant: Syndra intersects every declared operation with
  its own policy, and the policy wins ([chapter 7](#7-operations)).
- **One container on its own network**, shared with the backend and nothing
  else, with no published port, behind a Compose profile, so that a
  deployment without the app never runs a container holding its credential.
- **A store, never a queue.** Work waits only in Syndra's queue, the one that
  knows why a change exists. The add-on MUST keep, on disk:
  - **a binding table**, subject → `account_id`, for the life of the
    account. `POST /apply` carries no `account_id`, so this table is the only
    link from a person to their account. Only `account.release` and
    `account.purge` remove an entry (`PutBinding` and `DeleteBinding` in
    `addons/truenas/store.go`);
  - **a call record** of each write's reply, so a replay is safe
    ([Every write](#every-write-once-per-call_id));
  - **a mutation log**, so a write is accountable ([`GET /health`](#get-health)).

  It MAY also keep a mirror of the app, so a read can answer while the app
  is down ([Observations](#observations-taken_at-current-stale)).

**The two planes.** If a thing has a steady state, it is a **field** of
desired state, converged every time from whatever is there
([chapter 3](#3-the-manifest-get-capabilities)). Only if it cannot have one
is it an **operation**, a one-shot event ([chapter 7](#7-operations)). A lock
is a field: as an operation it once left an account dark after access was
restored (`addons/README.md`).

### Words used in this guide

| Word | Meaning |
|---|---|
| subject | A person, by Syndra's id for them (`subject` on the wire). Never the app's name for them |
| account | The subject's account on the app, found by its `account_id`: the app's own stable, opaque id (a uid, a UUID) |
| binding | The add-on's record that a subject's account is a given `account_id` |
| desired state | What Syndra wants true for one subject, keyed by entitlement field. Level-triggered: sent again, it changes nothing |
| observed | What the app reports after a write, read back |
| role | What a field is, from `v2/registry/roles.json` ([chapter 3](#3-the-manifest-get-capabilities)) |
| managed set | The values of a `membership` field Syndra can remove: what `GET /values/{field}` lists |
| withdraw-only | A desired state carrying only the `account` and `channel` fields, all `false` |
| standing | Whether an existing account can be used yet, and who is waited on (`v2/schema/common/standing.json`). Observed, never desired |
| fingerprint | A digest of the subject's state on the app that a plan was computed against, checked again immediately before the write |
| `call_id` | The deduplication token every write carries in its body |
| request identity | SHA-256 over the RFC 8785 canonical JSON of a write's identity object; what makes a same-`call_id` request a replay |
| current | `current: true`: this answer comes from a read of the app that began after this request arrived |
| stale | Why a reply is not current (`v2/schema/common/stale.json`) |
| kind | What sort of system the add-on fronts, from `v2/registry/kinds.json` ([chapter 10](#10-kinds)) |
| feature | A capability beyond the core, declared in the manifest (`v2/registry/features.json`, [chapter 14](#14-evolving)) |
| mutation log | The add-on's append-only record of every write it sends the app, its head anchored by Syndra |

## 2. Before you write a line

Four steps, in order. Each ends on something you can check.

1. **Record what the app answers.** Before writing a decoder, call each read
   method you will use and each write you will send, against a throwaway
   account on an instance the operator lets you touch, and write what came
   back to `addons/contract/<product>_observed.json`. The recording MUST
   hold keys and JSON types at every level a decoder reads and no values (a
   value there is a member's name), and MUST hold each refusal as the
   request field path it named, without the app's prose (it can carry a
   submitted password). A recorded read's field selection MUST be the union of every
   selection your add-on issues against that method; a subset cannot catch
   the decode that reads the field it left out. The shipped example is
   `truenas_observed.json`; `addons/contract/README.md` says how it was made
   and what each omission cost. *Done when:* the file names the app release
   it was recorded from, and every method your decoders read and every write
   you send is in it.
2. **Gate on what you recorded.** Read the app's version at start and on
   every health check. Against a version you have not recorded, the add-on
   MUST refuse every write that changes the app (an apply, a write
   operation, a `PUT /settings` whose setting the app holds) with 422
   `TARGET_VERSION_UNSUPPORTED`, MUST report `version_tested: false` with a
   `version_note` in `/health`, and MUST go on answering reads, `/plan`
   included. `/lifecycle` changes only the add-on and is accepted in every
   state ([`POST /lifecycle`](#post-lifecycle)). An app with no version
   endpoint is gated on what you can probe, and the probe is recorded like
   any read (`MajorSupported` in `addons/truenas/nas.go`). *Done when:* a
   test starts the add-on against an unrecorded version and sees an apply
   refused and `/subjects` and `/plan` answered.
3. **Pick the stable id.** Find the app's own id for an account that
   survives a rename. It travels as `account_id`, an opaque non-empty string
   (`v2/schema/common/field.json#/$defs/AccountId`); an integer id is
   formatted as a string. A name is display, never identity. If the app has a
   field you can write at create to hold the Syndra subject id, note it: it
   is the second way to find an account after a crash
   ([The one create](#the-one-create)). *Done when:* you can name the id
   field and your evidence that a rename keeps it, or have listed that as
   unverified.
4. **Decide what the credential can do, and have it issued without the
   rest.** List the app scopes or roles your recorded reads and writes need,
   and nothing beyond them. The credential MUST NOT hold a scope that acts on
   the world (unlocking a door, starting a machine) or destroys data
   (deleting an account, editing stock, writing off a loss) wherever the app
   offers a narrower one. The operator issues it, so this, not your code, is
   the guard. `/health` reports the scopes as `credential_scopes`
   (self-reported, so advisory), and the kit warns on your kind's denylist in
   `v2/registry/kinds.json`. Where the app offers nothing narrower, say so,
   as `addons/truenas/README.md` does for TrueNAS's `ACCOUNT_WRITE`, which can
   delete. *Done when:* your README lists every scope with its reason, none
   is on your kind's denylist, and `/health` reports the same list.

## 3. The manifest: `GET /capabilities`

Schema: `v2/schema/manifest.json`. This is the TrueNAS add-on's manifest; the
reference add-on builds its own in `capabilities` in
`addons/reference/reference.go`, and `v2/fixtures/manifest.json` shows every
key, the designed features' included.

<!-- fixture: v2/fixtures/manifest_truenas.json -->
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
    "results",
    "freshness",
    "calls",
    "stats"
  ],
  "entitlement_schema": [
    {
      "name": "group",
      "role": "membership",
      "label": "Group"
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
      "available": true
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
  "metrics": [
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
      ]
    },
    {
      "name": "storage.home_bytes",
      "instrument": "gauge",
      "unit": "bytes",
      "label": "Home folder used",
      "per_subject": true,
      "against": "home_quota"
    },
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
      ]
    }
  ],
  "stats_zone": "Asia/Kolkata",
  "connection": {
    "protocol": "smb",
    "host": "nas.example.org"
  },
  "limits": {
    "subjects_per_plan": 200,
    "fresh_read_cooldown": "PT30S",
    "stats_buffer": "P90D"
  }
}
```

Rules the schema cannot express:

- `contract_version` MUST be `2`. Syndra compares it by equality at
  registration ([What Syndra refuses](#what-syndra-refuses-at-registration)).
- `product` is what people call the product ("TrueNAS"); Syndra shows it as
  the system's name until an operator names the system. `product_version` is
  the app's own version string, `""` until the add-on has read it: the
  manifest is served while the app is unreachable, and a version is never
  invented. `/health` then says `version_tested: false`.
- `kind` MUST be one word of `v2/registry/kinds.json` ([chapter 10](#10-kinds)).
- `features` MUST list only what the add-on serves in full.
- `entitlement_schema` has one entry per field of desired state, each with
  exactly one `role`, and exactly one field with role `account`. Pick the
  role by what the field does on the app; what each role means on an apply
  is [What `desired` means](#what-desired-means):

  | Role | Use it for | Wire value |
  |---|---|---|
  | `account` | The account's on/off switch, set by Syndra's resolver (on when any role reaches the app, unless an allowance denies it); never mapped | `true` / `false` |
  | `channel` | A way of using the account (SMB, remote printing) | `true` / `false` |
  | `membership` | A set of app-defined values a role confers (groups, door policies); roles combine by union | sorted, de-duplicated strings |
  | `flag` | One thing a role gives or does not (a home folder, a materials right); roles combine by "any" | `["true"]` or `[]` |
  | `limit` | A numeric ceiling in a `unit` (`v2/registry/units.json`) | decimal strings |

- A field's `name` is the add-on's and is stored by Syndra in snapshots and
  bindings; its role is fixed for ever, and a new meaning MUST be a new name.
- `per_person: true` lets an operator set the field for one person over what
  roles give.
- A limit's `unit` is the closest word in `v2/registry/units.json`; one that
  is missing is a registry entry ([chapter 14](#14-evolving)), never a word
  of your own.
- `operations[]` is [chapter 7](#7-operations); its `params` is a hint for
  the UI only.
- `metrics[]`, `stats_zone` and `limits.stats_buffer` belong to feature
  `stats` ([Statistics](#statistics-get-stats-feature-stats)).
- `connection` is how a member reaches the app: `host` MUST be the name a
  member types, not the API address. A good default is to leave it out until
  the deployment says.
- `limits.subjects_per_plan` is the most subjects one `/plan` carries
  ([`POST /plan`](#post-plan)). Every declared duration MUST be ISO 8601
  (`PT30S`, `P90D`). Syndra's own caps live in `v2/registry/constants.json`,
  and an add-on never declares them.
- A key that begins `x-`, at the top or inside a field, operation, metric or
  dimension object, is vendor data: Syndra ignores it for every decision and
  shows it quoted on the system's page. Any other key the schema does not
  list fails the kit (`unevaluatedProperties: false`), so a misspelt
  `unlimted` is a kit failure, never a quota of zero.
- The manifest MUST validate against its schema with format assertion on,
  and MUST be served while the app is unreachable.

## 4. The core routes

Every add-on MUST serve all seven core routes, and the kit does not pass
without them: `GET /capabilities` ([chapter 3](#3-the-manifest-get-capabilities))
and the six below. The rules in this first part hold on every route.

### Transport

Specified, with vectors, in `transport_derivation.json`; the reference copy
is `addons/reference/transport.go` (`deriveKeys`, `signedTarget`,
`authenticator`). A good start is to copy it and prove the copy against the
vectors.

- One per-target secret, as its trimmed UTF-8 bytes (never hex-decoded),
  gives two keys through HKDF-SHA-256 (RFC 5869) with the target name as
  salt: an Ed25519 key (info `syndra/addon-tls/v1`) and an HMAC key (info
  `syndra/addon-hmac/v1`). The target name is the deployment's name for your
  add-on: lowercase `[a-z][a-z0-9_]*`, because Syndra lowercases each name in
  `ADDON_TARGETS` before it derives anything (`splitTargets` in
  `backend/internal/addons/registry.go`). The add-on MUST derive with that
  lowercase name.
- The add-on MUST serve HTTPS only, TLS 1.3, with a certificate carrying the
  derived Ed25519 key. Syndra pins that key and trusts nothing else.
- The add-on MUST verify `X-Syndra-Addon-Signature: t=<unix>,v1=<hex>` on
  every route, `/capabilities` and `/health` included: HMAC-SHA-256 under the
  HMAC key over `<unix>.<METHOD>.<target>.<body>`, where `target` is the
  escaped path exactly as sent, plus `?` and the raw query when the query is
  not empty. That is RFC 9421's `@query` as sent, except that an empty query
  adds nothing. The add-on MUST verify the raw query exactly as received, so
  a reordered query is a different query and fails.
- The add-on MUST refuse a timestamp more than two minutes from its clock
  before checking the MAC, MUST compare MACs in constant time, and MUST act
  on the very bytes it verified.
- Every failure MUST be 401 `UNAUTHENTICATED`, the same whichever half
  failed. A good place for the reason is your log.
- Every reply MUST be the route's own answer: Syndra refuses to follow a
  redirect.
- Your own tests MUST reproduce every vector in `transport_derivation.json`
  (`derivations`, `signature`, `raw_query_signatures`).

### Every request and every reply

- **Strict requests.** The add-on MUST decode every request body strictly. A
  field the route does not know MUST be refused with 400
  `UNKNOWN_REQUEST_FIELD`; a body that is not the route's JSON with 400
  `BAD_REQUEST`. Syndra sends a request field only to an add-on that
  declared the feature introducing it (`v2/registry/features.json`), so a
  strict add-on is never surprised.
- **`contract_version` in every request body.** Absent or not `2` MUST be
  422 `CONTRACT_VERSION_MISMATCH` with `caller_version` (0 when absent) and
  `addon_version` (fixture in [chapter 6](#6-errors)). A body wrong both ways
  MAY answer either refusal: Syndra reads the two alike
  ([How Syndra reads a reply](#how-syndra-reads-a-reply)).
- **Absence is never a value.** Every reply MUST carry every field its schema
  lists in `required`. Syndra reads a missing `current`, `truncated`,
  `taken_at`, `standing.usable` or `effect` as "could not look", never as
  `false` or empty, so an add-on that forgets one has made its reply
  unusable, not quietly different.
- **Replies MAY carry more.** Outside the manifest's field, operation and
  metric objects, a reply MAY carry keys its schema does not list, and
  Syndra ignores them; a good name for yours is `x-<product>-…`.
- **Size.** A reply MUST fit in `constants.max_reply_bytes`; Syndra reads no
  further.
- **Unknown routes.** Every other method and path, a feature route your
  manifest does not declare included, MUST be 404 `UNKNOWN_ROUTE`,
  authenticated like the rest.

### Every write: once per `call_id`

`POST /apply`, a write operation (`POST /operations/{name}` for an id whose
registry `effect` is not `read`), `POST /lifecycle` and
`PUT /settings/{name}` carry `call_id` in the body, inside the signature. A
write's **request identity** is the SHA-256 of the RFC 8785 (JCS) canonical
JSON of its identity object: `v2/schema/identity/apply.json`,
`operation.json`, `lifecycle.json`, `setting.json`.

- The identity is the add-on's own: it never crosses the wire, so neither
  Syndra nor the kit can check its algorithm. What both can see, and what
  the add-on MUST get right, is the behaviour: the same JSON in any spelling
  (`"a"` and `"\u0061"`, `[ "x" ]` and `["x"]`) is the same identity. A good
  way to get it is to canonicalise decoded values (`plain` in
  `addons/reference/reference.go`).
- Reproducing the four vectors
  `v2/fixtures/identity_{apply,operation,lifecycle,setting}.json` is
  RECOMMENDED: they prove the canonicalisation.
- On an operation, Syndra omits an empty `params` and an empty `subject`
  (`callEnvelope` in `backend/internal/addons/transport.go`, `omitempty`);
  the identity MUST treat them as `{}` and `""`.

The rules, from draft-ietf-httpapi-idempotency-key-header where it has one:

1. Same `call_id`, same identity: the add-on MUST answer the recorded reply,
   byte for byte, without acting again, before the lifecycle gate and before
   the fingerprint comparison. Syndra re-plans before it re-sends, so the
   fingerprint will have moved, and an apply that already happened is not
   refused for it.
2. The add-on MUST record only a 2xx reply. A refusal is evaluated afresh
   when the same `call_id` comes again.
3. Same `call_id`, another identity or another kind of write: the add-on
   MUST answer 422 `CALL_ID_REUSED`, whether or not the first is still
   executing.
4. Same `call_id` and identity while a request carrying it executes in this
   process: the add-on MUST answer 409 `CALL_IN_PROGRESS` with `Retry-After`. It MUST hold
   this as an in-memory lease, taken before the mutation-log intent and
   released when the reply is written, and MUST NOT carry it across a
   restart or infer it from an intent with no result: a re-send after a
   crash relies on lookup before create ([The one create](#the-one-create)),
   and would otherwise be refused for ever.
5. The add-on MUST keep call records on disk for at least
   `constants.call_memory`.

**A read operation is not a write.** For an operation whose registry
`effect` is `read`, the add-on MUST ignore `call_id`: no record, no
in-flight lease, no reuse check. Syndra sends the same `call_id` for every
read (`read-<target>-<subject>`, the subject empty for `health.get`, from
`Read` in `backend/internal/addons/results.go`), so an add-on that leases reads answers
409 to the second of two page loads, and one that checks reuse answers 422
to `activity.get` after `storage.status`. The kit's `operations.reads`
check sends one `call_id` to every read you declare.

### Observations: `taken_at`, `current`, `stale`

Which replies carry `taken_at`, `current` and `truncated` is in each reply's
schema. `/values` carries none of them, because a list of legal values is
not an observation. The rules the schemas cannot express:

- `current: true` MUST mean exactly: this answer comes from a read of the
  app that began after this request arrived. A read shared by concurrent
  requests counts, if it began after this one arrived. An answer from a
  mirror, a cache, or an earlier read MUST be `current: false`, however
  recent. Syndra's drift sweep compares only against current reads; a mirror
  marked current would make every change since the mirror a false finding.
- `stale` MUST be present exactly when `current` is `false`
  (`v2/schema/common/stale.json#/$defs/Invariant`). `stale.reason` MUST be
  the true reason, each defined in the schema; Syndra treats every reason
  alike, as "not current". `retry_after` is in integer seconds, like
  `Retry-After`; `note` is a sentence for a person, in your words.
- `taken_at` (RFC 3339) MUST be an instant during the read of the app (the
  TrueNAS add-on takes its end), not when you answered.
- An add-on MUST send `Date` (RFC 9110 §6.6.1) from the same clock as
  `taken_at`, at the moment it answers; Go's `net/http` server does. Syndra
  dates a kept copy by its age on your clock, `Date` less `taken_at`
  (`age_value`, RFC 9111 §4.2.3), so a clock that runs ahead or behind does
  not make a copy read as new. A `Date` more than a second before `taken_at`
  is not from that clock and is ignored. Only a reply with no usable `Date`
  falls back to `taken_at` as sent, where one more than two minutes ahead of
  Syndra's clock is read as now (`ahead` in `backend/internal/addons/read.go`).
  Both are logged once per target: keep the add-on host's clock synchronised.
- `truncated: true` MUST be set whenever a read stopped short of everything
  (a cap, a page limit). A truncated read cannot support a conclusion about
  absence, and Syndra draws none from it.
- **A slow or failing app yields `stale`, not an error.** When the app is
  slow or down, a read that carries `current` MUST answer the last value it
  read, with `current: false`, `stale.reason: unreachable` and that read's
  `taken_at`. A good bound for your own wait on the app is well inside
  Syndra's 30-second timeout per call (`callTimeout` in
  `backend/internal/addons/deps.go`; a Syndra setting, not a contract
  constant), so the answer arrives.
- **Nothing to serve is an error, not an empty list.** With no earlier read
  to answer from, the read MUST answer an error, as `handleSubjects` in
  `addons/truenas/server.go` does: 503 `TARGET_UNREACHABLE` with
  `Retry-After` when no request reached the app (nothing was sent), 503
  `NO_CURRENT_READ` when the app was asked and the read failed. An empty
  list says the app holds none. `/values` has no `current`, so when it cannot read it
  MUST answer `TARGET_UNREADABLE`: 503 when the app could not be reached,
  429 when it is rate-limiting, 502 when it answered with a failure.
- An add-on answering from its mirror on purpose MUST say `cached`, as the
  TrueNAS add-on does. Syndra asking for a read of the app now is the
  `freshness` feature ([Fresh reads](#fresh-reads-feature-freshness)).

### Fresh reads (feature `freshness`)

Syndra sends `fresh=true` on `GET /subjects`, `GET /values/{field}`,
`GET /reach` and `GET /homes`, and `fresh: true` in the body of an
`effect: read` operation, to ask for a read of the app now (the one value
RFC 9111's request `Cache-Control: no-cache` would carry); never on
`/plan`, `/apply` or `/stats`, and only to an add-on declaring the feature
(`send` in `backend/internal/addons/pipe.go` refuses it anywhere else).
Who sends it: the drift sweep's `/subjects` read, always (it is the one
scheduled sender). Answered inside the cooldown (429, or the last value as
`cooldown`), the sweep waits once and asks again: the `retry_after`, or your
declared cooldown when it names none, never longer than your declared
cooldown and never longer than a minute (twice
`constants.cooldown_default`, so `P1D` and `PT24H` alike hold a pass back a
minute); only a second cooldown answer is a skipped pass (`sweepWait` and
`ReconcileAddon` in `backend/internal/services/drift/addon.go`). "Check
now" on a target's page runs the same pass inside the person's request and
never waits: a cooldown answer is a skipped pass at once, saying when to
ask again (`ReconcileAddonNow`). A `fresh_read_cooldown` that is not an ISO
8601 duration of days and clock time (`PT30MS`, `PT+30S`, `PT30S5M`; or
weeks, months, years, which have no fixed length) is read as the default.
The dormant
accounts list and its removal, always (every row is a candidate for
removal); and a person pressing "Read again" on a page. Every other read,
a page load or a poll, accepts your copy and shows its `taken_at`. An
add-on that declares `freshness`:

- MAY answer a read without `fresh` from its mirror, as `current: false`
  with `stale.reason: cached`;
- MUST, given `fresh: true`, read the app now, or answer the last value it
  read with `stale.reason: cooldown` and `retry_after` (whole seconds,
  rounded up, so a caller never arrives early). When the last read failed,
  the cooldown still holds, and the answer is that failure's: the last
  value as `unreachable`, or the error, never "read moments ago";
- MUST coalesce concurrent fresh reads of the same thing into one read of
  the app (request collapsing), and hold a per-target and a per-subject
  cooldown of `limits.fresh_read_cooldown` (default
  `constants.cooldown_default`). A request that joins a read already under
  way arrived after it began, so its answer is that read as `cooldown`,
  never `current`; a request that joins a read that fails is answered as
  that failure is (the last value as `unreachable`, or the error), and does
  not read the app again. Run the cooldown from when a read *begins*:
  concurrent fresh reads then collapse into one even before a join;
- MUST report `health.target_reads`, every read it has made of its app
  since it started, never decreasing while it runs (it may start again at
  0, like an OpenMetrics counter), so that the kit can prove the rules
  above;
- MUST answer 429 `FRESH_READ_COOLDOWN` (`Retry-After` the seconds left) or
  503 `TARGET_UNREACHABLE`, only when it has nothing at all to serve;
- SHOULD forget its copy, its cooldown and any read under way on every
  write it makes, so a person waiting on their own change sees it on the
  next read without asking for a fresh one: a read arriving after a write
  never joins a read begun before it.
- SHOULD drop its copy when a read is refused on its merits (a 4xx other
  than 429): the copy no longer says what the app holds.
- MUST NOT call a reply built from several reads `current` unless every one
  of them is: read now, it is as fresh as its least fresh part. Answered
  later from your copy of it, it is that copy (`cached`, `cooldown` or
  `unreachable`, as the copy's own answer is), dated no later than its
  oldest part: never the label a part had when the copy was made. A copy
  answers `cooldown` only to a fresh read the cooldown refused.

`/plan` keeps its meaning: computed against a read of the app, never cooled
down, `current: false` only when the mirror had to answer.

The TrueNAS add-on serves `/subjects`, `/reach`, `/homes`, `storage.status`
and `activity.get` through one `fresher` (`addons/truenas/fresh.go`), keyed
per route and per subject, with a five-minute copy and a 30-second
cooldown; every write clears it, reads under way included (`mutate` in
`addons/truenas/apply.go`). It remembers each key's last failure for the
cooldown, so a fresh read during an outage is the copy as `unreachable`
(or, for `/subjects` after a restart, the mirror on disk), and 429 only
when there is nothing at all. `storage.status` reads its folder
permissions through the same `/reach` key, fresh when it is asked fresh,
and is `current` only when they are.
Answered from its copy, `storage.status` keeps the copy's own reason,
dated by the folder read when that is older (`Observed.with`).
`health.get` declares its own result with no `current`, so it cannot say a
copy is one, and is read live every time, like `/values`. Syndra never
counts a cooldown against the add-on: a `FRESH_READ_COOLDOWN` neither opens
nor clears its breaker.

The kit measures reads by `target_reads` against a baseline of two
back-to-back `/health` reads, in units of what one fresh read costs (one
read of your app may be several of its calls: the TrueNAS add-on's is three
or more). Anything else reading your app during a run
(a running Syndra's 30-second health probe) moves the count, so quiet it, or
expect a false `fail`.

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

- The add-on MUST serve it while the app is unreachable. `reachable` MUST
  say whether a real call to the app just succeeded, not whether the add-on
  is up.
- `circuit_open` MUST be `true` while the add-on's own breaker is holding
  back its calls to the app (backing off to avoid a lockout or overload).
  It is distinct from `reachable: false`, so an operator does not look at
  the network for what is the add-on's choice.
- **The mutation log's head**, which Syndra anchors so that a truncated or
  rewritten log is detected rather than believed
  (`ClassifyLogHead` in `backend/internal/db/log_anchor.go`):
  - `log_head` MUST be non-empty from the first `/health`: the digest of the
    empty log until the first record (`emptyLogDigest` in
    `addons/reference/reference.go`). Syndra refuses an empty head at first
    sight. Its format is yours: Syndra only compares heads for equality.
  - Each record MUST change `log_head`, and `log_records` MUST never go
    down, across restart, rotation or compaction. Syndra flags as tampering
    fewer records than before, the same count with another head, and more
    records with the same head.
  - `log_records` MUST advance on every write the add-on sends the app, a
    write the app then refuses included, and MUST stay put on a read, a
    replay and a refusal before a write is attempted. One record per write
    sent is the least; more MAY be added, since Syndra sees only the head
    and the count. The TrueNAS add-on writes two per write, the intent and
    the result, so its count grows by two.
  - The log itself MUST be append-only, mode `0600`, and written ahead, as a
    database journals: the intent record synced to disk before the write it
    describes (and the directory synced on every open and after every
    rotation, before the next record), each record carrying the digest of
    the one before. An intent whose sync failed never happened: the add-on
    MUST NOT send that write, and answers `APPLY_FAILED` or
    `OPERATION_FAILED` 500, which Syndra reads as "may have acted" and a
    person sees, as a failing disk needs; never `TARGET_UNREACHABLE`, since
    the app was reachable. The 500 is conservative, not a claim: that write
    was not sent. The TrueNAS add-on makes every write to the app and every
    change to a binding or a setting, whatever call asked for it (an
    apply's create binds, a purge unbinds), through one function (`mutate`
    in `addons/truenas/apply.go`), each its own intent and result; a store
    write that would change nothing is not made. The result says
    `succeeded`, `failed` (the app answered no: only an answer is a
    refusal, `classifyNASError` in `addons/truenas/nas.go`), `not_sent` or
    `unknown` (it may have landed, as after a timeout or a reset). An intent
    with no result is a write in doubt: at start, before it serves, the
    add-on settles each with an `unknown` result and says it once
    (`Recover` in `addons/truenas/mutationlog.go`), so a second start adds
    nothing and the head and count only move forward. Adding fields to a
    record later, keep a record that lacks them digesting as it
    did, so the anchored chain verifies across the upgrade.
- `lifecycle`, `lifecycle_note`, `in_flight` and `drained` are
  [chapter 9](#9-lifecycle-and-degradation); `version_tested` and
  `version_note` are [chapter 2](#2-before-you-write-a-line), step 2;
  `credential_scopes` is step 4. `credential_expiry` is `set` (with
  `credential_expires_at`), `none` (issued without an expiry) or
  `unrecorded` (nobody told the add-on).
- `unaudited_shares` and `shares_readable` are the storage add-on's; a good
  default is to leave them out unless they mean something for your app.
  `target_reads` belongs to `freshness`.

### `GET /subjects`

Schema: `v2/schema/subjects.json`. The same reply answered from a mirror is
`v2/fixtures/subjects_stale.json`, which leaves `home_quota` out because the
mirror holds no limits: absent in `state` is "not known".

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

- The add-on MUST list every account the app holds, managed or not, in one
  reply. Syndra matches accounts to people itself; an account nobody
  manages is listed and left alone. The add-on's own service account, the
  one its credential belongs to, MUST be marked `self: true`; a good way is
  to ask the app who the credential is (`readSelf` in
  `addons/truenas/nas.go`).
- `id` is the app's record key (it MAY equal `account_id` when the app has
  one id), `account_id` the stable id
  ([chapter 2](#2-before-you-write-a-line)), `username` display only.
  `groups`, `home`, `enabled` and `smb_enabled` on a subject are the storage
  add-on's own; every value Syndra reads is in `state`.
- `state` MUST hold the account's current value for every field of your
  entitlement schema that you could read, by field name, in its role's wire
  shape, `[]` for none. A `limit` MUST be exactly one value, the effective
  limit the app holds (`["0"]` when unlimited), and MUST be left out when it
  could not be read.
- A `membership` field in `state`, and in an apply's `observed`, MUST hold
  only values of the managed set. A value the app holds outside it (one
  `/values` leaves out) MUST be left out, though the account keeps it
  ([Unlisted values are preserved](#what-desired-means)). Syndra compares the
  whole list with desired (`Classify` in
  `backend/internal/services/merge/classify.go`), and no mapping can name
  such a value, so it would be a difference for ever. The TrueNAS add-on
  leaves built-in groups out for this reason (`readSubjectsWhere` in
  `addons/truenas/subjects.go`); the kit's `unlisted-hidden` checks it on
  every `current: true` reply, and skips a stored copy, which may lag
  `/values`.
- `standing` MUST be on every subject, because every listed subject has an
  account. It is observed: why this account can or cannot be used yet, and
  who is waited on (`waiting_on`, from the schema).
  - An account the `account` field holds off MUST be `usable: false`,
    `waiting_on: none`: nobody is waited on, because off is what Syndra
    asked for (`standing` in `addons/reference/reference.go`).
  - A difference the app imposes and Syndra cannot change (an invitation not
    yet accepted) MUST be reported with the `account` field as Syndra set it,
    and `standing` `{usable: false, waiting_on: target}`. Drift compares
    `state` with desired, so reporting the app's "off" as `false` would
    re-queue the apply for ever.
  - `note` and `next` are shown to people as you wrote them, so they MUST be
    sentences with no internal words. When two things wait on the same
    person, the add-on SHOULD write one `note` naming both and one `next`.
- One reply, no paging: `subjects.paged` is deferred until a target needs it
  (about 300 members fit well inside 1 MiB). A capped read MUST say
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

- Every field of the entitlement schema MUST answer; any other name MUST be
  404 `UNKNOWN_FIELD`.
- The `account` and `channel` fields, whatever you named them, MUST answer
  `enumerable: true` with `["false","true"]`, a `flag` `["true"]`, and a
  `limit` `enumerable: false` with `values: []`.
- For a `membership` field, `values` MUST be the managed set: the values
  that exist now and that Syndra gives and takes away, sorted, with `labels`
  for people. Values the app owns that Syndra never manages (built-in
  groups) MUST be left out, counted in `builtin_count` and sampled in
  `builtin_examples`.
- A `membership` field MUST answer `enumerable: true`: a set Syndra cannot
  list it cannot manage, because `/values` is what defines the managed set
  that `state` shows and a mapping names. A target object too numerous to
  list is an operation parameter (`format: target_id`), never a membership.
  `enumerable: false` is for a `limit`.
- The add-on MUST read it live: values are the app's runtime state.

### `POST /plan`

Schemas: `v2/schema/plan_request.json`, `v2/schema/plan_response.json`.

<!-- fixture: v2/fixtures/plan_request.json -->
```json
{
  "contract_version": 2,
  "subjects": [
    {
      "subject": "289471021834760193",
      "email": "rey_mysterio@example.org",
      "desired": {
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
      "detail": "An account named seth_rollins already exists and is bound to another subject.",
      "consequence": "Nothing would change. If it is not theirs, release it from that subject, then adopt it here.",
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
- Above `limits.subjects_per_plan` subjects, the add-on MUST answer 422
  `PLAN_TOO_LARGE` with `subjects` and `limit`. Syndra splits its plans into
  chunks of exactly your limit, and sends one plan of every subject when you
  declare none (`Plan` in `backend/internal/addons/read.go`).
- One outcome per subject, in the shape an apply answers
  (`v2/schema/apply_response.json`), computed by the same decision as the
  apply ([chapter 5](#5-plan-and-apply-agree)). Every outcome that is not
  `blocked` MUST carry the `fingerprint` the apply will send back. `detail`
  says what changes; `consequence` says what the subject holds afterwards.
- For a subject with no account and a desired `account: true`, the outcome
  MUST be `apply` with the `username` the create would derive and a
  `fingerprint`, and no `account_id` or `standing`: there is no account yet,
  and nothing is created.
- `email` MAY be empty (the kit's over-limit probe sends `""`). With no
  binding and no email there is no name to create under: the outcome SHOULD
  be `blocked` (`no_change` for withdraw-only), as `applyOne` in
  `addons/truenas/apply.go` does. The reference derives `member-<subject>`
  instead (`username` in `addons/reference/reference.go`). Whichever you
  choose, plan and apply MUST agree.
- `current: true` MUST mean computed against a read of the app that began
  after this request arrived. Planning from a mirror MUST be
  `current: false` with `stale`, and Syndra applies neither a non-current nor
  a truncated plan.

### `POST /apply`

Schemas: `v2/schema/apply_request.json`; the 200 body
`v2/schema/apply_ok.json`. A blocked apply is the problem
`v2/fixtures/error_apply_blocked.json`; a binding conflict is the last
fixture here. The plan above and this apply are one subject under one
fingerprint: same `username`, same `account_id`, and a new fingerprint for
the state the write left.

<!-- fixture: v2/fixtures/apply_request.json -->
```json
{
  "contract_version": 2,
  "call_id": "0f8b7a1c-6d2e-4c53-9a10-2b7c9d4e5f60",
  "subject": "289471021834760193",
  "email": "rey_mysterio@example.org",
  "fingerprint": "sha256:9d1c4f0b7e2a",
  "plan_id": "3c9e2f81-44ab-4d0e-8f21-77aa10bb33cd",
  "actor": "operator-289471021834760199",
  "desired": {
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

<!-- fixture: v2/fixtures/apply_response.json -->
```json
{
  "subject": "289471021834760193",
  "effect": "applied",
  "detail": "Added fabrication.",
  "consequence": "In lab_makers, fabrication. Enabled, SMB on.",
  "username": "rey_mysterio",
  "account_id": "3051",
  "standing": {
    "usable": false,
    "waiting_on": "member",
    "note": "No password set yet",
    "next": "Set a storage password on your page"
  },
  "fingerprint": "sha256:4be07a91c3d5",
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
  "detail": "An account named seth_rollins already exists and is bound to another subject.",
  "code": "BINDING_CONFLICT",
  "retry": "never",
  "subject": "289471021834760194",
  "effect": "blocked",
  "consequence": "Nothing was changed. If it is not theirs, release it from that subject, then adopt it here.",
  "conflict": {
    "username": "seth_rollins",
    "account_id": "3077",
    "adoptable": false,
    "bound_to": "289471021834760195"
  }
}
```

The add-on MUST work in this order:

1. Decode strictly and check `contract_version`. `call_id`, `subject` or
   `desired` absent or blank is 400 `BAD_REQUEST`; `fingerprint` absent is
   400 `FINGERPRINT_REQUIRED` (here or after step 2).
2. Answer a replay ([Every write](#every-write-once-per-call_id)), identity
   `{subject, desired}`. `email`, `plan_id` and `fingerprint` are outside the
   identity, because a re-plan changes them for the same call.
3. Refuse while the lifecycle is not `active` (503 `LIFECYCLE_REFUSAL`,
   [chapter 9](#9-lifecycle-and-degradation)) or against an unrecorded app
   version (422 `TARGET_VERSION_UNSUPPORTED`,
   [chapter 2](#2-before-you-write-a-line)).
4. Find the subject's account: by its binding first.
5. Read that account on the app now, decide ([chapter 5](#5-plan-and-apply-agree)),
   and compare the fingerprint of what you read with the request's. A
   difference is 409 `PLAN_STALE` and nothing is written. The comparison
   MUST happen immediately before the write, from that read. A blocked or
   conflicting outcome writes nothing either, so which of it and
   `PLAN_STALE` answers first is yours (the two shipped add-ons differ).
6. Append the intent to the mutation log, synced to disk, before each write
   ([`GET /health`](#get-health)).
7. Write, read the account back, and answer. `observed` MUST be what the
   read-back reports, not what was sent; when the read-back fails, the reply
   MUST set `unverified: true` and leave `observed` out.

If the app cannot be reached, step 7 never starts: see
[Writes while the app is unreachable](#writes-while-the-app-is-unreachable).

#### What `desired` means

The kit checks each of these with `-mutate`.

- **Absent is unmanaged.** A field absent from `desired` MUST NOT be
  touched. A field present and empty MUST be made empty, except a `limit`:
  a `limit` present and empty (no role sets it) MUST set nothing, and the
  app's limit stays as it is and `state` reports it. Syndra always sends the
  `account` field and every `channel`, and every other mapped field unless
  the state is withdraw-only, `[]` when no role the person holds sets it
  (`Desired` in `backend/internal/services/entitlement.go`); clearing a
  limit means "no limit" on most apps (`converge` in
  `addons/reference/reference.go`).
- **Only `account: true` creates.** For a subject with no account, a desired
  `account: false`, withdraw-only included, MUST be `no_change`: nothing is
  created, and the outcome carries no `account_id` and no `standing` (in a
  plan, the fingerprint of "no account", like every outcome not blocked).
  For an existing account, withdraw-only MUST switch the account and every
  channel off and leave every other field as it was.
- **A channel follows the account.** Every `channel` field MUST be `false`
  whenever the `account` field ends `false`, whatever `desired` said.
- **Unlisted values are preserved.** On a `membership` field, a value the
  app holds outside the managed set MUST NOT be removed, and `state` never
  shows it ([`GET /subjects`](#get-subjects)). Where the app's API
  replaces a subject's whole set, read, merge, then write. Where it replaces
  a set shared by many subjects (a group's member list), the add-on MUST
  serialise read, merge and write per shared set, and preserve the members
  Syndra does not manage as well as the values, so that two applies for two
  subjects never lose each other's change.
- **A limit is combined once, here.** Desired carries one value per role
  that sets the field; the add-on MUST combine them by `max`, the
  `unlimited` value dominating: max(`"0"`, 5 GiB) is no limit when `"0"` is
  unlimited. `observed` MUST carry the effective limit, one value, or leave
  it out when it could not be read.
- **Every value is checked against its role** (a `flag` value is `"true"`,
  a `limit` value a non-negative decimal integer, a `membership` value one
  the app has), and every key against your entitlement schema. A desired
  state the app cannot hold MUST be 422 `APPLY_BLOCKED`, its outcome as
  extension members, `consequence` saying what was left as it was, and the
  plan for the same input MUST be `blocked` (one error type, `unholdable`
  in `addons/truenas/apply.go`, answers both). Whether
  a limit may carry leading zeros or a sign, and its largest value, are
  yours, decided once for plan and apply.

#### The one create

The three studied apps have no idempotent create, so the add-on carries the
whole burden.

- **Name it from the email.** A new account's username MUST be derived from
  the local part of `email`, once, at create (`DeriveUsername` in
  `addons/truenas/username.go`, which keeps `[a-z0-9_.-]`, caps at 32 and
  falls back to a digest of the subject id when nothing usable is left; the
  rules are yours); the binding is authoritative from then on.
  The kit's subject is `conformance-<UTC time>` and its email
  `conformance-<UTC time>@example.org`, and the kit owns an account only if
  its `username` begins `conformance-` (`owner` in
  `addons/conformance/owner.go`), so an add-on that names accounts any other
  way fails `plan-equals-apply` and every check after it.
- **Look up before you create.** An **unbound** account that matches by the
  app's external-reference field or by the derived name MUST be adopted
  **only** when the add-on's own mutation log holds a create intent for this
  subject with no recorded id: the crash between a create and its record.
  Any other match, or two candidates, MUST be 409 `BINDING_CONFLICT` with
  `conflict` (`username`, `account_id`, `bound_to` when another subject
  holds it, and `adoptable`, `true` exactly when `account.adopt` for this
  subject would succeed: no other subject holds the account, the add-on
  knows its own service account and this is not it, and this subject has no
  binding), and an
  operator decides through the confirmed
  `account.adopt` operation, which every add-on that creates accounts
  therefore serves ([The account operations](#the-account-operations)).
  Anyone with console access can type an external reference, and a wrong
  adoption hands one person's data to another. Tie the intent to the account
  it creates by the app's own id where the create can choose it (TrueNAS
  asks for a uid from the add-on's range and logs it on the intent), since a
  name is display, never identity. Log that id on every binding write too,
  and read every subject's records of it: an id is not unique on trust (a
  reused orphan group, a restored store). The create in doubt is this
  subject's, its latest result unknown, with no binding of that id by
  anyone, and none of this subject, logged since; another subject's create
  of the id that did not plainly fail is a second candidate, so a conflict.
  The adoption's own logged bind settles the doubt, so it adopts once. Adopt
  only an account still exactly as your create leaves it (TrueNAS: no usable
  password, SMB off), since a restore of the whole volume brings an old
  doubt back with its log. Plan MUST predict the adoption as apply makes it,
  and what can block the apply is checked before the bind. (`boundAccount` and `locate`
  in `addons/truenas/apply.go`, the adoption rule `leftByCrashedCreate` in
  `addons/truenas/apply.go` and `CreateInDoubt` in
  `addons/truenas/mutationlog.go`,
  `adoptAccount` in `addons/truenas/operations.go`; the conflict's words,
  one source for plan and apply, are `BindingConflict.words`.)
- **Record it before you answer.** The add-on MUST write the mutation-log
  intent before the create, MUST record the binding and the reply under the
  `call_id` before it answers, and MUST answer a replay from that record.
- **A rename is not a loss.** A rename on the app MUST be recognised by
  `account_id`, reported, and never taken for a missing account.
- **A bound account the app no longer holds**, by `account_id` or name, MUST
  be created again under the binding's name by the next apply that wants
  it, as `locate` in `addons/truenas/apply.go` does. Syndra's drift sweep
  shows it to an operator, who re-provisions it (that apply) or lets it go
  (`account.release`).
- **A bound name another account now holds** (the original deleted, the
  name reused) is a `BINDING_CONFLICT` with `adoptable: false`, never this
  subject's account: converging it hands the subject's access to a
  stranger. `account.adopt` refuses a subject that has a binding, so the
  operator releases first (`account.release`), then adopts that account
  only if it is theirs, or renames it in the app.

#### The reply

A 200 carries `effect: applied` or `no_change`. `account_id` and `standing`
travel together and only when an account exists (`dependentRequired` in the
schema). A blocked or conflicting outcome MUST be the problem
`APPLY_BLOCKED` (422) or `BINDING_CONFLICT` (409), the outcome's fields as
extension members (RFC 9457 §3.2), never a 200. Every other refusal and what
Syndra concludes from it is [chapter 6](#6-errors).

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

- The operator's stop switch, so every add-on MUST serve it. `state` takes a
  value from the schema ([chapter 9](#9-lifecycle-and-degradation)); any
  other MUST be 422 `UNKNOWN_LIFECYCLE_STATE`. A blank `reason` MUST be 400
  `REASON_REQUIRED`: a state nobody explained looks like a fault.
- It is a write, deduplicated by `call_id` with identity `{state, reason}`,
  and it MUST be accepted in every state.
- The reply MUST carry `in_flight` and `drained` as `/health` does.
- The state is the add-on's own, and neither shipped add-on keeps it across
  a restart (the TrueNAS add-on starts in `LIFECYCLE_STATE` from its
  environment); `/health` reports it, so an operator sees what a restart
  did.

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
  `{name, value}`, and refused while the lifecycle is not `active`. An
  unknown name MUST be 404 `NO_SUCH_SETTING`.
- A value that is not offered now (and is not `""`) MUST be refused with
  nothing stored: 400 `INVALID_BODY`, or a registered code that names the
  reason more precisely, as the storage profile's 422 `NOT_A_HOME_PLACE`
  does.
- The reply MUST be the setting as now held, with `set_by` (the `actor`) and
  `set_at`.

### Statistics: `GET /stats` (feature `stats`)

Schema: `v2/schema/stats.json`; the manifest's `metrics[]`, `stats_zone`
and `limits.stats_buffer` are in `v2/fixtures/manifest.json`. The TrueNAS
add-on and the reference serve it, and Syndra collects it (`Collect` in
`backend/internal/services/stats/collect.go`).

Syndra keeps aggregates and never copies records. A number over time ("laser
minutes per day") travels here and is kept; a list of things with ids (what
a member holds, what they did) is a live read through an operation, never
stored. The vocabulary is the OpenTelemetry metrics data model's (metric,
instrument, unit, attributes as `dimensions`), without its machinery.

**Declaring metrics.** Each entry of `metrics[]` is a descriptor, and Syndra
stores and draws it with no code for your product. The limits on names,
instruments, units and dimensions are in `v2/schema/manifest.json`. The
rules the schema cannot express:

- `name` MUST have your kind or product as its first segment
  (`laser.job_seconds`), and no `_total` suffix: `instrument` says it.
- A `counter` is how much happened in the day (an OpenTelemetry monotonic
  sum); a `gauge` is a level: the last sample in the day, or its maximum.
  There are no histograms: an average is two counters, a maximum is a gauge.
- `per_subject: true` when the metric is also reported per person. A
  per-person gauge is a level that belongs to the person (bytes stored,
  items on loan); its total is the level of the whole, by sum or by maximum
  as the quantity means.
- Each dimension MUST name a **resource, never a person** (a machine, a
  share, a category): a person is a `subject`. Syndra keeps a dimension
  value or gap reason that names a bound person as you sent it, flags it,
  and shows it to admins only; a member's own page leaves it out. That is
  Syndra's guard for a broken add-on, not a second way to report a person.
  The kit fails a dimension value equal to
  the username or `account_id` of the account it created, and warns on one
  equal to those of any other account `/subjects` lists (a service account
  may share a resource's name: a `scanner` share written by a `scanner`
  user).
- `against` names a `limit` field of the same unit, and Syndra then draws the
  gauge as a meter against that person's observed limit. The schema allows
  it only on a `per_subject` gauge with no dimensions: a meter is one series
  per person, so a level that is a share of a limit in one place only is its
  own metric (the TrueNAS add-on's `storage.home_bytes` beside
  `storage.used_bytes`).
- A metric's `instrument` and `unit` are fixed for ever; a new meaning MUST
  be a new name. Syndra will refuse a manifest that changes them.
- `stats_zone` is required with the feature and MUST be the IANA zone your
  add-on counts days in (its `TZ`; `Asia/Kolkata` in this deployment). Every
  bucket is a calendar day there. Syndra counts days in the makerspace's one
  zone and will not collect statistics counted in another.

**The collector is yours.** The add-on MUST read usage from the app on its
own schedule, with its own backoff, within the app's rate limits, from the
app's own log, and keep daily points in its own buffer. Syndra never asks
the app for anything and never decides when the app is read; the add-on
never calls Syndra. Where the app pushes events to the add-on, a good
approach is to fold them into the same buffer; Syndra still pulls. The
collector's reads of the app count in `target_reads` like any other, so a
good habit is to switch it off for a kit run (the TrueNAS add-on's
`STATS_COLLECT_EVERY=0`): the freshness checks then measure only what they
send.

**The buffer.** A point MUST stay in the buffer for `limits.stats_buffer`
(default `P90D`, at most `P400D`) after its day ends, whether or not Syndra
has collected it, under a size ceiling of your choosing that drops the
oldest days first. Syndra's first collection of a metric asks from now
minus `stats_buffer`. When Syndra asks from further back than you still
hold, the add-on MUST answer a gap from the day `since` falls in to the
first day it holds, with a reason (the TrueNAS add-on says "The add-on no
longer holds these days"), and go on from what it has.

**A point is the whole day.** A point's `start` MUST be a midnight in
`stats_zone` and its `end` the next one, both RFC 3339 with the zone's
offset (`2026-11-07T00:00:00+05:30`); a calendar day, which across a
daylight-saving change is 23 or 25 hours. A day's midnight is its first
instant: 00:00, or, on a day whose 00:00 a change skips, the instant the
clocks resume (`2026-09-06T01:00:00-03:00` in `America/Santiago`). A
counter point MUST be the complete total for that day; a gauge point the
day's level. Points are not
OpenTelemetry deltas, which add up: a point with the same `(metric,
dimensions, subject, start)` replaces the one before, so sending partials
undercounts. They are not cumulative counters either: a good way is to
derive each day from the app's log, so a restart loses nothing and a day the
app missed can be filled in later. Within a window you observed, an absent
counter point is zero and an absent gauge point is "no new sample"; the
add-on SHOULD NOT send a counter's zeros.

**The watermark, per metric.** `watermark` is the instant through which this
metric is final, absence included.

- It MUST be a midnight in `stats_zone`, no later than the start of the day
  `taken_at` is in: the day still running is never final.
- The add-on MUST move a metric's watermark only forwards, and never past a
  window it can still fill: while the laser PC is off, `laser.job_seconds`
  stays at the last day it could read, while another metric moves on.
- A point that ends at or before the watermark is final and MUST keep its
  value. A point after it is provisional: send it again as it changes, and
  Syndra replaces its provisional window with each reply, so a provisional
  3 that turns out 0 is corrected by its absence.
- A window you could not observe MUST be a `gap` with a `reason` in a
  member's words, never a zero ("The laser PC was off" is not "nobody used
  it"). A gap is keyed by `(metric, dimensions, start)`; one ending after
  the watermark is provisional and is withdrawn when the window fills, one
  at or before it is final. A good habit is to join a day's gaps into
  windows, but never across the watermark, so a final gap keeps its end.
  When the app comes back, read its log for the missed window and fill it;
  the watermark had not passed it, so Syndra collects it.

<!-- fixture: v2/fixtures/stats.json -->
```json
{
  "contract_version": 2,
  "metric": "laser.job_seconds",
  "taken_at": "2026-11-10T07:40:00Z",
  "watermark": "2026-11-08T00:00:00+05:30",
  "points": [
    {
      "start": "2026-11-07T00:00:00+05:30",
      "end": "2026-11-08T00:00:00+05:30",
      "dimensions": {
        "machine": "speedy-400"
      },
      "value": 3660
    },
    {
      "start": "2026-11-07T00:00:00+05:30",
      "end": "2026-11-08T00:00:00+05:30",
      "subject": "289471021834760193",
      "dimensions": {
        "machine": "speedy-400"
      },
      "value": 2520
    }
  ],
  "unbound": {
    "accounts": 2
  },
  "gaps": [
    {
      "start": "2026-11-08T00:00:00+05:30",
      "end": "2026-11-10T00:00:00+05:30",
      "dimensions": {
        "machine": "speedy-400"
      },
      "reason": "The laser PC was off"
    }
  ],
  "next_cursor": "eyJzIjoiMjAyNi0xMS0wN1QwMDowMDowMCswNTozMCIsInUiOiIyODk0NzEwMjE4MzQ3NjAxOTMiLCJkIjp7Im1hY2hpbmUiOiJzcGVlZHktNDAwIn19"
}
```

**The reply.**

- One metric per call: `GET /stats?metric=<name>&since=<that metric's
  watermark>[&cursor=…]`. An undeclared metric MUST be 404
  `UNKNOWN_METRIC`; a `since` that is not RFC 3339, or a cursor you did not
  give, 400 `BAD_REQUEST`. `stats` takes no `fresh`. Before your collector
  has read the app once, a good answer is 503 `NO_CURRENT_READ`.
- The reply MUST hold no point and no gap with `end <= since`.
- A point without `subject` is the total for the whole app (or for its
  dimensions) and MUST include everyone, managed or not. On a
  `per_subject` metric every day that has any per-person point MUST also
  have that total, so that Syndra can delete per-person points at their
  retention and keep the total. Every value MUST be at least 0; a counter's
  per-person points MUST sum to no more than its total, and each of a
  gauge's MUST be no more than its total (a part is never more than the
  whole).
- A point with `subject` MUST be on a `per_subject` metric and MUST carry
  the Syndra subject id, found through the binding the add-on recorded at
  apply (by the account's stable id, never its name), never a local name.
- Usage by an account you cannot bind (a local account, a shared desk
  login) MUST count in the totals and in `unbound.accounts` (distinct
  accounts across the reply's days), and never as a per-person point. **No
  name of an unbound account crosses the wire.**
- Pages MUST be keyed: points in `(start, subject, dimensions)` order, the
  total first, `next_cursor` encoding the last one sent and `null` on the
  last page, each page under `constants.max_reply_bytes`. `taken_at`,
  `watermark` and `unbound` are the same on every page; a good habit is to
  send `gaps` whole on the first page and `[]` after. `taken_at` MUST be
  when your collector last read the app, not when you answered. There is no
  `current`: the watermark says what is final.

The TrueNAS add-on's `storage.sessions`, a share whose auditing does not
record everyone a gap rather than a zero:

<!-- fixture: v2/fixtures/stats_storage.json -->
```json
{
  "contract_version": 2,
  "metric": "storage.sessions",
  "taken_at": "2026-11-10T07:40:00Z",
  "watermark": "2026-11-10T00:00:00+05:30",
  "points": [
    {
      "start": "2026-11-09T00:00:00+05:30",
      "end": "2026-11-10T00:00:00+05:30",
      "dimensions": {
        "share": "makers"
      },
      "value": 9
    },
    {
      "start": "2026-11-09T00:00:00+05:30",
      "end": "2026-11-10T00:00:00+05:30",
      "dimensions": {
        "share": "projects"
      },
      "value": 4
    },
    {
      "start": "2026-11-09T00:00:00+05:30",
      "end": "2026-11-10T00:00:00+05:30",
      "subject": "289471021834760193",
      "dimensions": {
        "share": "makers"
      },
      "value": 3
    },
    {
      "start": "2026-11-09T00:00:00+05:30",
      "end": "2026-11-10T00:00:00+05:30",
      "subject": "289471021834760193",
      "dimensions": {
        "share": "projects"
      },
      "value": 1
    }
  ],
  "unbound": {
    "accounts": 1
  },
  "gaps": [
    {
      "start": "2026-11-09T00:00:00+05:30",
      "end": "2026-11-10T00:00:00+05:30",
      "dimensions": {
        "share": "scratch"
      },
      "reason": "Auditing does not record everyone on this share"
    }
  ],
  "next_cursor": null
}
```

**What Syndra does with it.** At registration it keeps each metric's
descriptor for good (`acceptMetrics` in `backend/internal/addons/stats.go`):
a stored metric declared with another `instrument` or `unit` refuses your
manifest, and a descriptor that breaks a rule (a unit outside the registry,
more than four dimensions, an `against` that is not a same-unit limit of a
per-person gauge with no dimensions, the 51st metric) is dropped alone and
logged once. A `stats_zone` other than Syndra's own zone drops the feature:
Syndra never rebuckets a day. Then, hourly, one metric at a time: it asks
`/stats` since that metric's stored watermark (the first time, now minus
your `limits.stats_buffer`), page by page, never with `fresh`; in one
transaction it deletes what it held after the watermark, upserts every page
by key, and writes the watermark last, floored to a midnight and never past
the start of today. A page that fails rolls the whole collection back and
the next run asks again from the same watermark. It drops and counts a point
that is not one makerspace day, has no value or a negative one, names a
subject with no binding on your app, carries a `subject` on a metric that is
not `per_subject`, names an undeclared dimension, is older than retention,
or starts a series beyond `constants.series_per_metric`. Per-person points
are kept 400 days (an operator may shorten that per system), totals and gaps
five years. Admins see the totals and each person's numbers, and Insights
names who used what; a member sees their own. The
kit's `stats.*` checks hold every MUST above that a black box can see.

## 5. Plan and apply agree

- The add-on MUST compute a plan's outcome and an apply's write with one
  pure decision: from the subject's account as just read, and `desired`, to
  the effect, the consequence and the writes. The plan serialises the
  decision; the apply reads again, decides again, compares fingerprints and
  executes that decision. There is no second implementation for the plan.
- The fingerprint MUST be a digest of exactly the state the decision read,
  the same state always giving the same value, and "no account" MUST have a
  value of its own, distinct from "an account holding nothing"
  (`fingerprint` in `addons/reference/reference.go`). It is opaque: Syndra
  never parses it and sends back what the plan said.
- `consequence` MUST be built the same way in plan and apply
  (`describeHolding` in `addons/truenas/apply.go` serves both). A value the
  plan cannot know, such as the app's default for a limit nobody set, is
  named as such, never guessed.
- The kit's `plan-equals-apply` check holds you to it on every convergence
  it makes: a plan's `apply` is the apply's `applied`, with the same
  `consequence`.
- In code: `decide` in `addons/reference/reference.go` is called by its
  `plan` and its `apply`; `decide` in `addons/truenas/apply.go` by
  `planOne` in `addons/truenas/plan.go` and by `converge` in
  `addons/truenas/apply.go`.

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
  against `error.json`: `type` is `urn:syndra:addon:error:<CODE>`, `title` is
  the registry's title for the code, `status` is the HTTP status, and `code`
  is a registered code with that status among the registry's.
- `retry` MUST be the code's class from the registry. A `by_status` code
  MUST resolve per reply: `backoff` for 429 and every 5xx, `never`
  otherwise. A `retry_after` code MUST send a `Retry-After` header
  (RFC 9110), in seconds. Syndra does not read it today: it asks again on
  its next pass.
- Anything else a refusal carries MUST be an extension member (RFC 9457 §3.2):
  the outcome of a blocked apply, `caller_version` and `addon_version`,
  `subjects` and `limit`, `lifecycle`, `subject`.
- `detail` MUST be a sentence for an operator, in the add-on's own words,
  carrying nothing of the app's own error text (it can echo a submitted
  password), the request body or any secret. A bad input MUST be named by
  its field path (`desired.printers`), not by its value.
- A new code is a registry entry and a contract change
  ([chapter 14](#14-evolving)); an add-on MUST use registered codes only.

### How Syndra reads a reply

`classifyStatus` in `backend/internal/addons/transport.go` and `send` in
`backend/internal/addons/pipe.go`, which is why the codes matter:

| Reply | Syndra concludes |
|---|---|
| 2xx, `outcome: succeeded` | it acted, and said so |
| 2xx, `outcome: no_change`, from an add-on declaring `calls` | it checked, and nothing needed doing; recorded as such, never as done |
| 2xx to an operation with no `outcome`, or one outside `common/outcome.json` (or `no_change` without `calls`) | **possibly acted**, as a 5xx: absence is never a value |
| `LIFECYCLE_REFUSAL` (by code alone) | not reached: a maintenance window; it waits |
| `CONTRACT_VERSION_MISMATCH`, `UNKNOWN_REQUEST_FIELD`, or 404 `UNKNOWN_ROUTE` (or no code) on a feature route | not reached, and the manifest is read again: the add-on is not the one Syndra thought |
| `CALL_IN_PROGRESS`, `TARGET_UNREACHABLE`, any 429 | not reached; the change stays queued and is asked again later |
| 408, any other 5xx | **possibly acted**: never resent automatically, and shown to a person |
| any other 4xx | it checked the request and did nothing; the change is dropped |

So the add-on MUST answer a 4xx only when it wrote nothing, and SHOULD
answer a write with a 5xx (other than `TARGET_UNREACHABLE` and
`LIFECYCLE_REFUSAL`) only when it cannot tell whether the app acted: a 5xx
sent before any write costs an operator a check for nothing. Which 5xx is
yours; Syndra reads them alike.

### Writes while the app is unreachable

- A write (an apply, a write operation, a `PUT /settings`) that could not
  reach the app, and so sent it nothing, MUST answer 503
  `TARGET_UNREACHABLE` with `Retry-After`. Syndra keeps the change queued.
- "Sent nothing" means certain: no connection, a refused connection, a DNS
  or TLS handshake failure before any request byte, or the add-on's own
  breaker holding the call back. A write that failed after its request may
  have reached the app (a timeout, a reset) MUST answer `APPLY_FAILED` or
  `OPERATION_FAILED` with a 5xx: the app may have acted. So MUST every
  failure after an earlier write in the same request, never
  `TARGET_UNREACHABLE` or a 429, which Syndra re-queues by itself.
- A read that fails before any write sent the app no write, and MAY be
  answered `TARGET_UNREACHABLE`. The TrueNAS add-on answers `APPLY_FAILED`
  when the read failed on an open session, the conservative choice, and
  503 `TARGET_UNREACHABLE` when there was no session at all.
- A good way to know: tell "no session could be had" apart from "a call on
  a session failed" (`ErrNotSent` in `addons/truenas/nas.go`), answer
  `TARGET_UNREACHABLE` only from the first, before any write, and send every
  failure after a write through one function (`acted` in
  `addons/truenas/apply.go`).

## 7. Operations

### `POST /operations/{name}` (feature `operations`)

Schemas: `v2/schema/operation_request.json`,
`v2/schema/operation_response.json`. An adoption and its reply, as Syndra
sends and the TrueNAS add-on answers:

<!-- fixture: v2/fixtures/operation_request.json -->
```json
{
  "contract_version": 2,
  "call_id": "5a2d9e30-1b44-4f7c-9e08-6d3a2c1b0f95",
  "operation": "account.adopt",
  "subject": "289471021834760193",
  "actor": "operator-289471021834760199",
  "params": {
    "username": "rey_mysterio"
  }
}
```

<!-- fixture: v2/fixtures/operation_response.json -->
```json
{
  "operation": "account.adopt",
  "subject": "289471021834760193",
  "outcome": "succeeded",
  "detail": "Bound the existing account rey_mysterio.",
  "account_id": "3051"
}
```

**Registry first, policy second, manifest third.** An operation id is
contract vocabulary: `v2/registry/operations.json` names every id Syndra will
ever have a policy for, with its meaning, its broadest scope, whether it
needs confirming, its parameters, its secret parameters and its `effect`.
Syndra's own policy is held equal to the registry by test. A new operation
is a registry entry and a policy entry ([chapter 14](#14-evolving)), then a
line in your manifest.

The manifest entry is a ceiling:

- `scope` is the broadest you offer it at; Syndra uses the narrower of yours
  and its policy's.
- `confirm`: Syndra asks the person to confirm when either side says so.
- `secret_params`: Syndra treats as secret the union of yours and its
  policy's.
- An operation the app cannot do right now MUST be declared
  `available: false` with an `unavailable_reason`, and kept in the list.
- `params`: a JSON Schema for the UI's labels and pickers, never used for a
  decision; Syndra validates parameters against its policy alone.

Rules for the route:

- The path and the body name the operation; a disagreement MUST be 400
  `OPERATION_MISMATCH`, resolved in neither direction. An unknown name MUST
  be 404 `OPERATION_FAILED`, before any gate and with nothing recorded; a
  missing parameter 400 `OPERATION_FAILED`. A write operation with a blank
  `subject` MUST be 400 `BAD_REQUEST` (the registry's "a required field is
  missing"), before the call record and the lifecycle gate.
- **A member scope binds the subject.** For a member operation Syndra sets
  `subject` to the member acting. The add-on MUST act on the body's
  `subject` and on no subject a parameter names.
- **A write operation** (any `effect` but `read`) MUST be deduplicated by
  `call_id` with identity `{name, subject, params}` minus every secret
  parameter, so a re-send with the same `call_id` and another password is a
  replay of the first, and the store never holds even a hash of a password.
  It MUST be refused while the lifecycle is not `active`, and MUST be in
  the mutation log. It MUST be refused against an unrecorded app version
  exactly when the manifest declares it `available: false`: one function
  answers both (`unavailable` in `addons/truenas/server.go`), so an
  operation that touches only the add-on (`account.release`) stays usable.
- **A read operation** (`effect: read`) MUST write nothing, MUST leave the
  mutation log and the call record alone, MUST answer in every lifecycle
  state, and MUST ignore `call_id`
  ([A read operation is not a write](#every-write-once-per-call_id)).
  `readOperation` in `addons/truenas/operations.go` and `healthGet` in
  `addons/reference/reference.go` are the examples. Syndra sends every
  read through one path and never records one
  ([Read operations and `result`](#read-operations-and-result-feature-results)).
- The reply is thin by design: `outcome` and a `detail`. There is no field
  for the app's own error payload.
- The member credential operation is `password.set`. A PIN or a card is
  another credential and gets its own id the day a target needs one.

#### The account operations

Every add-on that creates accounts can answer `BINDING_CONFLICT`
([The one create](#the-one-create)), and so MUST declare the `operations`
feature and serve both:

- `account.adopt` binds the account named in `params.username` to the
  subject, after reading it, and answers its `account_id`
  (`handleAdoptAccount` in `backend/internal/handlers/inventory.go`). While
  the add-on cannot yet tell which account is its own service account, it
  MUST refuse with 422 `OPERATION_FAILED` (nothing was done, and the
  sentence reaches the operator, who tries again) rather than risk binding
  that account. `account.purge` refuses the same way, through the same check
  (`requireSelfKnown` in `addons/truenas/operations.go`).
- `account.release` forgets the subject's binding and touches nothing on the
  app. It is how an operator lets go of an account (`handleReleaseBinding`
  there, and `releaseOnTarget` in
  `backend/internal/handlers/merge_findings.go` for one the app no longer
  holds), so it MUST answer while the app is unreachable. It SHOULD answer
  the `account_id` it let go of when one was bound, so a release Syndra
  settles from its record (`GET /calls`) forgets Syndra's copy only while
  that copy names the same account (`ResolveBindingCall` in
  `backend/internal/db/addon_operations.go`).

`account.adopt` and `account.release` are the only way inside Syndra to
resolve a conflict (renaming the account in the app is the other); without
them a conflict sits refused until someone does that.

**`effect`**, in the registry for every id, says what an operation touches.
There is no destructive effect, because there is no destructive operation:
removal disables, and `account.purge` is an admin act with a separately
issued key. Syndra's policy reads it: a `read` goes through the one read
path ([below](#read-operations-and-result-feature-results)), and a
`physical` one is previewed, confirmed and settled as
[Physical operations](#physical-operations-and-get-callscall_id-feature-calls)
says.

### Read operations and `result` (feature `results`)

An `effect: read` operation answers what the app says now about one subject
(or, for `health.get`, about the app). Syndra sends every one through one
path, `Read` in `backend/internal/addons/results.go`: never dispatched,
never written to its operations log, never confirmed. A member reads for
themself on their own page; an admin reads for any person on that person's
page, where the person is the `subject` and the admin the `actor`. Your add-on
answers both the same way, about the body's `subject`.

An add-on that declares `results` answers each read with one `result`
object, valid against the schema `v2/registry/operations.json` names for the
id under `result`:

| Schema | What it is | Read by |
|---|---|---|
| `v2/schema/common/records.json` | things the subject holds or did: `id`, `label`, and `category`, `since`, `due`, `standing` when known | a generic list on the member's and the person's page |
| `v2/schema/common/activity.json` | what the subject did, newest first, with the `resource` each event happened on, the resources not recording them (`uncovered`), and `truncated` | the person page's activity |
| `v2/schema/common/status.json` | whether the subject can use the app now, and meters of use against a ceiling | a generic status block |
| `v2/schema/profiles/storage/status.json` | `storage.status`: the status shape, plus places and password state, for kind `storage` | the storage card |

An id with no `result` in the registry (`health.get`) declares its own
object, shown raw until a page reads it. Every `result` is an observation:
it carries `taken_at` and `current`, and `stale` exactly when `current` is
false ([Observations](#observations-taken_at-current-stale)). Syndra reads a
result missing a key its schema requires as no reading at all. A key that
begins `x-`, on the result or inside a record, event or meter, is vendor
data, shown quoted on the page and never used for a decision. The per-person
list for an inventory, as Syndra sends `holdings.list` and an add-on answers
it:

<!-- fixture: v2/fixtures/operation_response_records.json -->
```json
{
  "operation": "holdings.list",
  "subject": "289471021834760193",
  "outcome": "succeeded",
  "result": {
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
      },
      {
        "id": "item-0931",
        "label": "Digital caliper, 150 mm",
        "category": "measuring",
        "since": "2026-10-20T11:00:00Z",
        "due": "2026-11-03T17:00:00Z",
        "standing": {
          "usable": false,
          "waiting_on": "member",
          "note": "Overdue since 3 November",
          "next": "Bring it back to the front desk"
        }
      }
    ],
    "taken_at": "2026-11-10T09:30:00Z",
    "current": true
  }
}
```

Rules:

- An add-on that declares `results` MUST answer every read with `result`
  and SHOULD NOT also send a pocket. Syndra reads `result` only from an
  add-on that declares `results`, and from one that does not, only the typed
  pocket the three original reads had (`storage`, `activity`, `health`, the
  registry's `pocket`), which goes at the next major. A new add-on declares
  `results`.
- A read the registry gives no `pocket` (`holdings.list`) can be answered
  only under `result`: declared without `results`, Syndra shows it
  unavailable with that reason, and the kit's `results` check fails.
- A new per-person list (holdings, bookings, jobs in a queue) is a read
  operation whose `result` is `common/records.json`: a registry entry and a
  policy entry ([chapter 14](#14-evolving)), and the page that lists it
  exists.
- `secret_result`, in the registry for every id, marks one whose reply
  carries a secret (a generated PIN). The add-on MUST record such a reply
  redacted: a replay and `GET /calls` answer `recorded` without it. Syndra
  drops the reply of such an id unread whatever the add-on sends
  (`secretResult` in `backend/internal/addons/calls.go`). No registered
  operation has a secret reply yet.

### Physical operations and `GET /calls/{call_id}` (feature `calls`)

An operation whose registry `effect` is `physical` moves something in the
world: stock leaves a shelf. `item.checkout` is the one registered (scope
`member`, `confirm: true`, one parameter `item_id` with
`format: target_id`, result `common/records.json`); the reference add-on
serves it over a catalogue of four items (`checkout` in
`addons/reference/checkout.go`). Syndra offers one only from an add-on that
declares `calls` (and `results`, for its record); declared without them it
is shown unavailable, with the reason, and never sent
(`physicalNeeds` in `backend/internal/addons/policy.go`).

**1. Previewed, then confirmed, and the confirmation names the thing.**
Syndra first sends the operation with `preview: true` and no `call_id`
(`Preview` in `backend/internal/addons/results.go`), only to an add-on
declaring `calls`:

<!-- fixture: v2/fixtures/operation_request_preview.json -->
```json
{
  "contract_version": 2,
  "operation": "item.checkout",
  "subject": "289471021834760193",
  "actor": "289471021834760193",
  "params": {
    "item_id": "item-2207"
  },
  "preview": true
}
```

The add-on MUST run every check the confirm would (lifecycle, reachability,
the item, the subject's categories and limit) and answer what the confirm
would answer, `outcome` and the record in `result`, writing nothing: no
mutation-log record, no call record. The member confirms that, and the
confirm is a new request with a fresh `call_id`, the id of Syndra's own
record of it. A preview kept under an id the confirm reused would be
replayed as "done" when nothing happened. A `preview` with a `call_id`, or
to an add-on that has no physical operation, MUST be 400 `BAD_REQUEST`.

**2. At most once.** An attempt whose answer was lost is executed again by
no one:

- Syndra never resends one: the transport sends a write once
  (`addonAttempts` in `backend/internal/addons/transport.go`), and the
  member page's one automatic resend is refused for a physical operation
  (`ErrPhysicalNotResent` in `backend/internal/services/addonop/dispatch.go`).
- While a subject's earlier call of the operation is open (sent or about to
  be, and neither settled nor answered since), Syndra refuses the next in
  the insert of its record, one check under one advisory lock per subject
  and operation, so two confirms at once cannot both pass
  (`OpenCallError` in `backend/internal/db/addon_operations.go`); nothing
  is recorded or sent. The add-on MUST refuse it too: 409
  `CALL_UNSETTLED`, under any `call_id`, the same one included.
- The add-on MUST write the mutation-log intent before it calls the app, and
  MUST NOT execute a physical call whose intent has no recorded result.

<!-- fixture: v2/fixtures/error_call_unsettled.json -->
```json
{
  "type": "urn:syndra:addon:error:CALL_UNSETTLED",
  "title": "Call unsettled",
  "status": 409,
  "detail": "An earlier checkout for this member is still being settled, so this one was not run.",
  "code": "CALL_UNSETTLED",
  "retry": "never"
}
```

**3. Settled through `GET /calls/{call_id}`.** Syndra asks, by the call's
id, what the add-on recorded (`CallAnswer` in
`backend/internal/addons/calls.go`), for any write whose answer it lost, not
only a physical one:

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

- `recorded`, with the first call's reply (left out when redacted): it
  happened. Syndra settles its record as happened.
- `in_doubt`, no `reply`: the intent is in the log with no result, or a
  request with the id is running now, or a write acted and its reply was not
  recorded. The add-on is still finding out, and Syndra asks again.
- `unknown`, no `reply`: no effect recorded. The request never arrived, was
  refused before any write, or every write it made changed nothing. Syndra
  settles a physical call's record as not happened, and the member may try
  again; any other `unknown` it leaves to a person (below).

<!-- fixture: v2/fixtures/call_in_doubt.json -->
```json
{
  "call_id": "0b7d4c1e-6a5f-4e2b-9c3d-8f1a2e4b6c70",
  "state": "in_doubt"
}
```

An add-on MUST answer `unknown` only when it is sure nothing acted: when in
doubt, `in_doubt`. It MUST answer in every lifecycle state (asking writes
nothing) and for every kind of write, apply and lifecycle included, since
Syndra settles any record whose answer it lost this way (`handleCall` in
`addons/truenas/calls.go` reads its call store, then its lease, then its
mutation log back to `constants.call_memory`; `callState` in
`addons/reference/checkout.go` reads the target). The member sees "we are
checking whether it went through" while Syndra asks; a call still
`in_doubt` after `constants.in_doubt_bound` becomes an item for a person at
the desk (`PhysicalHold` there), who answers it as they answer any change
nobody could confirm.

What settles a record (`settleByCall` in
`backend/internal/handlers/unconfirmed.go`): `recorded` always, as
happened. `unknown` only for a physical operation sent within
`constants.call_memory`, as not happened, because rule 2 has its add-on log
every call before acting, so there no record is no effect. Any other
`unknown` settles nothing and the record stays for a person: an add-on may
keep no intent for a non-physical write (an older build, a write sent
before its log was written ahead), and past `call_memory` it may have
forgotten. An `account.adopt` or `account.release` that is `recorded` is
settled together with Syndra's own copy of the binding, written by the same
function its handler runs after a live answer, in one transaction with the
settle (`ResolveBindingCall` in `backend/internal/db/addon_operations.go`);
an adoption's recorded reply MUST therefore carry `account_id`, or the
record stays for a person. When the subject's binding was acted on again
since (a later adopt, release or purge that may have reached the add-on, or
a copy Syndra recorded after the call naming another account), the record
settles as happened and the copy is left to the later write.

**4. Naturally idempotent.** Checking out an item this subject already
holds MUST answer `outcome: no_change` with the record, and write nothing
(`log_records` unchanged); `no_change` is in `common/outcome.json` under
feature `calls`, so only an add-on declaring it may say it, and Syndra
records the call as `no_change`, never as done:

<!-- fixture: v2/fixtures/operation_response_no_change.json -->
```json
{
  "operation": "item.checkout",
  "subject": "289471021834760193",
  "outcome": "no_change",
  "detail": "You already have this item. Nothing changed.",
  "result": {
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
        }
      }
    ],
    "taken_at": "2026-11-10T09:30:00Z",
    "current": true
  }
}
```

One someone else holds is a refusal, never an outcome. The refusals a
physical operation answers, each with nothing done and each naming what it
is about in an extension member (RFC 9457 §3.2), never a value typed:

| Code | Status | Extension members | Meaning |
|---|---|---|---|
| `UNKNOWN_RESOURCE` | 404 | `param` | the parameter names nothing on the app |
| `NOT_ENTITLED` | 403 | `param` | the subject's entitlements (categories) do not reach it |
| `RESOURCE_TAKEN` | 409 | `param` | someone else holds it |
| `LIMIT_REACHED` | 409 | `limit`, `unit`, `count` | the subject is at a limit |
| `CALL_UNSETTLED` | 409 | | an earlier call for this subject is unsettled |

<!-- fixture: v2/fixtures/error_limit_reached.json -->
```json
{
  "type": "urn:syndra:addon:error:LIMIT_REACHED",
  "title": "Limit reached",
  "status": 409,
  "detail": "This member already holds as many items as they may.",
  "code": "LIMIT_REACHED",
  "retry": "never",
  "limit": 2,
  "unit": "items",
  "count": 2
}
```

**5. Serialised per subject**, so two concurrent checkouts cannot both pass
the limit, and the checks and the write in one section, so another subject
cannot take the item between them.

**6. Remembered** for at least `constants.call_memory`.

Also, a physical operation MUST be refused unless the lifecycle is `active`
and the app is reachable now (never queued: the member is at the shelf now
or not), and the subject's limit and categories MUST be checked before the
app is called, so that a refusal is in the add-on's words and no
half-record exists on the app.

`params[].format: target_id` in the registry marks a parameter that is an
id on the thing itself, typed or scanned off its label: Syndra's form gives
it a text input that a barcode scanner can type into, never a picker.

## 8. Security rules

- **Secrets.** The add-on MUST NOT store or log a declared secret anywhere:
  not in a table, the mutation log, the mirror, a call record (secret
  parameters are outside the request identity), an error, a panic, or
  request-logging middleware. A secret parameter MUST NOT appear in any
  later byte of any reply; the kit plants one and looks.
- **The app's credential** is scoped as in [chapter 2](#2-before-you-write-a-line),
  step 4, and MUST live only in the add-on's container; a good way is to
  read it from a file.
- **No destructive default.** Removal MUST disable, never delete. Deleting
  an account is an admin operation with a separately issued key
  (`account.purge`), never a side effect of convergence.
- **Read only what you need.** Every read SHOULD name the columns it
  selects. The TrueNAS add-on names its columns because `user.query`
  otherwise returns password hashes, and a hash that is never read cannot
  leak (`readSubjectsWhere` in `addons/truenas/subjects.go`).
- **The transport** is [Transport](#transport); the mutation log is
  [`GET /health`](#get-health).
- **What crosses the wire about people.** Every person MUST be named by
  Syndra's subject id; no local name of an account Syndra does not manage
  leaves the add-on.
- **Text is quoted, never trusted.** Syndra renders your `label`, `note`,
  `next`, `reason`, `detail` and `x-` content escaped and length-capped.
  They MUST be written for people and MUST carry no secret, token or app
  error text.
- **Fixtures and examples** MUST use WWE ring names, `example.org` and
  RFC 5737 addresses, never a real member.

## 9. Lifecycle and degradation

- The lifecycle states are in `v2/schema/lifecycle_request.json`. In
  `active` the add-on accepts work; in `draining` it MUST finish what is
  running and refuse new writes; in `read_only` it MUST refuse every write.
  Every write refused for the lifecycle MUST
  be 503 `LIFECYCLE_REFUSAL` with `Retry-After`, the `lifecycle` extension
  member saying which; Syndra detects it by that code alone.
- **Reads answer in every state.** `/capabilities`, `/health`, `/subjects`,
  `/values`, `/plan` and every `effect: read` operation MUST answer whatever
  the lifecycle, and so MUST `/lifecycle` itself.
- `in_flight` MUST count writes executing now, lifecycle changes not
  included; `drained` MUST say whether a `draining` or `read_only` add-on
  has finished what was running. In `active` its value is yours (TrueNAS
  `false`, the reference `in_flight == 0`): Syndra shows it and decides
  nothing on it.
- **"Could not look" is never "nothing".** Every read that can partly fail
  MUST say which part is missing: `truncated` for a short read, `stale` for
  an old one, `unverified` for a write that could not be read back, an
  absent `limit` in `state` when the limit could not be read,
  `values_read: false` for a setting's options, and in the storage profile
  `quota_read: false` (one share's usage unread is enough). An empty list
  means the app holds none.
- **When the app is down**: `/health` says `reachable: false`; reads answer
  their last value as `stale` ([Observations](#observations-taken_at-current-stale));
  writes answer `TARGET_UNREACHABLE`
  ([Writes while the app is unreachable](#writes-while-the-app-is-unreachable)),
  and Syndra's queue holds the change until the app answers.

## 10. Kinds

Every add-on MUST declare one `kind` from `v2/registry/kinds.json`. A good
choice is the word for what the app is to a member. The kind:

- keys the credential-scope denylist the kit warns on;
- decides where a member meets the system: a `storage` system folds into
  Network storage; any other kind gets its own row in the member's
  navigation and its own page, `/systems/{target}`;
- selects a **profile**, routes and shapes only that kind has. Only
  `storage` has one. Every other kind declares the kind and uses the core
  routes alone until a member page needs doors, items or materials.
  Syndra's generic pages (mappings, drift, plans, people on a system) read
  only the core.

A new word is one registry entry and a contract release
([chapter 14](#14-evolving)); a profile and a UI module come only when a
page needs them.

### The storage profile: `GET /reach` and `GET /homes`

Features `storage.reach` and `storage.homes`, declared only by an add-on of
`kind: storage`. Schemas: `v2/schema/profiles/storage/reach.json` and
`v2/schema/profiles/storage/homes.json`; fixtures `v2/fixtures/reach.json`
and `v2/fixtures/homes.json`. Both are observations and follow
[Observations](#observations-taken_at-current-stale), and the kit's
`current` check reads both. `/reach` answers what each group opens (shares,
folders, access) and what everyone can reach; the TrueNAS add-on serves it
from a five-minute cache as `stale.reason: cached`, and 503
`REACH_NEVER_READ` before its first read. `/homes` answers every home folder
and its use against its limit, Syndra's "how much now"; `quota_read: false`
says the limits could not be read while the folders were, a partial read and
not a stale one.

## 11. Worked example: the reference add-on

`addons/reference` is a complete contract-2 add-on over an in-memory print
queue (`kind: machine`): accounts, one field of each role, a built-in
`lobby` printer as the value Syndra never manages, `health.get` as its one
read operation (two with `-features results`, which adds `holdings.list`),
`password.set` and one setting. CI runs the kit against it
on every push. Read `addons/reference/reference.go` in this order; each row
says what to take from it.

| Symbol | What it shows |
|---|---|
| `fields`, `printers`, `shown` | One field per role, the managed set beside the app's own value (`lobby`), and `state` showing the managed set only |
| `standing` | Observed standing: off and waiting on nobody, waiting on the member for a password, or usable |
| `New`, `emptyLogDigest` | A log head that is never empty, from the first `/health` |
| `Handler` | Every route behind the signature check, and every other route 404 `UNKNOWN_ROUTE` |
| `capabilities`, `health` | The manifest; health with lifecycle, `in_flight`, `drained`, the log's head and count |
| `subjects` | `current: true` because it reads now, `truncated: false` because it never caps |
| `values` | By role: booleans, the managed set with `builtin_count`, a limit not enumerable |
| `plan` | Strict, `PLAN_TOO_LARGE`, the same `decide` as the apply, nothing written |
| `apply` | The contract's order: replay before lifecycle and fingerprint, `PLAN_STALE` before any write |
| `decide` | Withdraw-only never creates, a name collision is a conflict, a create, `no_change` |
| `converge`, `combine` | Convergence by role: absent untouched, an empty limit untouched, the app's own values kept, channel follows account, unlimited dominates |
| `commit` | Conflict and blocked as problems with the outcome as extension members, the log written, `observed` and `standing` with `account_id` |
| `parseDesired` | Each desired value checked against its field's role |
| `fingerprint`, `username` | A fingerprint with its own value for "no account"; a name derived once, at create |
| `operation`, `healthGet` | Path and body agree; a write with the secret outside the identity and the lifecycle gate; a read that ignores `call_id` |
| `holdingsList`, `serves` | A read answering `common/records.json` under `result`, declared only with feature `results` |
| `checkout`, `decideLoan`, `reachableAndActive` (`checkout.go`) | A physical operation: the preview that writes nothing, every check before the write in one section, the intent before the loan, `CALL_UNSETTLED` while one is open, `no_change` for an item already held |
| `callState`, `settle` (`checkout.go`) | `GET /calls`: recorded, in doubt or unknown, and an open intent settled by reading the target, never while it cannot be read |
| `setLifecycle`, `putSetting` | Lifecycle and the one setting, both through the call record; `LIFECYCLE_REFUSAL` |
| `once` | The call rules and the in-flight lease, only a 2xx recorded |
| `identityDigest`, `plain` | Request identity: SHA-256 over canonical JSON of decoded values |
| `parse`, `problem` | Strict decoding, the version check with both numbers, the RFC 9457 body with its retry class and `Retry-After` |

And in its other files: `deriveKeys` and `signedTarget` in
`addons/reference/transport.go` (key derivation, the pinned certificate, the
MAC over the raw query); `authenticator` there (body cap, the time window
before the MAC, constant-time compare, a 401 that says nothing); `main` in
`addons/reference/cmd/reference-addon/main.go` (TLS 1.3, the secret from a
file); `addons/reference/identity_test.go` and
`addons/reference/transport_test.go` (the four identity vectors and every
transport vector).

`addons/reference/defects.go` is not an example: it switches on deliberate
violations so the kit's own tests can watch each check fail, and nothing in
it is to be copied.

What the reference simplifies, and a real add-on MUST do in full:

- Bindings live in memory (`Addon.bindings`); keep them on disk for the
  life of the account ([chapter 1](#1-what-an-add-on-is)).
- Call records live in memory for the life of the process (`once`); keep
  them on disk for `constants.call_memory`.
- The mutation log is a hash chain in memory (`record`); write it to a
  `0600` file, synced before each write.
- The target is in memory, so there is no crash between a create and its
  record, no read-back that can fail, no mirror that can be stale and no
  outage. A real add-on needs lookup before create, `unverified`, `stale`
  and `TARGET_UNREACHABLE`.
- `Retry-After` is a constant 30; a real add-on sends what its app's state
  says.
- It declares neither `account.adopt` nor `account.release`, though it can
  answer `BINDING_CONFLICT`; a real add-on MUST
  ([The account operations](#the-account-operations)).
- It declares `results`, `freshness` and `calls` only when asked
  (`Config.Features`, `-features results,freshness,calls`), so the kit's
  checks for each are watched both ways. With `freshness`, `/subjects` is
  served from a mirror cleared by every write, the cooldown is `PT1S`, and
  `Config.Down` (test-only) holds the unreachable rule the kit cannot. With
  `calls` and `results` it declares `item.checkout` over four items, one at
  a time per member; `Config.LoseReply` (test-only) loses an answer before or
  after the loan, which the kit cannot. Its intents live in memory; a real
  add-on writes them to its mutation log.
- With `stats` (`-features stats`) it reports `machine.jobs` (a counter per
  printer) and `machine.longest_job_seconds` (a gauge against `job_seconds`)
  from a deterministic job log, plus a `guest` login no binding names
  (`statsDay` in `addons/reference/stats.go`). It reads its in-memory app
  on every `/stats`, before answering, where a real add-on reads its app on
  its own timer; its buffer lives in memory, and `Config.Down` (test-only)
  holds the watermark back, which the kit cannot.

## 12. The TrueNAS add-on

`addons/truenas` (`kind: storage`) is the add-on in production; its README
has the full map.

| Field | Role | On TrueNAS |
|---|---|---|
| `group` | `membership` | SMB group membership, by name; built-in groups never listed, never removed |
| `home` | `flag` | A private home folder under the one home location (a setting) |
| `home_quota` | `limit`, `bytes`, `"0"` unlimited, `per_person` | The user quota on the home dataset; `/subjects` reports the effective limit (`subjectViews` in `addons/truenas/server.go`) |
| `enabled` | `account` | `locked`, inverted |
| `smb_enabled` | `channel` | SMB on the account; TrueNAS turns it on only together with a password, so it waits for the member's next `password.set`, and `standing` says so |

- Features `operations`, `settings`, `storage.reach`, `storage.homes`,
  `results`, `freshness`, `calls`, `stats` (`features` and `operationSet` in
  `addons/truenas/capabilities.go`). It has no physical operation: `calls`
  is declared for `GET /calls`, which answers every write from its call
  store, its lease and its mutation log (`handleCall` in
  `addons/truenas/calls.go`), and a `preview` is refused. `account_id` is the
  uid as a string; a new name is derived once from the email's local part
  (`DeriveUsername` in `addons/truenas/username.go`).
- Plan and apply share `decide` (`addons/truenas/apply.go`). `/subjects`
  answers its copy (`cached`, `cooldown`) or a read of the NAS now, the last
  read with `stale.reason: unreachable` when the NAS cannot be read, and with
  nothing to serve 503 `TARGET_UNREACHABLE` (nothing was sent) or
  `NO_CURRENT_READ` ([Fresh reads](#fresh-reads-feature-freshness)).
- Version gate on `TRUENAS_SUPPORTED_MAJORS`, recorded in
  `addons/contract/truenas_observed.json`. Credential: a dedicated user with
  `ACCOUNT_WRITE`, `READONLY_ADMIN`, `SYSTEM_AUDIT_READ`, never `FULL_ADMIN`.
  `ACCOUNT_WRITE` can delete an account and TrueNAS has nothing narrower, so
  the separate purge key is an audit separation, not a capability one.
- Read operations (`storage.status`, `activity.get`, `health.get`) are never
  recorded, held or gated (`readOperation` in `addons/truenas/operations.go`),
  and answer under `result` (feature `results`): `storage.status` the storage
  profile, `activity.get` the share as each event's `resource`, only when
  it is a known enabled share (`auditRow.share`, the collector's rule; on a
  `[homes]`-style share Samba's service is the username, so none).
  Call records last 30 days (`idempotencyTTL` in `addons/truenas/store.go`).
- A NAS that cannot be reached answers a write `TARGET_UNREACHABLE`
  (`ErrNotSent` in `addons/truenas/nas.go`).
- Statistics (`statsMetrics` in `addons/truenas/stats.go`): `storage.sessions`
  (distinct SMB sessions per account, share and day, from one bounded
  `audit.query` per run), `storage.home_bytes` (home folder use, against
  `home_quota`) and `storage.used_bytes` (use per share), each owned by the
  account's uid, never its name. A share whose auditing does not record
  everyone is a gap, never a zero. The collector runs every
  `STATS_COLLECT_EVERY` (`PT1H`; `0` switches it off), reads only, and keeps
  its days in the add-on's bbolt store, in the container's `TZ`
  (`collectStats` in `addons/truenas/stats_collect.go`).

Sketches for other apps (UniFi Access, a Trotec Ruby laser, an inventory
system) are unverified and in [Appendix A.2](#a2-mapping-sketches).

## 13. Validating before you ask to be connected

Steps, in order; each ends on a check.

1. **Pass your own suite.** Your tests MUST reproduce every vector in
   `transport_derivation.json`, and every reply you produce MUST validate
   against its schema with format assertion on (in Go,
   `github.com/santhosh-tekuri/jsonschema/v6`, as both Syndra ends use).
   The contract is files, not a Go module to import: read them from disk,
   adding each schema under its `$id` so the relative `$ref`s resolve, as
   `v2Schema` in `addons/truenas/contract2_test.go` does. Reproducing the
   four `identity_*.json` vectors is RECOMMENDED
   ([Every write](#every-write-once-per-call_id)). *Done when:* the suite is
   green.
2. **Build the kit.** From the repository root:
   `go build -C addons/conformance -o "$PWD/syndra-addon-conform" ./cmd/syndra-addon-conform`.
   It embeds the contract it was built from; there is no override. It is a
   binary to run, not a package to import. *Done when:*
   `./syndra-addon-conform -h` lists `-url`, `-target`, `-secret-file`,
   `-mutate`.
3. **Run it reads only**, against your add-on in front of a test instance of
   the app (or your own fake of it):
   `./syndra-addon-conform -url https://192.0.2.10:8443 -target <name> -secret-file ./secret`.
   `-target` is the same lowercase name your add-on derives its keys with.
   *Done when:* it exits 0, meaning no MUST failed. A `WARN` line is
   advisory; a good place for it is your README, with its reason.
4. **Run it with `-mutate`**, against a non-production instance only. It
   creates one subject named `conformance-<UTC time>`, with email
   `conformance-<UTC time>@example.org`, converges it through every write
   check, and leaves one disabled account, which its last line names;
   removal disables, so it cannot leave the app as it found it. The kit
   writes only to an account whose `username` begins `conformance-`
   ([The one create](#the-one-create)). Add `-mutate-settings` if you serve
   `settings` and the instance can have a setting written back. Add
   `-json report.json` for the report. *Done when:* it exits 0 and the last
   line (`left:`) names the one disabled account.
   (Exit 1: a MUST failed. Exit 2: the run could not start or `-mutate` was
   refused, for example because the subject exists. Exit 130: interrupted,
   after it put back what it changed.)
5. **Ask to be connected,** attaching the JSON report, your README (the
   scopes with their reasons, and every unverified item), and for each new
   operation id the registry entry it needs ([chapter 14](#14-evolving)).
   The operator then adds your target to `ADDON_TARGETS` with
   `ADDON_<TARGET>_BASE_URL` and `ADDON_<TARGET>_SECRET_FILE`, gives your
   add-on a Compose service on its own network behind a profile, and mounts
   the same secret into both containers. *Done when:* the system's page in
   Syndra shows your manifest accepted.

Against production, the kit runs reads only.

### What the kit cannot check

A green kit is not every MUST. These cannot be seen from outside, so your
own tests carry them:

- that the add-on alone holds the app's credential, and that it holds no
  more scopes than it reports ([chapter 2](#2-before-you-write-a-line), step
  4; `dangerous-scopes` reads the self-reported list);
- the recording, the version gate against it, and your README (chapter 2);
- the mutation log on disk: mode `0600`, append-only, each write's intent
  on disk before the app sees the write (a fake app that reads the log as
  the write arrives), no write sent when that sync fails, its head and count
  surviving a restart, rotation and a change of record format (the kit sees
  them move within one run);
- the binding table for the life of the account, and call records for
  `constants.call_memory`, both surviving a restart (`replay.29d` skips);
- concurrency: `CALL_IN_PROGRESS`, and serialised writes to a shared set;
- an outage: on reads the last value as `stale.reason: unreachable`, or
  `TARGET_UNREACHABLE` or `NO_CURRENT_READ` with nothing to serve; on writes
  `TARGET_UNREACHABLE` against `APPLY_FAILED`, and `unverified` on a failed read-back
  (the kit cannot take your app down);
- lookup before create after a crash between the create and its record;
- that a value outside the managed set survives an apply (`state` never
  shows it, so only the add-on's own test sees it:
  `TestAnUnlistedValueIsKeptAndNotShown` in
  `addons/reference/unlisted_test.go`);
- that no secret is stored or logged (the kit reads only replies);
- statistics: the watermark held back while the app is down, and the days
  filled once it is back (the reference's `TestTheWatermarkIsHeldWhileTheAppIsDown`
  in `addons/reference/stats_test.go`); the buffer's size ceiling; usage by
  an account outside any binding (the kit can only make bound ones), and
  per-person points owned by the account's stable id, not its name.

### What Syndra refuses at registration

Syndra checks each manifest it reads (`validate` in
`backend/internal/addons/manifest.go`, `acceptRoles` and `Refresh` in
`backend/internal/addons/registry.go`). It refuses the manifest, logs the
reason as the target's last error, and keeps using the last manifest it
accepted, on:

- a `contract_version` that is not its own (both numbers are in the error);
- a `kind` outside the registry;
- a field with no name, or a field declared twice;
- a role outside `v2/registry/roles.json`;
- not exactly one field with role `account`;
- a field whose role changed since an earlier manifest;
- a different field declared as the `account` field.

It drops, logs and otherwise accepts an operation id the registry lacks. The
kit is stricter on two counts: it fails an operation id missing from the
registry, and an `operations` list without the `operations` feature.

A mapping on a `membership` field that answers `enumerable: false` is refused
when it is made, not at registration: the manifest does not say it, `/values`
does.

## 14. Evolving

**Features, not versions.** `contract_version` is a major, compared by
equality; everything after it is additive and travels as a feature in
`v2/registry/features.json`, which lists every route, request field,
manifest key, health field and reply field each one introduces.

1. A reply can always carry more, never less. Syndra decodes every reply
   tolerantly and shows what it does not know; the kit validates strictly,
   so a typo fails the kit and never Syndra.
2. A request carries a new field only under a feature: Syndra sends it only
   to an add-on that declared that feature.
3. An add-on ahead of Syndra declares a feature Syndra does not know:
   nothing is sent for it, nothing breaks.
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
| vendor data | `x-<product>` keys | nothing; Syndra ignores them for decisions |
| an operation | `v2/registry/operations.json` and Syndra's policy | two entries |
| an error code | `v2/registry/errors.json` | one entry, with title, statuses and retry class |
| a unit | `v2/registry/units.json` | one entry and one formatter case in Syndra's UI |
| a kind word | `v2/registry/kinds.json`, with its denylist | one entry; a profile and a UI module only when a page needs them |
| a new route or request field beside the core | `v2/registry/features.json` and the schema | a feature |
| a field role | `v2/registry/roles.json`, Syndra's resolver and UI | a core change, rare by design |
| an effect class | `v2/registry/operations.json` and Syndra's retry rule | a core change, rare by design (`physical` was the last) |
| a per-person list | a read operation with `result` `common/records.json` | a registry entry and a policy entry ([Read operations and `result`](#read-operations-and-result-feature-results)) |
| a metric | your manifest's `metrics[]` ([Statistics](#statistics-get-stats-feature-stats)) | nothing else: its descriptor says how Syndra keeps and draws it |

Every contract change is an entry in `addons/contract/CHANGELOG.md` naming
the feature or major, the schema files touched and the fixtures that
changed, and fails the test suites of both ends until both move. That is the
point.

## Appendix: Designed, not yet served by Syndra

Apps nobody has built an add-on for yet: the inventory worked case, whose
operations Syndra does serve, and the mapping sketches. A feature is
declared only once it is served in full
([chapter 3](#3-the-manifest-get-capabilities)).

### A.1 Worked case: an inventory checkout

An inventory system where a member borrows items through Syndra (the
mapping is in [A.2](#a2-mapping-sketches)):

- Desired state says only who can borrow: `borrower` (`account`),
  `categories` (`membership`, "May borrow from"), `max_items` (`limit` in
  `items`, `"0"` unlimited). Holdings are observed, never desired: the desk
  lends and takes back without Syndra, and converging "holdings" would undo
  a return the desk recorded.
- `holdings.list` (scope `member`, `effect: read`, no parameters, result
  `common/records.json`) answers what the member holds now, read live,
  never stored by Syndra:

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
  `confirm: true`, one parameter `item_id`, result `common/records.json`:
  the record made or closed). Under a fictional member, Kofi Kingston:
  1. He scans an item. Syndra sends `item.checkout` with `preview: true` and
     no `call_id`. The add-on checks lifecycle, reachability, his
     `categories` and `max_items`, and answers the record it would make,
     with the item's label and due date, writing nothing.
  2. He confirms. Syndra sends the same operation with a fresh `call_id`.
     The add-on takes the per-subject lock, writes the intent to the
     mutation log, calls the app, records the reply under the `call_id`,
     and answers.
  3. The reply is lost. Syndra marks the call indeterminate, never resends,
     and polls `GET /calls/{call_id}`: `recorded` with the record, or
     `unknown` (safe to try again), or `in_doubt` until the add-on knows. A
     second checkout meanwhile is 409 `CALL_UNSETTLED`.
  4. The item comes back at the desk. Syndra learns it on the next
     `holdings.list`; a return never depends on Syndra.
- Overdue is `standing`: `{usable: true, waiting_on: "member", note: "1
  item overdue", next: "Return it at the desk"}`.

### A.2 Mapping sketches

Three apps mapped onto the contract, none built. Each names what nobody has
verified; a good habit is to verify each before relying on it.

#### UniFi Access (`kind: access`)

From the official API reference (read 10 Nov 2026; no lab console).

| Field | Role | On UniFi Access |
|---|---|---|
| `policy` | `membership`, "Access policy" | Access policy ids on the user (`PUT /users/:id/access_policies`); `/values` lists the policies Syndra gives, by name |
| `enabled` | `account` | `status` `ACTIVE` or `DEACTIVATED`, set by Syndra's resolver like any `account` field |

- `account_id` is the user's `id` (a UUID). At create the add-on writes the
  Syndra subject id to `employee_number`, and adopts an unbound user
  matching it only under a logged create intent
  ([The one create](#the-one-create)).
- `PUT /users/:id/access_policies` replaces the user's whole set: read,
  merge the policies Syndra does not manage, then write. A console edit
  between the read and the write is lost (the API has no ETag); the next
  drift sweep finds it. This race is accepted and named.
- `standing`: no credential (no card, PIN or Touch Pass) is
  `waiting_on: member`, "Enrol a card at the front desk". `PENDING` (an
  invitation not yet accepted) is reported as `enabled: true` with
  `standing` `{usable: false, waiting_on: target}`, so the apply never loops
  on a difference only the user can close; the add-on never sets
  `PENDING`.
- The credential MUST NOT hold `edit:space`, which unlocks doors. Minimum:
  `view:user`, `edit:user`, `view:policy`, `view:space`, `view:credential`,
  `edit:credential`, and `view:system_log` for usage. The registry has no
  door-opening operation, so Syndra cannot be asked to open one.
- Removal deactivates; delete is never sent. Drift is found by polling
  users (there is no webhook for user or policy changes). Usage (door
  openings per day, per door) would come from `system/logs` with `since`,
  under `stats`.
- There is no version endpoint: the API reference names the app version
  each endpoint needs, so the add-on gates on what it can probe and records
  each probe like any read.
- The API is unavailable once the console moves to UniFi Identity
  Enterprise.
- **Unverified:** whether `employee_number` is unique and filterable (if
  not, the add-on pages every user and matches itself); whether
  deactivation revokes entry at once and keeps policies and credentials;
  whether the duplicate-name error blocks two members who share a name;
  rate limits and the page-size ceiling; the scope name `view:policy`
  (inferred); the exact string the webhook signature covers; how to pin the
  console's self-signed certificate.

#### Trotec Ruby (`kind: machine`)

From read-only checks against a live install (5 Nov 2026).

| Field | Role | On Ruby |
|---|---|---|
| `active` | `account` | The user's `active` switch |
| `role` | `membership` | Roles `User`, `Admin`; never `Superadmin` or `Hmi`, which `/values` never lists |
| `group` | `membership` | Ruby groups |
| `materials` | `flag` | The `MdbAdmin` role with the `CanModifyMdb` permission |

- Ruby's user API is its internal API (`0.1-InternalApi`), unversioned and
  free to change with any update, and Ruby updates itself with no way to
  defer. So the version gate is essential: `/health` reads the
  unauthenticated `GET /api/Info/GetApiInformationalVersion`, and on a
  WebApi build with no recorded contract the add-on refuses every write with
  422 `TARGET_VERSION_UNSUPPORTED` and goes on answering reads, `/plan`
  included ([chapter 2](#2-before-you-write-a-line), step 2).
- The credential is a dedicated Ruby admin account that signs in (personal
  access tokens reach no user route), with the EULA accepted; its password
  lives only in the add-on's secret.
- `UpdateUserData` takes the whole user: read, merge, put.
  `ReplaceUsersInGroup` replaces a group's whole member list, a set shared
  by many subjects: serialise read, merge and write per group, and keep the
  members Syndra does not manage.
- Create sets a random password nobody sees; the member sets their own
  through `password.set`, mapped to `ResetUserPassword`. Removal sets
  `active: false`; `DeleteUsers` is never sent (it hands a person's designs
  and jobs to the superadmin).
- The laser PC is sometimes off: `/health` says `reachable: false`, reads
  answer their last value as `stale`, writes answer `TARGET_UNREACHABLE`,
  and changes wait in Syndra's queue. Usage (laser time per member, per
  machine) would be `laser.job_seconds` under `stats`, collected from the
  job history when the PC answers.
- **Unverified:** the type and stability of a Ruby user's `id`; whether
  `ResetUserPassword` sets `temporaryPassword` and forces a second change at
  the laser; whether `CreateUser` emails the person; whether a read shows a
  password the member has not yet set (which would make it `standing`
  waiting on the member); whether the job history names the user by id;
  rate limits on the internal API.

#### The inventory system (`kind: inventory`)

**Its API is unknown: nothing about it has been researched.** This sketch
tests the contract, not the product, and uses the operations of
[A.1](#a1-worked-case-an-inventory-checkout).

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
  whether an API credential can check an item out on a member's behalf;
  and whether returns are events or state. A product with no borrower
  profile does not fit this sketch: an add-on keeping `borrower`,
  `categories` and `max_items` in its own store would report its own claim
  as the app's observed state, and that is a design question of its own.
