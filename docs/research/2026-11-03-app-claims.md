# What each OIDC app reads from the token: research for Syndra claim templates

Researched 2026-11-03. Sources are official docs or upstream source code on the default branch at that date. Unverified items are marked **(unverified)**.

## Facts that apply to every app

- Zitadel's built-in roles claim (`urn:zitadel:iam:org:project:roles`) is a nested object, not a list. NetBird's Zitadel guide therefore uses an Action that writes a flat `groups` claim to both Pre Userinfo and Pre Access Token. <https://docs.netbird.io/selfhosted/identity-providers/zitadel>
- Where a claim lands depends on the Zitadel app settings:
  - `preaccesstoken` claims reach only a JWT access token (Auth Token Type = JWT).
  - `preuserinfo` claims reach the ID token only when "User Info inside ID Token" is on.
  - Both settings appear in the NetBird and Vaultwarden Zitadel notes above. NetBird and Supabase's custom provider depend on exact placement, so a template has to fix it.
- **Not one of these apps accepts a CSV or space-separated string.** A string is either read as one group (NetBird, GitLab, Dex) or thrown away (Home Assistant). Every template must use an array.
- **No app reads a nested path itself.** The only exception is Supabase's generic ID-token parser, which keeps a top-level `custom_claims` object (see Supabase below).

---

## NetBird (self-hosted)

**Identity.**
- The user ID comes from `sub`, configurable through `WithUserIDClaim`. NetBird also reads `email`, `name` and `preferred_username`, all top-level strings. <https://github.com/netbirdio/netbird/blob/main/shared/auth/jwt/extractor.go#L93>
- `name` is the display name; `preferred_username` is used when `name` is absent. <https://docs.netbird.io/selfhosted/identity-providers/generic-oidc>
- NetBird validates **the token the dashboard sends**: the access token unless `NETBIRD_TOKEN_SOURCE=idToken` is set. <https://github.com/netbirdio/dashboard/blob/main/docker/init_react_envs.sh>
- `NETBIRD_AUTH_AUDIENCE` is the Zitadel client ID. <https://docs.netbird.io/selfhosted/identity-providers/advanced/zitadel>

**Groups.**
- Settings are in the dashboard under Settings → Groups: "Enable JWT group sync", "JWT claim" (stored as `JWTGroupsClaimName`) and "JWT allow groups" (`JWTAllowGroups`). <https://docs.netbird.io/manage/settings/groups>
- The claim is read from the top level only: `claims[claimName]`. <https://github.com/netbirdio/netbird/blob/main/shared/auth/jwt/extractor.go#L144>
  - A `[]string` is used as-is.
  - **A plain string becomes one group.** `"a,b"` becomes a single group literally named `a,b`.
  - Any other type, or a missing claim, gives an empty list.
- The sync runs on every login: <https://github.com/netbirdio/netbird/blob/main/management/server/account.go>
  - It creates JWT-issued groups that don't exist yet and **removes the user from JWT-issued groups that are no longer in the claim**.
  - An existing group with the same name that was **not** JWT-issued is **not** joined, because only groups with `Issued == jwt` are added.
- With allow groups set, a user who holds none of them is refused: "user does not belong to any of the allowed JWT groups". <https://github.com/netbirdio/netbird/blob/main/management/server/auth/manager.go>
- The docs warn that saving an allow group you aren't in locks you out. <https://docs.netbird.io/manage/settings/groups>
- NetBird has no JWT mapping for owner, admin or user roles. Those roles are managed inside NetBird **(unverified: no such setting was found in the extractor or settings)**.

**Embedded IdP mode (v0.62+).**
- If Zitadel is added as a connector to NetBird's built-in Dex, NetBird configures it with `insecureEnableGroups: true` and `getUserInfo: true`. <https://github.com/netbirdio/netbird/blob/main/idp/dex/connector.go>
- Dex then reads `groups` from the ID token merged with userinfo. It takes an array of strings, or a single string as one group. <https://github.com/dexidp/dex/blob/master/connector/oidc/oidc.go>
- Whether Dex re-emits `groups` in the token NetBird validates depends on the `groups` scope being requested **(unverified)**.

