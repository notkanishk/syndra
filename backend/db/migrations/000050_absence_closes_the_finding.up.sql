-- A pending target_only finding claims "Zitadel holds this and Syndra cannot
-- explain it." When the grant it describes has since disappeared from
-- Zitadel entirely — per a COMPLETE observation, the only kind absence can be
-- concluded from (one-truth-many-checks) — that claim is no longer true.
--
-- `resolved` closes such a row without asserting what `attributed` and
-- `marked_external` assert: not that Syndra explains the grant, not that an
-- operator excluded it — just that there is nothing left to explain.
ALTER TABLE drift_items DROP CONSTRAINT IF EXISTS drift_items_status_check;
ALTER TABLE drift_items
    ADD CONSTRAINT drift_items_status_check
    CHECK (status IN ('pending_triage', 'attributed', 'revoked', 'marked_external', 'resolved'));
