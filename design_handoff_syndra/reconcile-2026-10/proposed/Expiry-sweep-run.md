# PROPOSED — Expiry sweep run time

Status: proposed 6 Oct 2026. Needs backend work before the UI can ship it.

## Why
Expiring access promises that Syndra ends each grant on its date. The page cannot say whether that is still happening: the sweep (backend/internal/services/expiry/sweep.go, driven by periodic.Runner) runs on a timer and records nothing. If it stalls, access stops ending and no screen shows it.

## Backend
- Store each sweep run: started_at, finished_at, ended (count), error (nullable).
- Expose the latest run and the configured interval on GET /review/expiring-grants (or /system/jobs): { last_run: { finished_at, ended, error }, interval_seconds }.
- Late = now − last_run.finished_at > 2 × interval, or the last run errored.

## UI (route-expiring.dc.html)
- Read line, left: healthy dot · "Access last ended on schedule at 09:00, 14 minutes ago · next at 10:00".
- Late: amber dot and amber Notice above the header: "Access hasn't ended on schedule since 06:00." Body says what is still in place and that nothing is lost.
- Until the backend lands: ReadFreshness, subject "Syndra's own list".
