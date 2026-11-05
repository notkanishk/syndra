# Second pass: A-20's first wave

Source: `design_handoff_syndra/reconcile-2026-10/from-design/A-20-second-pass.md` (owner: "all yes", 5 Nov 2026). Each agent adds its own section below.

## P2 P3 P6

- **P2, the welcome keeps its seat.** `WELCOME` is now "Welcome to Syndra." and is a title's second sentence. My access: "Morning, Priya. Welcome to Syndra." replaces "Here's what you can use." on a first visit, and the ladder still speaks below it ("From today, you can use ..." is never hidden); on a first visit with nothing else to say the calm rung reads `ABOUT` ("It keeps the makerspace's list of who can use what, and why.") with no dot. Home: the welcome takes the headline's seat only on a calm, loaded, error-free day; on a working day the work is the welcome. The inserted row is gone, so the date line never moves.
- **P3, Notices under the header.** `components/shell/ShellNotices.tsx` holds one seat: `PageHeader` (and Home and My access, which have their own heads) mount `<ShellNotices />` after the header; `AppShell` mounts only `ShellNoticesProvider`. One outlet claims the seat, so two headers say it once. A page with no header leaves the seat unclaimed and the provider paints the Notices at the top of the content, so an outage is never silent. The banners take `under` (16 above rather than 16 below). `PageHeader`'s root is now an outer div (arrival, margin) around the old flex row.
- **P6, the phone's top bar.** `TopBar` shows the 16px `SyndraMark` at left and a 44px "Go to" button at right below tablet. The button asks the single Go to that `GoToHost` mounts, by the `OPEN_GO_TO` window event; no fifth tab, no new sheet. `SyndraMark` skips the one-shot bloom when it is not on screen (`checkVisibility`), so the rail's hidden mark cannot spend it before the phone's.
- **Deviation from the task text, per A-20.** Operators get Go to in both views. Members get the mark but no Go to button: A-20 P6 says "Members do not get it: three pages need no jump."