**What breaks.**
- A role rename changes the group name. NetBird creates a new group and drops the old membership, so policies bound to the old group stop matching.
- A CSV value creates one junk group.
- If the claim sits only in userinfo, NetBird sees no groups in the access token. A missing claim empties every JWT group.

**Template.**
- Emit `groups`: a top-level array of strings, the app's chosen roles under the app's own names, for example `netbird-staff`.
- Place it in the access token (JWT) **and** in userinfo/ID token, so both direct and embedded-Dex modes work.
- Identity claims are Zitadel's standard ones (`sub`, `email`, `name`, `preferred_username`).
- Renaming matters here: the values are visible NetBird group names. Keep them stable, and send only the roles that are meant to be network groups.

## Supabase (self-hosted GoTrue / Supabase Auth)

There are two very different code paths. Which one the owner's deployment uses needs checking **(unverified)**.

**A. Keycloak provider (`GOTRUE_EXTERNAL_KEYCLOAK_*`)**
- Reads **userinfo only**, at the hard-coded path `<URL>/protocol/openid-connect/userinfo`. The auth and token paths are hard-coded Keycloak paths too. Zitadel's endpoints are `/oauth/v2/*` and `/oidc/v1/userinfo`, so this path needs a rewrite in front of Zitadel. <https://github.com/supabase/auth/blob/master/internal/api/provider/keycloak.go#L78>
- Only `name`, `sub`, `email` and `email_verified` are typed. **Every other userinfo key**, including `given_name` and `preferred_username`, is copied into `CustomClaims`. <https://github.com/supabase/auth/blob/master/internal/api/provider/keycloak.go#L96>
- With the current v1 script this gives `user_metadata.custom_claims.groups` and also `user_metadata.custom_claims.custom_claims.groups`, a double nesting that adds nothing.

**B. Custom OIDC provider (`custom:<id>`, Auth ≥ v2.196.0 / self-hosted ≥ 0.8.1)**
- Set up through the admin API with `provider_type: "oidc"`, `issuer`, `scopes`, `attribute_mapping` and `custom_claims_allowlist`. <https://supabase.com/docs/guides/self-hosting/self-hosted-custom-oauth-providers.md> and <https://github.com/supabase/auth/blob/master/internal/api/provider/custom_oauth.go>
- **When an ID token is present it is the only source.** Zitadel always returns one, so userinfo is never read. <https://github.com/supabase/auth/blob/master/internal/api/provider/custom_oauth.go#L194>
- The ID token is unmarshalled into the `Claims` struct, so only typed fields survive. <https://github.com/supabase/auth/blob/master/internal/api/provider/oidc.go#L445>
  - Typed fields include `name`, `given_name`, `family_name`, `preferred_username`, `email`, `email_verified` and `picture`.
  - So does a top-level **`custom_claims` object**, because of the json tag `custom_claims`. <https://github.com/supabase/auth/blob/master/internal/api/provider/provider.go#L115>
  - This is the "GoTrue only preserves `custom_claims`" behaviour the owner saw.
- `custom_claims_allowlist` also copies the listed top-level keys verbatim into `custom_claims`. <https://github.com/supabase/auth/blob/master/internal/api/provider/custom_oauth.go#L315>

**Where the claims land.**
- On **every** sign-in, both `identity_data` and `raw_user_meta_data` are rewritten from these claims. <https://github.com/supabase/auth/blob/master/internal/api/external.go> (AccountExists branch)
- The update is a key-by-key merge: absent keys are **kept** and nil values are deleted. <https://github.com/supabase/auth/blob/master/internal/models/user.go#L232>
- The Supabase access token carries `user_metadata`, so data is read as `auth.jwt()->'user_metadata'->'custom_claims'->'groups'`. <https://supabase.com/docs/guides/auth/jwt-fields>
- **Security:** "`raw_user_meta_data` – can be updated by the authenticated user … not a good place to store authorization data." <https://supabase.com/docs/guides/database/postgres/row-level-security>
  - RLS keyed on these groups can be bypassed by `supabase.auth.updateUser({data})`.
  - Authorization should copy the values from `auth.identities.identity_data` into `app_metadata` or a claim, using a Custom Access Token Hook **(design suggestion)**.

