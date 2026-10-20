# Dev box vs boards, pass 2 (20 Oct 2026)

Method: screenshots of the dev box (demo mode, operator session, Advanced view) beside each board frame at 1280x800; phone at 375x812. Read-only.

Caveat that shapes every populated-state comparison: the dev box has no working Zitadel in demo mode (Zitadel page says "Not connected"), so lists show "Not checked yet" and zero counts, and no person holds access. Populated board states (a person with several projects, counts, "2 not in Zitadel") could not be compared. Where a finding below is marked (data) it is a data or degraded-state difference, not a build defect.

Console: no React/hydration errors (#418, Minified) on any route. Only generic resource errors: one 404 (a URL I guessed) and 503s that match Zitadel being unreachable.

Cross-route:
- [med] board Basic/Advanced pill shows Advanced selected on load / dev pill paints "Basic" selected for a couple of seconds while the sidebar is already Advanced, then flips (seen on every route after a navigation). First-paint mismatch between server render and the stored choice.
- [low] board header shows "Advanced" selected and a name "Kanishk" / dev shows "Alice Rivera" (demo identity, not a defect).

## People
- [low] board 248 people with a Bundles chip and an access summary ("3 roles across 2 projects") / dev 5 people, Bundles "none", Access "Not checked yet" (data).
- [low] board "Unknown account" row greyed with "Not in Zitadel" / not present in dev data; none seen on this list.
- [low] board subtitle "248 people · 12 with access expiring" / dev "5 people" only (no expiry clause when unread); fine.
- Layout, header, search, Filters and Select placement match.

## A person's page (Alice Rivera and Maya Chen; nobody has several projects)
- [med] board holdings grouped into collapsible project cards with Given / Automatic sections and a project jump-chip row / dev shows only the empty state "holds no access yet" (data; the populated layout was not verifiable).
- [med] board has no hold panel / dev "On hold" panel lines start with a bare "true on truenas — put on hold by system": a raw value ("true") stands where a role name belongs.
- [low] board header action pair and tabs match; dev advisory line "Advisory · No Syndra bundle is assigned yet" with "?" glyph has no board equivalent.
- [low] board has the "What Zitadel confirmed was read 1 min ago" dot line between bundles and holdings / dev omits it on the empty state.

## Projects
- [low] board People column "41 / 2 not in Zitadel" / dev "0 / not checked yet" stacked, and Laser Lab shows "1" with "not checked yet" under it (a number and "not checked" together).
- [low] board Apps served chips with app names / dev chips match, plus "No app reads this yet" in muted text for Laser Lab and Platform Core (matches board's empty wording).
- [low] board has no Roles sort affordance / dev same. Matches.

## A project's page (Laser Lab)
- [med] board subtitle "41 people · 3 roles · serves Laser bookings → Laser badge reader →" with app links / dev "1 person · 3 roles · no app reads this yet  not checked yet  laser": a stray raw project key "laser" and a "not checked yet" run on in the same line, no app links.
- [low] board "When given" shows "Waits for you" with a half-moon dot / dev all "Straight away" (data).
- [low] board role name reads "Laser cutter · Operator" with key beneath / dev "Mentor mentor": name and key identical (data).
- [low] board people cell has a count plus "2 not in Zitadel" / dev "0 →" with arrow and "not checked yet"; arrow is extra.

## Roles
- [high] board Project column shows names ("Laser Lab") / dev shows raw numeric Zitadel project ids (e.g. 365951028870251526) on every row.
- [med] board Group and Used by filled ("2 bundles · 1 rule", "Unused" chip) / dev "—" everywhere (data).
- [low] board footer sentence "Same key, two projects, two different things" / dev has no footer.
- [low] board read line is a lime dot "read just now" / dev amber ring "Could not reach the connected system" (degraded state, correct).

## Apps
- [med] board each row shows protocol and claim ("OIDC · syndra.laser.roles") under the name / dev shows description and claim only ("ESP32 keypad bridge · door_groups"), no protocol.
- [low] board amber banner "Laser Lab is read in two formats." with "mixed in Laser Lab" tags / not triggered in dev (data).
- [low] board Format shown as array / csv / spaced / dev shows "space_delimited" raw for Material Counter vs board's "spaced".

## An app's page (Material Counter)
- [med] board claim field is empty with the default as grey placeholder, and Save sits next to "Nothing to save yet." / dev field is pre-filled with the word "roles" (looks already set, though copy above says "Nobody has set this yet"), and Save has no "Nothing to save yet" hint.
- [low] board "Copy preview" / dev "Select preview" (may be the intended http fallback).
- [low] board claims list below shows each claim with who reads it / dev shows a single row "roles  roles  used by this app" (key repeated).
- [low] board preview shows two lines with a faded second claim / dev one line plus a comment line explaining empty roles (data).

## Bundles
- [low] list matches the board in layout and wording; dev has only "Starter Bundle" (data), no "Unpublished changes" or "older version" marks to compare.

## A bundle's page (Starter Bundle)
- [high] board role groups are headed by project names ("Laser Lab  4 roles") / dev heading is the raw project id "382075332397630470".
- [med] board has the purple "2 changes, not published yet" panel with the consequence sentence / not present in dev (no draft; data).
- [low] board role rows "Laser cutter · Operator" + key + description / dev "community community": name and key the same, no description.
- [low] dev adds "Given to everyone who joins" and "Retire Starter Bundle" cards below; not on the board frame. Probably from another frame; fine.

## Systems (Connected systems, at /system/targets)
- [med] board row "TrueNAS SCALE 24.10 · network storage" and "Zitadel project Network storage" / dev "truenas_scale 25.10.7" (raw product id) and "Zitadel project a project Syndra couldn't name".
- [low] board People 48 / dev 0, and "5 accounts Syndra doesn't manage" / "1 thing needs a decision" (data).
- [low] note: /system (no targets) is a 404 page, correctly worded; the real index lives at /system/targets.

## The TrueNAS target page
- [med] board has a back link "← Connected systems", a descriptive line "TrueNAS SCALE 24.10 · network storage · truenas" and a plain-language sentence / dev has no back link, shows only a mono "TrueNAS 25.10.7", and the sentence is a generic "Whether Syndra can reach...".
- [med] board "Zitadel project Network storage" as a labelled two-column strip / dev shows labelled rows with "Unnamed project".
- [med] board "What it can do" uses plain verbs ("Creates an account", "Runs by itself") with the id in mono underneath, beside a Reconciliation card / dev lists raw ids (account.adopt, account.purge) with admin / member / "confirmation required" / "never logged: elevated_key" tags, full width, with "Check for differences" as a separate card.
- [med] board "What TrueNAS reports" rewrites device messages in plain words ("Uncorrectable error on /dev/sde ... its words, not Syndra's") / dev prints the device text as-is ("1 uncorrectable errors reported for sde (WP02DCDD)", "Storage pool raid-z1-slow ONLINE ...").
- [med] board Not-created-by-Syndra rows carry a reason ("Built in...", "Made by hand ... last signed in 5 weeks ago") / dev rows are bare account names (sai, test_user, kanishk.singh) with Assign to a person.
- [low] board "Accounts Syndra manages" is a table with Place a hold / Revoke per row / dev shows a large "Nobody yet" heading in the empty card (state-driven); the empty heading is larger than the card title.
- [low] board has no top-level "Maintenance" heading above the Active row / dev adds one. Maintenance wording is as decided; heading is extra.
- [low] dev "Waiting on a decision 0 / Merge findings" is a nested card with its own large heading inside "Waiting on you"; board has a single flat section.
- [low] dev adds a red line "One item under Waiting on you needs a decision before this can be trusted." not on the board; likely intended.

## Mappings
- [high] board "a gives everyone who holds one of Network storage's roles" / dev reads "holds one of 's roles" and "Zitadel project ·" with the project name empty (blank interpolation).
- [high] board panel fits the column / dev unpublished-change card overflows the right edge: "Publish as version 2" and its sentence ("...the next person to roll ba") are clipped.
- [med] board Add a mapping button sits in the page header and the roles table has headers (Role, On TrueNAS, People) with project-qualified role keys / dev puts Add a mapping inside the section header, no column headers, role shown as "Member / member", Remove wrapped onto its own row below.
- [med] board Matches/Publish card says "Matches version 4" with Publish disabled and the note / dev shows "Version 1, plus 1 change not yet published" with a raw change line ("Changed — / member group from lab-3d-printing to lab-electronics") and no table form.
- [low] board versions list rows with In force / Roll back / dev single highlighted row "Version 1 current" with a button "Roll back to this".
- [low] dev adds "Two settings you cannot map, on purpose" card; not on board. Fine.

## Zitadel
- [med] dev (not connected) banner shows environment variable names (ZITADEL_DOMAIN, ZITADEL_MACHINE_KEY_PATH) and "check backend startup logs"; jargon for an operator-facing page, board has no such state.
- [low] "Not connected" is stated three times (status line, banner title, banner body).
- [low] board header "auth.makerspace.tools · 5 projects · 212 people" / dev "6 projects, last known" (degraded).
- [low] board status line "Answering ... Show checks" / dev "Not connected" line, no checks (degraded).
- [low] "Look inside Zitadel" rows lose their arrows and go muted with "Unavailable while Zitadel isn't answering"; reasonable.
- [low] board signing key "Within its limit: 41 days old; replace by 24 Nov" / dev "Not set up" amber state with How to set it up (primary-filled button); matches intent.

## Insights
- [high] board role and project nodes labelled by name / dev shows the raw project id "382075332397630470" as a project node and as a role prefix ("382075332397630470 · Community").
- [med] board "The makerspace at a glance — How access travels today" insight card above the map / dev has none.
- [med] board lane nodes carry people counts (Laser cutter · Operator 9) / dev nodes have none (unread, but no "count unknown" hint).
- [low] board filter chips (2 waiting, 1 single route, 4 unused, 1 overlap) / dev (3 drift, 1 waiting, 17 unused); lens label "Recent use" on board / "Recent sign-ins" in dev.
- [low] board breadcrumb "Automation / Access map" / dev "Insights" (nav decision, N18).
- [low] board legend includes "Weight is people, Not explained by Syndra, Single route" / dev legend "Held in Zitadel, not given by Syndra, Waiting for you to send" and is cut off at the right edge in the 1280 view.
- Trends tab not opened.

## Phone, 375x812 (Home, Requests, People)
The route-mobile board has no Home or Requests frame; it draws sheets, the freshness dot, filters, Insights and the name gate, and points to each list's own board. Compared with its People frame (87a) and its list rules (#88/#89).
- [high] Home: "Sent, and not seen back yet" lists "Unknown account 366421618001445894 / — / interns" with a blank avatar, five times (85 in all). Raw ids stand for names, and the heading "98 things need you" counts these 85 though the section says nothing is waiting on the operator.
- [med] Home: reason truncates ("A grant, accepted 2 ...") while the board's phone rule (#88/#89) says a reason wraps in full. Each row stacks avatar, name, key, reason, button on four lines.
- [med] Home: "Systems Syndra could not check" heading wraps with its badge on a separate line, and the body text is squeezed into half-width beside "Open Zitadel".
- [low] Home: "Checked, nothing waiting: Requests · Expiring access · Holds due" wraps awkwardly with orphan dots at line starts.
- [low] Requests: matches the board's phone rules (why wraps, status line). Filter tabs wrap to two rows (Withdrawn, All on the second). Empty state "No open requests." is clear. Approved card is clean.
- [med] People: freshness ring is on its own line above the amber sentence instead of inline (board 86c has it inline), and Select drops to its own row under search + Filters (board 87a keeps Search and Filters together).
- [low] People: rows are name / subtitle / "Not checked yet"; board adds "6 roles across 4 projects" (data).
