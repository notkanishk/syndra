-- The one table migration 000026 missed (§23).
--
-- 000026 gave the add-on platform its target dimension: every table that
-- records something about a person on a system got a `target` column, because
-- with two add-ons registered a row that names only a person answers the wrong
-- question. `shadow_credentials` predates all of it and never got one — and
-- `my_storage.go` renders its status PER TARGET.
--
-- Latent at one target, and wrong the moment there are two: enrolling on the
-- NAS would report "set, last changed…" for the door system as well, and
-- setting one anywhere would clear `enrolled_before_cutover` everywhere,
-- silencing the re-enrolment notice for a system the member has not touched.
--
-- The table still holds no credential. This is metadata about enrolment, which
-- is exactly why it has to name what somebody enrolled ON.
ALTER TABLE shadow_credentials
    ADD COLUMN IF NOT EXISTS target TEXT;

-- Every surviving row describes an enrolment against the LLDAP bridge, which
-- 000034 retired. It has no target because the system it was for was not one —
-- it was the single directory every member shared. Named rather than guessed at
-- with a DEFAULT of some live target's name: the schema must not answer on
-- behalf of a statement that did not say, and "they enrolled against the thing
-- that is gone" is the fact these rows actually carry.
UPDATE shadow_credentials SET target = 'retired_bridge' WHERE target IS NULL;

ALTER TABLE shadow_credentials ALTER COLUMN target SET NOT NULL;

-- And the uniqueness moves with it. Keyed on user_id alone, a second target's
-- enrolment would UPDATE the first one's row rather than insert beside it —
-- which is the same bug as the missing column, expressed as data loss instead
-- of as a wrong answer.
ALTER TABLE shadow_credentials
    DROP CONSTRAINT IF EXISTS shadow_credentials_user_id_key;
DROP INDEX IF EXISTS shadow_credentials_user_id_key;
CREATE UNIQUE INDEX IF NOT EXISTS shadow_credentials_user_target_key
    ON shadow_credentials (user_id, target);
