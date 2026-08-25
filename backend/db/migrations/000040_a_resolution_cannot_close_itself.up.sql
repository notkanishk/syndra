-- A NULL CHECK passes (§30).
--
-- 000039's resolution constraint reads:
--
--   CHECK ((resolved_at IS NULL AND resolved_by IS NULL AND resolution IS NULL)
--       OR (resolved_at IS NOT NULL AND btrim(resolved_by) <> '' AND btrim(resolution) <> ''))
--
-- Set `resolved_at` and leave the other two NULL and the first arm is false,
-- the second is `true AND NULL AND NULL` = NULL, and `false OR NULL` = NULL —
-- which a CHECK treats as satisfied. So the constraint written to make sure a
-- finding cannot close itself permitted exactly that, and the partial unique
-- index below it followed: a self-closed row leaves `resolved_at IS NOT NULL`,
-- drops out of the open index, and a second standing finding lands on the same
-- account.
--
-- Found by probing the live database rather than by reading, and it is the same
-- trap 000038's own comment names about STRICT — three-valued logic in a
-- constraint over nullable columns. Writing that comment did not stop me
-- writing the bug two migrations later, which is why the guard on this one
-- reads the SQL rather than trusting the prose.
--
-- The fix is the explicit NOT NULL in the satisfied arm. `btrim(x) <> ''` says
-- nothing about a NULL x, and every constraint over a nullable column has to
-- say what it means about NULL before it says anything else.
ALTER TABLE target_binding_conflicts
    DROP CONSTRAINT IF EXISTS binding_conflict_resolution_is_attributed;
ALTER TABLE target_binding_conflicts
    ADD CONSTRAINT binding_conflict_resolution_is_attributed
    CHECK (
        (resolved_at IS NULL AND resolved_by IS NULL AND resolution IS NULL)
     OR (resolved_at IS NOT NULL
         AND resolved_by IS NOT NULL AND btrim(resolved_by) <> ''
         AND resolution  IS NOT NULL AND btrim(resolution)  <> '')
    );
