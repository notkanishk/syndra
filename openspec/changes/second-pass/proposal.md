# Second pass (A-20)

Source: `design_handoff_syndra/reconcile-2026-10/from-design/A-20-second-pass.md`. Owner said "all yes" on 5 Nov 2026. Other parts (P1, P2, P3, P5, P6, P8) are built on their own branches and add their sections here when they merge.

## P4 P7

**P4, headings leave with the rows.** `ListStates` takes a `head` prop and draws it only with rows. The 15 lists that drew `CardColumns` over their empty state move it into `head` (users, projects, roles, applications, bundles, policies, audit, targets, service accounts, roles that wait, requests, project page, grants, Zitadel projects, a person's requests). Pending changes and Unexplained access already hid theirs; left as they are. A guard test fails if a `CardColumns` sits straight above a `ListStates` again.

**P7, errors carry four parts.** `ErrorState` says what failed (and, when the caller passes `fine`, what did not), the state of the world, the next step and where, and the reference with "Quote this if you ask for help." The next-step line names the reference only when one exists. `describeFailure` maps a network fault (`TypeError`, "Failed to fetch", "Load failed", and the other engines' words) to "Syndra's server didn't answer." A project page and an app page keep their seeded name on error.

Deviation: the "which did not fail" half is an optional `fine` prop, not guessed from the error. No caller passes it yet; the Zitadel pages (P5's area) can.
