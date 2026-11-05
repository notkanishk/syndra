ALTER TABLE person_first_sign_in
    DROP CONSTRAINT IF EXISTS person_sign_in_order,
    DROP COLUMN IF EXISTS previous_sign_in_at,
    DROP COLUMN IF EXISTS last_sign_in_at;
