# Compare B: live (syndra.makerspace.tools) vs boards

Notes: the board draws ONE page body per route; the Basic/Advanced toggle on a board only changes the sidebar. So page-body diffs are marked "both". The Basic/Advanced switch is shared via localStorage with another helper, so the view flipped under me several times; the starting view was Basic and it is Basic again at the end. Console errors (pattern Error|#418|Minified): none on any page I visited. Boards: person frame (route-users-id ?frame=person) renders blank under its tabs; default frame was used instead.

## route-users (frame default)
- [med] (both) board shows the Team inside the name's subtitle ("Research assistant · Mechanical Eng") and has no Team column / live has a separate TEAM column that is "—" on every row and shows the email under the name instead
- [med] (both) board header row is PERSON ↑ / BUNDLES / ACCESS / ATTENTION, with a sort arrow and right-aligned attention in amber, red or violet / live header reads NEEDS ATTENTION, no sort arrow, and the Access cell truncates ("5 roles across…")
- [med] (both) board has a search field plus a "The people list was read just now" freshness line, and the subtitle adds "12 with access expiring inside 30 days" / live has the search field but no freshness line, and the subtitle is only "35 people"
- [low] (both) board's row-level attention says "Expires in 2 days" or "Open request" / live only ever shows "N unexplained"
- [low] (both) live paginates "Showing 10 of 35 · Show 25 more"; the board draws no pager
- [low] (basic) live Requests badge is an empty outlined "0" / board shows a filled violet count

## route-users-id (frame default; person frame blank)
- [high] (both) board groups access under per-project collapsible headers with a project chip bar and a freshness line, with rows reading "Laser cutter · Operator", Direct / Via bundle pills, "In Zitadel · Expires", and Revoke / Remove / Withdraw buttons / live draws one flat card per project with a "Read from Zitadel 14:10" stamp and no collapse, chips or row actions on the read-only rows (only "Decide on Drift →")
- [high] (basic) board header is name plus email · role · team / live Basic still shows the raw 18-digit user id in the header; Advanced adds a raw id on every project card
- [med] (both) live adds a "Full audit trail" link and a breadcrumb "People / Name"; board has neither
- [med] (both) live rows carry no description line, and "Granted by / Grant id" detail opens inline; board rows carry no equivalent
- [low] (both) tab strip (Access / Requests / Activity), Manage bundles and Grant direct access buttons match the board

## route-projects
- [med] (both) board columns are Project, Apps served, Roles, People, with sort arrows / live order is Project, People, Roles, Apps served, with no sort arrows
- [med] (both) board has a "Search projects or apps…" field / live has none
- [low] (both) board says "No app reads this yet" in an empty apps cell / live leaves the cell blank
- [low] (both) board puts "2 not in Zitadel" in amber / live uses red "N unexplained"; the freshness line is similar on both

## route-projects-id (frame default)
- [high] (both) board title is the project name / live loading state shows the raw project id as title with "0 roles · no app reads this yet" before data arrives; once loaded it shows the name but still prints the raw id in the meta line, in Basic too
- [med] (both) board has "serves" as links to the apps and a "Waits for you" mark in When given, plus a description under each role / live has "serves audio-dash" as plain text, no description, and only "Straight away"
- [med] (both) live has an extra "Token format" button beside New role; board has only New role, right-aligned beside the lede
- [low] (both) board's footnote explains When given / live's footnote says "None of … roles wait for you" with a link

## route-roles
- [med] (both) board has a search field and a quiet one-line note about roles made straight in Zitadel / live has no search field, shows a bold boxed "This list may be partial" block, and sets the Filters button on its own row
- [low] (both) board shows an "Unused" pill and "2 bundles · 1 rule"; sort arrow on PROJECT / live shows "—" and has no sort arrow
- [low] (both) freshness line sits above the note on the board, below it on live

## route-applications
- [med] (both) board columns are App (name plus "OIDC · claim key" under it), Reads roles from, Format, People / live has an extra KIND column, the claim key alone under the name, and Format after People
- [low] (both) board flags "mixed in Laser Lab" in amber and has a banner for two formats / live has neither (no data to show it)

## route-applications-id (frame default)
- [low] (both) board has an "Apps" eyebrow over the title and "uses roles from Laser Lab →" as a link / live has no eyebrow and "Netbird" is plain text
- [low] (both) live prints "id 3672637…" in the meta line in Basic too
- [low] (both) board's preview shows two claims with a faded line and "Save token format · Nothing to save yet" in view / live is the same two-card layout; otherwise matches

## route-bundles-v2 (frames index, bundle)
- [high] (both) board list shows Roles counts (4, 9, 5…) / live shows "—" for every bundle even though the bundle page says 3 roles
- [med] (both) board has a "Bundles and who has them was read just now" freshness line and the Lab Tech row's "Unpublished changes / 3 people on an older version" flags / live has no freshness line (flags absent; no data)
- [low] (both) board bundle page puts a "4 roles" count beside each project title and a description under each role / live puts "1 role" at the right and has no description
- [low] (both) board's draft banner ("2 changes, not published yet") has no live equivalent here (no draft); tabs What it gives / People / History match

## route-me (frame mine)
- [low] (both) layout matches: greeting, Recent changes card, then project cards, then the catalogue
- [low] (both) live verbs are "Revoked", "Withdrawn", "Given" / board "Taken away", "Approved", "Not approved", "Given … automatically" with reason quotes and an "Ask again →" link
- [low] (both) the board draws a member (My access / Requests / Network storage rail) while live shows the operator sidebar; live's "What else is here" rows have no description under the project name

## route-states
- [low] (both) the board is about dialogs, notices and error pages and I could not trigger those read-only. Seen: empty states ("Nothing yet", "No roles yet — nothing here can be granted") follow the board's plain voice
- [med] (both) loading: live list pages show a bare table header with no rows or skeleton for a moment (People, Projects); the app detail page shows one generic four-bar skeleton card under only the breadcrumb, not the page's shape
- [low] (basic) "Unknown account dev_admin" appears on Home under New people without a bundle (an account with no name; the board uses a name or a faded "Unknown account" plus id only for rows not in Zitadel)

## shell-account (frame ready) and Sidebar.dc.html
- [low] (both) top bar matches: breadcrumb, Basic/Advanced pill, theme toggle, avatar with name
- [low] (both) account sheet matches (who you are, Appearance, Sign out with its explanation); live adds "My access →", "Network storage →" and a "Sound when drift is found" panel that the board lacks
- [med] (advanced) board Advanced sidebar is ACCESS (Projects, Roles, Apps, Requests, Bundles), AUTOMATION (Automatic rules, Pending changes, Change history, Access map, Settings), REVIEW / live has ACCESS (+ Bundles, Automatic rules, Insights), CHANGES (Pending changes, Change history, Roles that wait), REVIEW; no Access map or Settings, and badges render as empty "0" outlines
- [low] (basic) Sidebar.dc.html is the older board (the first item reads "Today") / live Basic reads "Home" and otherwise matches: Home, People, ACCESS: Projects, Roles, Apps, Requests
