-- 000091_target_name (addon-contract 3.10a)
--
-- What members see a connected system called. A registration attribute, not
-- the add-on's: two deployments of one add-on image share its manifest, so a
-- name carried there could not tell two laser cutters apart. NULL means "not
-- named here", and the name shown is then the manifest's product, then the
-- id title-cased (handlers.systemName, the one place that is decided).
-- The CHECK is the one definition of what a name may be: never blank (a
-- cleared name is NULL) and short enough for a rail row. Additive: an older
-- backend ignores it.
ALTER TABLE targets ADD COLUMN IF NOT EXISTS name TEXT
    CONSTRAINT targets_name_check CHECK (name IS NULL OR (btrim(name) <> '' AND char_length(name) <= 60));
