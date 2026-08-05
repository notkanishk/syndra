# Tasks — the landing label, and the member trim

## Track 1 — Home

- [x] LM-01 `nav.ts`: `leaf("Today", "/")` becomes `leaf("Home", "/")`
- [x] LM-02 Rename `components/today/` to `components/home/`, `Today.tsx` to `Home.tsx`, and the exported `Today` to `Home`
- [x] LM-11 Sweep every prose reference to the page from "Today" to "Home" across 11 files, keeping "today" the day untouched
- [x] LM-12 Audit reason `"Extended from Today"` becomes `"Extended from Home"` — it names a screen, so it must name one that exists
- [x] LM-03 `nav.ts`'s member comment reworded — "No Home" rather than "No Today", and why a member's landing IS their access
- [x] LM-04 `nav.test.ts` and `Sidebar.test.tsx` assertions follow
- [x] LM-05 Confirm the header breadcrumb derives from the nav tree (`crumbsFor`) and needs no edit

## Track 2 — The member trim

- [x] LM-06 `MemberAccess` stops rendering `ShadowCredential`; the import goes with it
- [x] LM-07 A comment stands where the card did, naming the condition for restoring it
- [x] LM-08 `ShadowCredential`, its queries, its tests and the backend vault left intact — withdrawal is one line, not a deletion

## Track 3 — Verification

- [x] LM-09 `bun run test && bun run lint && bun run build`
- [x] LM-10 Rail and breadcrumb checked in the browser
