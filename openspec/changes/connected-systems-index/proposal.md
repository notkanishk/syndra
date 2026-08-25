# The target plane has a seat in the rail whether or not a target is registered

## What is wrong

Two defects, found together on the production deployment at
`syndra.makerspace.tools` the day after the add-on platform merged to `main`.

### 1 — A deployment with no add-on says nothing about add-ons

`targetNav` returned `ADVANCED_NAV` untouched when the roster was empty:

```ts
export function targetNav(targets: string[]): NavEntry[] {
  if (targets.length === 0) return ADVANCED_NAV;
  ...
```

So a deployment that had registered no target had no add-on anywhere in the
product — not a row, not a page, not a sentence. The operator who had just
shipped the platform opened the deployment, found the rail exactly as it was
before, and read that as *the feature did not ship*. It had; `ADDON_TARGETS`
was empty.

Those are different facts and only one of them was true. The product had no
way to say the true one.

This is the same principle `lib/nav.ts` already states and enforces everywhere
else: a section with nothing in it **keeps its seat** and renders a hollow
zero, because disappearing rows teach people not to trust the rail. The target
plane was the one place that rule was not applied — and it is the place where
the absence is most easily misread, because the alternative explanation
("this build does not have it") is plausible.

### 2 — A project with no roles broke the projects table

`/projects` rendered

> No roles yet — nothing here can be granted

*inside* the Roles column, which is `w-[60px]` and right-aligned. A
43-character sentence in a 60px box wraps to six lines: the row grew to roughly
four times the height of its neighbours, the sentence rendered as a ragged
right-aligned column of single words, and the table read as broken on any
deployment holding one role-less project.

The sentence itself is right — a bare `0` beside a member count reads as a
project that is merely quiet, and the original comment says so. The narrow
right-aligned count column is simply not the place to say it.

## What changes

**One row, one page, one relocation.**

- `Connected systems` becomes a static leaf of the System group in
  `ADVANCED_NAV`, present on every operator deployment. `targetNav` appends the
  per-target rows *after* it, unchanged.
- `/system/targets` is a new index: one row per registered add-on with its
  reachability and operation count, or — when none is registered — a sentence
  saying so and naming `ADDON_TARGETS` as what would connect one.
- The role-less sentence moves out of the count column and under the project
  name, which is the column with room. The count column stays a count.

## What does not change

- Registration remains **deployment configuration**. The index lists what was
  registered, never what is reachable and never what this operator can see.
- Basic keeps its four destinations. System is an Advanced section and the new
  row is inside it; Basic gains nothing.
- No per-target behaviour, route, or contract moves. `/system/targets/{target}`
  is untouched.
