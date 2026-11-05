# Trotec Ruby: can Syndra manage who uses the laser?

5 Nov 2026 · feasibility only, nothing built · sources: the network session's notes from the live install (read-only, plus its proxy setup), Trotec's public help pages

## Short answer

**Yes, as a second add-on, the same way TrueNAS works. Not as single sign-on.**

Ruby has no SSO of any kind: no OIDC, SAML, LDAP or OAuth in its code, and none
documented. Its sign-in is always a Ruby account, email and password. So
Zitadel cannot become Ruby's login.

But Ruby has a full user API, and Syndra already has the right shape for this:
an add-on that keeps a target's accounts in step with who holds a role. A
`ruby` add-on would create the Ruby account when someone is given the laser
role in Syndra, set its role and groups, and switch it off when the role ends.
People still type a Ruby password at the laser, but who has an account, what
they may do, and when it stops is decided in Syndra, audited, and reversible.

Estimated size: **medium**, about the TrueNAS add-on's first slice. The add-on
contract, transport keys, plans, the operations queue and the revocation path
all exist; this is one adapter.

## What the install is

- **Ruby 2.11.1**, on-premises, on a Windows PC on the machines VLAN. This is the
  latest release (26 Nov 2025). One laser.
- **Several services:**
  - an Angular UI on 2402;
  - a .NET (Kestrel) API on 5001, with uploads and a SignalR hub;
  - a tray-app hub on 5016;
  - status on 5043;
  - an updater on 46146.
- **The PC must be on**, with the tray app running, for any of it to work.
- **Reverse proxy:** as of today, `laser.makerspace.tools` is reverse-proxied by
  the internal Caddy on all of those ports, LAN only, set up by the network
  session. Ruby's own CA is trusted on the proxy; it is valid to 2036.
- **Accounts:** Superadmin, Admin and User roles, a "Materials" right, groups,
  and an active/inactive switch per user.
- **CSV:** CSV import and export of users, semicolon-separated:
  `email;name;active;admin;materials`.

## The user API (seen in the UI bundle, `/api/User/*`)

| Area | Calls |
|---|---|
| Session | `SignIn`, `SignOut`, `GetLoggedInUser`, `ReauthenticateSuperAdmin` |
| Users | `GetUsers`, `CreateUser`, `UpdateUserData`, `DeleteUsers` |
| Passwords | `ResetUserPassword`, `UpdateUserPassword` |
| Groups | `GetGroups`, `CreateGroup`, `UpdateGroupName`, `DeleteGroups`, `ReplaceUsersInGroup` |
| Bulk | `ImportUsers`, `ExportUsers` |
| Tokens | `GeneratePersonalAccessToken`, `GenerateUserPersonalAccessToken`, `ResetUserPersonalAccessTokenSecretKey` |

**This is Ruby's internal API.** It reports scheme `0.1-InternalApi`. It is
what the UI itself calls, not the public REST API Trotec released as a beta in
2.9; that one is meant for job integration. An internal API can change with any
Ruby update without notice. That is the main risk, and the design below is
built around it.

## How it would map onto the add-on contract

| Contract route | Ruby |
|---|---|
| `GET /subjects` | `GetUsers` + `GetGroups`: each user's active flag, admin, materials, groups |
| `GET /values/{field}` | the groups that exist, and the role values User and Admin |
| `POST /plan` → `/apply` | `CreateUser`, `UpdateUserData` (active, admin, materials), `ReplaceUsersInGroup` |
| `POST /operations/reset-password` | `ResetUserPassword`, a self-service password like TrueNAS's |
| `GET /health` | `GetVersionInfo`, which needs no auth, plus whether the PC answers |

**Entitlements in Syndra's words:**
- holding a role in a "Laser" project (for example *Operator*) gives an active Ruby account;
- *Materials* gives the materials right;
- a *Laser admin* role gives Admin;
- groups follow bundles or rules, as the TrueNAS add-on's groups do.

**Removal disables, never deletes.** Ruby moves a deleted user's designs and
jobs to the superadmin. Disabling keeps a person's work where it is and is
reversible.

## Things that decide the design

