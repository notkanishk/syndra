-- Restore the foreign key. NOT VALID first, then validated separately: by the
-- time this runs there may be trigger rows pointing at bundles that have since
-- been deleted, and a plain ADD CONSTRAINT would fail on them. Rolling a
-- migration back must not be blocked by data the migration made legal.
ALTER TABLE onboarding_triggers
    ADD CONSTRAINT onboarding_triggers_bundle_id_fkey
    FOREIGN KEY (bundle_id) REFERENCES bundles(id) NOT VALID;
