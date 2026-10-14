# PROPOSED — DegradedBanner: last known data

Status: proposed, owner-requested 2 Oct 2026. Changes Locked row 97's behaviour; needs backend work. Owner, 13 Oct: Syndra never shows sample data in place of Zitadel (N23).

## Purpose
When Zitadel stops answering, show what Syndra last read from it, marked with its age, instead of sample data. Everything on screen stays real. Giving and revoking pause until Zitadel answers.

## Why
Sample data asks an operator to ignore the whole screen. A labelled old read lets them keep working on what does not depend on the minute: reading Drift, planning, answering a member's question.

## Props
```ts
interface DegradedBannerProps {
  /** ISO time of the last good read from Zitadel. Absent = nothing was ever read; show nothing, never sample data. */
  lastReadAt?: string;
  /** For whoever runs the Syndra server. */
  reason?: string;
}
```

## States
- **Last known** (`lastReadAt` set): amber, breathing dot. Title "Zitadel isn't answering. This is what it held at 09:40." Body: "Everything below is real, read 14 minutes ago, and may have changed since. Giving and revoking access is paused until Zitadel answers, because a decision made on an old read can be wrong." The page is not dimmed.
- **Never read** (`lastReadAt` absent): no data at all. Amber, breathing dot. Title "Zitadel isn't answering, and Syndra hasn't read it yet." Body: "There is nothing to show until it answers. This page tries again every minute on its own, and giving and revoking access waits until then." Page headers drop their counts; lists render no card; search keeps its seat. Owner, 13 Oct: never sample data, in any state.
- **Back**: the banner leaves by Motion 6a.

## Elsewhere
- Every list shows `ReadFreshness` with `current: false` (the provisional reading) at its foot.
- Buttons that give or revoke are disabled. Reading, filtering and navigating stay live.

## Backend needed
`/system/mode` returns `last_read_at`; reads return the last good snapshot with `current: false` rather than seeded rows.
