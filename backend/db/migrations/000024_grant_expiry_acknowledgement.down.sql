-- Reverting drops every recorded "letting this lapse" decision. The audit rows survive, so who
-- acknowledged what is still answerable; what is lost is the queue's knowledge of which rows have
-- already been looked at, and every one of them returns as undecided.
DROP INDEX IF EXISTS idx_grant_expiry_ack_by;

DROP TABLE IF EXISTS grant_expiry_acknowledgements;
