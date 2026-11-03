# Actions v2 claim delivery: integrity assessment

3 Nov 2026. Read-only. Covers `main` at `4c76bca2`, plus one docs commit.

Nothing was edited, committed, deployed, or sent to Zitadel or TrueNAS.

**Scope.** Everything a token passes through on its way to an app with Syndra's claims inside it:

- Zitadel's execution and target configuration
- the network path from Zitadel to Syndra
- Syndra's data plane (`/api/action/inject`)
- the cache that feeds the data plane, and the compiler that writes it
- the event listener that would tell Syndra a sign-in happened
- deploys and restarts
- Zitadel upgrades

**Owner's bar.** Short of the LXC shutting down, claim delivery must not stop working, and a bad setting elsewhere must not make it fail fully or partly *without anyone knowing*. Where the owner asks for it, **prevention comes before detection.**

**Design scale (owner, 3 Nov).** The design is sized for the following, not for today's ~20 people:

- **Thousands of users in total.** The design point is 5,000; it is checked to 10,000.
- **Hundreds active at the same time.**
- **Bursts.** Term start, or a whole class signing in within one minute.
- **A growing app estate.** About 60 apps across about 30 projects, with new apps, shape edits and renames happening often.

It stays on **one LXC**. §4.0a shows the numbers for why that holds.

**Notation.**

- `file:line` refers to this repository.
- `[Z-a]` … `[Z-q]` refer to Zitadel source or docs citations, listed in §0.
- *Silent* means: no error reaches the user, the app, or the operator, and the only evidence is a stdout log line or a claim hidden inside the token.

---

## TL;DR

0. **Actions v2 is almost certainly delivering no Syndra claims to any app in production right now, and has been since it went live.**
   - **What Zitadel sends.** `user_grants[]` elements are `query.UserGrant`, whose tag is `json:"project_id"` ([Z-b]).
   - **What Syndra expects.** It decodes `json:"projectId"` (`backend/internal/handlers/action.go:31`). Go's case-insensitive key matching does not ignore the underscore, so `ProjectID` is always `""`.
   - **The result.** `dedupeNonEmpty` returns zero projects, and the handler answers **HTTP 200 `{"append_claims":[]}`** with the log line `No project grants in request` (`action.go:140-143`). That is the prod log line, and it is not "some apps don't request role scopes": it is *every* token whose `user_grants` is non-empty.
   - **Why nobody noticed.** The hand-written test fixture uses Zitadel's (wrong) camelCase docs (`action_v2_contract_test.go:143-…`). Actions v1 is still active and still puts its claims in the token, so apps keep working.
   - **This is exactly the failure the owner described:** sign-ins succeed and nobody knows.
   - **To confirm without touching anything:** in prod logs, count `[DATA PLANE] Cache hit` (expected 0) against `No project grants in request` (expected about one per v2 call). A claims debug-mode capture of any sign-in will also show `"project_id"` in its stored `Request` body.
   - **The fix** is one tag (`json:"project_id"`), plus a contract test fed by a *recorded* payload (§4h). The day v1 is switched off, this bug becomes an outage of every app's roles, so the fix is a **blocker for removing v1**.

1. **Today, every failure on this path is silent by construction.**
   - Zitadel calls the target with `interruptOnError:false`. Any error, timeout, non-200 status or bad JSON means the token is still issued, just without Syndra's claims.
   - Syndra's own degraded paths answer HTTP 200 with empty `append_claims` and write a log line.
   - Nothing in Syndra counts, stores or shows these outcomes.
   - Nothing compares "people signed in" with "claims were delivered".
   - Nothing reads Zitadel's target and execution configuration back.
   - In short, nothing observes the system from outside.

2. **Two defects break the latency budget the code assumes it has:**
   - **The Redis reads are not bounded to 50ms.** The go-redis client is built without `ContextTimeoutEnabled` (`backend/internal/db/redis.go:20-23`). In go-redis v9.18 that means a context deadline is not applied to socket reads or writes (`baseClient.context`, go-redis `redis.go:888-893`). A Redis that hangs rather than refusing the connection blocks each GET for the default 3s `ReadTimeout`, which is the whole of Zitadel's budget.
   - **The degraded path has no deadline at all.** `claimFailureModeRead` reads Postgres with the request context only (`backend/internal/handlers/action.go:351`).

3. **The most frequent silent failure is self-inflicted: the deploy window.**
   - Every push to `main` runs `docker compose up -d --build` (`.github/workflows/deploy-prod.yml`).
   - Between the old container stopping and the new one listening, every token is issued without claims.
   - The new backend does not listen until migrations, a Redis ping, Zitadel client init, add-on registry init and a blocking add-on refresh (up to 5s) have all finished (`backend/cmd/api/main.go:183-196`, `backend/internal/addons/deps.go:22`).
   - Postgres being down at boot is fatal (`backend/internal/db/postgres.go:34-36`). So is Redis (`backend/internal/db/redis.go:25-28`).

4. **The token path depends on more than it needs to.** Each token can touch Redis up to three times and Postgres up to twice (shapes, then failure mode). Facts are compiled from Postgres and the directory, and their TTL (24h) is only twice the warm interval (12h). A warm skipped twice in a row, which it is whenever the directory is "not current" (`backend/internal/cache/compiler.go:371-374`, and it returns `nil`), expires everyone's facts. Everyone then gets `fail_closed`, silently.

5. **At thousands of users, three write-side costs grow faster than the user count.** None of them is visible at 20 people.
   - **Every cache refresh SCANs the entire Redis keyspace.** `redisScanKeys` with `MATCH mapping:{u}:*` (`cache/deps.go:75-82`, called at `compiler.go:304-305`) makes the warm O(users × keys).
   - **Every compile re-reads all global tables.** That means all rules, all bundles and all service accounts, once per person (`compiler.go:75`, `deps.go:42, 111`).
   - **The directory's user list is rebuilt every 30s.** Each rebuild does a **per-user metadata call** to Zitadel (`directory/zitadel.go:27, 195-224`). A warm that runs for minutes therefore drives tens of thousands of calls into the same Zitadel that is issuing everyone's tokens.

   There is also a structural race that term start turns into a mass event. A member's **first** token is issued before the asynchronous onboarding has given them anything (§2.7 SC4).

6. **Recommended shape. Prevention first, designed for the scale above.**
   - **(P0, first) Fix the `project_id` tag (F-0)**, so that v2 delivers anything at all.
   - **(P1) A self-sufficient hot path, in two tiers.**
     - All *config* (shapes and failure modes for every project and app) lives in an in-memory, versioned snapshot. It is reloaded when a Redis generation counter moves.
     - *Facts* are one Redis `HGET` per token from a hash per person, using real (fixed) timeouts.
     - Behind that sits an in-process **last-known-good** LRU, so a Redis stall still answers for everyone recently served.
     - No Postgres, no Zitadel, and nothing that can block for longer than 50ms.
   - **(P2) A dedicated listener** with its own timeouts and panic guard. Caddy retries on dial failure (`lb_try_duration`).
   - **(P3) A signing keyring read from Zitadel.** Zitadel rotates with no grace period and signs with one key.
   - **(P4) A separate small `syndra-claims` container.** At this scale it is **recommended, not optional**. Each deploy today costs tens to hundreds of claim-less tokens, deploys will be frequent while apps are being built, and admin-side sweeps over thousands of users would otherwise share the data plane's CPU, memory and crash domain.

   Then detection:
   - white-box outcome telemetry
   - a sign-in ↔ delivery correlator, which is the key silent-failure detector
   - a read-only reconciler of Zitadel's configuration
   - a canary probe that obtains a real token from Zitadel
   - an external dead-man's-switch ping, so "Syndra itself is down" is also loud

---

## 0. Zitadel behaviour this relies on (with citations)

Zitadel v3.4.15, v4.20.1 and `main` (d9d933f, 2 Nov 2026) were read at source. Production runs v4.15.2 or later: DEPLOY.md:233-246 records the v4.15.2 deny-list change, so the v4 lines below apply.

