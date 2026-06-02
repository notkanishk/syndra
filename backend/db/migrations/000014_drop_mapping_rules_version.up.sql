-- Wave 2 · Part 3 — D4: mapping_rules.version is removed. audit_logs is
-- the historical record per the May 2026 audit-resolution design §2.
-- IF EXISTS keeps re-application idempotent (mirrors the down migration's
-- guarded constraint re-add) for manual-recovery / dirty-state scenarios.
-- Dropping the column also auto-removes the single-column
-- ck_mapping_rules_version_positive CHECK constraint added by 000004.

ALTER TABLE mapping_rules DROP COLUMN IF EXISTS version;
