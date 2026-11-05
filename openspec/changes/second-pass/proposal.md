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
