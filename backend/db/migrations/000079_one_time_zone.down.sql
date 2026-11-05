-- 000079_one_time_zone, down. Snapshots stay on the makerspace's day: putting
-- them back on the UTC day would only bring the error back. Recounted sign-in
-- totals stay too, for the same reason.
DROP TABLE IF EXISTS sign_in_daily_recount;
