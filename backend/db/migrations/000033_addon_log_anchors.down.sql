-- 000033_addon_log_anchors.down.sql
--
-- Dropping this forgets where every add-on's log head was, which is the one
-- thing the chain cannot reconstruct for itself: the next read after a rollback
-- becomes a first read, and a truncation performed in between would be adopted
-- as the new baseline.
--
-- Refused while any anchor records a violation. Those rows are the only surviving
-- evidence that an add-on's forensic log was trimmed or rewritten, and a schema
-- rollback is not a decision about a security finding.
DO $$
DECLARE open_violations INT;
BEGIN
    SELECT COUNT(*) INTO open_violations FROM addon_log_anchors WHERE violation_reason IS NOT NULL;
    IF open_violations > 0 THEN
        RAISE EXCEPTION 'refusing to drop addon_log_anchors: % target(s) record an unresolved log-integrity violation', open_violations;
    END IF;
END $$;

DROP TABLE IF EXISTS addon_log_anchors;
