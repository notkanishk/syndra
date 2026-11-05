-- 000078: drop what roles that wait replaced, and the retired per-app overrides
-- (change rules-on-the-pipeline; reconcile-build 3.9, claims-shaping 4.9).
--
-- bundles.confirmation_mode and mapping_rules.confirmation_mode, with the
-- global default for new ones, decided whether a bundle's or rule's changes
-- waited for the operator. Since the switch over (000058) only roles that
-- wait decide, and the modes were read only to be ignored. They are inert
-- only while confirmation.model = 'roles', so this refuses anywhere it is not.
--
-- app_claim_overrides held per-application claim shapes from before each app
-- had its own shape (A-14). Nothing has written it since; its one reader
-- folded a row into an app's first shape. Dropping a row would drop a claim an
-- app may read, so this refuses while any row exists. Production had none
-- (checked read-only, 5 Nov 2026).
--
-- Staging runs this on last night's copy of production before production does,
-- so a refusal is seen there first. Both refusals change nothing.
DO $$
BEGIN
    IF COALESCE((SELECT value FROM config_settings WHERE key = 'confirmation.model'), '') <> 'roles' THEN
        RAISE EXCEPTION 'confirmation.model is not ''roles'': the bundles'' and rules'' own confirmation modes still decide delivery here'
            USING HINT = 'Finish the switch over on Roles that wait (PUT /api/v1/config/confirmation-model {"model":"roles"}), then run the migration again.';
    END IF;
    IF EXISTS (SELECT 1 FROM app_claim_overrides) THEN
        RAISE EXCEPTION 'app_claim_overrides has % row(s): dropping it would drop claims an app may still read',
            (SELECT count(*) FROM app_claim_overrides)
            USING HINT = 'Save each override as that app''s own claim shape, delete the rows, then run the migration again.';
    END IF;
END $$;

ALTER TABLE bundles       DROP COLUMN IF EXISTS confirmation_mode;
ALTER TABLE mapping_rules DROP COLUMN IF EXISTS confirmation_mode;
DELETE FROM config_settings WHERE key = 'global.default_rule_confirmation_mode';

DROP TABLE IF EXISTS app_claim_overrides;
