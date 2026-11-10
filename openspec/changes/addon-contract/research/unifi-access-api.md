# UniFi Access API: research for a Syndra add-on

Researched 2026-11-10.

## Sources and trust

- **[PRIMARY]** "UniFi Access API Reference" (official PDF, Chromium-generated 2026-03-23), https://assets.identity.ui.com/unifi-access/api_reference.pdf. Read in full via pdftotext.
  - Latest changelog entry is v4.2.16. Every "Requirement: X or later" below is a UniFi Access application version quoted from this PDF.
  - Every fact below is **VERIFIED** unless it is tagged **INFERRED** or **UNVERIFIED**.
  - Section numbers (e.g. #3.6) refer to this PDF.
- **[SECONDARY]** Ubiquiti help centre pointer, https://help.ui.com/hc/en-us/articles/30076656117655. Seen only as a search snippet and not read. It points to the developer portal.
- **[UNOFFICIAL]** A community-hosted OpenAPI rendering of the PDF (URL deliberately not recorded: a third-party mirror, not a source). Seen only in search results. It agrees on port 12445, the token location and the user and access-policy paths. Do not cite it as authoritative.
- **NOT consulted:**
  - The API docs page served by the Access app itself.
  - Home Assistant `unifi_access` and community clients.
  - Real hardware.

  Anything the PDF does not say is flagged as such and needs a lab console to confirm.

## 1. Access model

All of this is from the PDF.

- **User** (#3.1)
  - Fields: `id` (UUID string), `first_name`, `last_name`, `full_name`, `alias`, `user_email` (needs Access 1.22.16+), `email_status`, `phone`, `employee_number` (string), `onboard_time` (int, epoch seconds).
  - Credentials on the user: `nfc_cards[]`, `pin_code.token`, `license_plates[]`, `touch_pass`.
  - Access: `access_policy_ids[]` and `access_policies[]` (the latter only with `expand[]=access_policy`).
  - Status: `status` in {`ACTIVE`, `PENDING`, `DEACTIVATED`}.
    - `PENDING` is an admin invited by SSO who has not yet accepted.
    - `DEACTIVATED` is the disabled state.
- **Visitor** (#4)
  - A separate object under `/api/v1/developer/visitors`, with its own CRUD.
  - Credentials: NFC, PIN, QR, license plate. Visitors have time-bound access.
  - Out of scope for member sync.
- **User group** (#3.11-3.22, Access 2.2.6+)
  - Fields: `id`, `name`, `full_name`, `up_id`, `up_ids[]`.
  - Groups are hierarchical via `up_id`.
  - Members are added with `POST /user_groups/:id/users`.
  - A group can carry access policies (`PUT /user_groups/:id/access_policies`). Members get those policies.
  - `GET /users/:id/access_policies?only_user_policies=true|false` toggles direct-only versus including policies inherited via groups.
- **Access policy** (#5)
  - Fields: `id`, `name`, `resources[]` of `{type: door|door_group, id}`, and `schedule_id`.
  - This is the unit of "who can open what, when": one policy = a set of doors or door groups + one schedule.
- **Schedule** (#5.13)
  - Fields: `id`, `name` (must be globally unique), `is_default`, `type`.
  - `weekly` holds per-weekday arrays of `{start_time, end_time}` as `HH:MM:SS`. Not specified means open every day.
  - `holiday_group_id` plus `holiday_schedule[]` give the hours that apply on holidays (Access 1.20.11+).
- **Holiday group** (#5.7): a name, a description and holidays. Each holiday has `repeat` and RFC3339 `start_time`/`end_time`.
- **Door group** (#7)
  - Door group types are `building` (all doors, a synthetic group from `/door_groups/topology`) and `access` (custom groups). `group_name` must be globally unique.
  - Doors have `id`, `name`, `full_name`, `floor_id`, `is_bind_hub`, `door_lock_relay_status` and `door_position_status`.
- **Credentials** (#3, #6)

  | Credential | Notes |
  |---|---|
  | NFC card | `token` (hash) plus a display `id`. Only UA cards are supported. Third-party card IDs can be imported. |
  | PIN | Generated server-side and assigned. |
  | Touch Pass | Mobile credential, one per user. Needs purchase and enrolment by the user (#6.11-6.17). |
  | QR | Visitor only in the API. A user QR image download exists (#6.18). |
  | License plate | Assign and unassign (3.3.10+). |
  | Face | Only device settings (#8.3) and a profile picture upload (#3.30). No face enrolment API found. |

- **How "member M may open door X during schedule Y" is expressed**
  1. Create an access policy with `resources=[door X (or a door group)]` and `schedule_id=Y`.
  2. Attach the policy to the user, or to a user group the user belongs to.
  3. The user must be `ACTIVE` and hold at least one credential.

  There is no per-user schedule or per-door ACL. The policy is the atom.

## 2. Authentication and transport

- **Base URL:** `https://<console-ip-or-host>:12445/api/v1/developer/...` (#1.2, #2.6). The PDF says "UniFi Access application 1.9.1 or later" is required (#1.5).
- **Token:**
  - Created in UniFi Portal, then the console, then Access > Settings > General > Advanced > API Token > Create New (#1.1).
  - You give a key name, a validity period and permission scopes.
  - It is shown **once**. Expired or deleted tokens return an auth error.
  - Sent as `Authorization: Bearer <token>`.
- **Scopes** (the "Permission Key" on each endpoint):

  | Scope | Use |
  |---|---|
  | `view:user`, `edit:user` | Users and user groups |
  | `view:policy`/`edit:policy` | Policies and schedules. Only `edit:policy` is shown in the PDF text I read. `view:policy` is **INFERRED**. |
  | `view:space`, `edit:space` | Doors. `edit:space` also covers remote unlock. |
  | `view:credential`, `edit:credential` | Credentials |
  | `view:system_log` | System logs |
  | `edit:webhook`, `edit:api_server`, `view:device` | Webhooks, API server and devices |

  Minimum for a member-sync adapter: `view:user`, `edit:user`, `view:policy`, `view:space`, `view:credential`, `edit:credential`, and `view:system_log` for observation. **Never grant `edit:space`**: it carries unlock.
- **TLS:**
  - The server certificate is "self-generated and untrusted" (#2.6). All PDF examples use `--insecure`.
  - An operator-supplied key and cert can be installed via `POST /api_server/certificates` (multipart `key`, `cert`; Access 2.2.10+). `DELETE` removes it and needs an app restart (#12).
  - **INFERRED:** the adapter should pin the console certificate's SHA-256 fingerprint, or trust a CA bundle after installing a proper cert. Never `InsecureSkipVerify`. The PDF says nothing about pinning.
- **Rate limits:** not documented. HTTP 429 exists in the status table (#2.2) and the PDF only advises the integrator to rate-limit. The actual thresholds are **UNVERIFIED**.
- **Response envelope** (#2.8):
  - Shape: `{"code":"SUCCESS","msg":"success","data":...}`. Lists add `"pagination":{page_num,page_size,total}`.
  - HTTP statuses: 200, 400, 401, 402 ("request failed"), 403, 429, 5xx.
  - Errors have string codes in `code`, e.g. `CODE_PARAMS_INVALID`, `CODE_RESOURCE_NOT_FOUND`, `CODE_UNAUTHORIZED`, `CODE_OPERATION_FORBIDDEN`, `CODE_USER_NAME_DUPLICATED`, `CODE_CREDS_NFC_HAS_BIND_USER`, `CODE_CREDS_PIN_CODE_CREDS_ALREADY_EXIST`, `CODE_ACCESS_POLICY_SCHEDULE_NOT_FOUND`, `CODE_DEVICE_DEVICE_OFFLINE`.
  - Success can come back as 200 while `data` is `null` or the string `"success"`. Branch on `code`, not on HTTP status alone.

## 3. Provisioning endpoints

All paths are under `/api/v1/developer`, for example `POST /api/v1/developer/users`.

| Op | Method and path | Notes |
|---|---|---|
| Create user | `POST /users` | Body: `first_name`\*, `last_name`\*, `user_email`, `employee_number`, `onboard_time`. Returns `{id, first_name, last_name, user_email}`. |
| Update user | `PUT /users/:id` | All fields optional: names, `user_email`, `employee_number`, `onboard_time`, **`status`** (`ACTIVE`/`DEACTIVATED`). Returns `data:null`. |
| Fetch user | `GET /users/:id?expand[]=access_policy` | Includes credentials, `status` and `access_policy_ids`. |
| List users | `GET /users?page_num=1&page_size=25&expand[]=access_policy` | Paginated with `pagination.total`. Page size ceiling and ordering are **UNVERIFIED**. |
| Search users | `GET /users/search?keyword=&user_id=&status=&only_admin=&page_num=&page_size=` | Access 3.1.30+. Status filter is repeatable. What `keyword` matches is **UNVERIFIED**: the PDF text does not say whether it covers `employee_number`. |
| Delete user | `DELETE /users/:id` | Access 3.1.30+. **Only users already disabled/deactivated may be deleted.** |
| Set user's policies | `PUT /users/:id/access_policies` body `{"access_policy_ids":[...]}` | **Full replacement.** `[]` clears all. Idempotent set semantics. |
| Read user's policies | `GET /users/:id/access_policies?only_user_policies=` | Access 2.2.6+. |
| Group CRUD | `POST/GET /user_groups`, `GET/PUT/DELETE /user_groups/:id` | `name`, `up_id`. Create's sample response shows no `id`, so create-then-find-by-name is needed (**INFERRED** from the sample). |
| Group membership | `POST /user_groups/:id/users` (body is a **bare JSON array** of user IDs), `POST /user_groups/:id/users/delete` (unassign) | Add and remove are incremental. |
| Group members | `GET /user_groups/:id/users` (direct only), `GET /user_groups/:id/users/all` (incl. subgroups) | |
| Group policies | `PUT /user_groups/:id/access_policies` body `{access_policy_ids}`; `GET` the same path | Replacement semantics are **INFERRED** from the user variant. |
| Policies | `GET/POST /access_policies`, `GET/PUT/DELETE /access_policies/:id` | |
| Schedules | `GET/POST /access_policies/schedules`, `GET/PUT/DELETE .../schedules/:id` | |
| Holiday groups | `GET/POST /access_policies/holiday_groups`, `GET/PUT/DELETE .../:id` | |
| Doors | `GET /doors`, `GET /doors/:id` | |
| Door groups | `GET /door_groups`, `GET /door_groups/topology`, `GET/PUT/DELETE /door_groups/:id`, `POST /door_groups` | |

Credentials:

- **NFC enrolment is physical.**
  1. `POST /credentials/nfc_cards/sessions {device_id, reset_ua_card}` wakes a reader and returns `session_id`.
  2. Someone taps the card on that reader.
  3. `GET /credentials/nfc_cards/sessions/:id` is polled until it returns `{token, card_id}`.
  4. `PUT /users/:id/nfc_cards {token, force_add}` binds the card.
  5. `DELETE /credentials/nfc_cards/sessions/:id` cleans up.
- **Other NFC ops:**
  - Unbind: `PUT /users/:id/nfc_cards/delete {token}`.
  - Delete card: `DELETE /credentials/nfc_cards/tokens/:token` (1.22.16+).
  - List and read: `GET /credentials/nfc_cards/tokens[/:token]`.
  - Rename: `PUT .../tokens/:token {alias}` (3.1.30+).
  - Import a third-party card CSV: `POST /credentials/nfc_cards/import` (3.3.10+).
- **PIN can be fully remote.** `POST /credentials/pin_codes` returns a generated PIN. Then `PUT /users/:id/pin_codes {pin_code}` assigns it and `DELETE /users/:id/pin_codes` removes it. The user's fetched `pin_code.token` is a hash, so the PIN cannot be read back later.
- **Touch Pass:** assign with `PUT /users/:user_id/touch_passes/:touch_pass_id`. Purchase and enrolment are a user and app flow (#3.25-3.27, #6.12-6.17).

## 4. Idempotency and identity

- **External ID field:** `employee_number` (string) exists on create and update. It is the natural binding for a Syndra subject.
  - **UNVERIFIED:** whether it is unique-constrained. The error list has no `EMPLOYEE_NUMBER_DUPLICATED`, but absence in the doc is not proof.
  - **UNVERIFIED:** whether the list or search endpoints can filter by it. The PDF lists none, so assume you need to page the full list and match client-side.
  - The create-field text says "Omit this parameter if it doesn't need to be updated", and the update sample sends `""`. Clearing semantics are ambiguous.
- **Email:** `user_email` is optional, needs Access 1.22.16+, and is unverified as unique.
- **Duplicates:** the only duplicate error documented is `CODE_USER_NAME_DUPLICATED` ("provided name already exists"). **UNVERIFIED** whether it fires on `first_name`+`last_name` collisions at create. If it does, two members with the same name would collide.
- **Create is not idempotent by design.** There is no client-supplied ID or idempotency key. **INFERRED:** Syndra must bind by listing and matching `employee_number` first, and persist the returned Access `id` immediately after create. Crash windows between create and persist are the main hazard.
- **Replacement setters are idempotent:** `PUT .../access_policies` and `PUT .../users/:id` with the same body.
- **NFC duplicate bind:** `CODE_CREDS_NFC_HAS_BIND_USER`, unless `force_add` is true. A duplicate PIN gives `CODE_CREDS_PIN_CODE_CREDS_ALREADY_EXIST`.

## 5. Observability

- **System logs:** `POST /system/logs?page_num=&page_size=` (scope `view:system_log`).
  - Body: `{topic, since, until, actor_id}` with epoch-second times.
  - Topics: `all, door_openings, critical, updates, device_events, admin_activity, visitor`.
  - Hits have `actor`, `event {type, display_message, result ACCESS|BLOCKED, published(ms), reason}`, `authentication {credential_provider, issuer}`, `target[]`.
  - `admin_activity` covers access-policy updates, settings changes and user management. Use it for **drift detection** (a non-Syndra actor changed users or policies). Whether it names the changed user and the actor's token is **UNVERIFIED**.
  - `door_openings` gives usage. `POST /system/logs/export` exports.
- **Webhooks:**
  - Register with `POST /webhooks/endpoints {name, endpoint, events[], headers}` (2.2.10+). `GET`, `PUT` and `DELETE` exist. The response carries a per-endpoint `secret`.
  - Delivery is signed with a `Signature: t=<epoch>,v1=<hex>` header. The PDF's Go sample uses HMAC-SHA256 with the secret and a 5 s timeout. The exact signed string (likely `t.payload`) was not confirmed. **UNVERIFIED**, so check the sample's `validatePayload` before implementing.
  - Events: `access.door.unlock`, `access.doorbell.incoming`, `access.doorbell.completed`, `access.doorbell.incoming.REN`, `access.device.dps_status`, `access.device.emergency_status`, `access.unlock_schedule.activate`/`.deactivate`, `access.temporary_unlock.start`/`.end`, `access.visitor.status.changed` (the last four need 3.3.10+).
  - **There is no webhook event for user, policy or credential changes.** Drift must be found by polling users, or by polling `admin_activity` logs.
- **WebSocket:** `wss://host:12445/api/v1/developer/devices/notifications` (1.20.11+). It carries doorbell and device events and is not useful for provisioning.

## 6. Lifecycle and safety

- **Deactivate:** `PUT /users/:id {"status":"DEACTIVATED"}`. Delete requires prior deactivation.
  - The PDF does **not** say whether deactivation immediately revokes door entry or removes credentials and policies.
  - **INFERRED**, from the product behaviour of "disabled users cannot unlock", that entry stops. This is **UNVERIFIED** and a lab test is required.
  - **UNVERIFIED:** whether credentials and policy assignments are retained across deactivate then reactivate. It is probably retained, since the policy and credential lists remain on the object.
- **Delete is destructive.** It needs a prior deactivation and removes the user. Whether it frees the NFC card for reuse is **UNVERIFIED**. Syndra should deactivate only, and delete by explicit operator action.
- **Remote unlock:** `PUT /doors/:id/unlock` (`edit:space`, optional `actor_id`, `actor_name`, `extra`). Also `PUT/GET /doors/:id/lock_rule` (temporary lock or unlock), `PUT/GET /doors/settings/emergency` (lockdown or evacuation), and doorbell trigger `POST /devices/:id/doorbell`.
  - **Syndra must never call these.**
  - Do not grant `edit:space` to the adapter's token. The token should be unable to unlock even if the adapter has a bug.
- **Version detection:** there is no version endpoint in the PDF. Gate on the requirements per endpoint:
  - 1.9.1: API exists.
  - 1.22.16: email, card delete.
  - 2.2.6: user groups.
  - 3.1.30: user delete, search, card update.
  - 3.3.10: license plates, more webhooks.
  - 4.x: Hub Gate and doorbell trigger.

  **INFERRED:** probe capability by calling an endpoint and treating `CODE_*` or 404 as unsupported.
- **Other limits:** the API is **unavailable after upgrading to UniFi Identity Enterprise** (error `CODE_OTHERS_UID_ADOPTED_NOT_SUPPORTED`; #1.5). Check whether the makerspace may migrate. The Identity-related endpoints (#10) are separate and not needed here.

## 7. Mapping sketch (INFERRED design, not from docs)

Syndra desired state per person per target: level-triggered fields, a set of groups, quotas, and an absent field means unmanaged.

| Syndra | Access | Notes |
|---|---|---|
| Subject binding | `employee_number` = Syndra subject id, plus a persisted Access `id` | Match by `employee_number`, never by name. |
| `enabled` | `status` ACTIVE or DEACTIVATED | Lifecycle field and **withdraw-only**: Syndra may deactivate, but reactivation is a deliberate act. Not a quota. |
| groups (set) | Access policy ids on the user, via `PUT /users/:id/access_policies` | Replacement semantics make a level-triggered full-set reconcile straightforward. Policies Syndra does not manage must be preserved (read first, merge, then write). |
| Alternative for groups | User-group membership | Needs per-group incremental add and remove, and a policy-to-group binding done in the Access UI. |
| Doors, schedules | Defined in Access by an admin | Syndra only references policy IDs, never creates schedules or policies. |
| Quotas | None | Access has no quota concept. |
| Credentials | Not managed | NFC enrolment is physical. PIN could be remote but is a member-facing secret. |

- **Preferred mapping:** a Syndra group maps to an Access **access policy** (door set + schedule). That is the unit the API sets atomically per user. User groups are the alternative when the makerspace already organises doors by group.
- **Operator-facing choice:** the add-on catalogue should list policies by name and ID so the operator picks which ones Syndra owns.
- **Mismatches and open questions:**
  1. Whether `employee_number` is unique and filterable. Lab test required. If not filterable, the adapter must page all users (`total` is exposed).
  2. A user created without an NFC card cannot enter until enrolment. Syndra's "converged" must not claim door entry works. Report "provisioned, no credential" as a distinct state.
  3. Whether deactivation revokes entry immediately, and whether it keeps policies.
  4. The ambiguity in `PUT /users/:id/access_policies`: it replaces the full set, so unmanaged policies assigned in the Access UI would be erased if the adapter writes only its own. Always merge.
  5. Whether the duplicate-name error blocks two members who share a name.
  6. Rate-limit thresholds, and the page-size ceiling.
  7. Drift detection depends on polling, since there are no user-change webhooks.
  8. Certificate pinning against a self-signed console certificate.
  9. `PENDING` status: must be treated as "present, not yet active", and the adapter must never set it.