1. **Unversioned API.** The add-on reads `GetVersionInfo` first and works only
   against versions it has a recorded contract for. TrueNAS already has this: an
   observed-responses file in `addons/contract/`. On an unknown version the
   add-on reports itself read-only and plans nothing, so a Ruby update can
   never half-apply a change. A Ruby update then means one recorded check and a
   version bump.
2. **Which credential it runs as.** A dedicated Ruby admin user that signs in;
   personal access tokens can't reach the user routes (checked below). The
   secret stays inside the add-on container, like the TrueNAS API key.
3. **Passwords.** A new Ruby account gets its first password by email from
   Ruby. Syndra can do better, the way it does for network storage: the member
   sets their laser password on their own My access / storage-style page, and
   the add-on applies it with `UpdateUserPassword` or `ResetUserPassword`.
   Ruby's email then becomes a fallback.
4. **The PC is sometimes off.** Plans and operations wait in Syndra's queue, the
   same unconfirmed-operation path TrueNAS uses, and settle when Ruby answers
   again. Revocations show under Unfinished revocations until they land.
5. **Reachability.** The add-on container needs a route to the Ruby API on 5001,
   through the internal proxy or straight to the machines VLAN. It needs that
   port only; it never needs the UI.
6. **Existing accounts.** The first `GET /subjects` lists today's Ruby users as
   unmanaged, as TrueNAS's unmanaged inventory does. An operator then matches
   each one to a person by email before Syndra manages it. Nothing is changed
   on the first sync.

## Optional, separate: a gate in front of the UI

Caddy could require a Zitadel sign-in (forward-auth) before anyone reaches
`laser.makerspace.tools`. It would not replace Ruby's login, and it has three
costs:
- it must cover every port Ruby uses, including the SignalR WebSockets;
- it protects only the hostname: the PC's own address on the machines VLAN
  would still answer unless the firewall closes it;
- the laser's own client traffic must be left alone.

Worth it only if anonymous access to the login page is itself a concern. The
add-on comes first either way.

## Seen while looking, not caused by us

Ruby's API reflects any Origin with credentials allowed (CORS). Its session is a
bearer token in localStorage, not a cookie, which limits what a foreign page
could do with that. It's still worth keeping the Ruby PC reachable only from
the LAN, which it is.

## Checks done, 5 Nov 2026 (read-only, in the owner's admin session)

These settle the design. Everything was read from Ruby's own OpenAPI documents
(`/swagger/0.1-InternalApi/swagger.json` and `/swagger/1.0-OpenApi/swagger.json`
on port 5001), the UI bundle, the settings pages, and two unauthenticated
version calls from the production host. Nothing was created, changed or signed
into.

1. **Personal access tokens cannot manage users.**
   - Tokens belong to the *public* API, `1.0-OpenApi`. Its scheme is
     `Authorization: PersonalAccessToken <token>`, and its 16 routes are all
     jobs, designs, queue and device status.
   - It has no user routes at all.
   - Every `/api/User/*` route is in the *internal* API, `0.1-InternalApi`, under
     a separate `oauth2` scheme. That scheme accepts the session token that
     `SignIn` returns, and admin routes also require `RequirePermissionIsAdmin`.
   - No internal route accepts the token scheme.
   - **So the add-on signs in as a dedicated Ruby admin account.** It calls
     `POST /api/User/SignIn` with `{email, password}`, which returns an
     `AuthenticatedUser` carrying the bearer token. It signs in again when the
     token expires.
   - The account's password lives only in the add-on's secret, like the TrueNAS
     API key. That account must have accepted the EULA (`AcceptEula`), or its
     calls may be refused.
2. **Creating a user sets their password.**
   - The call is `POST /api/User/CreateUser?password=<p>&language=<l>` with a
     `ManageableUser` body.
   - Ruby's own screen generates a temporary password and shows it to the
     admin, so creation does not depend on email.
   - A `language` parameter is sent, so Ruby may also email the person.
     Confirm on the first real create.
   - The new account is flagged `temporaryPassword`, so Ruby asks for a new
     password at first sign-in.
   - For Syndra this means: the add-on creates the account with a random
     password nobody sees, then the person sets their own laser password in
     Syndra, through `PUT /api/User/ResetUserPassword?userId=&newPass=`, as with
     network storage.
   - Whether a reset also sets `temporaryPassword`, and so forces a second
     change at the laser, is the one behaviour to confirm when building.
