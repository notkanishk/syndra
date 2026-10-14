# Connected systems: one Zitadel project, and mappings from the add-on's lists (owner, 8 Oct 2026)

Boards: route-system-targets-id (project gate, options 1a–1c on explore-system-project), route-system-targets-id-mappings, route-systems.

## 1 · Each system has one Zitadel project
- Add `project_id` to the target's record. Set it with `PUT /targets/{t}/project {project_id}`. It is a Syndra-side write and changes nobody's access.
- Refuse a change while any mapping exists (`409 MAPPINGS_EXIST`, with the count). The UI says to remove the mappings first. This avoids a silent re-scope of every mapping, where roles from the old project would match nothing.
- Until it is set, refuse every write against the target with `409 PROJECT_REQUIRED`: mappings, adopt, hold, take away, reconcile, lifecycle. Reads continue: health, system health, inventory. The page renders from these reads.
  - Lifecycle (drain / read-only / make active) is **exempt** (owner, 8 Oct). It acts on the add-on, not on anyone's access.
- `GET /api/v1/targets` returns `project: {id, name} | null` per target, so Connected systems shows it read-only. A null project puts "Choose its Zitadel project →" in that row's Needs you.
- `GET /targets/{t}/mappings` returns `409 PROJECT_REQUIRED` while there is no project. The mappings route renders the no-project frame, and the system page offers no link to it.
- **A project serves one system** (owner, 8 Oct). Add a unique constraint on `targets.project_id`. `PUT` refuses a taken project with `409 PROJECT_TAKEN`, naming the system. The picker lists taken projects disabled, with the system's name ("Laser Lab · used by Door controller"), rather than hiding them. It also shows each project's role count, or "no roles yet".
- The gate is drawn as option 3a, on route-system-targets-id frame `noproject`. A violet panel (Notice anatomy, accent tone, Step 1 of 2) holds the picker. Health and lifecycle stay live. The waiting regions render whole at 40% opacity, inert, inside one 1.5px `--line-strong` outline, with one heading-size line at its top. When the project is set, the panel becomes its result (frame `projectset`) for that visit only.

## 2 · A mapping is a role and a group
- Mapping create and edit no longer take `project_id`. It is the target's. The uniqueness constraint becomes `(target, role_key, field)`. The picker offers only roles in that project that are not already mapped.
- **Mappable settings come from the add-on.** The manifest declares them, e.g. `mappable_fields: [{ name: "group", label: "Group", enumerate: "groups.list" }]`. With one field the UI shows its label and no picker. With two or more, a Select appears above the value. Nothing in the frontend lists field names, which keeps the add-on as the only authority (the reason the text box existed).
- **Values come from the add-on.** `GET /targets/{t}/mappable-values?field=group` returns `{ values: string[], read_at, current }`. It is the same enumeration the write-path check already runs (M4), exposed as a read.
  - When the target is paused or not answering, serve the last list Syndra saw, with its `read_at` and `current: false`. The dialog labels it with its time (degraded shows last known data, never a guess).
  - If no list has ever been read, the Select is disabled with "TrueNAS hasn't listed its groups yet", and Add waits.
- Keep the value check on the write path. A group deleted between the list read and the preview still refuses with `422 VALUE_NOT_FOUND`. The dialog re-reads the list and says the group went away. A typed typo can no longer happen, so the near-miss suggestions are gone.
- Keep the fail-open rule: if the check cannot run, the edit passes and reports `checked: false`.

## 3 · Copy owed in code
- Role labels on the mappings page drop the project ("Laser operator", not "Laser Lab · Operator"). The project is named once, in the meta line.
- The system page's census line names the project: "Two roles in Network storage reach TrueNAS through mappings…".

## 4 · The setup strip
Once a project is set, the panel's seat holds a thin Card under the page header. It shows Zitadel project (a link to the project), Role mappings ("2 roles to TrueNAS groups · version 4", or "None yet"), and on the right a violet text link, "Mappings →" or "Add the first mapping →". This is the only link to the mappings route. The census sentence drops its own link, and the meta line drops the project. Data: `target.project` and the mappings summary `{count, version}`, both already read for the page.
