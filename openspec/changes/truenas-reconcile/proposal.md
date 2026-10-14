## Why

The October design reconcile (`design_handoff_syndra/reconcile-2026-10/`) settled how the TrueNAS add-on should look and behave, and found three ways the shipped product diverges from what the system can honestly claim:

- A member can set a password on a paused system, and nothing receives it.
- "Offline" is declared on a single slow answer.
- Mappings accept a free-typed group from any project. That includes TrueNAS's built-in admin groups, so one typo or one wrong pick hands NAS administration to everyone holding a role.

This change finishes the add-on against those boards. Roadmap: **Phase 4, the Target Plane**.

## What Changes

**Storage (B1, B2).**
- `POST /me/targets/{t}/credential` refuses a paused system with `409 TARGET_PAUSED` before dispatching. The existing `503 TARGET_UNREACHABLE` stays.
- A shared target health monitor probes each active target every 30s. It declares offline after 3 consecutive failures and records `checked_at` and `reachable_since`.
- `GET /me/targets` and the operator pages read the monitor, so both pages answer with one rule.
- The member storage page is rebuilt to `route-storage`:
  - one block per share
  - the password as a next step
  - one Notice per card
  - the connection guide
  - held and no-host states

**Connected systems (B3).**
- **BREAKING (data):** each target gets one Zitadel project (`targets.project_id`, unique). Migration 000051 infers it from existing mappings, and refuses to guess when a target's mappings span several projects.
- A composite key binds every mapping to its target's project.
- `PUT /targets/{t}/project` sets the project. It is refused with `409 MAPPINGS_EXIST` or `409 PROJECT_TAKEN`.
- Every write against a target, except lifecycle, is refused with `409 PROJECT_REQUIRED` until a project is set. The `zitadel` target is exempt.
- Add-on manifests declare `mappable_fields` (name, label, enumerate).
- `GET /targets/{t}/mappable-values` serves the add-on's list. While the target can't answer, it serves the last list it read, marked with its age.
- The mapping dialog picks a role from the target's project, and a group from that list. The free-text field is removed.
- The project gate (3a), the setup strip, and the project read-only on Connected systems.

**TrueNAS add-on.**
- Built-in groups are excluded from enumeration and refused on the write-path value check.
- A group read that hits the read cap is an error, not a silent partial list.

**Assign an account (#57).** A Modal with a people search replaces the Syndra-ID text field.

## Capabilities

### New Capabilities
- `target-health`: the confirmed-failure rule, the monitor, and what each page may claim from it.
- `target-project-scope`: one Zitadel project per target, the write gate, and the migration rule.
- `mappable-values`: manifest-declared mappable fields and the last-known value list.
- `member-storage`: what the member's storage page refuses, shows, and never claims.

### Modified Capabilities
- `truenas-addon`: built-in groups excluded from enumeration and value checks; a truncated group read is refused.

## Impact

- **Control plane:** a new migration (000051), new target routes, a write gate in the target write transactions, and the health monitor in `services/periodic`.
- **Data plane:** none. Token shape is untouched (`internal/claims`).
- **Bridge (target plane):** the manifest contract gains `mappable_fields`, read by the backend with a fallback for add-ons that don't declare it. The TrueNAS add-on's enumeration filters built-in groups.
- **UI:**
  - `components/storage/MyStorage.tsx`
  - `app/system/targets/**`
  - `AddMappingDialog`
  - `TargetOverview` (`AdoptPanel`, census, setup strip)
  - new DS components: `Notice`, `ConnectionGuide`
- **Design:** `to-design/Q-01` is open for the operator-side states between "answering" and "offline", the two meanings of "paused", and the case where a NAS lists no groups of its own.
