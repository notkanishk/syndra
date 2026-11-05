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
2. **Which credential it runs as.** One option is a dedicated Ruby admin user
   with a personal access token. We don't yet know whether a token can call the
   `/api/User/*` routes. If it can't, the add-on signs in as that user and
   holds the session's bearer token. Either way the secret stays inside the
   add-on container, like the TrueNAS API key.
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

## To find out before building (all read-only)

- [ ] Can a personal access token call `/api/User/*`, or only the public REST
      API? This decides how the add-on authenticates.
- [ ] What does `CreateUser` need, and does it always send the welcome email?
      (Ruby needs SMTP or internet for that.)
- [ ] Does `UpdateUserData` carry active, admin and materials in one call?
- [ ] Is the API reachable from the backend host's network to the machines
      VLAN on 5001?
- [ ] How often does Ruby update itself (the updater on 46146), and can updates
      be held until the add-on has a recorded contract for the new version?

## Sources

- The network session's notes, 5 Nov 2026, from the live install: version
  endpoints, the UI bundle's endpoint map and API routes, and the Caddy setup.
- Trotec help: [user management](https://www.troteclaser.com/en/helpcenter/software/ruby/user-management),
  [Ruby 2.9 notes (REST API beta, personal access tokens, CSV import)](https://www.troteclaser.com/en/helpcenter/software/ruby/ruby-2-9),
  [release notes (2.11, 2.11.1)](https://www.troteclaser.com/en-us/helpcenter/software/ruby/release-notes),
  [FAQ](https://www.troteclaser.com/en/resources/faqs/ruby).
