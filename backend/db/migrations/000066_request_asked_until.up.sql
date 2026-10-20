-- A request that asks for access "until the end of term" (or any fixed date)
-- carries that date itself, so a late approval grants to it rather than to
-- now plus the days it was worth when asked. Null for a days-based ask, which
-- keeps counting from the decision.
ALTER TABLE access_requests
    ADD COLUMN asked_until TIMESTAMPTZ;
