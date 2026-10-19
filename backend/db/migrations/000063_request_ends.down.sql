ALTER TABLE access_requests
    DROP COLUMN IF EXISTS ends_after,
    DROP COLUMN IF EXISTS ended_before;