**What breaks.**
- On path B, a top-level `groups` with no allowlist is silently dropped.
- When a user ends up with no roles and the claim is omitted, `CustomClaims` is nil and `omitempty` drops the key. The merge then **keeps the previous `custom_claims`**, so the old groups stay. This is inferred from the code **(verify on a live box)**.
- Groups refresh only on a new Zitadel sign-in. Supabase refresh tokens do not re-pull them.

**Template.**
- Emit `custom_claims: { groups: [..] }`, a nested object, in the **ID token** (and userinfo). Emit it **always**, with `[]` when the user has no roles.
- Leave out top-level `groups` unless the deployment uses the allowlist.
- `given_name`, `family_name` and `name` inside `custom_claims` are redundant on path B, because those typed fields already land in `user_metadata`. Keep them only if app code reads `custom_claims.*`.
- Renaming: use whatever the app's RLS checks for.

## GitLab (self-managed, omniauth `openid_connect`)

**Identity.**
- `uid` comes from `uid_field`, default `sub`. Info maps `name`, `email`, `nickname ← preferred_username`, `first_name ← given_name` and `last_name ← family_name`. <https://github.com/omniauth/omniauth_openid_connect/blob/master/lib/omniauth/strategies/openid_connect.rb#L63>
- `raw_info` is **userinfo merged with the decoded ID token**; on a conflict the ID token wins (L263–273, same file). This is the upstream gem; the version bundled with GitLab was not checked **(unverified)**.

**Groups (Premium/Ultimate only; the code lives in `ee/`).**
- Keys go under `args.client_options.gitlab`: <https://docs.gitlab.com/administration/auth/oidc/>
  - `groups_attribute`, default `groups`
  - `required_groups`, `admin_groups`, `auditor_groups` and `external_groups`, each a list of exact strings
- The read is `Array.wrap(raw_info[groups_attribute])`: top-level only, and a string becomes one element. <https://gitlab.com/gitlab-org/gitlab/-/blob/master/ee/lib/gitlab/auth/oidc/auth_hash.rb> and <https://gitlab.com/gitlab-org/gitlab/-/blob/master/ee/lib/gitlab/auth/oidc/config.rb>
- On every sign-in: <https://gitlab.com/gitlab-org/gitlab/-/blob/master/ee/lib/gitlab/auth/oidc/user.rb>
  - Sign-in is refused if `required_groups` is set and no group matches.
  - `user.admin`, `user.auditor` and `user.external` are each **overwritten** whenever their list is configured.
  - External membership cancels admin and auditor.

**What breaks.**
- A missing or renamed claim gives `[]`, so all users are locked out (if `required_groups` is set) and **every admin, including manually made ones, is demoted** (if `admin_groups` is set).
- A CSV string never matches.
- On CE these keys do nothing.

**Template.**
- Emit `groups`: a top-level array of strings, with the app's own words matching the config, for example `gitlab-users`, `gitlab-admins`, `gitlab-external`. Put it in userinfo or the ID token; either works.
- Renaming is optional, because GitLab's lists can name Syndra's keys directly. A rename has to change in step with `gitlab.rb`.

## Vaultwarden (≥ 1.35.0, upstream SSO)

SSO shipped in 1.35.0 on 2025-12-27. <https://github.com/dani-garcia/vaultwarden/releases/tag/1.35.0>

**Identity.**
- The account link is keyed on `{iss}/{sub}` from the ID token.
- `email` is read from the ID token, falling back to userinfo. Login fails with an error if neither has one.
- `email_verified` is read the same way; signup is blocked when it is false or missing, unless `SSO_ALLOW_UNKNOWN_EMAIL_VERIFICATION` is set.
- `preferred_username` is used as the display name.
- Sources: <https://github.com/dani-garcia/vaultwarden/blob/main/src/sso.rs> (L284–300) and <https://github.com/dani-garcia/vaultwarden/wiki/Enabling-SSO-support-using-OpenId-Connect>

**Zitadel specifics.**
- The ID token `aud` includes the project ID, so set `SSO_AUDIENCE_TRUSTED='^<projectId>$'`.
- Add `offline_access` to get a refresh token. (Same wiki.)

**Groups.**
- **None.** Upstream has no role, group or organization mapping. (Same wiki.)

