# Second pass: tasks

Each agent adds its own section.

## P2 P3 P6

- [x] P2: `WELCOME` is the second sentence; `ABOUT` for the calm rung; `headerLine` takes `firstVisit`; `MemberAccess` and `Home` stop inserting a row
- [x] P2 tests: Home calm versus working day with the date line in the same seat; My access title, ladder visible, no extra row; `headerLine` first-visit rungs
- [x] P3: `ShellNotices` seat; `AppShell` stops mounting the banners; `PageHeader`, Home and `MemberAccess` mount the outlet
- [x] P3 tests: order is header, lede, Notice, content; one seat for two headers; headerless fallback; design-system guard updated
- [x] P6: `TopBar` mark and Go to below tablet; `GoToHost` listens for `OPEN_GO_TO`; `SyndraMark` blooms only where visible
- [x] P6 tests: mark and Go to for operators in Basic and Advanced; member has the mark and no Go to; the event; tap floor by token, no tab
- [ ] Look at it on a real phone (board `a20-first-visit-and-phone`): the mark's bloom after sign-in, the bar at 360px with the view switch and avatar beside Go to
