-- Back to 000100: a gap's kind goes, with its check; its reason stays, which is
-- all a 000100 backend reads.
ALTER TABLE addon_metric_gaps DROP COLUMN IF EXISTS kind;
