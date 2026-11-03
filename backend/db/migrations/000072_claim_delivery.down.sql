DROP TABLE IF EXISTS claim_payload_fingerprints;
DROP TABLE IF EXISTS claim_delivery_last;
DROP TABLE IF EXISTS claim_delivery_hour;
DROP TABLE IF EXISTS claim_delivery_bucket;
DROP AGGREGATE IF EXISTS claim_hist_sum(BIGINT[]);
DROP FUNCTION IF EXISTS claim_hist_add(BIGINT[], BIGINT[]);
