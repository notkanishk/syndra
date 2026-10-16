-- The switch over to roles that wait (N14 B), decided by the owner on 16 Oct
-- 2026 for every deployment: only roles marked in Roles that wait hold a
-- change for the operator; a bundle's or rule's own confirmation_mode no
-- longer does. No role is marked here. The owner marks the ones that need it.
--
-- DO NOTHING keeps a switch an operator already made (and who made it).
INSERT INTO config_settings (key, value, updated_by, updated_at)
VALUES ('confirmation.model', 'roles', 'migration', NOW())
ON CONFLICT (key) DO NOTHING;
