# The layout check

The pre-merge check for every UI PR. jsdom lays nothing out, so the suite cannot see a column starved
to 36px, a heading 300px off its cells or a count cut to "2 u". This opens every page in Chrome at
390, 768, 1024, 1100, 1280 and 1440, in Basic and Advanced, and fails on what the 19 Nov 2026 audit
measured on staging.

```bash
cd ui
bun run layout:check -- --base http://192.168.5.16:3001     # staging, once the branch is deployed there
```

It exits 1 on any of these, and prints one line each (`page width view theme kind …`):

| kind | what |
|---|---|
| `overlap` | two pieces of text or controls (a control's whole reach, 44px taps included) crossing by more than 2px both ways |
| `clipped` | text more than 2px past a box that does not scroll and is not a `truncate`/ellipsis or line clamp (a negative margin is a deliberate bleed, not a spill) |
| `heading` | a column heading more than 4px off its cells: left-aligned, its text against the column's box (so the avatar, dot or chip padding inside the column is the column's, as D-1 draws Person over the avatar); right-aligned, text against text |
| `seat` | from tablet up, an action seat off its row's first line although the row fits: its fixed seats at their widths, its shares at their words, held to their minimums |
| `pageScroll` | the page scrolls sideways |

Exit 2: signed out (the server has no demo sign-in for `--user`). Exit 3: it could not start Chrome.

Options: `--widths 390,1100`, `--pages /users,/roles` (default: every list page, plus the first
detail pages linked from People, Projects, Bundles and Apps), `--views basic`, `--themes light,dark`
(they never differed), `--user dev_admin`.

**Before deploying:** check a local build against staging's data. Record staging's API answers once,
then replay them into `bun next dev` (no `ZITADEL_DOMAIN`, any `SESSION_SECRET`):

```bash
bun run layout:check -- --base http://192.168.5.16:3001 --record /tmp/staging-tape.json
SESSION_SECRET=local bun next dev -p 3010 &
bun run layout:check -- --base http://localhost:3010 --replay /tmp/staging-tape.json
```

The tape holds real names: keep it outside the repo.

It only reads: it signs in as the demo identity, loads pages and never clicks. It uses the system
Chrome through `playwright-core` (no browser download); `CHROME_PATH` points it at another. It is not
in CI, which cannot reach the LAN.
