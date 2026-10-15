# TrueNAS through Syndra, end to end

How a person comes to use network storage, from the operator's side and the member's, in Syndra and on TrueNAS. Each step is marked with what's true today, and what this change does about it. Traced from the code on 14 Oct 2026, and checked against a read-only audit of the production NAS.

Legend: **ok** works as intended · **fixed** fixed in this change · **owed** a gap with a task below · **NAS** a TrueNAS-side setting, not Syndra's

---

## Part 1 · The operator

### 1.1 Connect the system, once

**Syndra:**
- The add-on is registered by environment: `ADDON_TARGETS`, plus `ADDON_TRUENAS_BASE_URL` and `ADDON_TRUENAS_SECRET[_FILE]` (`addons/registry.go:140`).
- The backend reads the add-on's manifest at boot, and again on the `ADDON` periodic job. The manifest declares:
  - its operations;
  - an entitlement schema: `group`, plus the lifecycle fields `enabled` and `smb_enabled`;
  - one connection host, `TRUENAS_SHARE_HOST`.
- Health is read every 30s. A system is reported **offline only after three failed reads in a row** (**fixed**: D1).

**TrueNAS:** the add-on signs in with one API key (`READONLY_ADMIN`, `ACCOUNT_WRITE`, `SYSTEM_AUDIT_READ`). It never touches shares, ACLs or datasets. It reads them, and writes only accounts.

### 1.2 Choose the system's Zitadel project (new)

- `PUT /targets/{t}/project` sets it.
- Until it is set, every write that **gives** access on that system waits (`409 PROJECT_REQUIRED`). Taking access away never waits (**fixed**: D3–D5, migration 000051).
- Production's two mappings already share one project, so the migration infers it.

### 1.3 Map roles to groups

**Syndra:**
- A mapping says "holding this role puts you in this TrueNAS group" (`target_role_mappings`).
- Before it is written, the operator rehearses it and sees who it reaches. The write then queues a convergence for everyone who holds the role.
- Sets of mappings are published as versions, and a version can be rolled back.

**TrueNAS:** the group must already exist there. Syndra never creates groups, and never offers or writes built-in groups (`builtin_users`, `builtin_administrators`, …) (**fixed**: D8).

**NAS:** the group must also be given access on the folders it is meant to open. That ACL is the only thing that decides what a group can open. Syndra reads it and never writes it.

### 1.4 Give a person a role

There are four ways to give a role. Each must lead to the NAS account.

| Path | Reaches TrueNAS today |
|---|---|
| Bundle assigned or published, rule change, role members (`services/cascade.go:262`) | **ok**: queues a convergence |
| A request approved (`handlers/access.go:353`) | **fixed**: converges the system its project serves |
| A direct grant: add, replace or revoke | **fixed**: same, through `grantDirect` |
| A grant made in Zitadel's console (webhook `grant_added`) | **by design, no**: a Zitadel change with no Syndra trace is drift. It reaches TrueNAS once an operator accepts it in drift triage |

Behind all of them, the periodic reconcile now also creates the account for anyone who holds a mapped role and has none. It skips anyone with a change already waiting (**fixed**).

### 1.5 Send

- A convergence for TrueNAS waits in Pending changes for the operator to send ("Waiting for you to send →"). Per role, that becomes "Roles that wait" (reconcile step 3).
- Sending runs `/plan`, then `/apply`:
  - **No account yet.** `user.create` with:
    - the derived username
    - the mapped groups
    - a personal group
    - shell `nologin` (**fixed**)
    - password authentication off
    - SMB off. TrueNAS refuses SMB on an account with no password.
  - **An account exists.** Its groups are replaced by the mapped set, with built-in memberships carried through (**fixed**). It is unlocked or locked to match the entitlement.
  - SMB is never switched on here. TrueNAS accepts that only in the call that sets a password (**fixed**: sending it alone used to fail the whole update).

### 1.6 Afterwards

The system page shows:
- health and lifecycle
- the census of mapped roles
- people on the system, and the unmanaged inventory (adopt)
- merge findings and dormant accounts
- the mutation log anchor

A hold, or lifting one, now queues that person's convergence at once (**fixed**). So does Zitadel deactivating someone.

### 1.7 Taking access away

