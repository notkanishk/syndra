-- How each person came to hold a bundle (reconcile B7), for the People tab:
-- "Given by …", "Moved from v2", "Welcome bundle".
--
-- All nullable: rows written before this say nothing, and the API omits the
-- fields rather than guess. assigned_by is NULL for automatic assignments
-- (welcome). 'request' is reserved: an access request asks for one role, never
-- a bundle, so nothing writes it today.
ALTER TABLE user_bundle_assignments
    ADD COLUMN assigned_by        VARCHAR(255),
    ADD COLUMN source             TEXT CHECK (source IN ('direct', 'request', 'moved', 'welcome')),
    ADD COLUMN moved_from_version INT;
