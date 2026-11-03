# A person's network storage, and their sign-ins (A-10)

## Why
Design answer A-10 (3 Nov 2026, board `a10-person-storage-and-sign-ins.dc.html`) puts a person's storage on their page for operators, read-only, and says where they sign in. The owner's call on Part 2 (3 Nov, "1a with statistics") overrides A-10's option A: keep each person's last sign-in per app and daily totals per app naming nobody; "who signed in, and when" comes only from Zitadel's own log through a reasoned, audited lookup.

## What changes
- **Backend.** `GET /users/{id}/targets` (operators) reuses `describeMyTarget` per registered system and adds adoption, the home limit and where it comes from, holds in force, the last known read while offline (in memory), setup state and a preview for an account still to be made. The storage read passes the operator as actor; the add-on operation is unchanged.
- **Migration 000069.** `usage_last_seen` keyed by app (old rows become "app unknown" for their project); `sign_in_daily` (app, day, count), counted in the same statement only when the last-seen row moves strictly later, so a redelivered webhook counts once. Pruned with the 400-day sweep.
- **Reads.** `GET /users/{id}/usage` per app with `receiving` and `receiving_since`; `GET /applications/{id}/usage` (people in 7 and 30 days, last sign-in, daily totals).
- **Lookup.** `POST /sign-ins/lookup` (operators; person and/or app, 7/30/90 days, reason). Audited (`sign_ins.looked_up`, details: reason and range) before Zitadel is asked; not made if it can't be audited. Reads `POST /admin/v1/events/_search` through one client function, `SearchEvents`, and filters with the webhook's own sign-in translation.
- **UI.** The Network storage card per system on the Access tab (every A-10 §4 state), a chip per system on the read line, a last-sign-in line in each project heading, the app page's meta line and 30-day strip, Look up sign-ins on the person's Activity tab and the app page, TrueNAS's account rows land from `?person=` with the glide and flash, Pending changes takes `?q=`.

## Not in this change
- The `norole` line's second sentence ("Fabrication lead and Archive admin do.") needs the roles mapped to the system, which the view doesn't carry.
- Single-folder holds (owner, 3 Nov: left out until needed).
- Paging through Zitadel's log past 1,000 sign-in events per lookup; the answer says when it was cut off.
