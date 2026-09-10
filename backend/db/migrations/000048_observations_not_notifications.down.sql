-- Reverts 000048. The observation history is dropped; the grants index keeps
-- its rows, which a re-application will re-observe on the next sweep.
DROP INDEX IF EXISTS idx_zitadel_observations_recent;
DROP TABLE IF EXISTS zitadel_observations;

ALTER TABLE zitadel_grants_index
    DROP COLUMN IF EXISTS observed_at;