| Ref | Behaviour | Source |
|---|---|---|
| **Z-a** | **The function body is `json.Marshal(oidc.ContextInfo)`.** Keys: `function`, `userinfo`, `user`, `user_metadata`, `org`, `user_grants`, `application` (v4.0+), `actor` (v4.17+), `response`. | `internal/api/oidc/userinfo.go` L556-570 (v4.20.1) |
| **Z-b** | **`user_grants[]` is `[]query.UserGrant`, whose JSON tags are snake_case**: `project_id`, `project_name`, `grant_id`, `user_id`, `roles` (`omitempty`), `state`, and so on. Same in v3.4.15. Zitadel's own docs show camelCase (`projectId`) and are **wrong**; a real payload in issue #12524 confirms snake_case. Each element is one per *grant* (roles is an array inside it), not one per role. | `internal/query/user_grant.go` L24-60 (v4.20.1 `json:"project_id,omitempty"`); docs `apps/docs/content/guides/integrate/actions/usage.mdx` L123-140; github.com/zitadel/zitadel/issues/12524 |
| **Z-c** | `user.id` is `json:"id"`. `application.client_id` is `json:"client_id"`, since v4.0.0 (commit 416a35537, #10339). `function` is `"function/preaccesstoken"` or `"function/preuserinfo"`. | `internal/query/user.go` L29-42; `userinfo.go` L568-570; `internal/domain/action.go` L64-80 |
| **Z-d** | **`user_grants` is filtered to the role audience**, and the query keeps `project_id = any($3) AND state = 1`. `$3` is the project ids from `urn:zitadel:iam:org:project:id:{id}:aud` scopes, plus the client's own project **only if** the project's *assert roles on authentication* is on, **or** a `urn:zitadel:iam:org:project:role:*` scope was requested, **or** the audience is already non-empty. Otherwise `user_grants` is empty, and with `omitempty` it is **absent**. The per-app access-token and ID-token role-assertion flags do not change `user_grants`. Client-credentials and JWT-profile grants pass `projectID=""`, so a machine user's grants appear only with an `:aud` scope. | `internal/query/userinfo_by_id.sql` L35-47; `userinfo.go` `prepareRoles` L140-163, `assertRoles` L212-220; `token_client_credentials.go` L56; `token_jwt_profile.go` L66 |
| **Z-e** | **`preaccesstoken` runs only inside `createJWT`**, so only for apps with JWT access tokens. A bearer (opaque) app never triggers it at issuance. **`preuserinfo`** runs on ID-token creation (whenever `openid` is in the scope), on the userinfo endpoint, and on **introspection**. On introspection `client_id` is the *introspecting* client and the grants are that client's project. `idTokenUserinfoAssertion` does not gate the call. | `internal/api/oidc/token.go` L43-55, L79, L123; `userinfo.go` L63; `introspect.go` L99-107 |
| **Z-f** | Functions run on authorization code, implicit, device code, **refresh_token (every refresh)**, client credentials and JWT profile. Token exchange calls them only for JWT or ID-token outputs. Legacy v1 scripts run **before** v2 in the same function. | `token_code.go` L52; `token_refresh.go` L31, L84; `token_exchange.go` L436-463, L509; `userinfo.go` L292-306 (v4.20.1) |
| **Z-g** | **Merge rules.** A key with prefix `urn:zitadel:iam` is dropped **silently** (no log). A key already present in `Claims` (set by v1, or by an earlier entry) is dropped, and `key "x" already exists` goes into `urn:zitadel:iam:action:function/<fn>:log`. Registered claims (sub, aud, iss, exp, iat, nbf, jti, client_id; name and email for userinfo) live in struct fields, so an appended one is **silently ineffective**. | `userinfo.go` `appendOrLogClaim` L455-469 (v4.20.1), `ClaimPrefix` in `client.go` L23; zitadel/oidc `pkg/oidc/util.go` L9-12 |
| **Z-h** | **Call outcomes.** Any status outside 200-299, transport error, timeout, deny-list block or signing error is an execution error. With interrupt off it is swallowed and the loop continues. A 2xx with an empty body is skipped. A 2xx with invalid JSON gives `ResponseIsNotValidJSON`, also swallowed. Unknown response fields are ignored. A response containing `forwardedStatusCode` or `forwardedErrorMessage` is treated as an error. Redirects are followed (v4.15.2+: at most 5, each re-checked against the deny list). TLS is verified. Body cap is 20MiB (v4.15.2+). | `internal/execution/execution.go` L40-72, L235, L246-283 (v4.20.1); `userinfo.go` L613-615; `cmd/defaults.yaml` L1112-1138; `internal/api/http/client.go` L35-96 |
| **Z-i** | **Deny list.** v4.15.2+ checks `HTTPClient.DenyList` **at dial time, by resolved IP**, and again on target create and update. Defaults deny all RFC1918, loopback and link-local ranges. A blocked call fails with `address is denied by …`. | `cmd/defaults.yaml` L1124-1138; `internal/command/action_v2_target.go` L44; issue #12326 |
| **Z-j** | **Several targets on one function** run in `position` order. Each gets the same payload, plus the previous restCall's response as `response`. All responses unmarshal into **one shared** `ContextInfoResponse`. A later restCall that returns an `append_claims` array replaces the earlier one, so `[]` would erase it (inferred from `encoding/json` slice semantics, not run). Webhook and async targets cannot change claims. `includes` is not resolved at runtime. | `internal/query/instance_by_id.sql` L47; `internal/execution/target/router.go` L15-31; `userinfo.go` L613-621; `internal/api/grpc/action/v2/execution.go` L16-22 |
| **Z-k** | **Target config.** Types are restWebhook (response ignored), restCall, and restAsync (fire-and-forget). The timeout wraps the whole call including the body read. Dial timeout 5s; global `HTTPClient.Timeout` 30s (v4.15.2+). `payload_type` JSON/JWT/JWE exists since **v4.8.0**. With JWT the body is a compact JWS string but the header still says `Content-Type: application/json`, so a JSON decoder fails. The response stays JSON. | `proto/zitadel/action/v2/target.proto` L93-119; `execution.go` L134-148, L235 |
| **Z-l** | **Signing.** Header `ZITADEL-Signature: t=<unix>,v1=<hex(HMAC-SHA256(key, "<t>.<body>"))>`, matching Syndra. The 300s tolerance is a *receiver-side* helper; the sender enforces nothing. **Only one `v1=` is ever sent.** `expiration_signing_key` accepts only `0s` (immediate swap) despite the proto promising a grace period. **`GetTarget` and `ListTargets` return the signing key decrypted** to holders of `action.target.read`. | `pkg/actions/signing.go` L22-45; `execution.go` L248; `internal/command/action_v2_target.go` L151-158, L228-230; `internal/api/grpc/action/v2/query.go` L106; `action_service.proto` L800-808 |
| **Z-m** | **Config caching.** Targets and executions are read with the instance (`instance_by_id.sql`). The default `Caches.Instance.Connector: ""` disables caching, so a change applies after projection lag (not measured). | `internal/query/instance_by_id.sql` L20-47; `cmd/defaults.yaml` L425-430; `internal/query/instance.go` L657-661 |
| **Z-n** | **Observability inside Zitadel.** Only an `Error` log `"error calling target"` (v4.8+; none in v3). Invalid-JSON 2xx responses are logged nowhere. **No event, metric or API exposes call outcomes.** Config events exist (`target.*`, `execution.*`). | `execution.go` L60, L111; `internal/repository/target/target.go` L13-16; `internal/repository/execution/execution.go` L12-15 |
| **Z-o** | **Read APIs.** `zitadel.action.v2.ActionService` (v4.0+): `GET /v2/actions/targets/{id}`, `POST /v2/actions/targets/search`, `POST /v2/actions/executions/search`. Needs `action.target.read` and `action.execution.read`, granted by `IAM_OWNER_VIEWER` (read-only) or `IAM_OWNER`. v2beta (`…/_search`) is deprecated. | `proto/zitadel/action/v2/action_service.proto` L236-284, L521-529; `cmd/defaults.yaml` L1706-1710, L2272-2276 |
| **Z-p** | **Events to correlate.** `oidc_session.added` carries `userID`, `clientID`, `scope`, `audience`, `authTime` and so on (no project or app id). It is pushed **once per initial grant**, together with `oidc_session.access_token.added`. A **refresh** pushes `access_token.added` plus `refresh_token.renewed`, with no new `added`. **`user.token.added` is no longer emitted** (no caller outside tests in v3.4.15, v4.20.1 or main), so Syndra's binding to it is dead. Event jobs are queued in the event's own DB transaction. | `internal/repository/oidcsession/oidc_session.go` L13-40; `internal/command/oidc_session.go` L140-223; `internal/repository/user/user.go` L21; `internal/eventstore/v3/push.go` L120-130 |
| **Z-q** | **Open issues that bear on this.** #11388: `preuserinfo` claims missing from opaque or PAT introspection (v4.9-4.17). #12974: an interrupting `preaccesstoken` failing on refresh *consumes the refresh token* (one more reason to keep interrupt off). #12939: `UpdateTarget` cannot flip `interruptOnError` from true to false on a never-changed target. #12976: target 5xx becomes HTTP 400 at the token endpoint. | github.com/zitadel/zitadel/issues/11388, 12974, 12939, 12976 |

`github.com/zitadel/zitadel/blob/<tag>/` prefixes every path. Line numbers are v4.20.1 unless stated. `execution.go` is byte-identical between v4.20.1 and main.

---

## 1. End-to-end path, with a timing budget per hop

The path below is for the access-token case. The userinfo and ID-token case is the same path with `function/preuserinfo`; see §0 for which grant types and settings call which function.

| # | Hop | What happens | Budget (p99 target) | Today |
|---|---|---|---|---|
| 1 | App → Zitadel `/oauth/v2/token` | Code exchange, refresh, or (for machine users) client credentials or JWT profile. Zitadel builds the token claims, including v1 *Complement Token* actions, which run first. | n/a | n/a |
| 2 | Zitadel: execution lookup | Finds the executions for `function/preaccesstoken`. A cached or eventual-consistency read may be involved, see §0. | ~0 | n/a |
| 3 | Zitadel: payload build | `user`, `user_grants` (the token's role audience only), `org`, `user_metadata`, `userinfo`, `application.client_id` (v4+). Signed: `ZITADEL-Signature: t=…,v1=HMAC(t.body)`. | ~0 | n/a |
| 4 | Zitadel protected HTTP client | v4.15.2+ checks the resolved IP at dial time against `HTTPClient.DenyList` ([Z-i]; DEPLOY.md:233-270). | ~0 | One config value away from silently blocking every call |
| 5 | DNS | `SYNDRA_EXTERNAL_URL` host is resolved **from the Zitadel host's resolver**. | ≤ 50ms (cached) | Not checked from Zitadel's side |
| 6 | TLS to Caddy (<caddy-host>) | ACME certificate. Connection reuse is likely, so full handshakes are rare. | ≤ 50ms | Expiry not watched |
| 7 | Caddy → LXC <app-lxc>:8080 | `handle /api/action/*` → `reverse_proxy <APP_HOST>:8080` (DEPLOY.md:295-306). No retry is configured. | ≤ 2ms | A dial failure becomes a 502 |
| 8 | Docker port publish → `syndra_backend:8080` | Docker NAT or proxy. | ≤ 1ms | Container gap during each deploy |
| 9 | Go `net/http` | `ReadHeaderTimeout 10s`, `IdleTimeout 120s`, **no ReadTimeout or WriteTimeout** (main.go:233-239). Middleware, outermost first: `withPanicGuard` → `withMaxBody(1MiB)` → `withSecurityHeaders` → mux → `withCORS` → `withCancelGate` → `withZitadelActionSignature` (router.go:418-424). | ≤ 1ms | Shared with the whole admin API |
| 10 | Signature check | Reads the body, accepts if any `v1=` matches the **one** configured secret, requires \|t − now\| ≤ 300s (zitadel_action_auth.go:53-143). | ≤ 0.1ms | A failure is a 401 and a log line |
| 11 | Decode | Lenient `encoding/json` into `ActionV2Request`; requires `user.id` (action.go:104-117). | ≤ 0.1ms | A type change in *any* declared field is a 400 |
| 12 | Project selection | Dedupe `user_grants[].projectId`. 0 projects means `[]` (log). More than 1 means refused, `[]` (log) (action.go:133-153). | ~0 | Absent and empty are indistinguishable |
| 13 | Shape read | Redis `GET claim_shapes:{p}` (ctx 50ms, but see F-1). On a miss or any error: Postgres `ResolveProjectShapes` (500ms), then Redis SET with TTL 300s (action.go:282-310). | Hit ≤ 1ms. Miss ≤ 10ms. | **Worst case about 3.5s** |
| 14 | `LandsIn(function)` | An unknown function string answers `true` (claims.go:208-217). | ~0 | n/a |
| 15 | Facts read | Redis `GET mapping:{u}:{p}` (ctx 50ms, really up to 3s) (action.go:180-193). | ≤ 1ms | **Up to 3s** |
| 16 | Degraded branch | Redis `GET claim_mode:{p}`, then **Postgres with no deadline**, then Redis SET (action.go:336-370). | ≤ 10ms | **Unbounded** (cut only when Zitadel hangs up) |
| 17 | Shape, envelope, write | `claims.Shape`, then `{"append_claims":[…]}` with status 200. | ≤ 1ms | n/a |
| 18 | Zitadel merge | v1 scripts ran first. Each key is appended unless it is already set (dropped, noted in the `…:log` claim), reserved (`urn:zitadel:iam*`, dropped silently) or a registered claim (overwritten silently) ([Z-f], [Z-g]). | ~0 | Seen only inside the token |
| 19 | Token issued | The app gets the token, with or without the claims. | n/a | **Nobody is told** |

**What the budget should be.** Zitadel's hard limit is 3s, and that includes connecting and TLS.

- Syndra's server-side answer should be **p99 ≤ 25ms, with a hard ceiling of 250ms**.
- That leaves more than 2.5s for network, a Caddy dial retry, and Zitadel's own overhead.
- At about 20 people this costs nothing, provided hops 13 to 16 become in-memory reads (§4, P1).
- Today's real worst case is roughly 3s + 3s + 3s + unbounded. It goes over the limit on the first stalled Redis call.

---

## 2. Failure-mode table (FMEA)

Each row gives two cases (*silent-full* = no Syndra claims at all; *silent-partial* = wrong or reduced claims), then:

- **Det. today**: is it detected today, and how
- **Blast**: who is affected
- **L**: likelihood, H, M or L, for this deployment
- **Correct system**: what a correct system would do

Severity is implied: anything silent and wide is critical.

### 2.1 Zitadel configuration drift

| ID | Failure | Mode | Det. today | Blast | L | Correct system |
|---|---|---|---|---|---|---|
| Z1 | Target endpoint changed or wrong (this happened: the dev box posted into a dead IP for months, per the deploy-prod.yml comment) | silent-full | Only at deploy time: an unsigned POST to Syndra's *own idea* of the URL must return 401. Zitadel's configured endpoint is never read. | Every app, every token | M | Reconciler reads `GetTarget` and compares `endpoint` with `SYNDRA_EXTERNAL_URL + /api/action/inject`. The correlator (§4b) catches it within minutes anyway. |
| Z2 | Target type changed (restCall → restWebhook or restAsync, where the response is ignored) | silent-full | No | Every token | L | Reconciler: type must be `restCall`. |
| Z3 | Target timeout lowered (for example to 500ms) | silent-partial (tail) | No | Slow calls | L | Reconciler compares with expected. Telemetry compares p99 with the configured timeout. |
| Z4 | `interruptOnError` flipped to true | Loud outage: a Syndra fault now blocks sign-in | Users notice | Every sign-in, while Syndra is degraded | L | Reconciler flags any deviation. The owner's policy stays false, made safe by P1 and detection. |
| Z5 | Execution unbound (`register.sh --remove`, a console click, or an operator "cleaning up") | silent-full | No | Every token | M | Reconciler: both function conditions must list exactly `[syndra-claim-injector]`. Correlator. |
| Z6 | Execution's target list *replaced*. `SetExecution` sets the whole list (see register.sh:226-240 `PUT /executions`, §0). | silent-full | No | Every token | L-M | Same as Z5. |
| Z7 | A second target added to the same function (another integration) | silent-partial **or full**: responses unmarshal into one shared struct, so a later restCall returning `append_claims` replaces Syndra's list ([Z-j]) | No | Apps whose keys collide | L | Reconciler requires an exact list. Probe checks the dropped-claim log. |
| Z8 | Target deleted and recreated (new id and new signing key) | silent-full (401) | stdout `[ACTION] Signature verification failed` | Every token | L | 401 counter plus alert. The reconciler records the target id and `changeDate`. |
| Z9 | Signing key rotated. Zitadel only supports an immediate swap and sends a single `v1=` ([Z-l]), so `rotate.sh`'s `"0s"` (rotate.sh:150-155) is the only option, and the env and restart always lag behind it | silent-full (401) for the whole gap | stdout only | Every token | **M** (every rotation) | Dual-key acceptance (`…_SIGNING_KEY_NEXT`), see §4 P3. A 401 is a page, not a log line. |
| Z10 | Target `payloadType` changed to JWT or JWE (v4.8+; the body becomes a JWS string while the header still says JSON, [Z-k]) | silent-full (400: the body is not JSON) | stdout only | Every token | L | Reconciler requires `PAYLOAD_TYPE_JSON`. Decode-failure counter. |
| Z11 | A v1 Complement Token action is active (it is today, the owner removes v1 himself) or re-added, and sets a key Syndra also sets | silent-partial: Syndra's value dropped | Only the `urn:zitadel:iam:action:function/…:log` claim inside the token | Apps whose keys collide | **H while v1 lives** | Reconciler reads v1 flows and triggers (Management API) and compares the keys v1 scripts set with each shape's top-level keys. The canary probe asserts there is no log claim. |
| Z12 | A Zitadel upgrade renames or splits a function, or changes the `function` string format | silent-full or partial | No | Every token | L | Payload fingerprinting (§4a) and the canary probe. |
| Z13 | Zitadel `HTTPClient.DenyList` reverted on upgrade or config rewrite, or DNS for the Syndra host moves into a denied range | silent-full (call refused inside Zitadel) | No | Every token | M (upgrade-coupled) | Only something that *goes through Zitadel* sees this: the correlator and the canary probe. |
| Z14 | An instance feature or flag disables function executions (v3 era; *unverified*, and no such gate was found in v4.20.1) | silent-full | No | Every token | L | The correlator covers it whatever the cause. |
| Z15 | Execution or target config applied after projection lag (instance cache is off by default, [Z-m]), so a fix appears not to work for N seconds | Confusing, not silent | n/a | n/a | L | Reconciler and probe report what Zitadel is actually *doing*, not only what was configured. |

### 2.2 App and project settings in Zitadel

| ID | Failure | Mode | Det. today | Blast | L | Correct system |
|---|---|---|---|---|---|---|
| A1 | App access token type is Bearer, so `preaccesstoken` is never called ([Z-e]); an API that introspects gets `preuserinfo` under *its own* client id, and issue #11388 reports appended claims missing from introspection. Combined with a shape that `lands: access_token`, the app receives nothing at all. | silent-full for that app | Explain check 1, **only when someone opens that app's claims** | One app | **M** | Reconciler runs explain check 1 (`services/claim_explain.go`) for **every** app on a schedule and raises a finding on a mismatch. |
| A2 | "User Info inside ID Token" off, and the app reads the ID token | silent-full for that app | Same as A1 | One app | M | Same. |
| A3 | The project's *assert roles on authentication* is off **and** the app requests no `…:project:role:*` scope, so `user_grants` is empty or absent ([Z-d]). The prod "No project grants" line is today dominated by F-0, so A3's real share is unknown until F-0 is fixed. | silent-full for that app | stdout line per token | One app | **H (already seen)** | Outcome class `no_grants`, counted per client. Finding: "app X signs people in but its tokens name no project". |
| A4 | Project "role check" on: a person without a role cannot sign in | Loud | User | n/a | n/a | n/a |
| A5 | App recreated, giving it a new `client_id`. `ByClient` misses (claim_shape.go:50-55), the project default is used, and the claim key may change. | silent-partial | Logged **once per process lifetime** (action.go:246-268) | One app | M | Reconciler: every shape's `client_id` must be a live app. Telemetry outcome `default_shape_for_unknown_client`, which is not a one-off log line. |
| A6 | A token spans two projects (an extra `:aud` scope) | silent-full (refused by design, ADR-002) | stdout | One app | L-M | Outcome class `refused_cross_project`, per client, with a finding. |
| A7 | Project deleted and recreated (new project id) | silent-full | No | Its apps | L | Reconciler: every shape, rule and failure-mode row names a live project. |

### 2.3 Payload contract across Zitadel upgrades

| ID | Failure | Mode | Det. today | Blast | L | Correct system |
|---|---|---|---|---|---|---|
| P1 | `user_grants` key mismatch. **Happening now:** Zitadel sends `project_id` and Syndra reads `projectId` (F-0, [Z-b]). Any future rename does the same. | silent-full: every token goes "no grants" | Indistinguishable from A3 | Every token | **Certain today**; L-M per upgrade after the fix | Decode `user_grants` as presence-aware (`*[]` or `json.RawMessage`). An absent field is a **contract fault**, a separate outcome class and a page. Recorded per-version fixtures (§4h). |
| P2 | `application.client_id` absent (Zitadel before v4) or renamed | silent-partial: per-app shapes ignored, project defaults used | Logged once | Every app with its own shape | L | Outcome dimension `client_id_present`. Finding if any call lacks it while the version expects it. |
| P3 | `user.id` moves | silent-full (400) | stdout | Every token | L | Counter `bad_request`, which is a page. |
| P4 | A field Syndra declares but **does not need** changes type (for example `user_grants[].roles` becomes objects, `application` becomes an array, `function` becomes an object) | silent-full (decode 400) | stdout | Every token | L | Decode only what the answer needs. Make `Roles` a `json.RawMessage`, used only by debug capture (action.go:30-33). Fuzz the decoder. |
| P5 | The `function` value format changes | silent-partial **or a leak**: `LandsIn` defaults to `true` (claims.go:214-215), so an access-token-only shape lands in userinfo | No | Shapes that are not `both` | L | An unknown function is its own outcome class. Answer as `both`, but count it and raise a finding. |
| P6 | Response contract changes (`append_claims` renamed, value types restricted) | silent-full | No | Every token | L | Only a real token shows this, so the canary probe. |
| P7 | Body over Syndra's 1MiB cap (large `user_metadata`); Zitadel allows up to 20MiB ([Z-h]) | silent-full for that person (400) | stdout | One person | L | Outcome class. Raise the cap on this route to 4MiB. A cap is still needed, because the HMAC is checked only after the body is read. |
| P8 | Payload gains fields | Harmless (the decoder is lenient, contracts.go:30-42) | n/a | n/a | n/a | Keep lenient. **Never** use `decodeJSONStrict` here. Fingerprint so the change is *noticed*. |

### 2.4 Syndra internal

| ID | Failure | Mode | Det. today | Blast | L | Correct system |
|---|---|---|---|---|---|---|
| S1 | Clock skew between the Zitadel host and the LXC above 300s | silent-full (401 stale) | stdout | Every token | L | Outcome `sig_stale` carrying the measured skew. Alert at more than 60s skew, before 300s. |
| S2 | Redis refusing connections (container down) | silent-full or partial, depending on failure mode | stdout per token | Every token | L-M | P1 snapshot: keep serving last-known-good. Staleness SLI. |
| **S3** | **Redis hung** (stopped process, AOF fsync stall on a full disk, half-open TCP) | **silent-full: each GET blocks about 3s** | No | Every token | L-M | **F-1**: `ContextTimeoutEnabled:true` with short timeouts. Better, P1: no Redis on the request path. |
| S4 | Redis data loss (volume wiped, FLUSHALL, AOF corrupt so Redis will not start, and then the backend will not start either) | silent-full until a warm | No | Everyone | L | The boot warm already exists. Snapshot LKG. Readiness says "facts: 0 entries" loudly. |
| S5 | Facts expire: TTL 24h vs warm 12h (compiler.go:20, main.go:249). A warm is **skipped with nil** when the directory is not current (compiler.go:371-374); a per-user warm failure keeps the entry but **not its TTL** (compiler.go:319-321, 384-386). | silent-full per person after 24h | One log line per pass | Everyone if the directory is flaky for 24h | M | Facts carry `compiled_at`. Staleness SLI: alert when any served facts are older than about 13h. A skipped warm is a finding. Do not let expiry be the failure mode (see §3 F-6). |
| S6 | Person never compiled (new person, or a grant made only in the Zitadel console) | silent-full or partial | stdout "Cache miss" | That person | M | A miss is a *repair signal*: enqueue `RefreshUser` off the hot path, without blocking. Outcome class `no_facts`. |
| S7 | Compile fails for one person (bad row, directory 404) | silent-full for that person: a change-driven refresh **drops** keys (compiler.go:310-318) | stdout | One person | M (one prod user 404s today) | Classify a 404 as `gone` and stop retrying it, as an orphan finding. Other failures are counted and listed per person. |
| S8 | Postgres slow, locked, or pool exhausted. pgxpool defaults to `MaxConns = max(4, NumCPU)` (postgres.go:24-29 sets nothing), shared with the drain, reconcile, observe and warm jobs. | silent-full on shape-cache miss (500ms, then degraded) and on the degraded path (**unbounded**) | No | Every token that misses the cache | M | P1: no Postgres per token. Until then, a whole-request deadline and a bounded failure-mode read (F-2). |
| S9 | Migrations at startup are slow or failing, so the listener never comes up | silent-full | The deploy job fails, *after* the old container is gone | Everyone | L-M | Listen before migrating; serve the data plane from the snapshot. P4 split. |
| S10 | Shape cache-aside race: a read that started before a save writes the **old** shape back after the save's DEL (action.go:294-308 vs claim_shape.go:204-207) | silent-partial for ≤ 300s | No | One project's apps | L | Write-through on save, plus a generation counter (P1). |
| S11 | `claim_mode:{p}` is never invalidated when it is edited (deps.go:318-323; no DEL anywhere). After 5 min of a Postgres outage, `minimal_safe` collapses to `fail_closed`. | silent-partial | No | Projects using minimal_safe | L | Write-through, no TTL (P1). |
| S12 | Panic in the handler | silent-full for that call (500) | withPanicGuard log | One call | L | Outcome `panic`, which is a page. |
| S13 | **Panic in the `claimdebug` worker goroutine**: `explain` runs Zitadel and DB reads with no `recover` (claimdebug.go:196-228). | **Process crash, restart window, silent-full for everyone** | Container restarts | Everyone | L | `defer recover()` in `work`/`add`. This is the general case for isolation (P2 and P4). |
| S14 | A panic in any other unguarded goroutine (for example addons.RefreshAll, registry.go:442) | Process crash | Restart | Everyone | L | Same. |
| S15 | Startup fatal on Postgres or Redis down, or on add-on registry init (main.go:183-193, postgres.go:30-36, redis.go:25-28) | silent-full: crash loop | Container restarting | Everyone | L | The data plane starts with Redis or the snapshot only. Postgres down means the admin side is degraded and loud, not "no claims". |
| S16 | One-time logs: an unmatched client is logged once per process (action.go:246-268), so a persistent misconfiguration is silent after its first line | silent-partial | Once | n/a | n/a | Counters, not one-shot logs. |
| S17 | Log volume: a "Cache hit" line on every token (action.go:207), and the Docker json-file driver has no rotation configured, so the disk fills and Redis AOF and Postgres fail | Eventually silent-full | No | Everyone | L | Drop per-hit logs and keep counters. Set compose `logging: max-size`. |
| S18 | `withCORS` and `withCancelGate` wrap the data plane (router.go:421-422). Both are unnecessary there; the cancel gate wraps the ResponseWriter. | Latent coupling | n/a | n/a | L | Own mux with only what the route needs (P2). |
| S19 | Dev-mode passthrough when `ZITADEL_DOMAIN` is empty: unsigned requests are accepted | Security, not silence | The deploy check expects 401 | n/a | L | Keep the deploy check. Have the startup self-probe assert a 401 too. |

### 2.5 Network and deploy

| ID | Failure | Mode | Det. today | Blast | L | Correct system |
|---|---|---|---|---|---|---|
| N1 | **Deploy window.** Every push to main stops the old backend (≤ 10s drain) and starts a new one that listens only after migrations, Zitadel init, add-on init and add-on refresh (≤ 5s) (main.go:181-196) | silent-full for N seconds per deploy | No | Anyone signing in or refreshing during a deploy | **H (every merge)** | Caddy `lb_try_duration` retries dial failures within the budget. Listen first. P4 split, so a backend deploy does not restart the data plane. |
| N2 | Caddy certificate expiry (ACME renewal fails) | silent-full | No | Everyone | L | L1 probe goes through the public name and reports days to expiry. |
| N3 | Caddy route edited, or Caddy down | silent-full | Only at deploy time | Everyone | L | L1 probe every minute. |
| N4 | DNS for the Syndra host changes or splits (Zitadel's resolver ≠ the LXC's) | silent-full | No | Everyone | L | Only the L2 canary or the correlator sees *Zitadel's* resolution. |
| N5 | LXC IP or firewall change | silent-full | No | Everyone | L | Same. |
| N6 | Zitadel host clock drifts | Same as S1 | | | | |
| N7 | Event listener broken (restAsync target or its executions). Also: the `user.token.added` binding is dead ([Z-p], F-26), so `oidc_session.added` is the only sign-in signal | The *detector* goes blind: sign-ins stop arriving | No | Detection | M | Inverse correlator: v2 calls arriving with no sign-in events for 24h is a finding. The reconciler covers the event executions too. |

### 2.6 Data

| ID | Failure | Mode | Det. today | Blast | L | Correct system |
|---|---|---|---|---|---|---|
| D1 | Person deleted in Zitadel but still holding Syndra records (prod has one: Zitadel 404 on every warm) | Noise that **masks** real failures: "Warmed N−1 of N" looks the same every day | One log line | n/a | H | 404 means "gone", raised once as an orphan finding and excluded from the warm-health ratio. |
| D2 | A role a shape maps (rename or include list) is deleted from the project | silent-partial: `Words` drops it | No | One app | L | Reconciler: a shape's role keys must be a subset of the project's roles (the directory already has them). |
| D3 | Zitadel grant made by hand (not in Syndra's ledger). Facts compile from `direct_role_grants`, bundles and rules (compiler.go:31-45), so Zitadel's own roles claim has the role and Syndra's does not. | silent-partial (by design: intent, not observation) | A drift finding, via webhook (webhook.go:299, 306-334) | One person and one role | M | Fine as designed. Add invariant I3 (below), so a disagreement *at serve time* is counted. |
| D4 | Role end date passes between compiles | Correct: the expiry sweep refreshes (expiry/sweep.go:83) | n/a | n/a | n/a | Belt and braces: drop roles whose `RoleEnds` date is in the past at serve time. Cheap, since it is already in the facts. |

### 2.7 Scale (5,000 to 10,000 users, hundreds active, about 60 apps)

| ID | Failure | Mode | Det. today | Blast | L at scale | Correct system |
|---|---|---|---|---|---|---|
| SC1 | **Refresh SCANs the whole keyspace.** `refreshUser` → `redisScanKeys("mapping:{u}:*")` walks *every* key, 100 per round trip (`cache/deps.go:75-82`). At 15,000 to 30,000 keys that is 150 to 300 round trips per person. A warm of 5,000 people is about 1M SCAN calls, and the same cost is paid on every grant event. | Slow warm and slow event handling. Redis is single-threaded, so the data plane's GETs queue behind the SCANs. A warm that outlasts its 12h interval quietly stops keeping facts fresh. | No | Everyone (tail latency, freshness) | **H** | One Redis **hash per person** `facts:{u}` with one field per project. Refresh is `HGETALL` + `MULTI{HSET…; HDEL…}`, so it is atomic and O(own projects). The data plane does `HGET facts:{u} {p}`. The keyspace becomes O(users). |
| SC2 | **Each compile re-reads global tables**: all active rules (`compiler.go:75`), all bundles (`deps.go:111`), all service accounts (`deps.go:42`), once per person | A warm does 3 × 5,000 full-table reads. The pgx pool (4 conns by default) is saturated for minutes. | No | Admin-side latency, the webhook backlog, and pre-P1 the data plane's shape-miss path | H | Load the global tables **once per pass** into a compile context, and use per-person queries only for grants and bundle assignments. Size the pool explicitly (`pool_max_conns=20`). |
| SC3 | **Directory rebuild storm.** `Users()` caches for 30s (`directory/zitadel.go:27`). Each rebuild pages all users **and fetches each person's metadata** (`:195-224`). A warm that runs for minutes rebuilds every 30s: at 5,000 users, ~5,000 metadata calls each time, so tens of thousands of IdP calls per warm. `FindUser` on a miss also calls `GetUser` for each one (`:226-260`), and a deleted user (prod's 404) hits Zitadel on every pass. | **Zitadel slowed by Syndra's own warm.** Every token issued then waits longer, and with `interruptOnError:false` a slow Zitadel → Syndra round trip is still silent. Directory "not current" also *skips* the warm (F-6). | No | Everyone | **H** | A warm takes **one** directory snapshot at the start of its pass and passes it in. Metadata overlay moves to its own longer TTL, or is refreshed on `user.metadata.*` events. A 404 is `ok=false` ("gone"), never retried within a pass. Warm becomes incremental: only people whose `compiled_at` is older than 11h, spread over the interval. |
| SC4 | **First sign-in races onboarding.** Google JIT-creates the person during their first login. `user.human.added` reaches Syndra asynchronously, then the welcome bundle, the outbox, and finally the Zitadel grants. The token issued in that same login has **empty `user_grants`**, so `no_grants`, so no claims. The next token comes only at refresh, which can be hours later (the access-token lifetime). | silent-full for **every new member's first session**. At term start that is hundreds of people at once. | Logged as "No project grants" | Every new member | **H at term start** | Measure first: outcome class `first_sign_in_before_access`, where the person's directory creation is under 10 min old, with an SLI for onboarding latency (created → first served token). **Owner's options:** (a) keep it, and have apps re-read userinfo or shorten the access-token lifetime for the welcome app; (b) **welcome facts on the data plane**: when `user_grants` is empty, resolve `client_id` → project from the config snapshot, and if that project has the welcome bundle and the person has no record, serve the welcome bundle's facts (a pure computation, no write; ADR-002's ponytail note already anticipates resolving the client's project). Zitadel's own roles claim still lacks the role until the grant lands, and I3 marks that as expected. Recommend (b) behind a per-project switch, decided once S1 numbers exist. |
| SC5 | **Sign-in bursts on the event path.** Every `oidc_session.added` does a Postgres upsert and a directory app lookup (`webhook.go:489-506`) on the shared 4-conn pool, beside drains and sweeps. | Webhook latency, then Zitadel redeliveries. The correlator lags. | No | Detection, usage data | M | Explicit pool size. Sign-in recording batched in memory and flushed every 5s (the upsert is idempotent and "keep latest"). |
| SC6 | **Redis memory and eviction.** No `maxmemory` is set (`docker-compose.yml:24-30`). At 10,000 users × 3 projects × ~1KB that is about 30MB of facts plus overhead, so it fits. The risk is the *policy*: an `allkeys-lru` added later "for safety" would evict facts, and an evicted fact is a silent `fail_closed`. | silent-partial if anyone adds eviction | No | Random people | L | `maxmemory 512mb` with `maxmemory-policy noeviction` (writes fail loudly, reads keep working), `used_memory` in `/healthz/claims`, and an alert above 70%. |
| SC7 | **Telemetry cardinality.** Rows per (minute × client × function × outcome) could reach about 360 a minute at 60 apps. | Postgres bloat, not silence | n/a | n/a | M | 5-minute buckets (about 17,000 rows a day at worst), with an hourly rollup after 48h kept for 90 days. Detection uses the in-memory state, not the table. |
| SC8 | **Correlator memory at bursts.** At a peak of about 50 calls a second over a 15-min window, a ring of the last 2,000 calls loses data. | False `CLAIMS_NOT_CALLED` | n/a | Detection | M | Keep a **map of the last call per (user, client)** rather than a ring: O(active pairs), a few thousand entries, under 1MB. Pending sign-ins are kept in a time-bounded queue (15 min). |
| SC9 | **App churn.** A new app has no shape and gets the project default (`claim_shape.go:50-55`) until a save or the hourly seed. A recreated app gets a new `client_id` (A5). Shape edits are frequent, and each edit today DELs a per-project key that races read-through (F-8). | silent-partial: "my new app gets the wrong claim name" | Once-per-process log | New apps | **H** | The config snapshot is keyed by `client_id` and rebuilt on `claims:gen`, so an edit lands everywhere within 1s, atomically. Telemetry `default_shape_for_unknown_client` is per client, so a *new* client shows up the first time it signs anyone in. Reconciler R6 runs on app create events as well as on schedule. |
| SC10 | **The deploy window now hits real people.** At a daytime average of about 2 calls a second (hundreds active; access-token refreshes plus new sign-ins), a 15s restart gap is about 30 claim-less tokens. At term start (about 10 a second) it is about 150, and app development means several deploys a day. | silent-full per affected token | No | Dozens to hundreds per deploy | **H** | P2 + P4 (§4.0b). |
| SC11 | **Debug capture under load.** `Wants` takes a mutex per token while any session is on (`claimdebug.go:178-186`). The worker runs `explain` (Zitadel and Postgres) per capture, with a 64-slot queue that drops. | Negligible latency. A crash risk without recover (F-11). | n/a | n/a | L | Keep it, add recover, and keep the one-watched-account rule. Cheap at any scale. |
| SC12 | **Probes at scale.** L1 every 60s (1,440 a day) and L2 every 15 min (96 tokens a day). | n/a | n/a | n/a | n/a | Unchanged. They are negligible next to real traffic and excluded from the SLIs. |

**Invariants worth checking continuously.** Each costs nothing on the hot path, since the request already carries the data.

- **I1.** Every sign-in event for app X is matched by a v2 call for (person, X) within ±2 min, with outcome `served`. *(This is the correlator.)*
- **I2.** For each v2 call: `user_grants` is present; it names exactly one project; and that project is the client's own project (directory: client_id → project).
- **I3.** For each served call: the set of roles Zitadel lists in `user_grants[p].roles` equals the served facts' roles, after removing open drift items. A difference means stale facts or unknown drift. Count it, sample it, and raise a finding when it persists.
- **I4.** Served facts are newer than 13h. The served snapshot generation is newer than its source's last write.
- **I5.** Each app's shape `lands` matches its Zitadel token settings (A1, A2).
- **I6.** Zitadel's target and executions are byte-identical to the manifest (`zitadel/actions/targets.json`).

---

## 3. Code-level findings (fragile today)

Ordered by severity times likelihood.

**F-0. The payload key is wrong, so v2 serves nothing (see TL;DR 0).**

`ActionV2UserGrantRef.ProjectID string \`json:"projectId"\`` (`action.go:31`) does not match Zitadel's `project_id` ([Z-b]). The comment on `ActionV2Request` (`action.go:42-47`) and the fixture (`action_v2_contract_test.go:143-…`) both encode the docs' mistake.

*Fix:* change the tag to `project_id`. Add a fixture with Zitadel's real keys, taken from `query.UserGrant`'s tags and the #12524 sample, and later a recorded one. Add a test asserting that a real-shaped payload yields a cache read for the right key.

The event payloads Syndra decodes **are** camelCase (`userId`, `projectId`, `roleKeys`, `clientID`: Zitadel's `repository/usergrant` and `oidcsession` structs), so `webhook_translate.go:45-50, 199-205` is correct. The mismatch is only in the function payload, which marshals a *query* struct, not an event.

**F-25. No reserved-key validation on shapes.**

`claims.Profile` validation (`claims.go:449-452` region) checks `lands` and rows but not keys. A key starting `urn:zitadel:iam` is dropped silently by Zitadel. `sub`, `aud`, `iss`, `exp`, `iat`, `nbf`, `jti` and `client_id` (and `name`, `email` and the other standard claims on userinfo) are overwritten silently ([Z-g]).

*Fix:* refuse those keys at save time, using the same rule list as Zitadel.

**F-26. Dead event binding.**

`targets.json` binds `user.token.added`, which Zitadel no longer emits ([Z-p]). It is harmless, but `oidc_session.added` is the **only** sign-in signal, so the correlator must not count on the other.

**F-1. The Redis per-call deadline is a no-op.**

`backend/internal/db/redis.go:20-23` builds `redis.NewClient(&redis.Options{Addr, DB})`, which leaves `ContextTimeoutEnabled` false. In go-redis v9.18.0, `baseClient.context()` returns `context.Background()` when that flag is false (`redis.go:888-893` in the module cache). Socket deadlines then come from `ReadTimeout` and `WriteTimeout`, which default to 3s.

`redisTimeout = 50ms` (`action.go:74`) therefore bounds only the pool wait, not a stalled read. One stalled GET uses Zitadel's entire 3s.

*Fix:* set `ContextTimeoutEnabled: true`, `DialTimeout: 200ms`, `ReadTimeout: 200ms`, `WriteTimeout: 200ms`, `PoolTimeout: 100ms`. Then add a test with a TCP listener that accepts connections and never answers; it should finish in under 300ms.

**F-2. The degraded path has no deadline.**

`claimFailureModeRead` calls `dbGetClaimFailureMode(ctx, …)` with the request context (`action.go:351`). `degradedResponse` is reached precisely when things are already failing (`action.go:174, 192, 198`), so it is the slowest code on the slowest day. The 27 Oct review raised this (§2.6); it is still open.

*Fix:* a single whole-request `context.WithTimeout(r.Context(), 250ms)` set in `HandleActionInject`, with every read deriving from it. A read that misses the deadline answers the failure mode it already has. If there is none, it answers `fail_closed` and counts `deadline_exceeded`.

**F-3. Absent `user_grants` is indistinguishable from empty.**

`UserGrants []ActionV2UserGrantRef` (`action.go:51`) makes the absent and empty cases look the same. `len(projectIDs)==0` then becomes an HTTP 200 with `[]` and a log line (`action.go:140-143`). Today that branch catches **every** token because of F-0, and it would keep hiding the next key mismatch the same way.

*Fix:* use `*[]…` or `json.RawMessage`. Absent means outcome `contract_fault`. Empty means `no_grants`. Both are counted per `client_id`.

*Built (S0, 3 Nov 2026), with one correction:* Zitadel's `ContextInfo.UserGrants` is itself `json:"user_grants,omitempty"` (v4.20.1), so "no role audience" (A3) arrives *absent*, never as `[]`. Absent is therefore `no_grants`. `contract_fault` is what a key mismatch actually looks like on the wire: `user_grants` absent beside an unknown top-level array of objects (a renamed list), rows that name no project in either spelling (F-0 itself), a non-list, or no `user.id`.

**F-4. Decoded but unused fields can fail every token.**

`ActionV2UserGrantRef.Roles []string` (`action.go:32`) is only used by debug capture (via `Grants`) and by the proposed I3. If Zitadel changes its type, the whole decode returns 400 (`action.go:110-112`).

*Fix:* decode it into `json.RawMessage` and interpret it lazily, best-effort. The same applies to `Function`: if it ever stops being a string it becomes a decode error.

**F-5. `LandsIn` fails open.**

`claims.go:214-215` returns `default: return true`. An unknown `function` value sends access-token-only claims into userinfo, and the reverse.

*Fix:* count unknown functions as an outcome class. Keep the answer `both`, because missing claims are worse than over-delivery to the *same* app, but raise a finding.

**F-6. TTL and warm are coupled so that expiry becomes the failure mode.**

- `cacheTTL = 24h` (`compiler.go:20`) and the warm runs every 12h (`main.go:249`).
- `Warm` returns `nil` when the directory is not current (`compiler.go:371-374`). The scheduler then logs nothing beyond that one line.
- A per-user warm failure keeps the old value but does not refresh its TTL (`compiler.go:319-321`).
- Two bad warms in a row means everyone's facts expire, and everyone gets `fail_closed`.

*Fix:*

- Make TTL much longer than the warm interval (7d), or drop it, and make staleness a **measured SLI** (`compiled_at` is already in the facts, `compiler.go:196`). Revocation correctness then rests on events, the expiry sweep and the warm, as it does today, while expiry stops being an outage.
- A skipped warm becomes a finding.
- `Warmed X of Y` excludes known-gone people and raises a finding when X/Y drops.

**F-7. A cache miss is not a repair signal.**

On a miss, `claimsForProject` answers degraded (`action.go:185-193`) and nothing schedules a compile. The first sign-in of a newly-granted person can keep failing until some event or the next warm.

*Fix:* a non-blocking `select { case repair <- key: default: }` feeding a single worker that runs `cache.RefreshUser`.

**F-8. The shape cache-aside race.**

`projectShapesRead` writes what it read from Postgres back to Redis after the read (`action.go:302-308`). `invalidateClaimShape` deletes on save (`handlers/claim_shape.go:204-207`). If a read straddles a save, the old shape is cached for up to `claimFailureModeCacheTTL()` = 300s (`action.go:316-323`; also misnamed, since it is the shape TTL too).

*Fix:* write-through on save, with versioned keys or a generation counter (P1).

**F-9. The `claim_mode:{p}` cache is never invalidated.**

There is no DEL on save (search: only `deps.go:318-323` touches it). An edit to the failure mode takes up to 5 min to land, and the protection the cache was written for (C5) lapses after 5 min of a Postgres outage.

**F-10. The redis-nil and redis-down cases look the same.**

In `projectShapesRead` (`action.go:286-292`), a Redis *error* (down or timeout) falls through to Postgres in the same way as a miss. When Redis is down, every token then does a Postgres read and a Redis SET attempt, which multiplies latency.

*Fix:* only `redis.Nil` reads Postgres. Any other error answers from the last-known-good copy or the failure mode.

**F-11. A goroutine panic kills the data plane.**

`claimdebug.(*Recorder).work` runs `explain` (Zitadel and Postgres reads) with no recover (`claimdebug.go:196-228`). The same is true of `addons.RefreshAll`'s goroutines (`registry.go:442-449`). `withPanicGuard` (`router.go:616-634`) covers only handler goroutines.

**F-12. Startup ordering puts the data plane last.**

`main.go:181-196` does all of these before `ListenAndServe` (`main.go:460-465`), and some are fatal:

- `ConnectPostgres` with migrations
- `ConnectRedis`
- `zitadel.InitClient`
- `directory.Init`
- `addons.Init`
- `addons.RefreshAll` (blocking, up to 5s)
- the demo seed
- the welcome check

Every deploy and every crash-restart is therefore at least one migration plus one add-on round trip of silent delivery.

**F-13. The server has no per-route timeouts.**

`main.go:233-239` sets only `ReadHeaderTimeout` and `IdleTimeout`, on one server shared with long admin handlers, so the data plane cannot have a `WriteTimeout`. *Fix:* its own `http.Server` (P2).

**F-14. Signature verification accepts one secret.**

`zitadel_action_auth.go:55` reads a single env var. Multiple `v1=` signatures are handled (`:116-123`), but there is only one key to try. Combined with rotate.sh's hard swap (`rotate.sh:150-155`), every rotation causes an outage.

*Fix:* use a keyring read from Zitadel (P3). Zitadel sends only one `v1=` and rotates with no grace period ([Z-l]), so the receiver must learn the new key within seconds of a rotation.

**F-15. The 401s are invisible.**

The signature middleware logs and answers 401 (`zitadel_action_auth.go:78-82`). That is the correct answer, but nobody counts it. A wrong key means 100% of tokens lose their claims.

**F-16. Health is not the data plane's health.**

`/healthz` pings Postgres only (`health.go:11-20`). It says "ok" while Redis is down, facts are empty, the key is wrong, or Zitadel has unbound the execution. No compose healthcheck exists for the backend or Redis (`docker-compose.yml:24-30, 36-155`; Redis `depends_on: service_started`).

**F-17. The smoke test only checks shape.**

`scripts/smoke-test-action-v2.sh` POSTs a fictional user and accepts any `append_claims` array (27 Oct review §2.4, still open). The deploy job only proves that the URL refuses unsigned requests (`deploy-prod.yml`, "Verify the Action target URL").

**F-18. The contract fixture is hand-written.**

`canonicalV2Payload` (`action_v2_contract_test.go:143-…`) was authored by hand, not recorded from Zitadel. Field names like `projectId` and `client_id` are assumed. §0 checks them against the source; the fixture should be *recorded* from prod (debug mode already captures `Request` bodies: `action.go:122-128`, `claimdebug.go:204-228`).

**F-19. Per-token logging is the only telemetry, and it is noisy.**

Every hit logs `Cache hit` (`action.go:207`), and every miss and every degrade logs too. At about 20 users this is fine for disk once the json-file driver is rotated, but log lines are neither a count nor a page.

**F-20. Shared Postgres pool.**

`pgxpool` uses its defaults (`postgres.go:24-29`), so `MaxConns = max(4, NumCPU)`. The data plane competes for those connections with drains, sweeps and the warm.

**F-21. A keyspace SCAN on every refresh** (`cache/deps.go:75-82`, `compiler.go:304-305`). This is O(total keys) per person, so O(N²) for a warm. See SC1. *Fix:* a per-person hash `facts:{u}`.

**F-22. Global reads inside a per-person compile** (`compiler.go:75`, `cache/deps.go:42-50, 111`). See SC2.

**F-23. The directory rebuild does per-user metadata calls every 30s** (`directory/zitadel.go:27, 195-224`). `FindUser` falls back to a `GetUser` per miss, and a 404 surfaces as an error rather than `ok=false` (`:244-247`). See SC3 and D1.

**F-24. The pgx pool is the default size** (`db/postgres.go:24-29`), which is `max(4, NumCPU)`. At scale it is shared by sweeps over thousands of people, the webhook bursts and (pre-P1) the data plane. *Fix:* `pool_max_conns` set explicitly in `DB_DSN`, or `config.MaxConns`, and pool stats shown on `/healthz/claims`.

---

## 4. Recommended design

**Principles:**

1. Prevent the failure classes that can be designed out.
2. Make every remaining failure produce a *count* within one minute and a *finding* within five.
3. Have one external signal that fires when Syndra itself cannot speak.

Minimal moving parts: Redis and Postgres are already present. The only new external piece recommended is a free dead-man's-switch endpoint, and even that is optional.

### 4.0a Capacity model: why one LXC still holds

These figures are for the design point and its 2× check. Assumptions:

- 5,000 people, checked to 10,000
- 3 projects held per person on average
- 60 apps across 30 projects
- facts of about 1KB per (person, project)
- 2 to 3 v2 calls per sign-in (`preaccesstoken`, plus `preuserinfo` for the ID token or userinfo), plus access-token refreshes

| Quantity | Design point | 2× check | Comment |
|---|---|---|---|
| Facts entries | 15,000 | 30,000 | One hash per person after SC1: 5,000 to 10,000 keys |
| Redis memory (facts + config + telemetry) | ~25MB | ~50MB | `maxmemory 512mb noeviction` leaves 10× headroom |
| Config snapshot (shapes + modes for 30 projects and 60 apps) | ~200KB | ~400KB | A full reload on every edit takes about 1ms |
| In-process LKG LRU (facts recently served) | 20,000 entries, ~40MB | same cap | Bounded. Covers everyone active in the last day or so. |
| Peak v2 calls (term start: a class of 200 in 60s, plus background refreshes) | ~15/s | ~50/s design ceiling | Daytime average about 2/s |
| CPU per call (decode a 5-20KB payload, HMAC, map lookups, shape) | ~0.3ms | n/a | 50/s × 0.3ms = **1.5% of one core** |
| Redis ops per call | 1 (`HGET`) | n/a | 50/s against a capacity above 50,000/s |
| Warm, after SC1-SC3 fixes | ~5,000 × ~3ms ≈ 15s per pass, spread incrementally | ~30s | Today: minutes, about 1M SCANs, and tens of thousands of IdP calls |

**Conclusion.** Throughput never argues for a second host. What breaks at scale is *coupling* (deploys, shared pools, a shared crash domain) and the *write side's* O(N²) costs. Both are fixed inside one LXC: process isolation (P4), cgroup limits, and the SC1-SC3 fixes.

A second host would only protect against an LXC or host failure, which the owner has excluded. Its cost would be replicated Redis and Postgres, failover, and split-brain risk, which is more moving parts than everything else here combined.

**Minimum LXC sizing:** 2 vCPU and 4GB. Compose limits keep the admin side from starving claims: `backend: cpus 1.5, mem_limit 2g`; `claims: mem_limit 256m`; `redis: mem_limit 768m`.

### 4.0b Prevention (structural), weighed at the design scale

| # | Option | What it prevents | Cost | Verdict at 5,000 to 10,000 people / ~60 apps / one LXC |
|---|---|---|---|---|
| **P1** | **Self-sufficient hot path, in two tiers.**<br>**Config tier:** an immutable, versioned *config snapshot* `{gen, built_at, shapes by client_id and project default, failure modes, client_id→project, welcome facts per project}` held in memory (`atomic.Pointer`). The write side becomes write-through: shape, mode and seed saves write `claim_shapes:{p}` and `claim_mode:{p}` with **no TTL**, then `INCR claims:gen`. The data plane polls `GET claims:gen` every 1s and reloads the whole config (about 200KB) when it moves.<br>**Facts tier:** one `HGET facts:{u} {p}` per token (SC1 layout), with real 50ms timeouts (F-1). The result goes into an in-process **LKG LRU** (20,000 entries). On a Redis error or timeout, serve from LKG within the project's staleness bound; otherwise use the failure mode.<br>A request is at most one bounded Redis read plus map lookups. **No Postgres, no Zitadel.** | S2, S3 (bounded to 50ms, then LKG), S8, S10, S11, SC9, F-1/2/8/9/10, and most of S15 | ~300 LOC: a `claimserve` package with config loader, LKG and serve, plus write-through in two save handlers and the seed, plus the `facts:{u}` hash migration (a warm rebuilds it; no data migration). Removes Postgres code from `action.go`. | **Do it.** It removes the slow and partial failure classes, makes each answer reproducible (`gen` is recorded with each call), and the code gets *smaller*. *Why not all facts in memory?* At 30,000 entries it would fit (about 60MB), but keeping it current needs a change feed (a Redis Stream), gap detection and a full resync: about 150 LOC more and another way to be stale. One HGET with a real timeout plus LKG already gives a bounded worst case. `ponytail:` upgrade to stream-fed full facts only if the Redis tail-latency SLI ever breaches. |
| P1b | Last-known-good file: the loader writes `claims-snapshot.json` (atomic rename) after each reload, so boot can serve before Redis answers | S4, and boot ordering | ~30 LOC plus a volume | **Skip for now.** Redis AOF already persists across restarts and the boot warm rebuilds. Add it only if P4 is built *and* Redis loss is ever observed. |
| **P2** | **Dedicated listener.** A second `http.Server` on `:8090` serving only `POST /api/action/inject` and `GET /healthz/claims`. Its own mux and panic guard; no CORS or cancel gate. `ReadTimeout 2s`, `WriteTimeout 2s`, `MaxHeaderBytes 16KiB`, body cap. Started **first in `main`**, before migrations. Caddy: `handle /api/action/* { reverse_proxy <APP_HOST>:8090 { lb_try_duration 2s  lb_try_interval 100ms } }`. Caddy retries only dial failures for POST by default, so this is safe for a non-idempotent call. | F-12, F-13, S18, and most of N1 (a restart shorter than about 2s becomes latency) | ~60 LOC, one compose port, one Caddy line | **Do it.** It is cheap and turns the commonest silent window into extra latency. |
| **P3** | **A keyring read from Zitadel, not from env.** Zitadel generates the new key itself at rotation, swaps it immediately and signs with that key alone ([Z-l]). An env-only `_NEXT` slot therefore cannot be filled ahead of time. Instead the backend reads the target's key through `GetTarget` (needs `action.target.read`, [Z-o]) every 60s, and again at once (single-flight, at most once every 10s) after any signature mismatch. It publishes `{current, previous (kept 15 min)}` into the config snapshot. The env key stays as the bootstrap and the fallback. Rotation is then simply `rotate.sh`: Syndra follows within seconds, with no restart. Telemetry records which key matched. | Z8, Z9, F-14 | ~60 LOC. The service account gains `IAM_OWNER_VIEWER`, which can read the key. Under P4 the key reaches the claims container through Redis, which sits on the internal network and is ACL-scoped. | **Do it.** This makes Zitadel the one write place for the key (it already is) and Syndra a reader. The fallback, if the owner refuses the read permission: env `_NEXT` plus a rotation script that writes `.env` and restarts *before* swapping. That leaves a gap of seconds, not minutes. |
| **P4** | **Process split.** A `syndra-claims` container (`cmd/claims`): signature, decode, snapshot loader, serve, and telemetry counters to Redis. Its own Dockerfile with a *narrow* `COPY` (go.mod, go.sum, `internal/claims`, `internal/claimserve`, `cmd/claims`), so its image only changes when that code does. A backend deploy then does not recreate it (compose only recreates on an image or config change). Redis ACL user: read `mapping:*`, `claim_*`, `claims:gen`; write `claims:tel:*`. | N1 fully. S13, S14, S15 (a crash elsewhere cannot touch it). Admin load cannot starve it. | ~150 LOC beyond P1 and P2, one service, one Dockerfile, one Caddy upstream change. Telemetry moves through Redis (the backend folds it into Postgres). | **Do it, right after P1 and P2.** At the design scale the case is concrete:<br>(1) SC10: each deploy costs about 30 claim-less tokens on an ordinary day and about 150 at term start, with several deploys a day while apps are being built.<br>(2) Admin work grows with the user count: sweeps, drains, warms and bulk operations over thousands of people. Those allocations, CPU and pool waits share a crash domain with the data plane only if the two share a process.<br>(3) Once P1 has removed Postgres and Zitadel from the hot path, the split is mostly packaging.<br>Cgroup limits (§4.0a) make the isolation real inside one LXC. The *throughput* argument stays nil (1.5% of a core). |
| P5 | Read-only Postgres role for the data plane | Accidental writes from the hot path | Small | **Moot under P1.** The data plane has *no* Postgres. That is better than read-only. |
| P6 | Bulkheads and admission control (a semaphore on the data plane, load shedding) | Overload | Small | **Skip a semaphore; keep the limits.** A design peak of 50/s at 0.3ms each is far from any saturation, and a queue in front of a sub-millisecond handler only adds tail latency. P4's container, with its own cgroup limits, is the bulkhead that matters. Add admission control only if a public route ever shares the listener. |
| P7 | Per-hop deadlines | Stalls | Tiny | **Keep as a belt:** one 250ms request deadline, and the F-1 Redis options for the loader. Under P1 nothing on the request path can block. |
| P8 | Interrupt on error, so failures become loud | Silence (by failing sign-in) | A config flip | **Do not.** It turns every Syndra blip into a sign-in outage. With refresh rotation, a failed interrupting `preaccesstoken` also *consumes the refresh token* (#12974, [Z-q]). Keep it off, make Syndra hard to break (P1-P4), and make every miss loud through detection. Revisit per app only if an app would rather refuse sign-in than run without claims (that is what `fail_closed` approximates without blocking). |
| P9 | Serve-time correctness guards: drop roles whose `RoleEnds` date is in the past (D4), and repair-on-miss (F-7). At scale the repair channel is **deduplicated and rate-limited**: a set of pending user ids, one worker, at most 20 compiles a second. A burst of misses at term start then cannot become a compile storm. | Stale or absent facts | ~30 LOC | **Do it.** |
| P10 | **Write-side scale fixes** (SC1-SC3, F-21 to F-24): the per-person facts hash, a compile context loaded once per pass, one directory snapshot per warm, 404 = gone, an incremental warm, an explicit pool size | An O(N²) warm, Syndra slowing its own IdP, freshness quietly lapsing | ~200 LOC | **Do it before the user count grows.** These are invisible at 20 people and dominant at 5,000. |
| P11 | Welcome facts for the first sign-in (SC4 option b) | A new member's first session going without claims | ~40 LOC plus one owner decision | **Measure in S1, then decide.** |

**Why not more?** No message bus, no Prometheus stack, no separate Postgres for telemetry, no second LXC. Redis already *is* the bus between compile and serve. Postgres already holds rollups. Caddy already retries. An external dead-man's switch is the only thing that must live outside the LXC, because the LXC cannot report its own death.

### 4a. White-box telemetry for every v2 call

**Outcome classes.** These are exhaustive and mutually exclusive, and are set by a wrapper around the **whole route**, so the signature middleware is included:

`served`, `served_lkg` (snapshot older than its staleness bound), `degraded_no_facts`, `degraded_shape_unreadable`, `degraded_malformed`, `no_grants`, `contract_fault` (user_grants absent, or user.id missing), `refused_cross_project`, `not_landing` (the shape does not land in this function), `unknown_function`, `sig_missing`, `sig_stale`, `sig_mismatch`, `bad_request`, `deadline_exceeded`, `panic`

**Dimensions.** `project_id`, `client_id` (or `-` when absent), `function`, `key_slot` (current or next).

**Flags.** `client_known` (the client has its own shape) and `payload_fp` (fingerprint).

**Latency.** A fixed-bucket histogram of server-side milliseconds: 1, 2, 5, 10, 25, 50, 100, 250, 500, 1000, 3000, +Inf.

**Storage, without per-call rows:**

- **In memory:** `map[key]*counters` (atomic) for the current 5-minute bucket, plus `lastCall map[(user, client)] → {at, function, outcome, gen, ms}`. That is O(active pairs): a few thousand entries, under 1MB, pruned after 20 min. The correlator reads it. Per-call cost is one map write under a sharded mutex, about 100ns.
- **Every 60s,** off the hot path: upsert one row per non-zero key into `claim_delivery_bucket(bucket_5m, project_id, client_id, function, outcome, n, buckets int[12])`.
  - At about 60 apps that is at most about 17,000 rows a day. Realistically it is a few thousand, since most (client, outcome) pairs are zero.
  - A nightly job rolls rows older than 48h into `claim_delivery_hour`, kept for 90 days.
  - Pruning rides on the existing usage-prune scheduler.
- **Also every 60s:** upsert `claim_delivery_last(client_id, function) → (last_at, last_outcome, last_served_at)`.
- **Under P4,** the claims process `HINCRBY`s into `claims:tel:{minute}` (TTL 2h). The backend folds that into the same table.
- **If the flush fails,** counters are kept in memory and merged into the next flush, with a cap so they cannot grow without bound, and a `telemetry_flush_failed` gauge is shown in the UI.

**Payload fingerprint.** A sorted set of JSON key paths with array indexes collapsed (for example `user_grants[].projectId`), hashed. The first time a new fingerprint is seen, store an example (signature stripped, PII redacted) and raise the informational finding "Zitadel's payload changed shape". That catches Z12, P1-P4 and P6 at the first token after an upgrade, and **records the fixture for §4h automatically**.

**Exposed through** `GET /api/v1/claims/delivery?window=24h`, an operator-only endpoint.

### 4b. The silent-failure detector: sign-in ↔ delivery correlation

Syndra already receives `oidc_session.added` and `user.token.added` through the event listener. It resolves them to (person, project, app) and upserts `usage_last_seen` (`webhook.go:133-136, 489-532`; `webhook_translate.go:145-157, 218-252`).

**Add the following:**

1. **Record each sign-in** as `{event_at, user, app_id, client_id}` in a time-bounded pending queue (15 min). At term start that is at most a few thousand entries.
2. **Every minute, the detector** takes sign-ins aged between 2 and 15 minutes that are not yet checked. Async events can lag, so they are not checked sooner. For each one it looks in `lastCall[(user, client)]` for a call with `|call.at − event_at| ≤ 120s`.
   - Several calls per sign-in collapse into "the latest", so the map also keeps the *worst* outcome seen inside the window.
   - People created less than 10 min before the event, with outcome `no_grants`, are classed `first_sign_in_before_access` (SC4). That class is counted and measured, not alarmed, unless the owner picks option (b).

   It then classifies the result:
   - **No call at all** gives finding `CLAIMS_NOT_CALLED{app}`. This covers Z1, Z2, Z5-Z8, Z10, Z13, Z14, N2-N5, and anything that keeps Zitadel from reaching Syndra.
   - **A call whose outcome is not `served`** gives finding `CLAIMS_NOT_SERVED{app, outcome}`.
   - The expected functions are exact, from the event and the app ([Z-e], [Z-p]). A JWT-access-token app expects `preaccesstoken`. A `scope` containing `openid` (in the `oidc_session.added` payload) expects `preuserinfo`, for the ID token. An app whose settings mean *no* function is ever called is a reconciler finding (A1/A2), not a correlator one.
3. **The inverse check:** calls are arriving but no sign-in events have arrived for 24h (with at least one call in that time). That gives `SIGNIN_EVENTS_MISSING`, meaning the detector is blind (N7).
4. **Findings upsert by (kind, subject)** with `first_seen`, `last_seen`, a count and one sample. They resolve automatically after 3 consecutive clean matches. The pattern is the same as drift items: one row per condition, never one row per event.
5. **A restart empties both rings.** Sign-ins from before a restart count as "unchecked", never as failed. The detector says so ("checking since 14:02").

Matching is on (user, client) within a time window. That stays exact at hundreds active, because a person signs in to one app at a time.

*What it proves:* for every real sign-in, Zitadel actually reached Syndra and Syndra actually served. It is black-box with respect to Zitadel's configuration, so it does not need to know *why* something broke.

*What it cannot see:*

- Zitadel dropping a key Syndra served (Z11): the probe covers that.
- An app that never signs anyone in.
- Sign-ins Zitadel never emits an event for, if any. Machine-user client-credentials tokens do not create OIDC sessions; see §0.

### 4c. Read-only reconciler of Zitadel's configuration

Run every 10 minutes, and once at startup, reusing the existing `managementClient`. It needs `action.target.read` and `action.execution.read` ([Z-o]): grant the service account `IAM_OWNER_VIEWER`, which is read-only. Note that this lets it read the signing key, which Syndra already holds. If the service account lacks it, the reconciler raises `CANNOT_CHECK{actions}`, which is itself a finding and is never silent.

| Check | Source | Expected |
|---|---|---|
| R1 target `syndra-claim-injector` | `POST /v2/actions/targets/search` or `GET /v2/actions/targets/{id}` ([Z-o]) | endpoint = `SYNDRA_EXTERNAL_URL/api/action/inject`, type restCall, `interruptOnError:false`, timeout 3s, payload JSON.<br>**Signing key:** the API returns it decrypted ([Z-l]). Compare `HMAC(key, "syndra-key-check")` with the same computed from the env key; never log or store the key itself. A mismatch is a finding *before* any token fails (Z8, Z9).<br>Record id and `change_date`. |
| R2 executions `function/preaccesstoken`, `function/preuserinfo` | `POST /v2/actions/executions/search` | Target list **exactly** `[syndra-claim-injector]` |
| R3 event executions | `ListExecutions` | Every event in `targets.json` bound to `syndra-event-listener` |
| R4 v1 flows | Management v1 flows (Complement Token: pre access token and pre userinfo triggers) | No action script sets a top-level key used by any shape. The comparison is a regex over scripts for `setClaim("key"` and `api.v1.claims.setClaim`. Report "v1 still active on N keys" until the owner retires it. |
| R5 per app (all ~17) | `TokenSettings` (`zitadel/applications.go:141-176`), the same read as explain check 1 | The app's shape `lands` is reachable: a JWT for access-token shapes, ID-token userinfo assertion or userinfo use for userinfo shapes |
| R6 shapes ↔ apps | Directory applications vs `claim_shape_versions.client_id` | Every shape's client id is live (A5). Every live app has a resolvable project. |
| R7 shapes ↔ roles | Directory project roles | The role keys in each shape's rename and include list exist (D2) |
| R8 instance features (v3) | Feature service, if present | Actions-v2 executions enabled |
| R9 Syndra-side config | env | Key present; NEXT key age; `SYNDRA_EXTERNAL_URL` is https and resolves; `claims:gen` advancing; snapshot age; warm health |

The output is the same findings table as §4b, with kind `CONFIG_DRIFT{check}`. Each finding carries "expected" and "found", and links to the one place where it is fixed: the Zitadel console page or the Syndra setting (one write place).

### 4d. Synthetic end-to-end probes

**L1, Syndra → its own public URL. Every 60s.**

- It signs a request with the current key, using a fixed canary payload `{user.id:"syndra-canary", user_grants:[{projectId:<canary project>}], application.client_id:<canary client>, function:"function/preaccesstoken"}`.
- It POSTs to `SYNDRA_EXTERNAL_URL/api/action/inject`, so it goes through public DNS, Caddy, TLS and the LXC.
- The snapshot carries a fixed canary facts entry (written by the compiler like any other entry, so it also exercises the compile → Redis → snapshot path).
- It asserts the **exact** `append_claims`.
- It also records the TLS certificate's `NotAfter`.
- It is labelled `probe` in telemetry and excluded from the SLIs.

*Proves:* DNS from the LXC, the certificate, the Caddy route, the listener, the signature with the key Syndra holds, decode, snapshot freshness, and the shaper.

*Does not prove:* Zitadel's configuration, Zitadel's key, Zitadel's network path or resolver, Zitadel's merge rules, or the payload contract.

**L2, a real token from Zitadel. Every 15 minutes.**

- One-time owner setup: a canary project with one role, and a canary **machine user** with **access token type JWT** holding that role, with a key Syndra stores (like the existing service-account key). Client-credentials and JWT-profile grants pass no project, so the request **must** carry `urn:zitadel:iam:org:project:id:{canary}:aud`, or `user_grants` is empty ([Z-d], [Z-e]). This also exercises F-0's key on every probe.
- Syndra obtains a token using the JWT-profile or client-credentials grant, decodes it (it does not need to verify it; it is Syndra's own request), and asserts:
  1. Syndra's canary claim is present with the exact value.
  2. There is no `urn:zitadel:iam:action:function/preaccesstoken:log` claim, so no key was dropped.
  3. Telemetry saw the matching call with the canary's `client_id`, which is the client_id contract (P2).
- The same is done for userinfo: call `/oidc/v1/userinfo` with the token, which proves `preuserinfo`.

*Proves:* the whole loop, including Z1-Z14, N1-N6 from Zitadel's side, and the live payload and response contract of the running Zitadel version. This is the industry-standard black-box check: real traffic shape, a real dependency, an assertion on the outcome.

*Confirmed in source:* machine-user grants do fire the functions; they go through `accessTokenResponseFromSession` ([Z-f]). `preuserinfo` fires on the userinfo call and on ID-token creation, the latter only if `openid` is requested.

The "two levels" exist because L1 is cheap and local (it isolates *Syndra's* half), while L2 is end-to-end but sparser. When L2 fails and L1 passes, the fault is on Zitadel's side: Zitadel's configuration, the deny list, its DNS, or a key mismatch.

### 4e. Time budgets and fallbacks (Syndra always answers well within 3s)

| Layer | Budget | On breach |
|---|---|---|
| Caddy dial retry | `lb_try_duration 2s`, interval 100ms | 502 (counted by the correlator, since there is no call) |
| Data-plane server | `ReadTimeout 2s`, `WriteTimeout 2s` | Connection closed |
| Whole request (ctx) | 250ms | Answer the failure mode from the snapshot; outcome `deadline_exceeded` |
| Hot path under P1 | In memory, under 1ms | n/a |
| Snapshot loader (background) | Redis options from F-1 (200ms), reload ≤ 2s | Keep the previous snapshot; staleness SLI |
| Before P1 (interim) | Redis 50ms (real, after F-1); Postgres shapes 150ms; Postgres failure mode 100ms | Failure mode |

**Fallback ladder.** Each rung is counted.

1. Current snapshot.
2. Last-known-good snapshot if it is younger than the per-project bound (default 6h). The bound is configurable per project beside `claim_failure_mode`, because "serve what we knew an hour ago" is a policy choice about revocations.
3. The project's `minimal_safe` claims.
4. `fail_closed` (`[]`).

There is never an empty answer *without* a count.

### 4f. Startup and config validation that fails loudly

Make a clear distinction between kinds of failure:

- **Config invalid.** Refuse to start (`log.Fatal`). Cases:
  - signing key empty in production (exists)
  - a key shorter than 32 bytes or not printable
  - `SYNDRA_EXTERNAL_URL` empty or not https
  - `ZITADEL_DOMAIN` set but `SYNDRA_EXTERNAL_URL` unset
  - the canary configuration half-set
- **A dependency down.** Start anyway, degraded and loud. The data plane starts on the snapshot or Redis. Postgres down affects the admin and compile side only. `/healthz/claims` reports which dependency is down.
- **Self-check after listen,** within 10s: the L1 probe runs once and the reconciler runs once. The results show on `/healthz/claims` and on Home.
- **Deploy job:** replace the unsigned-401 check with "L1 probe passed after start, **and** `/healthz/claims` is ok". Keep the unsigned-401 assertion as a demo-mode guard.

### 4g. Where it surfaces, and the alert channel

Structure never moves with data (basic-advanced-ia):

- **Home:** one fixed line, always present. *"Claims: delivered to every sign-in in the last 24h"*, or *"Claims: 2 problems"*, linking to the view below. Same position whether or not there are problems.
- **System › Zitadel:** a fixed **Claims delivery** section with four parts:
  1. **Live:** per app, last sign-in, last delivery, outcome mix over 24h, p99 latency.
  2. **Configuration:** the R1-R9 checklist with expected and found.
  3. **Probes:** L1 and L2, last pass, and certificate days left.
  4. **Keys:** age, and which key slot Zitadel is using.

  This page already owns "what Zitadel is doing", so it is one place, not a new nav entry.
- **Apps › an app's claims part:** "Last delivered 14:02, served", beside the preview and the explain checks. The explain step gains a sixth check: *"Zitadel reached Syndra for this app recently"*.
- **`GET /healthz/claims`:** unauthenticated and minimal: `{"status":"ok|degraded|failing","reasons":["CONFIG_DRIFT:R2", …],"snapshot_age_s":12,"probe_l1":"pass"}`. It carries no ids and no PII, so an external monitor can scrape it. The existing `/healthz` stays a liveness probe.
- **Alert channel (owner's decision; recommendation):** a **dead-man's switch** (healthchecks.io free tier, or an Uptime-Kuma push monitor if one already runs on the network).
  - Syndra pings the URL every 5 min **only while** `/healthz/claims` is `ok`, with the reasons in the body.
  - A missed ping produces an email or push to the owner.
  - One outbound HTTPS call, no inbound exposure, no SMTP credentials in Syndra.
  - It covers **everything**, including "the LXC is down", "the backend is crash-looping" and "Syndra's own alerting is broken", which nothing inside the LXC can report.
  - Alternative: Workspace SMTP from Syndra, which adds credentials and a mail dependency and still cannot report its own death.
  - Low-volume note: at about 20 people, rate-based SLO burn alerts are meaningless (one failed token out of three is 33%). Alert on *occurrence* for hard classes (`sig_*`, `contract_fault`, `bad_request`, `panic`, any `CONFIG_DRIFT`, `CLAIMS_NOT_CALLED`). Alert on *persistence* for soft ones: `degraded_*` on the same app for 2 of the last 3 sign-ins.

**SLIs and SLOs (monthly):**

- **Delivery SLI:** sign-ins with a `served` call ÷ sign-ins. SLO 99.5%. Error budget of about 3 misses per 600 sign-ins.
- **Latency SLI:** p99 server-side ≤ 25ms. SLO 99%.
- **Freshness SLI:** age of served facts. 99% under 13h.
- **Config SLI:** minutes with any `CONFIG_DRIFT` open. Target 0.
- **Golden signals per call:** traffic (calls per client), errors (outcome classes), latency (histogram), saturation (snapshot age and loader lag, which is the only saturation that matters here).

### 4h. Tests

- **Contract tests from recorded payloads.**
  - `backend/internal/handlers/testdata/zitadel/v4.15/preaccesstoken.json` and `preuserinfo.json` (and v3.x if that is ever relevant), captured by debug mode or the fingerprint recorder and redacted.
  - Each has a golden `append_claims`.
  - A test iterates over versions. Adding a version means adding a fixture; a Zitadel upgrade PR must add one.
- **Decoder fuzzing.** `FuzzActionV2Decode` (Go native fuzzing), with these invariants:
  - it never panics
  - absent `user_grants` gives `contract_fault`, never `no_grants`
  - a type change in an unused field never produces a 400
  - the outcome is always one of the enumerated classes
- **Signature fuzzing.** `FuzzZitadelSignature`: header permutations, whitespace, duplicate `t=`, many `v1=`, non-hex. It never panics and never accepts without an HMAC match.
- **Fault injection.** Each case runs through the handler with an `httptest` server and asserts both the outcome class *and* the elapsed time:
  - Redis hung: a TCP listener that accepts and never replies. Elapsed under 300ms (proves F-1).
  - Redis refused.
  - Postgres slow: stub delay, or `pg_sleep` under `SYNDRA_TEST_DATABASE_URL`.
  - Postgres pool exhausted.
  - Malformed snapshot entry.
  - Zitadel 404 during warm: classified as gone and excluded from the ratio.
  - A panic inside the shaper: recovered, outcome `panic`.
  - A panic in the claimdebug worker: the process survives.
- **Race tests.** `go test -race` with concurrent snapshot reload and serve; a shape save interleaved with a stale read-through (pre-P1) to prove F-8 and then its fix; concurrent `RefreshUser` against warm (exists in spirit, `compiler.go:283-302`).
- **Equivalence property.** For random facts, profiles, functions and client ids, the snapshot answer equals the simulator's answer (CLAUDE.md: "a preview computed by different code… is a preview of nothing").
- **Detector tests** (test the detectors, not only the code):
  - Correlator: a sign-in with no call gives `CLAIMS_NOT_CALLED` after 2 min. A sign-in plus a degraded call gives `CLAIMS_NOT_SERVED`. Lag tolerance. Ring reset after restart gives "unchecked", not failed.
  - Reconciler: table tests from recorded `ListExecutions` and `GetTarget` JSON, with each field mutated in turn.
  - Probe: L1 against an httptest TLS server with an expiring certificate.
- **Scale tests.**
  - `BenchmarkServe`: decode, verify, HGET (fake) and shape for a 20KB payload. p99 under 1ms.
  - A load test against the `:8090` listener on the dev box: 200 rps for 60s. Server-side p99 under 5ms, zero non-served outcomes, telemetry counts equal to requests sent.
  - The warm complexity test from S1b.
  - A correlator test with a 1,000-sign-in burst inside 10 min: no false `CLAIMS_NOT_CALLED`.
- **Chaos drill on the dev box (manual, quarterly, about 30 min):**
  - stop Redis
  - `kill -STOP` Redis (the hung case)
  - stop the backend during a sign-in
  - rotate the key without updating the env
  - unbind the execution
  - flip the target endpoint

  Each must produce the named finding and a missed dead-man ping within 5 min. Record the results in `openspec`.

---

## 5. Phased build plan

Each slice ships on its own, keeps current behaviour except where noted, and has a named proof.

| Slice | Contents | Proves | Size |
|---|---|---|---|
| **S0a: make v2 deliver at all (blocker for retiring v1)** | F-0: the `project_id` tag; a fixture with Zitadel's real keys; a test that a real-shaped payload reads `mapping:{u}:{p}`. Then one debug-mode capture in prod, saved as `testdata/zitadel/v4.x/preaccesstoken.json`. | In prod logs, `Cache hit` rises from 0 to about one per call. The recorded fixture's golden test passes. | ~1 hour |
| **S0: stop the bleeding** | F-1 (Redis options), F-2 (250ms request deadline and bounded failure-mode read), F-10 (nil vs error), F-11 (recover in claimdebug and addons goroutines), F-3/F-4 (presence-aware `user_grants`, `Roles` as RawMessage), F-6 (TTL 7d, a skipped warm is logged as an error, 404 = gone), F-7 (repair-on-miss), compose `logging.max-size` | The fault-injection tests: under any single dependency fault the handler answers within 300ms with a defined outcome | ~1 day |
| **S1: see every call** | §4a counters, ring, minute rollup table (one migration), `claim_delivery_last`, fingerprinting with a recorded fixture, `GET /api/v1/claims/delivery`, `/healthz/claims` | Every call is accounted for by class and app. "No project grants" becomes a per-app number. The first real fixture is recorded. | ~1.5 days |
| **S1b: the write side scales (P10)** | `facts:{u}` hash layout (data plane reads `HGET`, compiler `MULTI`), a compile context loaded once per pass, one directory snapshot per warm, a 404 classed as gone, an incremental warm, `pool_max_conns`, Redis `maxmemory noeviction` | A synthetic 10,000-person warm (fake Redis and DB counting calls) does O(own projects) Redis ops per person and 3 global reads per pass, not per person; the IdP calls per warm are ≤ ListUsers pages + 0 per-person calls | ~1.5 days |
| **S2: the hot path stands alone (P1, P9) and gets its own door (P2)** | `claimserve` snapshot and loader; write-through shapes and modes (no TTL); `claims:gen`; the dedicated `:8090` server started first; Caddy `lb_try_duration` | Redis stop, Postgres stop and a backend restart under 2s all show **0** non-served outcomes in S1's telemetry (dev-box drill) | ~2 days, plus the Caddy edit (owner, on <caddy-host>) |
| **S3: the silent-failure detector, and the alert** | §4b correlator, the findings table, the Home line, the System › Zitadel "Live" part, the dead-man ping | Unbinding the execution on the dev box raises `CLAIMS_NOT_CALLED` within 5 min **and** the phone goes off | ~1.5 days |
| **S4: configuration cannot drift unseen** | §4c R1-R9 reconciler, the "Configuration" part, explain check 6 | Mutating each target and execution field on the dev box raises the matching `CONFIG_DRIFT` before any sign-in | ~2 days (plus granting the service account actions-read, owner) |
| **S5: rotation without an outage (P3)** | A keyring read from Zitadel (60s, plus an immediate re-read after a mismatch), key-slot telemetry, `IAM_OWNER_VIEWER` for the service account (owner) | A key rotation on the dev box shows zero `sig_mismatch` | ~0.5 day |
| **S6: end-to-end probes** | L1 (with certificate expiry), then L2 canary (owner creates the canary project, app and machine user), deploy job switched to the probe | L2 catches a Z11 collision (re-add a v1 `setClaim` on the dev box) and a deny-list block | ~1.5 days |
| **S7: process split (P4), recommended at the design scale** | `cmd/claims`, narrow Dockerfile, compose service, Redis ACL, telemetry through Redis | Ten consecutive backend deploys on the dev box show zero data-plane restarts and zero non-served sign-ins | ~1.5 days |

**Order rationale.** S1b has to land before the user count grows past a few hundred, because its costs are quadratic. S0 removes the timing defects with almost no design change. S1 comes before the structural slices so that S2 and S7 are *measured*, not assumed. S3 is the point where "silent" ends. S4 to S6 move detection earlier, from "after a sign-in failed" to "before anyone signs in". S7 is the last structural step and is cheap once S2 exists.

**OpenSpec.** One change, `claims-delivery-integrity`, with this document's §2 as its spec of failure modes, ADR-004 ("the data plane serves a snapshot; no Postgres or Zitadel at request time"), and the tasks above tagged `ready-for-agent`, except the owner steps (Caddy, canary in Zitadel, service-account permission, dead-man account), tagged `ready-for-human`.
