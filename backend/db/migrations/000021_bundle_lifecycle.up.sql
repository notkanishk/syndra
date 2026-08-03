-- Bundles become deletable.
--
-- Everything that hangs off a bundle already cascades: bundle_roles,
-- bundle_versions, bundle_version_roles and user_bundle_assignments all carry
-- ON DELETE CASCADE, because none of them mean anything without the bundle.
--
-- onboarding_triggers does not, and must not. It is a log: "this person was
-- onboarded, and this is the bundle they were given". That sentence stays true
-- after the bundle is retired, so the row has to survive — and ON DELETE SET
-- NULL would not let it survive intact, it would rewrite the row to say they
-- were given nothing.
--
-- So the reference is dropped and the column kept, which is the shape this
-- codebase already uses for history: audit_logs.resource_id and
-- pending_zitadel_propagations.source_ref are both plain ids with no foreign
-- key, for exactly this reason. A log records what happened; it does not hold
-- the past open.
ALTER TABLE onboarding_triggers
    DROP CONSTRAINT IF EXISTS onboarding_triggers_bundle_id_fkey;
