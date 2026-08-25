# truenas

The add-on for [TrueNAS SCALE](https://www.truenas.com/truenas-scale/) — SMB
storage accounts, their group membership, and their credentials.

Read [`../README.md`](../README.md) first: it describes the model this
implements, and everything below assumes it.

```bash
go test ./... && go vet ./...
```

## What it maps

| Syndra says | TrueNAS does |
|---|---|
| `group: string[]` | Membership of the named SMB groups, resolved by NAME rather than gid — a mapping binds a role to a group name, and a number compared against a name makes every subject look like drift |
| `enabled: bool` | `locked`, inverted. TrueNAS's word for it is not Syndra's, and the translation belongs here |
| `smb_enabled: bool` | The account's SMB flag |

Both booleans are **lifecycle** fields: the backend's resolver computes them
from whether the subject holds any mapped role, overridable only by an
allowance. A mapping that named one would fight the derived state on every
resolution, so the backend refuses to accept one.

Eight operations: `password.set` (the only one a member drives),
`password.rotate`, `account.adopt`, `account.release`, `account.purge`,
`storage.status`, `activity.get`, `health.get`.

## Configuration

| Variable | Notes |
|---|---|
| `TRUENAS_URL` | `wss://host/api/current`. **Must be `wss://`** — TrueNAS revokes a user-linked API key presented over plaintext, so a `ws://` probe destroys the credential |
| `TRUENAS_API_KEY` | Prefer the `_FILE` form. A dedicated local user whose privilege carries `ACCOUNT_WRITE`, `READONLY_ADMIN` and `SYSTEM_AUDIT_READ`. Never `FULL_ADMIN`. The read roles are not decoration — the add-on calls `system.version` at start-up and treats a failure as *unreachable*, so `ACCOUNT_WRITE` alone yields a NAS that reports as down while answering every call perfectly |
| `TRUENAS_SHARE_HOST` | What a **member** types into a file manager. Not the API URL — they are frequently different names, and sending one where the other is meant produces mount instructions that fail for everybody. Unset, the manifest omits the connection block and the member page shows no instructions rather than inventing a host |
| `TRUENAS_SUPPORTED_MAJORS` | The releases this add-on has been recorded against, e.g. `25.04,25.10`. The version gate refuses mutations outside it |
| `TRUENAS_VERIFY_TLS` | Defaults to on. Turning it off hands the API key to whatever answers for the NAS's address |
| `ADDON_TARGET` | This add-on's name in the deployment **and** the HKDF salt. It must equal the backend's `ADDON_TARGETS` entry: a disagreement derives different keys from the same secret, and the symptom is a pin failure indistinguishable from a wrong secret |
| `ADDON_SECRET_FILE` | The one transport secret. Minted by the deployment's own `truenas-addon-secret` service into a volume mounted into both ends |

Profile-gated in Compose (`--profile truenas`). The container holds the NAS API
key, so a deployment with no NAS must not be running it.

## Layout

| File | Responsibility |
|---|---|
| `main.go` | Configuration and start-up. Refuses to start without a target URL |
| `server.go` | The six endpoints, and health |
| `transport.go` · `derive.go` | The authenticated channel. Both keys from one secret |
| `capabilities.go` | The manifest — entitlement schema, operation set, per-operation availability |
| `subjects.go` | The state read. The `select` is the security-relevant part |
| `plan.go` · `apply.go` | Rehearse, then converge against a plan id |
| `operations.go` | The one-shot half |
| `storage_status.go` | The member's own view: is my account usable, how much room is left |
| `mutationlog.go` | Append-only, `0600`, fsynced before the mutation it describes |
| `store.go` | bbolt. A result cache and a state mirror — never a queue |
| `lifecycle.go` | `active` · `draining` · `read_only`, at runtime |
| `username.go` | Deriving the account name. TrueNAS generates none and `user.create` demands one, so it comes from the Zitadel identity's email localpart — the one handle the IdP guarantees stable and unique |

## Two rules with scars behind them

**The `select` on `user.query` names every column, and only these.** TrueNAS
returns `unixhash` and `smbhash` otherwise, and an NT hash is a pass-the-hash
credential — possessing one is equivalent to possessing the password for SMB.
The two hash fields are absent by construction rather than stripped afterwards,
because stripping is a step somebody can forget and the forgetting would not be
visible.

**Never write a decoder against a shape nobody observed.** `activity.get`
decoded `message_timestamp` as a string. It is an integer, so `encoding/json`
failed the *entire* response and the operation answered "the audit log could
not be read" every time it was ever called — for its whole life, with both
suites green, because the recorded fixture stored key names and the names were
right.

Two guards came out of that and both must keep passing:

- `read_shape_test.go` parses this package's own source for the struct behind
  every `nas.call(...)` and asserts each json tag against
  [`../contract/truenas_observed.json`](../contract/truenas_observed.json) — the
  key exists, at the level it is read from, and the Go type can hold what the
  target puts in it. A method decoded here that the recorder never asked about
  fails too: an unrecorded read is a fixture that cannot disagree with anything.
- `truenas_rules_test.go` asserts this add-on's payloads against the target's
  recorded **refusals**. `user.create` refuses a payload with no password
  decision; a hand-written fixture answered that payload with a success, so
  account creation had never worked against any release.

Re-record with `scripts/record-truenas-fixtures.sh` on a major upgrade, and
commit the result. A recording names the release it came from, and a recording
from one major says nothing about another.

## Known state

`TRUENAS_API_KEY`'s permission set **can delete an account** — `ACCOUNT_WRITE`
is what `user.delete` requires, and TrueNAS publishes no narrower role. The
separately injected purge key is therefore an audit and blast-radius separation
— deletion stays out of the long-lived session and every one is traceable to a
credential issued for that single call — and **not** the capability separation
an earlier draft of the design claimed. That draft said the opposite for a
while, which is the worse failure: an unknown invites a check and a plausible
reason ends one.

SMB auditing is per share *and* per group: a share carries `enable`, a
`watch_list` that narrows recording to named groups, and an `ignore_list` that
excludes. `activity.get` reports the shares that were not recording the account
it is answering about; target health answers the separate question of whether
auditing is switched on at all. These are not the same list, and merging them
would send an operator to change a setting they would find already correct.