- **Take away** (`revoke-access`): a deny hold, a convergence, and a password rotation. The account loses its groups and SMB, and its password stops working.
- **Restoring access** later needs the member to set a new password before SMB comes back. That is TrueNAS's rule (D1 note in `apply.go`).
- **Dormant sweep:** purges accounts nobody holds.
- **Pause** (draining or read-only): stops changes, and leaves access as it is.

---

## Part 2 · The member

### 2.1 Get a role

They request one, or an operator gives one (1.4). They sign in to Syndra with their makerspace Google account through Zitadel.

### 2.2 Open /storage

`GET /me/targets` says:
- whether any role reaches TrueNAS
- the account name, once it exists
- whether a password is set, and when
- health (online, offline since, or can't tell)
- whether changes are paused
- usage per share

The page walks them through the three states: nothing reaches TrueNAS, the account is on its way, the account is ready.

### 2.3 Set the storage password

- `POST /me/targets/truenas/credential` hands the password to the add-on and keeps no copy.
- The add-on sets it, turns password authentication on, and **switches SMB on when the entitlement wants it** (**fixed**: it used to read the NAS's current state, which for a new account is always "off", so no Syndra-created account could ever use SMB).
- It is refused while the system is paused (`TARGET_PAUSED`, **fixed**) or offline.

### 2.4 Connect

Today the page offers `smb://<host>/<group>` and `\\<host>\<group>`. **It assumes a share is named after each group, and none is** (**owed**). On production, Community's real address is `\\drive.makerspace.tools\main\waddle_share`.

The usage panel has the same root cause. It lists every share where the member owns files, whatever their access, and leaves out the shares they can reach but haven't used yet.

---

## Part 3 · Where to connect, done properly

**The rule:** an address is only ever offered for a place the member's groups can actually open. TrueNAS decides that, in the folder ACLs. A mapping says only which group someone is in.

**Proposed (owed, group 8):** the add-on derives it from the NAS.
1. For each enabled SMB share, read the ACL at the share's root and at each first-level folder (`sharing.smb.query`, `filesystem.listdir`, `filesystem.getacl`).
   - `ponytail:` one level deep. That matches today's layout, where each first-level folder of `main` is its own dataset. Deeper grants show up as the folder above them.
2. For a set of groups, return every path where one of them has more than traverse:
   - share
   - sub-path
   - read or modify
   - dataset
   - the two addresses: `smb://host/share/sub` and `\\host\share\sub`
3. Cache the result per share, and refresh it on the reconcile pass. The page never waits on a dozen ACL reads.
4. `storage.status` reports usage per *reachable* path, from that path's dataset. A share the member can open but hasn't used shows 0 used, not absent.

**Why not configure it in Syndra:** a second copy of "who opens what" next to the ACL is a second definition that drifts. The NAS is already the authority.

**What it gives the operator too:** the same read answers "what does this group open" on the mappings page, which is the scope line in the reconcile boards.

**Recommended NAS layout (NAS, owner's call):** one SMB share per resource dataset (`waddle_share`, `quack_share`, …) instead of one `main` share with folders, with access-based enumeration on. Addresses become `\\drive.makerspace.tools\waddle_share`. A member then sees only what they can open, and step 1 above needs only the share root. The derivation works with either layout, so this can happen at any time.

---

## Part 4 · Gaps still owed after groups 1–3

| # | Gap | Where | Task |
|---|---|---|---|
| G1 | ~~Request approval and direct grants don't queue a TrueNAS convergence~~ fixed | | 9.1 |
| G2 | Mount addresses assume share = group. The add-on and backend now derive places from the ACLs; the page still renders the old derivation | `MyStorage.tsx:437` | 8.5 |
| G3 | ~~Usage lists shares by file ownership, not by access~~ fixed: usage is per place the member can change | | 8.4 |
| G4 | ~~A hold, or lifting one, queues nothing~~ fixed | | 9.2 |
| G5 | ~~Deactivation in Zitadel lands only at the next resolution~~ fixed | | 9.3 |
| G6 | The paused copy still promises the password is applied later | `MyStorage.tsx:400,584` | 5.2 |
| G7 | The withheld list can print `true` for an SMB denial | `MyStorage.tsx` withheld list | 5.2 |
| G8 | ~~No home directory is set~~ fixed: home folders by role (group 10) | | 10.x |
