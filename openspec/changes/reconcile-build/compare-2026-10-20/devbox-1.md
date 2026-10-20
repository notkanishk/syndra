# devbox-1: Home, Requests, Pending changes, Drift, Expiring access, Holds, Events, Revocations

Method: dev box (demo session Alice Rivera, operator) at 1280x800 vs boards on localhost:4400, screenshots side by side. Read-only; Advanced view. The sidebar groups, the 14|30 switch, "Your requests", Close wording and the phone Basic tab bar were not flagged.

Caveats
- The dev box data is a DEGRADED real read: Zitadel has not answered since 11 Aug, and the demo has few records. So Requests, Expiring, Holds and Events were only reachable in their empty state; populated layouts of those four could not be compared.
- Board frames used: home default + degraded, requests-fixedC default + empty, pending default + degraded, drift default / degraded / unread / opened, expiring default + empty, holds empty, events default + empty, revocations default + none-given-up.
- Console: no React errors (no "Error", "#418" or "Minified" matches). The only console entries were two "Failed to load resource: 502" lines, i.e. the backend reads that cannot reach Zitadel. Not a UI defect.
- Top-bar Basic/Advanced pill paints on Basic for a moment on full load, then slides (already known from compare-C).

## Home (/) - frames default, degraded

Verdict: still off.

- [high] board "Sent but not seen back 1": one row, a person's name ("Sai Khurana", role, sent time, "Read again") / dev "Sent, and not seen back yet 85": five rows of "Unknown account 3664..." (raw numeric Zitadel id, truncated), "— / interns" (role shown as a dash plus mono slug), "A grant, accepted 2 Aug", button "Look at their access". 85 rows in the count, 5 shown, no "show more".
- [high] board degraded frame: an amber notice above the title, "Zitadel isn't answering. This is what it held at 09:40.", a read line "Could not reach the connected system — this is the last state seen, 14 min ago", and the action buttons greyed / paused / dev shows NO page-level notice and the read line says "Checked, nothing waiting: Requests · Expiring access · Holds due" as if all was fine. Only the Pending card carries "Disabled — Zitadel is not answering", and the makerspace tile says "0 of 1 read". The Home page does not say it is looking at old data.
- [med] board headline "8 things need you" counts blocks, one per block, with a one-line lede ("What needs you today. Time-sensitive work changes on its own...") / dev "Evening, Alice. 98 things need you." The 98 is a sum of rows (3 + 1 + 85 + 4 + 5), not blocks; there is no lede; a greeting precedes the headline and a "Tuesday 20 Oct · last checked 18:50" line replaces it.
- [med] board "New people without a bundle 2": one grouped row ("Lena Brandt and Ravi Iyer", explanation, "Give a bundle" button) / dev lists each person as an avatar row with "Holds no bundle" and no action button, and the people are the operators themselves (Alice, Sam, Maya, Leo).
- [med] board makerspace section is three stat tiles and Lately / dev adds two more cards, "Gaps" (4 people have no access at all) and "Roles most people hold". The latter shows a stray "o" glyph above its amber error line and "Trainee / Laser Lab / not checked yet" below a "could not reach" error, so it contradicts itself.
- [low] board block names "Systems nobody could read", "Sent but not seen back" / dev "Systems Syndra could not check", "Sent, and not seen back yet". Dev explanatory paragraph under the second is long, three lines.
- [low] board Lately is sentence rows with the date at right and link "Open Audit →" / dev Lately is a three-column list (date, actor, action) with "Full audit log →"; removal actions in red.
- [low] board Time-sensitive block shows "Systems nobody could read" with a plain "Open TrueNAS →" link / dev shows the whole block dimmed with a disabled-looking "Open Zitadel" button (degraded styling applied without the notice that explains it).

## Requests (/requests) - frames default, empty

Verdict: close (empty state matches; populated state only partly checked).

- [low] board filter row: search field on the left, Open/Approved/Declined/Withdrawn/All on the right, plus a "Select" button / dev: the filter tabs sit on the left and the search field follows them, no Select (nothing open to select). Order is flipped.
- [low] board empty card "No open requests." with healthy dot, same copy / dev matches; card top is ~4px tighter in dev.
- [med] board table (All tab) has a header row WHO / WHAT THEY ASKED FOR / WHY / WHEN / DECISION, initials in the avatar, "Laser cutter · Operator" and a "For a month · until 3 Nov" subline, and a decision cell "Approved / Sent · not yet seen in Zitadel" / dev All tab (one row, Sam Patel): no header row, empty avatar circle without initials, role shown as "Laser Lab / trainee" with mono slug "trainee", the when as "19 hours ago", and decision "Approved" with no second line.
- [low] "Open" count in the sidebar shows 0 on dev (data); the board shows 3. Nothing to flag beyond data.

## Pending changes (/governance/pending) - frames default, degraded

Verdict: close.

- [med] board degraded: the amber notice sits ABOVE the title ("Zitadel isn't answering, so sending is paused.") / dev puts the notice BELOW the lede and above the search ("Zitadel is not answering." with "As of 18:51"). "Send 5 changes" is dimmed on both.
- [high] board WHO shows a name / dev row 2 reads "Unknown account 3671203719..." (truncated raw id) and its change "give — / admin" (a dash plus a mono slug where the role title should be).
- [med] board change cell "Give Studio · Booking" (capitalised verb, project · role, no mono) / dev "give Printing Lab / member", "revoke Printing Lab / member", "give Laser Lab / trainee": lower-case verb, slash, and the role slug in mono. Board shows the requirement mark and "Tried when it was approved at 10:42..." in amber; dev shows "Zitadel didn't answer at 23:40" in amber, same shape.
- [med] board CAUSED BY gives a human cause ("Her request, approved / by Kanishk Singh", "Lab Tech v3 / b_lab-tech") / dev shows "Given by hand" with a bare dash "—" under it on four rows, and "c_7686" (raw handle) on the revoke row. The "by whom" is missing.
- [low] board groups six bundle-made changes under one fold ("These 6 changes come from one edit...") / dev has no grouped rows (data has none).
- [low] board WAITING is relative ("1d", "5h") / dev mixes absolute "24 Aug", "3 Aug" and "19 hours ago".