**What breaks.**
- A missing `email`, or a missing `email_verified` without the override.
- A changed `sub` orphans the account (inferred).

**Template.**
- No custom claims. Make sure `email` and `email_verified` are present.
- Optionally override `preferred_username` with the display name, as the wiki suggests. Whether Zitadel Actions v2 lets a target overwrite `preferred_username` was not checked **(unverified)**.
- Access gating has to happen in Zitadel (project role check), because a claim can't enforce it.

## Kan (kan.bn, open-source Trello alternative)

Confirmed: <https://github.com/kanbn/kan>

**Configuration.**
- Generic OIDC is enabled when `OIDC_CLIENT_ID`, `OIDC_CLIENT_SECRET` and `OIDC_DISCOVERY_URL` are all set. It uses better-auth `genericOAuth` with scopes `openid email profile` and PKCE. <https://github.com/kanbn/kan/blob/main/packages/auth/src/plugins.ts> (L242–291)
- Other relevant settings are `BETTER_AUTH_ALLOWED_DOMAINS` (email-domain allow-list) and `NEXT_PUBLIC_DISABLE_SIGN_UP`. <https://github.com/kanbn/kan/blob/main/.env.example>

**Identity.**
- The name is the first one present of: `name`, then `display_name`, then `preferred_username`, then `given_name`+`family_name`, then `sub`.
- `email` is read directly. `email_verified` defaults to false. The image is `picture` or `avatar`.
- better-auth uses the **ID token only if it contains both `sub` and `email`**; otherwise it reads userinfo instead. The two are never merged. <https://github.com/better-auth/better-auth/blob/main/packages/better-auth/src/plugins/generic-oauth/index.ts> (L133–185)

**Groups.**
- **None.** Kan has no role or group mapping.

**What breaks.**
- An ID token with `sub` but no `email` is fine: Kan falls back to userinfo.
- An ID token with `email` but no `name` gives a `preferred_username` or `sub` display name.

**Template.**
- No custom claims, only the standard `email`, `email_verified`, `name` and `picture`.
- Gate access in Zitadel.

## Home Assistant

**Integrations.**
- The maintained integration with role mapping is `hass-oidc-auth` (domain `auth_oidc`), v1.2.1 released 2026-08-19. <https://github.com/christiaangoossens/hass-oidc-auth>
- `cavefire/hass-openid` exists but has only a `username_field`, with no group mapping. <https://github.com/cavefire/hass-openid>
- Home Assistant core has no built-in OIDC.

**Configuration (YAML).** <https://github.com/christiaangoossens/hass-oidc-auth/blob/main/docs/configuration.md>
- `claims.username`, default `preferred_username`
- `claims.display_name`, default `name`
- `claims.groups`, default `groups`
- `roles.admin`, default `admins`
- `roles.user`: when set, users without either group are **rejected**
- `groups_scope`, default `groups`

**Behaviour.** <https://github.com/christiaangoossens/hass-oidc-auth/blob/main/custom_components/auth_oidc/tools/oidc_client.py> (L599–646)
- Claims are read from the ID token. Userinfo **fills in** `groups`, display name and username when they are missing from it.
- The groups claim must be a **list**. Anything else logs "Groups claim is not a list" and is treated as `[]`.
- The read is top-level only.
- The resulting role ("invalid" is rejected at callback): <https://github.com/christiaangoossens/hass-oidc-auth/blob/main/custom_components/auth_oidc/endpoints/callback.py>
  - holds the admin group → `system-admin`
  - holds the user group, or no user group is configured → `system-users`
  - otherwise → "invalid"
- **The role is applied only when the HA user is first created.** HA core reads `user_meta.group` only inside `async_get_or_create_user` for new credentials, so later promotion or demotion in Syndra does not reach existing HA users. <https://github.com/home-assistant/core/blob/dev/homeassistant/auth/__init__.py> (L310–345)

**What breaks.**
- A CSV string means no admin and, if `roles.user` is set, a lockout.
- Renaming the admin role without changing `roles.admin` means new users silently get no admin.

**Template.**
- Emit `groups`: a top-level array of strings in userinfo or the ID token.
- Rename to HA's defaults, `admins` and `users`, or set `roles.*` to Syndra's words.
- Changes to the admin role must be made by hand in HA.

