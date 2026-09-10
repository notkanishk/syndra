-- A finding used to be a claim: "Zitadel has this and Syndra doesn't explain
-- it." Nothing said WHEN Zitadel was asked, or whether that ask was complete —
-- so the same finding could not be reproduced, only re-detected.
--
-- `zitadel_observed_at` is the citation: which read of the store produced
-- this row, so the finding can say "this is what Zitadel returned at HH:MM"
-- instead of asserting it timelessly. Set from db.LatestOrgObservation at
-- write time and refreshed on every re-detection — unlike upstream_actor
-- (COALESCEd, because the FIRST evidence is the true one), this is evidence
-- of RECENCY, and the most recent confirming read is the one worth keeping.
ALTER TABLE drift_items
    ADD COLUMN IF NOT EXISTS zitadel_observed_at TIMESTAMPTZ;
