-- Back to the form that admits a self-closed finding.
ALTER TABLE target_binding_conflicts
    DROP CONSTRAINT IF EXISTS binding_conflict_resolution_is_attributed;
ALTER TABLE target_binding_conflicts
    ADD CONSTRAINT binding_conflict_resolution_is_attributed
    CHECK ((resolved_at IS NULL AND resolved_by IS NULL AND resolution IS NULL)
        OR (resolved_at IS NOT NULL AND btrim(resolved_by) <> '' AND btrim(resolution) <> ''));
