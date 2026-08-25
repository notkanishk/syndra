-- 000028_plan_request_binding.up.sql
-- Add-on platform, group 3: binding an apply to the request it was rehearsed
-- for (change `addon-platform`, design §8).
--
-- The four Zitadel rehearse-then-apply surfaces re-submit their request body on
-- apply, because the body carries what the write needs and the per-subject
-- outcome does not: the project and role for an assignment, the duration, the
-- attribution source. The plan makes the COHORT and the DIFF durable, so the
-- apply no longer recomputes either — but the body beside it is still a body,
-- and an operator who reviewed a 30-day grant must not be able to apply a
-- permanent one by editing the field after the dialog.
--
-- So the plan records a digest of the request it was computed for, and the
-- claim compares it as one more dimension of the citation. Bound, not trusted.
--
-- A digest and nothing else. `plans` and `plan_subjects` deliberately carry no
-- column a caller can put an arbitrary value in (design §5, task 2.21), and a
-- `request_json` here would have reopened exactly that door — it is where the
-- next maintainer puts the submitted body, and a member's credential is a
-- submitted body somewhere else in this system. A fixed-length hex digest can
-- hold no secret, and the CHECK below is what keeps it one.
--
-- Empty means the plan binds no request: the add-on entitlement path issues
-- plans that are not computed from a submitted body at all, and comparing ''
-- against '' is the same equality every other citation dimension gets rather
-- than an exemption written into the predicate.
ALTER TABLE plans
    ADD COLUMN IF NOT EXISTS request_fingerprint TEXT NOT NULL DEFAULT '';

-- The default exists to fill the rows already here, and is then taken away, for
-- the reason every default in 000026 was: kept, it answers on behalf of a
-- statement that forgot to say, and '' is the one answer that binds nothing.
ALTER TABLE plans ALTER COLUMN request_fingerprint DROP DEFAULT;

ALTER TABLE plans
    ADD CONSTRAINT plans_request_fingerprint_shape_check
    CHECK (request_fingerprint = '' OR request_fingerprint ~ '^[0-9a-f]{64}$');
