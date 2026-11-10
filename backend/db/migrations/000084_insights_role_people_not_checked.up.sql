-- A day before Syndra ever read Zitadel has no holder counts: NULL means "not
-- checked", never a recorded number standing in for people holding a role
-- (owner, 10 Nov 2026; one-path design.md, W3b). Days already stored keep what
-- they have.
ALTER TABLE insights_snapshots ALTER COLUMN role_people DROP NOT NULL;