## Drift (/governance/drift) - frames default, degraded, unread, opened

Verdict: close.

- [high] board WHO is a name ("Meera Pillai", "Unifi sync / Service account") / dev rows 1-3 are "Unknown account" plus a 18-digit raw Zitadel id in mono, dimmed, with a blank avatar circle (no initials).
- [med] board WHAT THEY CAN GET INTO is "Electronics bench · Operator" with "electronics.operator" mono and a "Safety-gated" tag / dev is "— · Admin", "— · Community" with the slug beneath and a "Role unknown to Syndra" chip. The dash stands where the project name belongs.
- [med] board degraded: banner above the title, "Check Zitadel again" dimmed, Adopt/Revoke/Owned elsewhere paused / dev: no banner for Zitadel not answering on this page (the Zitadel-not-read-since-11-Aug notice stands in for "unread"), "This list was read just now" in the read line, and Adopt/Revoke/Owned elsewhere fully active. The page disagrees with Pending changes, which says Zitadel is not answering.
- [low] board stacks one notice at most under the read line (unread frame: "TrueNAS hasn't been read since 1 Oct 22:10") / dev stacks two before the first section ("Zitadel hasn't been read since 11 Aug 02:17" and "The comparison didn't load"), pushing the first row below the fold at 800px.
- [low] board right of the read line shows section chips "Not given by Syndra 3 | Gone from Zitadel 1" / dev has one section so no chips; fine, but the read line has no age ("read just now").
- [low] board first row has a red left accent bar and an amber "Zitadel usually tells Syndra about a change like this on its own" line / dev rows have neither (data dependent).
- [low] board "oldest found 6 days ago" / dev "oldest found 3 Aug" (absolute). Evidence panel (Show evidence): board is an inset rounded panel with "Made by Rohan Mehta in the Zitadel console", dev is a flat full-width band with "Made by unknown" and "Created unknown" lines.

## Expiring access (/review/expiring-access) - frames default, empty

Verdict: matches (empty state only; populated state not reachable on the dev box).

- [low] board empty card "Nothing expires in the next 30 days." plus plain lede / dev matches for 30; for 14 it says "Nothing ends in the next 14 days." (verb differs between the two windows). Dev has the 14|30 pills and a read line "Syndra's own list was read just now" (not flagged, decided).
- [low] board empty frame has no read line / dev shows one.
- [not compared] board Undecided / Let lapse sections, "Extend 90 days" + "Let it lapse" buttons, "2 days left" amber tag and left accent bar: no rows on the dev box.

## Holds due (/review/holds) - frame empty

Verdict: matches (empty state only).

- [low] board lede "Holds whose review date has passed..." plain / dev same text with a dotted-underline glossary term on "Holds".
- [low] board empty card sits ~6px lower than dev; copy identical ("No hold is due for review.").
- [not compared] populated rows, lifted, lift-failed, paused: no holds on the dev box.

## Events (/operations) - frames default, empty

Verdict: close (empty only).

- [med] board empty frame is a bare card "Nothing has happened yet." under the read line / dev draws the table header row (EVENT, WHAT HAPPENED, OUTCOME, TIME with sort arrow) above the empty message inside one card. Board has no table header when empty.
- [low] dev lede has a dotted glossary underline on "Zitadel"; board plain.
- [low] dev search placeholder, Filters button and read line "The timeline was read just now" match the board.
- [not compared] populated: day groups ("TODAY 7"), event name with mono id, outcome dots (Done / Not acted on), "Show details". No events on the dev box.

## Revocations (/governance/unconfirmed-revocations) - frames default, none-given-up

Verdict: still off.

- [high] board has a meta line ("2 given up · 3 on the way · 5 people"), a search field, a Filters button, a read line ("Syndra's send queue was read just now"), section chips (Given up 2 | Still on the way 3) / dev has none of these: title, lede, then the two section cards directly.
- [high] board sections are tables (WHO / WHAT WAS REVOKED / WHERE / REVOKED / attempts) with fold chevrons and avatars / dev sections are plain cards; the one "Still on the way" row is a single run-on line "Sam Patel on Zitadel · member revoked 24 Aug" with "c_7686" at the far right. No columns, no avatar, no "Try 1 of 8 / Next try in 1 min", no relative age ("4 min ago"), role shown as the lower-case slug "member".
- [med] board "Given up" empty section: slim header, one line "Nothing has been given up on. Every revocation still has a way through." with a healthy dot / dev "Given up 0" renders the full explanatory paragraph, then the same sentence without the dot, in low-contrast text; no fold chevron.
- [med] spacing: lede ends and the first card starts ~7px below it on dev; the board leaves ~24px. Cards look attached to the intro.
- [low] board action links ("Revoke again on Tomas's page ->") on given-up rows / dev has none (no given-up rows to see).
- [low] dev footnote "A c_ handle links to the edit that set the revocation off, in Change history." is not on the board; a raw `c_7686` handle is the only identifier in the row.
