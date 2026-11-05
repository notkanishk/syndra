# One path — tasks

## W1a · Zitadel screen pipes (branch `fix/zitadel-screen-pipes`)

- [x] 1.1 `services/roles.go` `writeRole`: trace (local row + audit, one tx, access lock) before the Zitadel call
- [x] 1.2 Read back on any error before undoing; undo audited as `role.not_changed`
- [x] 1.3 `CreateRole` on the pipe (key validated first; display name falls back to the key)
- [x] 1.4 `UpdateRole` keeps the local row in step (when there is one); `db.UpdateRole`
- [x] 1.5 `DeleteRole` refuses while Syndra gives the role, while holders are unknown, and until the observed-holder count is confirmed
- [x] 1.6 `handlers/discovery.go` role routes are thin callers; `zitadelAdd/Update/DeleteProjectRole` removed from `handlers/deps.go`
- [x] 1.7 Zitadel › People assign: `checkRoleOffered` + already-held on every role first, one direct grant per role, `services.Deliver`
- [x] 1.8 Zitadel › People remove: `services.DeleteDirectGrant` for a direct grant; a role a bundle or a rule gives is kept; only a role nothing gives is revoked directly
- [x] 1.9 Zitadel › People replace: remove-then-give over the difference; a refused give changes nothing
- [x] 1.10 UI: delete dialog shows who loses the role and confirms with that count; copy on both screens and the hub says they are recorded; `useUpstream` invalidates `users`, `roles`, `governance`, `propagations`
- [x] 1.11 Audit vocabulary: `role.updated`, `role.deleted`, `role.not_changed`
- [x] 1.12 Tests:
  - `services/roles_pipe_test.go`: trace before each write, no trace means no call, a 503 or timeout that landed is kept, a refusal is undone and audited, delete refusals;
  - `handlers/discovery_grants_test.go`: a role a bundle gives is kept, an unoffered or already-held role is refused, replace;
  - `handlers/discovery_roles_test.go`;
  - UI: `deleteRole.test.tsx`, `revokeGrant.test.tsx`.
- [ ] 1.13 `handlers/roles.go` (`POST /api/v1/roles`) to answer through `writeRoleWriteError`, so both screens answer refusals alike (it is not a W1a file) #ready-for-agent
- [ ] 1.14 Deploy, then delete a role nobody holds on the dev box and confirm both audit rows and the read-back (operator-gated) #ready-for-human
