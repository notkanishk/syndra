-- Retire keeps its history (owner decision, 19 Oct 2026). A bundle or a rule
-- that stops being given used to be deleted, taking its versions and what it
-- once granted with it. Retiring now keeps the row: it stops being given and
-- everything that says "in force" skips it, but its versions and history stay
-- readable. NULL retired_at is the live state; there is no separate flag to
-- keep true beside it.
ALTER TABLE bundles
    ADD COLUMN retired_at TIMESTAMPTZ,
    ADD COLUMN retired_by TEXT;

ALTER TABLE mapping_rules
    ADD COLUMN retired_at TIMESTAMPTZ,
    ADD COLUMN retired_by TEXT;

-- A retired one must not block a new one with the same name, or the same rule
-- logic. The names are kept so the handlers' constraint checks keep matching.
ALTER TABLE bundles DROP CONSTRAINT bundles_name_key;
CREATE UNIQUE INDEX bundles_name_key ON bundles (name) WHERE retired_at IS NULL;

DROP INDEX idx_mapping_rules_logic;
CREATE UNIQUE INDEX idx_mapping_rules_logic ON mapping_rules (
    source_zitadel_project_id, source_zitadel_role_key,
    target_zitadel_project_id, target_zitadel_role_key
) WHERE retired_at IS NULL;
