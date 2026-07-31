> **Status:** basic-advanced-ia delta — the token shape becomes an operator artefact | [< Index](../../../../INDEX.md)

# Requirement: Application Claim Selection & Shaping (delta)

## MODIFIED Requirements

### Requirement: The issued token and the previewed token MUST be produced by the same shaper

Claim shaping MUST live in exactly one place (`internal/claims`) and MUST be applied by both the Zitadel Actions v2 data-plane handler and the operator-facing simulator, from the same resolved profile set and the same cached facts.

A preview computed by different code from the token it claims to preview is a preview of nothing. Before this change the simulator emitted `{iss, sub, aud, source, project, <claim_name>: formatted_roles}` while the Actions v2 path emitted the raw compiled cache map and never read `claim_name` or `format_type` at all — so the screen operators used to debug "my app isn't seeing the roles it expects" showed a payload no application had ever received.

#### Scenario: A format change is visible in the preview and in the token

- **GIVEN** project `pLaser` has a claim profile with `claim_name = "mkauth.laser.roles"` and `format_type = "array"`
- **WHEN** an operator changes `format_type` to `csv` and saves
- **THEN** `GET /applications/{id}/simulate` MUST return `"mkauth.laser.roles": "trained,maintainer"`
- **AND** the next `POST /api/action/inject` for a user in that project MUST append the identical key and value

#### Scenario: The preview never invents an envelope

- **WHEN** a simulation is run for any application
- **THEN** the returned `custom_claims` MUST contain only keys the configured profiles emit
- **AND** MUST NOT contain `iss`, `sub`, `aud` or any other field the Actions v2 path does not append

### Requirement: Claim shaping MUST be resolved at token-issue time, not baked at compile time

`CompileUserCache` MUST persist facts (`roles`, `user_id`, `project_id`, `compiled_at`, and the profile attributes `email`, `name`, `title`, `team`). It MUST NOT persist a finished claim map.

Baking the shape at compile time would mean a claim-name or format edit applied only to users whose cache happened to be recompiled afterwards. An edit that takes effect per-user at random is worse than no edit at all.

Profile resolution at read time MUST be served through a Redis read-through cache keyed `claim_shape:<projectID>`, and every profile write MUST invalidate that key so an edit is never one TTL late.

#### Scenario: An edit lands on the next token for every user

- **GIVEN** users A and B both hold roles in `pLaser`, and only A's cache has been recompiled today
- **WHEN** an operator changes the project's claim name
- **THEN** the next token issued for A and the next token issued for B MUST both carry the new key

#### Scenario: A profile lookup failure degrades to the default shape, never to silence

- **WHEN** the claim-profile lookup fails for a project
- **THEN** the data plane MUST emit the roles under the built-in default claim name
- **AND** MUST NOT emit an empty claim set, because roles under the wrong key are a degraded token while no roles at all is a locked door

## ADDED Requirements

### Requirement: A claim profile MUST be able to project attributes and constants, not only roles

A profile consists of a roles claim (key + format), a map of claim key → profile attribute, and a map of claim key → constant value. Selectable attributes MUST be resolvable from the compiled cache entry alone, with no directory call at token-issue time: `user_id`, `project_id`, `email`, `name`, `title`, `team`, `role_count`, `compiled_at`.

An attribute the facts cannot supply MUST be omitted, never emitted as null: a claim reading `"email": null` tells an application the user has no email, which is a different and worse lie than the claim being absent.

#### Scenario: An attribute claim rides along with the roles

- **GIVEN** a profile with `attribute_claims = {"mkauth.laser.team": "team"}`
- **WHEN** a token is issued for a user whose cached facts carry `team = "Fabrication"`
- **THEN** the envelope MUST contain `mkauth.laser.team = "Fabrication"`

#### Scenario: An unresolvable attribute is omitted

- **GIVEN** the same profile and a user whose cached facts carry no team
- **THEN** the envelope MUST NOT contain the `mkauth.laser.team` key at all

### Requirement: An application override MUST be additive, and the token MUST carry every key on the project

A token issued for a project MUST carry the project default AND every application override configured on that project; each application reads its own key. The operator UI MUST state this arrangement and MUST attribute every emitted key to its owner, rather than leaving an operator to discover a sibling application's key by decoding a production token.

This is forced by the wire contract: the Zitadel Actions v2 function payload carries no client or application identifier — the documented `preaccesstoken` fields are `function`, `userinfo`, `user`, `user_metadata`, `org`, `user_grants` — so an override cannot be resolved by "which application asked".

#### Scenario: Both keys are present in one token

- **GIVEN** `pLaser` has a default claim `mkauth.laser.roles` (array) and application `app_badge` overrides it with `badge.roles` (csv)
- **WHEN** a token is issued for a user holding `trained` and `maintainer` in `pLaser`
- **THEN** the envelope MUST contain `mkauth.laser.roles = ["trained","maintainer"]`
- **AND** MUST contain `badge.roles = "trained,maintainer"`

#### Scenario: The simulation says which keys this application actually reads

- **WHEN** the simulation is run for `app_badge`
- **THEN** `owned_claims` MUST list only `badge.roles`
- **AND** `claim_owners` MUST attribute `mkauth.laser.roles` to the project default

### Requirement: Claim keys MUST be unique across every project

Saving a profile MUST be rejected when any key it emits — roles, attribute or static — is already emitted by another profile on any project, and the error MUST name the colliding key and its current owner.

A JWT is flat and holds one value per name, and a user with grants in several projects receives one token: a duplicate key means one application silently reads another's roles.

#### Scenario: A key owned by another project is refused

- **GIVEN** project `pStudio` emits `shared.roles`
- **WHEN** an operator saves `shared.roles` as the claim name for `pLaser`
- **THEN** the request MUST be rejected with a message naming `shared.roles`
- **AND** the existing profile MUST be unchanged

#### Scenario: Re-saving a project's own key is not a collision

- **GIVEN** `pLaser` already emits `mkauth.laser.roles`
- **WHEN** an operator saves `pLaser` again with the same claim name and a different format
- **THEN** the save MUST succeed

### Requirement: Multi-project tokens MUST NOT namespace configured keys

Keys are operator-authored and validated unique, so the previous unconditional `mkauth.<projectID>.<claim>` prefixing on multi-project tokens MUST NOT be applied to configured keys — it guaranteed no application ever received the key it asked for.

Where two projects nonetheless emit the same key — which in practice means two projects nobody has configured, both falling back to the built-in default — the merge step MUST namespace the colliding keys per project and log the collision, so both role sets survive rather than one silently overwriting the other.

#### Scenario: Two configured projects keep their own keys

- **GIVEN** `pPrint` emits `pPrint.roles` and `pDoor` emits `pDoor.roles`
- **WHEN** a token is issued for a user holding roles in both
- **THEN** the envelope MUST contain exactly `pPrint.roles` and `pDoor.roles`, unprefixed

#### Scenario: Two unconfigured projects do not overwrite each other

- **GIVEN** neither `pPrinting` nor `pDoors` has a claim profile, so both default to `roles`
- **WHEN** a token is issued for a user holding roles in both
- **THEN** the envelope MUST contain `mkauth.pPrinting.roles` and `mkauth.pDoors.roles`
- **AND** both role sets MUST be present
