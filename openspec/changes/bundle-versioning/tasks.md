# Tasks — Bundle Versioning

## 1. Schema ✅

- [x] 1.1 Migration `000020`: `bundle_versions`, `bundle_version_roles`, `user_bundle_assignments.version_id`
- [x] 1.2 Backfill v1 per bundle from the working copy; pin every existing assignment to it
- [x] 1.3 `version_id` made NOT NULL only after the backfill — order is the correctness of this migration
- [x] 1.4 Down migration leaves `bundle_roles` alone; guards in `bundle_versions_migration_test.go`

## 2. Repository ✅

- [x] 2.1 `LatestVersion`, `ListBundleVersions` (with holder counts), `GetRolesForVersion`
- [x] 2.2 `GetUserBundleRolesGrouped` — the version-aware closure input, one query
- [x] 2.3 `GetBundleHoldersByVersion`, `GetUserBundleVersions`, `GetStaleHolderCounts`
- [x] 2.4 `PublishVersionAndEnqueue`, `MoveHoldersAndEnqueue` — snapshot + repin + audit + outbox, one tx each
- [x] 2.5 `CreateBundle` publishes an empty v1; every assignment path pins the latest

## 3. Services ✅

- [x] 3.1 Every closure path resolves bundle roles through the pin, not the working copy
- [x] 3.2 `EditBundleWorkingCopy` replaces the two bundle-role cascades and enqueues nothing
- [x] 3.3 `BundleDraft` — the computed unpublished diff
- [x] 3.4 `RehearseBundlePublish` / `PublishBundleVersion` — per-holder plan from each holder's own version
- [x] 3.5 `RehearseMoveHolders` / `MoveHolders`
- [x] 3.6 Revoke-suppression tests ported from the removed cascade onto publish

## 4. API ✅

- [x] 4.1 `GET /bundles/{id}/versions`, `/holders`, `/draft`
- [x] 4.2 `POST /bundles/{id}/publish[?apply=true]`
- [x] 4.3 `POST /bundles/{id}/holders/move[?apply=true]`
- [x] 4.4 Publishing an unchanged bundle is a 409, not a 500
- [x] 4.5 `UserListItem.bundle_versions` for the People filter

## 5. UI ✅

- [x] 5.1 Draft strip on the bundle workspace with the diff and "Publish vN"
- [x] 5.2 `PublishVersionDialog` — the migrate question in compose, rehearsed apply
- [x] 5.3 `BundleVersions` — history, per-version holder counts, who is on what, move actions
- [x] 5.4 Bundle list marks unpublished changes and stale holders
- [x] 5.5 Person page bundle chip names the version and flags a newer one
- [x] 5.6 People filter: bundle → version, with independently clearable chips
- [x] 5.7 Tests: the migrate question has no default; the choice drives the request

## 6. Audit remediation ✅

Found by review after the change landed. The first two were P0.

- [x] 6.1 **Cache compiled claims from the working copy.** `CompileUserCache` now resolves through the pin — an unpublished edit could otherwise reach real tokens on the next rebuild
- [x] 6.2 **Assign projected the working copy while pinning the published version.** Both now come from `LatestVersionRoles`
- [x] 6.3 Bulk assign-bundle rehearsal counted working-copy roles (not in the audit; same bug class)
- [x] 6.4 `draftRoleSet` discarded its read error, so a failed read planned a revoke of everything. Removed; the error propagates
- [x] 6.5 Publish deltas and the snapshot now come from one working-copy read, threaded via `DraftDiff.Working`
- [x] 6.6 Move rehearsal validates version ownership before building a plan, and reads the target version number from the version list rather than inferring "v0" from its holders
- [x] 6.7 A working-copy edit invalidates the draft query, so the Publish button appears without a reload
- [x] 6.8 Regression tests for each: unpublished role cannot reach compiled claims; assign projects only published roles; a failed read does not publish; a foreign version is refused at rehearsal; the draft cache key matches

## 7. Second audit pass ✅

- [x] 7.1 **The pin and the projection resolved "latest" independently.** `LatestVersionRoles` returns `(version, roles)`; the version id is passed into the assignment transaction, so a publish landing between the two reads can no longer pin v3 while projecting v2
- [x] 7.2 **The removal panel still described an immediate revocation.** Rewritten in the conditional, amber rather than red, destructive confirm left where the revocation happens
- [x] 7.3 Regression test: the assignment pins the version whose roles it projected

## 8. Third audit pass ✅

- [x] 8.1 **Re-assigning an existing holder bypassed their pin.** The insert conflicted and kept their v1 pin while v2's roles were enqueued, and the audit recorded an assignment that never happened. The insert now `RETURNING`s the pin; a conflict writes nothing at all
- [x] 8.2 `CascadeResult.NoOp` distinguishes "already true" from "written, changed nothing"; the assign endpoint says which
- [x] 8.3 Regression test: an existing holder is offered a delta but nothing is enqueued, audited or drained

## 9. Follow-ups

- [ ] 6.1 Bulk "catch up every bundle" across the estate — currently per bundle
- [ ] 6.2 Surface stale-holder counts on Today, next to the other gaps
- [ ] 6.3 `MoveHolders` on a subset chosen by hand; today it is "everyone not already there"
