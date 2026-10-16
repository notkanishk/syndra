-- What a request asks for (owner, 16 Oct 2026). 'access' is a role they
-- don't have. 'extend' is "Ask to keep it longer" on a role that ends: it
-- moves the end of the grant they have and never gives the role twice. One
-- waiting request per person and role still holds across both kinds
-- (idx_access_requests_one_waiting).
ALTER TABLE access_requests
    ADD COLUMN kind TEXT NOT NULL DEFAULT 'access'
    CHECK (kind IN ('access', 'extend'));
