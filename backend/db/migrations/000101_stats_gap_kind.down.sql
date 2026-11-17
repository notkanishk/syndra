-- Back to 000100: a gap's kind goes, with its check; its reason stays, which is
-- all a 000100 backend reads. A gap kept with no reason takes words a 000100
-- backend can hold (its check wants one), so the window stays a gap.
UPDATE addon_metric_gaps SET reason = 'No reason given' WHERE reason = '';
ALTER TABLE addon_metric_gaps DROP CONSTRAINT IF EXISTS addon_metric_gaps_reason_check;
ALTER TABLE addon_metric_gaps ADD CONSTRAINT addon_metric_gaps_reason_check CHECK (char_length(reason) BETWEEN 1 AND 200);
ALTER TABLE addon_metric_gaps DROP COLUMN IF EXISTS kind;
