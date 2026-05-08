-- Welcome-bundle flag. Replaces the convention-based name match in
-- GetWelcomeBundle (May 2026 audit D1). The partial unique index enforces
-- "at most one welcome bundle" at the database layer; application code is
-- still expected to use a transaction when toggling.

ALTER TABLE bundles
    ADD COLUMN is_welcome BOOLEAN NOT NULL DEFAULT FALSE;

CREATE UNIQUE INDEX idx_bundles_welcome_unique
    ON bundles (is_welcome)
    WHERE is_welcome = TRUE;