## Navidrome

**No native OIDC.** The OIDC-plus-roles request #858 was closed as stale on 2025-08-06. <https://github.com/navidrome/navidrome/issues/858> The latest release checked was v0.64.2 (2026-09-24).

**Reverse-proxy header auth.** <https://www.navidrome.org/docs/getting-started/extauth-quickstart>
- Settings are `ExtAuth.UserHeader` (default `Remote-User`) and `ExtAuth.TrustedSources`.
- Users are created automatically, and **the first user becomes admin**.
- No role or admin header exists.
- Subsonic clients (`/rest`) still need a Navidrome password.

**Template.**
- Nothing Navidrome-specific. The proxy (for example oauth2-proxy) needs a stable username (`preferred_username` or `email`).
- It may also need `groups` (top-level array) if the proxy gates on a group. That depends on the proxy and was not researched here **(unverified)**.

## TrueNAS SCALE

- **No OIDC login to the web UI was found in 25.04, 25.10 or the 26 betas.**
  - A feature request from May 2026 says login is local, Active Directory or LDAP only, and has no staff reply. <https://forums.truenas.com/t/native-oidc-openid-connect-azure-entra-id-authentication-for-web-ui-login/66149>
  - The 26.0.0 BETA.1/BETA.2 notes list no SSO item. <https://update-master.freenas.org/scale/TrueNAS-26-BETA/TrueNAS-26.0.0-BETA.1.release-notes.txt>
- Forum users speculate that it may come only through TrueNAS Connect or Enterprise **(unverified)**.
- The TrueNAS 27 notes were not fully read **(unverified)**.
- **Template: none.** The Zitadel OIDC app for TrueNAS cannot be consumed by the TrueNAS UI. Access is better handled by `addons/truenas` through the API. Check what that Zitadel app is actually used for.

---

## What the shaper must support

The ranking counts the 7 apps that can actually consume a claim: NetBird, Supabase, GitLab, Home Assistant, Navidrome-via-proxy, Vaultwarden and Kan.

1. **A top-level array-of-strings roles claim with a configurable key: 4 apps** (NetBird, GitLab, HA, proxy). Syndra already has this. Templates must **pin `array`**, because CSV and space-delimited strings are wrong for every app here.
2. **Choosing which roles an app sees: 4 apps** (NetBird, GitLab, HA, Supabase). This matters most for NetBird, where every value becomes a NetBird group.
3. **Per-app renaming of roles: 3 to 4 apps.**
   - It is a hard requirement for NetBird, where the names are visible and a rename moves membership.
   - It is a convenience for GitLab and HA, whose config can name Syndra's keys instead.
   - For all of them, a rename is a breaking change for that app, so the UI should warn about it.
4. **Choosing which token or response a claim goes in: 3 apps.** NetBird needs the access token; Supabase path B needs the ID token; Supabase path A needs userinfo. The others merge or fall back. A template should say where its claims go, or always write both Pre Access Token and Pre Userinfo.
5. **Always emit the claim, with `[]` when the user has no roles: 1 app strictly (Supabase), but harmless everywhere.** If the claim is left out, Supabase keeps the old groups. GitLab's and NetBird's demotion logic already treat a missing claim and `[]` the same way.
6. **Nested claim paths (`custom_claims.groups`): 1 app** (Supabase, path B only). On path A the nesting is redundant.
7. **Overriding a standard claim (`preferred_username` as display name): 1 app** (Vaultwarden, optional). Whether Zitadel allows it is unverified.
8. **More attributes (given_name, family_name): 0 required.** Zitadel's standard profile claims already cover every app. They are needed only if Supabase app code reads them from `custom_claims`.
9. **Claims that depend on holding a role (`is_admin: true`), bundles, role end dates: 0 apps.** Every app here gates on membership in a group list, never on a boolean, so these can wait.

Several needs can't be met by any claim. They have to be handled by Zitadel's project role check:
- Vaultwarden and Kan have no groups at all.
- Home Assistant only applies roles when a user is first created.
- Supabase's `user_metadata` can be edited by the user, so it can't be trusted for authorization.
