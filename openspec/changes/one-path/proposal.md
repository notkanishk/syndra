# One standard path per operation

**Why.** The owner, 5 Nov 2026: "follow same pipe for same operation… core should remain same… consistent and safe behaviour… be elegant." The architecture audit of that day (job `146ae399`, `architecture-audit.md`) found operations that reach the same outcome by two pipes, where one pipe skips the other's guards. This change collects the waves that put each operation on one pipe. Each wave adds its own section here.

## W1a · Zitadel screen pipes (audit items 1 and 2; families 1c, 1d, 5, 11)

**Problem (verified on `main` at `4d668447`).**
- Zitadel › Projects created, renamed and deleted roles by calling Zitadel's Management API straight from `handlers/discovery.go` (`zitadelAddProjectRole`, `zitadelUpdateProjectRole`, `zitadelDeleteProjectRole`). There was no trace, no audit row, no local `roles` row, and no key check. Delete had no guard, and Zitadel takes a deleted role from every holder. Meanwhile `POST /api/v1/roles` (`services.CreateRole`) made roles another way: local row first, audit after with its error discarded, and a rollback on any error, which left an orphan role in Zitadel after a 503 or a timeout that had actually landed.
- Zitadel › People's grant remove (`handleRemoveZitadelGrant`) queued one revoke of every role on the Zitadel grant. It did not check whether a bundle or a rule still gave a role, which `services.DeleteDirectGrant` does. Assign skipped `checkRoleOffered`, the already-held check and `services.Deliver`.

**What.**
- **Roles: one pipe.** `services/roles.go` `writeRole` is now the only way a project role is written, from either screen. `CreateRole`, `UpdateRole` and `DeleteRole` each describe their write, and `writeRole` runs it the same way every time:
  1. writes the trace (Syndra's `roles` row plus an audit row, in one transaction under the access lock);
  2. calls Zitadel;
  3. on any error, reads the role back;
  4. undoes the record only if Zitadel is not in the state asked for, and audits that undo as `role.not_changed`.
- **Delete never strips silently.** It is refused (409 `ROLE_IN_USE`) while Syndra gives the role through a person, a bundle or a rule. It is refused (409 `ROLE_HELD`) while anyone holds it in Zitadel, with the holders' names and their count, until the caller sends that exact count back as `?confirm_holders=N`. It is refused (409 `HOLDERS_UNKNOWN`) when there is no complete, current observation to count from.
- **Grants: the person page's pipe.** The three `/api/v1/zitadel/users/{id}/grants` routes call the same pieces as People › Access:
  - Assign runs `checkRoleOffered` and `dbRequestHolding` on every role before any write, then writes one direct grant per role.
  - Remove sends a role with a direct grant through `services.DeleteDirectGrant`. A role Syndra still gives (`UserExpectsRole`) is kept. Only a role nothing in Syndra gives is revoked directly.
  - Replace is remove-then-give over the difference.
  - All three rebuild the person's cache and deliver through `services.Deliver`.
- **The legacy routes stay.** They are now thin callers.

**Not in scope.**
- Rule-target delivery for direct grants (audit 1b) belongs to W3a, the planner.
- `handlers/roles.go`'s error mapping for `POST /api/v1/roles` is unchanged. It is not a W1a file, and an `ErrRoleNotChangedInZitadel` there still answers 500.
- Target mappings and role waits that name a deleted role are not refused. They give nobody anything.
