-- A settled request to keep a role longer remembers both ends: the end the
-- person had when it was decided (ended_before), and the end approval set
-- (ends_after). A declined one keeps only ended_before. Both are null for any
-- other request; on an approved extension a null ends_after is "no end date".
ALTER TABLE access_requests
    ADD COLUMN ended_before TIMESTAMPTZ,
    ADD COLUMN ends_after   TIMESTAMPTZ;
