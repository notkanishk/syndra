-- 000041_target_merge_bases.down.sql
--
-- Dropping this returns reconciliation to a two-way diff, which is not a
-- neutral rollback: every difference becomes a winner again, and a hand edit on
-- a target is silently reverted with nothing recording that it existed.
--
-- The bases themselves are recoverable — each is written by the next successful
-- apply for that subject — so what is lost is the window between the rollback
-- and those applies, during which no difference can be attributed to anybody.
DROP TABLE IF EXISTS target_merge_bases;
