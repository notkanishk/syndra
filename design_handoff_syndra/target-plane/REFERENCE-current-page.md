# What the target page holds today

Attach this alongside the brief. It is reference, not a prompt.

`/system/targets/{target}` · eleven panels, in render order. `{t}` is the
target's id — `truenas` on this deployment.

---

### 1 · Health

`GET /api/v1/targets/{t}/health` · designed as §21

Readings, each with a tone dot and a sentence naming which machine to look at:

| Reading | Tone | Says |
| --- | --- | --- |
| Serving | healthy | product, version, answering and accepting changes |
| Not answering | red | the add-on did not answer — look at the add-on |
| Backed off | red | Syndra refusing its own calls after repeated failures — **not** the target being down; look at this host |
| Draining / Read-only | **accent, never amber** | somebody chose this on purpose, with their reason |
| Still settling | amber | *n* calls issued before the drain have not come back |
| Transport secret unreadable | red | a fault on this host; renders **above** reachability because it explains it |
| Key expiry unrecorded | amber | the credential can expire without Syndra knowing, and the day it does the target looks like an outage |
| Unaudited shares | amber | an activity report for a member on one of these can only come back empty |
| Untested version | amber | the release is outside the supported set |

Below the readings: a definition list — last answered, in flight, lifecycle — and
the freshness line for the snapshot.

Two **findings** also render inside this card, above the reachability reading.
They have no design anywhere. See prompt 3, screens D and E.

### 2 · What the target reports

`GET /api/v1/targets/{t}/system-health` · **no design**

The NAS's own health, as distinct from Syndra's ability to reach it. Alerts
(level, class, the target's own prose, when, dismissed); pools (name, status,
free/allocated/size); services (name, state, enabled); hostname, version, uptime.
A `degraded` list names which of those four could not be read — "nothing is
wrong" and "I could not look" must not render the same.

The live deployment carries one standing alert: an uncorrectable error on
`/dev/sde`, open since July.

### 3 · What roles reach here

`GET /api/v1/targets/mappings?target={t}` · designed in §24 **as a separate
screen**

One row per mapping: project, role key, field, value, how many people hold that
role, and two actions — Change, Remove. Editing one moves access for everybody
holding that role, and both edit and delete rehearse before they land.

### 4 · Published versions

`GET /api/v1/targets/{t}/mappings/versions` · §24, same caveat

A snapshot of (3) with a mandatory note, and a rollback per version. The working
copy can differ from the newest published version — "current version 4" can mean
version 4 plus three unpublished edits, and rolling back from there undoes work
listed nowhere.

### 5 · People with an account here

`GET /api/v1/targets/{t}/inventory` → `accounts[]` · **no design**

The managed roster. Per row: the person by name, their account name, uid, how
long bound, then **Hold** (pause what a role grants without touching the role)
and **Take away**. Reversible first, deliberately.

A binding whose account no longer exists can be forgotten:
`POST …/bindings/{subject}/release`. Syndra stops managing it, nothing is
deleted, and it can be bound again by adopting the account if it returns.

### 6 · Accounts nothing explains any more

`GET /api/v1/targets/{t}/accounts/dormant` · designed as §29

Accounts Syndra created and no longer has a reason for. It **refuses to act on
anybody still a member** — removing one of those locks the person out rather than
tidying up. Removal takes a separately injected credential the add-on does not
hold, so a compromise of the add-on cannot destroy anybody's files.

### 7 · Accounts Syndra did not create

`GET /api/v1/targets/{t}/inventory` → `unmanaged[]` · designed as §21

Never drift. A real NAS holds `root`, service accounts and whatever an admin made
by hand. Adoption is **blocked while the read is stale** — you cannot adopt from a
list that may have moved, and there is no undo. Syndra's own service account is
listed and named as not adoptable rather than silently missing.

### 8 · What it can do

From the roster's manifest · designed as §21

One row per operation: its id, its scope, whether it stops and asks for
confirmation, which parameters are never logged, and — when the target cannot
perform it — the reason, shown **disabled rather than omitted**. Omitted, an
operator wonders whether the feature exists at all.

Six today: `account.provision`, `account.converge`, `account.release`,
`account.purge`, `credential.set`, `activity.get`.

### 9 · Waiting on a decision

`GET /api/v1/targets/{t}/merge-findings` · **no design** · prompt 3, screen B

Differences reconciliation refuses to resolve. Three outcomes, five resolutions,
a mandatory reason. A decided row is **still standing** until a later pass sees
the target agree.

### 10 · Reconciliation

`POST /api/v1/targets/{t}/reconcile` · **no design**

"The scheduled sweep runs every six hours", and a **Reconcile now** that reads the
target and queues what is already owed. Returns bound / queued / stale counts.
Queueing is not applying.

### 11 · Maintenance

`POST /api/v1/targets/{t}/lifecycle` · designed as §21

Active, draining or read-only, each with a sentence, and a **mandatory reason**
because the person who reads it next is not the person who set it. Reads keep
working in all three.

---

## The live deployment, for reference

One target, `truenas`, answering, six operations, TrueNAS 25.10.5. **Nothing
bound**, two unmanaged accounts, no mappings, no published versions, no findings,
an empty change record. Figure 4 in the brief is this state.
