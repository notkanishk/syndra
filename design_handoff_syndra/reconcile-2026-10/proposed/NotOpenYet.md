<!--
Landed 14 Oct 2026 from the design project, handoff/2026-10-reconcile/proposed/NotOpenYet.md, verbatim; updated 15 Oct with A-03.
-->
# NotOpenYet · a section someone can't use yet

One pattern, everywhere a whole section exists but can't be used until a step is taken elsewhere. Owner, 14 Oct: "make the disabled sections consistent across the app."

## Anatomy
1. **Container:** 1px dashed `--line-strong`, the radius of where it sits (`--radius-inner` inside a card, `--radius-card` on a page), no fill.
2. **Lead line, first inside it, full strength:** neutral StatusDot · the title in ink, 600 (15px in a card; 19px display on a page), saying what opens it ("Set your password to open these") · one muted line on why · at the right, a violet link to the step, ending in ↑ or → ("Set it above ↑", "Choose it above ↑").
3. **Contents:** drawn as they will be, so the person sees what they are about to get, at opacity 0.45, `inert` (no focus, no pointer, no selection), `aria-disabled="true"`. No control inside works, copy buttons included.

The lead line carries the meaning at full contrast; the dimmed contents are a preview, exempt as inactive UI. When the step is taken, the dashes and the lead line go and the contents come up to full strength (Motion: settle, 200 ms).

## Not this pattern
- **One row that can't be chosen** (a locked role in the RolePicker, a locked row in Roles that wait): ticked grey with the why, per row.
- **Stale or last-known data** (a NAS that isn't answering): a Notice above and 0.62 on what was read; still readable, still selectable.
- **One control that can't act now** (password while TrueNAS is offline; a mapping's home folder before the system has a place for homes): the control is replaced by one sentence saying why, with the link to the step when there is one.

## Where
- route-storage · needs-password, reenrol, restored: Your folders.
- route-system-targets-id · g3 (no Zitadel project): everything below the project step.
