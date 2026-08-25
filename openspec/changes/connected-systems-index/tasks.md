# Tasks

## 1. The rail

- [x] 1.1 Move `Connected systems` into `ADVANCED_NAV`'s System group as a
      static leaf at `/system/targets`, with a pattern that matches the index
      exactly and never a target's detail route.
- [x] 1.2 Reduce `targetNav` to appending the per-target rows after it. An
      empty roster returns `ADVANCED_NAV` itself — nothing is derived to
      produce the row.
- [x] 1.3 Tests: the group's labels with an empty roster; `crumbsFor` naming
      the index (which a derived row could not have); the index not swallowing
      `/system/targets/{target}`. Mutation-checked by dropping the leaf's
      `pattern` — `does not let the index swallow a target's own route` fails.

## 2. The index page

- [x] 2.1 `/system/targets` — one row per registered add-on: name, id,
      reachability, operation count, link to the target's own page.
- [x] 2.2 The empty state names `ADDON_TARGETS` and points at DEPLOY.md, and
      says what Syndra is and is not reconciling while none is registered.
- [x] 2.3 Reachability is four distinct readings — transport failed, calls
      suspended, answering, no manifest yet — and an em dash rather than `0`
      operations for a target that has never answered.
- [x] 2.4 Tests: all five readings, and that no count is claimed when the
      roster is empty.

## 3. The projects table

- [x] 3.1 Move the role-less sentence under the project name; leave the count
      column a count.
- [x] 3.2 `shrink-0` on the fixed columns in both the header and the row, so
      the two cannot drift apart, and `min-w-0` on the name column.
- [x] 3.3 Regression tests, including one that walks the sentence's ancestors
      and fails if any of them is the 60px column. Mutation-checked by putting
      the sentence back where it was — four of five fail.

## 4. Verification

- [x] 4.1 `bun run test` (783), `bun run lint`, `bun run build`.
- [x] 4.2 Looked at, in a browser, at 1440 / 800 / 390: the defect before the
      fix, the fix, the index with a target, and the index with none.

## 5. Deployment

- [x] 5.1 `ADDON_TARGETS`, `ADDON_TRUENAS_BASE_URL` and the TrueNAS block added
      to the production `.env`; the NAS key mounted from
      `secrets/truenas-addon/api.key` at 0640 root:65532 rather than passed in
      the environment.
- [x] 5.2 `COMPOSE_PROFILES=truenas`, so the deploy workflow's plain
      `docker compose up -d --build` carries the add-on.
- [x] 5.3 `scripts/smoke-test-addon.sh truenas` — legs 1 and 2 pass; leg 3 is
      the NAS itself and is covered by the add-on reading `TrueNAS-25.10.5` at
      startup and the reconcile reporting `bound=0 queued=0 unmanaged=2`.

## Open

- [ ] 6.1 The four-column tables (`/projects`, `/roles`) are cramped between
      the tablet and desktop breakpoints: the fixed columns and the 252px rail
      leave the name column a few dozen pixels, and names truncate to one
      letter. Pre-existing, systemic, and not what this change is about — the
      table layout wants the `desktop:` breakpoint rather than `tablet:`.
- [ ] 6.2 Nothing is bound on the production target, so the index has still
      only been seen in its registered-and-empty state against real data.
