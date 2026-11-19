# The layout check

jsdom lays nothing out, so the test suite can't see a column starved to 36px, a heading 300px off its cells, or a count cut to "2 u". This check can. It opens every page in Chrome at 390, 768, 1024, 1100, 1280 and 1440, in Basic and Advanced. It fails on what the 19 Nov 2026 audit measured on staging.

## Before merging a UI PR

Run your local build against staging's data. This is the pre-merge gate.

1. Record staging's API answers once. The tape is written to the system temp directory (`syndra-layout.tape.json`), and a tape path inside the repo is refused.
2. Start a local dev server with no `ZITADEL_DOMAIN` and any `SESSION_SECRET`.
3. Replay the tape into it.

```bash
cd ui
bun run layout:check -- --base http://192.168.5.16:3001 --record
SESSION_SECRET=local bun next dev -p 3010 &
bun run layout:check -- --base http://localhost:3010 --replay
```

Use `--tape <file>` on both runs to name another tape file. The file must be outside the repo, because it holds real names.

If the replay meets a request the tape doesn't hold, the check fails with `tape missing …` for that request. Unanswered, the page would draw an error state and pass for the wrong reason. Record again when a page starts reading something new.

## After deploying

```bash
bun run layout:check -- --base http://192.168.5.16:3001
```

## What it reports

It prints one line per finding (`page width view theme kind …`) and exits 1 if there are any:

| kind | what |
|---|---|
| `overlap` | Two pieces of text or controls cross by more than 2px both ways. A control counts its whole reach, 44px tap areas included. An inline link that wraps counts line by line. |
| `clipped` | Text runs more than 2px past a box that doesn't scroll and isn't a `truncate`/ellipsis or line clamp. A negative margin is a deliberate bleed, not a spill. |
| `heading` | A column heading is more than 4px off its cells. A left-aligned heading's text is compared with the column's box, so an avatar, dot or chip padding inside the column belongs to the column (D-1 draws Person over the avatar). A right-aligned heading is compared text to text. |
| `seat` | From tablet up, an action seat is off its row's first line although the row fits. Fixed seats count at their widths, and shares count at their words, held to their minimums. |
| `pageScroll` | The page scrolls sideways. |
| `tape` | In a replay, a request the tape doesn't hold. |

Exit 2 means it was signed out: the server has no demo sign-in for `--user`. Exit 3 means it couldn't run: Chrome didn't start, or a tape path was inside the repo.

## Options

- `--widths 390,1100`
- `--pages /users,/roles`. The default is every list page, plus the first detail pages linked from People, Projects, Bundles and Apps.
- `--views basic`
- `--themes light,dark`. They never differed in the audit.
- `--user dev_admin`

## Good to know

- It only reads: it signs in as the demo identity, loads pages, and never clicks.
- It uses the system Chrome through `playwright-core`, so there is no browser download. Set `CHROME_PATH` to use another Chrome.
- It is not in CI, because CI can't reach the LAN.
