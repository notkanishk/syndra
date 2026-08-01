# Tasks — operator runbook surfaces

## Track 1 — Demo residue

- [x] ORS-01 `demo.ProjectIDs()` / `demo.UserIDs()` expose the seeder's fingerprint from the catalog, so adding a fixture widens every check that reads them.
- [x] ORS-02 `db.CountDemoResidue` — one query across the eight tables the seeder writes.
- [x] ORS-03 `GET /system/mode` reports `seed_residue` and `reset_command`. A failed count logs and reports zero; it never fails the probe and never reports a positive it did not measure.
- [x] ORS-04 `docker-compose.yml` drops `MKAUTH_SEED_DEMO=${MKAUTH_SEED_DEMO:-true}`; the backend's auto-detection governs again. `.env.example` says why leaving it unset is correct.
- [x] ORS-05 Degraded banner keys off `seed_residue`, states the count, and distinguishes "seeding still on" from "leftovers from an earlier run".

## Track 2 — Reset

- [x] ORS-06 `scripts/reset-data.sh demo|all`, dry-run by default, per-table counts, typed confirmation, single transaction, Redis flush.
- [x] ORS-07 `demo` mode names real accounts that lose access to a cascading fixture — `user_bundle_assignments` cascades on bundle delete, so those rows never appear in the per-table counts.
- [x] ORS-08 `make reset-demo-data` / `make reset-all-data`, both dry-run unless `APPLY=1`.
- [x] ORS-09 Drift guard: `demo` package test parses the script's own id lists and fails when they diverge from the catalog. Mutation-checked by removing `finance` from the script.

## Track 3 — Instructions

- [x] ORS-10 `CommandBlock` — copyable command, ordered follow-up steps, three tones including one for use on the amber banner. Clipboard failure is silent by design: the app runs on a LAN IP where the API is unavailable, and the text stays selectable.
- [x] ORS-11 Identity provider: signing-key panel restored — status as a sentence, `rotate_command` from the backend, env swap, restart, verification, and what happens during the swap window. `register` replaces `rotate` when no key is installed.
- [x] ORS-12 Hardware sync: the restart-looping sync container explained, with the log check, the stop command, and what to set when an LLDAP server exists.
- [x] ORS-13 Demo-residue banner carries the reset command inline, with the dry-run default stated.

## Track 4 — Verification

- [x] ORS-14 Backend: residue reported when seeding is off, count failure reports zero without failing the probe, fixture accessors cover every catalog entry, script/catalog id parity.
- [x] ORS-15 Frontend: banner survives the restart that used to silence it, uses the backend's command, six signing-key cases. Mutation-checked by removing the panel — six failures.
- [x] ORS-16 `go test ./... && go vet ./...`, `bun run test && bun run lint && bun run build` green.
- [x] ORS-17 Both reset modes dry-run against the live deployment. Caught and fixed two defects: `drift_items`/`zitadel_grants_index` use `project_id`, not `zitadel_project_id`; and a real account assigned to a demo bundle was being deleted without being counted.
- [x] ORS-20 The dry run rehearses the plan against the database inside a transaction ending in `ROLLBACK`, and prints the SQL. Counting rows never builds the statements that delete, so a plan that cannot parse still reported a row total and failed only after the operator typed the confirmation — which is how `TRUNCATE a b c` reached `--apply`. `"${ALL_TABLES[*]}"` joins on a space; the list is now comma-joined. Mutation-checked by restoring the space join: the rehearsal fails, prints the server's error, and exits before the prompt.
- [x] ORS-21 `demo --apply` exercised against the live deployment: 48 rows removed in one transaction, real accounts and their grants intact, the mode now reporting clean.

## Open

- [ ] ORS-18 `all --apply` has still never been committed against a real database. Its plan is rehearsed and rolls back clean, and `demo --apply` has run for real, but the `TRUNCATE ... RESTART IDENTITY CASCADE` itself has never been allowed to commit.
- [ ] ORS-19 `CountDemoResidue` matches audit rows by actor/target user id, but the seeder writes audit actors as display names (`alice.rivera`), so two of its four seeded audit rows are invisible to the count. Harmless while other tables carry residue; it would under-report on a database where only audit rows remain.