3. **One call carries all of a user's state.**
   - `PUT /api/User/UpdateUserData` takes the whole `ManageableUser`:
     `id, email, name, active, roles[], permissions[], createdOn, updatedOn`.
   - Ruby's own UI sends the stored user merged with the change. The add-on
     does the same: read, merge, put.
   - Roles: `User`, `Admin`, `Superadmin`, `MdbAdmin` (materials database),
     `Hmi` (the machine's touch panel).
   - Permissions: `IsUser`, `IsAdmin`, `IsSuperadmin`, `CanModifyMdb`.
   - Syndra should never grant `Superadmin` or `Hmi`, and the add-on's manifest
     should not list them.
   - Groups are separate: `POST /api/User/ReplaceUsersInGroup` sets a group's
     whole membership. Deleting users is `PUT /api/User/DeleteUsers`; we
     disable instead.
4. **Reachable.**
   - From the production app host, and from inside the backend container, the
     API answers on 5001, both through `laser.makerspace.tools` and at the
     PC's own address. Authenticated routes return `401`.
   - `GET /api/Info/GetApiInformationalVersion` needs no auth and returns the
     WebApi build (`20251105.4 live 02a201f …`). That is the add-on's version
     probe.
   - `GetVersionInfo` needs a session.
5. **Updates can't be held.**
   - Settings › Info shows `AutoUpdater: Running` and branch `live`. Settings ›
     Features only has "Allow update from touch panel". There is no switch to
     defer updates.
   - So the version gate is required, not optional. The add-on keeps a recorded
     contract per WebApi version, and on an unknown version it reports
     `read_only` and plans nothing. Unfinished changes wait in Syndra's queue
     until someone records the new version's contract (one recorded-responses
     check, as for TrueNAS), and the add-on resumes.

**Recorded for the build:**
- Internal scheme version `0.1-InternalApi`.
- Ruby 2.11.1.64391, frontend 20251125.11, WebApi 20251105.4.
- The `x-api-version` header defaults to `InternalApi`.

## When it's built: the slice

1. **Account.** Create a dedicated Ruby admin account for Syndra (owner, by
   hand), with the EULA accepted. Its email and password go into the add-on's
   secret.
2. **Add-on.** `addons/ruby`, following the contract in `addons/contract`:
   - `/subjects` from `GetUsers` and `GetGroups`;
   - `/values` for groups and for the roles User, Admin and MdbAdmin;
   - `/plan` and `/apply` through `CreateUser`, `UpdateUserData` and
     `ReplaceUsersInGroup`;
   - `/operations/set-password` through `ResetUserPassword`;
   - `/health` from the version probe plus a signed-in `GetLoggedInUser`.
3. **Mapping.** In Syndra, a Laser project:
   - an Operator role makes an active account;
   - a Materials role adds `MdbAdmin` and `CanModifyMdb`;
   - a Laser admin role adds `Admin` and `IsAdmin`;
   - groups follow bundles.
4. **Revoke.** Revoking sets `active: false`; it never deletes.
5. **First sync.** Existing Ruby users come in as unmanaged, and an operator
   matches each one by email before Syndra manages it.
6. **Member page.** Members get "Laser password" next to "Network storage",
   through the existing self-service password path.

## Sources

- The network session's notes, 5 Nov 2026, from the live install: version
  endpoints, the UI bundle's endpoint map and API routes, and the Caddy setup.
- Trotec help: [user management](https://www.troteclaser.com/en/helpcenter/software/ruby/user-management),
  [Ruby 2.9 notes (REST API beta, personal access tokens, CSV import)](https://www.troteclaser.com/en/helpcenter/software/ruby/ruby-2-9),
  [release notes (2.11, 2.11.1)](https://www.troteclaser.com/en-us/helpcenter/software/ruby/release-notes),
  [FAQ](https://www.troteclaser.com/en/resources/faqs/ruby).
