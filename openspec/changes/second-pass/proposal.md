# second-pass

A-20, the second pass: apply the rules already written (RULES G1 to G19) where the build still breaks them.
Source: `design_handoff_syndra/reconcile-2026-10/from-design/A-20-second-pass.md`. The owner answered "all yes" on 5 Nov 2026.

Each proposal is one labelled section below. Whoever builds a proposal adds its section here and its tasks in `tasks.md`.

## P1 · Names link, everywhere

**Rule.** G14: `UserName`, `ProjectName` and `BundleName` link to their page by default (ink at rest, accent and underline on hover, A-08 2a). Only a page's own subject passes `link={false}`.

**Why.** G14 was settled on 4 Nov and not built. It is what makes A-19's seeded pages whole: a seeded page needs a link to be opened from, and most names were plain text. Nothing moves; the reader gains a way in from every row.

**Built.**
- `components/names/NameLink.tsx` is the one place a name's link is drawn (`link-hover`, the words are the link's name). `nameHref` says where each kind lives.
- `UserName`, `ProjectName`, `ResolvedProjectName`, `BundleName` take `link` (default true); `RoleRef` passes it to its project half (the role key never links). `UserName` also takes `href`, for the rows that send a service account to its own page.
- A name nothing could place ("Unknown account") still links; its copy control stays outside the link.
- Inside a dialog (`Modal`) names are plain by default (`NameLinksContext`): a link there would be the first tab stop before Cancel and one tap from leaving a decision half made. An explicit `link` still wins.
- The hand-made wrappers (the two `PersonLink`s and about twenty `<Link><UserName/></Link>` copies from the A-17 sweep) are gone. Where a container is itself the link (a whole-row link, a button, an `ArrowLink`, the policies rows, the roles index), the name inside passes `link={false}`: one link, not two.
- A-19's seeding is unchanged: a name link opens a page whose `useSeeded*` reads the same catalog cache the name came from, so the name paints on frame one.

**Guards.**
- `components/names/__tests__/names-link.guard.test.ts` scans every `.tsx` in `src` and fails when a name part sits inside an `<a>`, `Link`, `ArrowLink`, `button`, `summary`, a `role=button|link|...`, or any element with `href`/`onClick`, unless it says `link={false}`.
- `NameLink` logs `NESTED_NAME_LINK` in development and in tests if it is ever rendered inside another link or control (the check for what a source scan cannot see: a name in a helper component used inside a row link). The whole suite runs clean of it.

**Not in P1 (left as they were).** The filter chip on Pending changes names the person plain (a chip is a state, not a door). Service accounts keep reaching their page where a row already knew they were one (`useAccountHref`); the resolver carries no "is a service account" flag, so a bare `UserName` sends a service account to `/users/:id`. Teaching the resolver the kind is a separate, small change.
# Second pass: A-20's first wave

Source: `design_handoff_syndra/reconcile-2026-10/from-design/A-20-second-pass.md` (owner: "all yes", 5 Nov 2026). Each agent adds its own section below.

## P2 P3 P6

- **P2, the welcome keeps its seat.** `WELCOME` is now "Welcome to Syndra." and is a title's second sentence. My access: "Morning, Priya. Welcome to Syndra." replaces "Here's what you can use." on a first visit, and the ladder still speaks below it ("From today, you can use ..." is never hidden); on a first visit with nothing else to say the calm rung reads `ABOUT` ("It keeps the makerspace's list of who can use what, and why.") with no dot. Home: the welcome takes the headline's seat only on a calm, loaded, error-free day; on a working day the work is the welcome. The inserted row is gone, so the date line never moves.
- **P3, Notices under the header.** `components/shell/ShellNotices.tsx` holds one seat: `PageHeader` (and Home and My access, which have their own heads) mount `<ShellNotices />` after the header; `AppShell` mounts only `ShellNoticesProvider`. One outlet claims the seat, so two headers say it once. A page with no header leaves the seat unclaimed and the provider paints the Notices at the top of the content, so an outage is never silent. The banners take `under` (16 above rather than 16 below). `PageHeader`'s root is now an outer div (arrival, margin) around the old flex row.
- **P6, the phone's top bar.** `TopBar` shows the 16px `SyndraMark` at left and a 44px "Go to" button at right below tablet. The button asks the single Go to that `GoToHost` mounts, by the `OPEN_GO_TO` window event; no fifth tab, no new sheet. `SyndraMark` skips the one-shot bloom when it is not on screen (`checkVisibility`), so the rail's hidden mark cannot spend it before the phone's.
- **Deviation from the task text, per A-20.** Operators get Go to in both views. Members get the mark but no Go to button: A-20 P6 says "Members do not get it: three pages need no jump."
# Second pass (A-20)

Source: `design_handoff_syndra/reconcile-2026-10/from-design/A-20-second-pass.md`. Owner said "all yes" on 5 Nov 2026. Other parts (P1, P2, P3, P5, P6, P8) are built on their own branches and add their sections here when they merge.

## P4 P7

**P4, headings leave with the rows.** `ListStates` takes a `head` prop and draws it only with rows. The 15 lists that drew `CardColumns` over their empty state move it into `head` (users, projects, roles, applications, bundles, policies, audit, targets, service accounts, roles that wait, requests, project page, grants, Zitadel projects, a person's requests). Pending changes and Unexplained access already hid theirs; left as they are. A guard test fails if a `CardColumns` sits straight above a `ListStates` again.

**P7, errors carry four parts.** `ErrorState` says what failed (and, when the caller passes `fine`, what did not), the state of the world, the next step and where, and the reference with "Quote this if you ask for help." The next-step line names the reference only when one exists. `describeFailure` maps a network fault (`TypeError`, "Failed to fetch", "Load failed", and the other engines' words) to "Syndra's server didn't answer." A project page and an app page keep their seeded name on error.

Deviation: the "which did not fail" half is an optional `fine` prop, not guessed from the error. No caller passes it yet; the Zitadel pages (P5's area) can.
