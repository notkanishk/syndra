# Board vs live comparison, group A (live = the live app, operator "test")

Method notes
- Live compared in tab-3 (a background tab; tab-1 only paints a corner of the screenshot at 1280x800). Live at 1280x800 for Home/Requests/Pending/Holds/Expiring; the Drift/Events/Revocations screenshots had to be taken at 1024x640 because of a capture quirk. Layout verified by JS at 1280.
- Console errors (pattern Error|#418|Minified): none on any page.
- Original view was Basic. Left in Basic.
- Live production has little data: requests open 0, holds 0, expiring 0, revocations 0. So board default frames with rows can only be compared structurally; the empty frames were compared for those routes.
- BOARD DEFECT: route-requests.dc.html does not render at all ("route-requests: missing ) after argument list"). Cause: unescaped double quotes in the `degraded` frame note in the f("degraded", ...) call near line 212 ("Zitadel didn't answer when it was approved" sits inside a double-quoted string). Every frame of that board is blank until fixed. I compared against a scratch copy with the quotes escaped.
- Cross-route: the Basic/Advanced pill highlights "Basic" for about a second after load in Advanced view, then flips (hydration flash). Basic view hides the Advanced-only nav entries but every Advanced-only route still renders the full page by URL, identical to Advanced.
- Cross-route: sidebar structure differs from every board. Board (Advanced): ACCESS = Projects, Roles, Apps, Requests, Bundles; AUTOMATION = Automatic rules, Pending changes, Change history, Access map, Settings; REVIEW = Drift, Unfinished revocations, ... Live (Advanced): ACCESS adds Automatic rules, Insights; a CHANGES group = Pending changes, Change history, Roles that wait; REVIEW = Drift, Unfinished revocations, Expiring access, Holds due, then Audit, Zitadel, Connected systems, Incoming events, Makerspace settings. No "Access map" on live. Breadcrumbs differ too: board "Automation / Pending changes", live "Changes / Pending changes".
- Cross-route: board topbar avatar says the person's name ("Kanishk"); live shows "TU  test user" (fine, data).

## route-home  (live /)   frame: default (Advanced), also read empty/loading by text
- [high] (advanced) board is two columns, "Time-sensitive" (Drift, Expiring access, Systems nobody could read, Sent but not seen back) and "When you're ready" (Requests, New people without a bundle, Merge findings, Pending changes), each block a small card with a count pill and an "Open X ->" link. Live is one single stack of full-width cards in a different order: New people without a bundle, Pending changes, Sent and not seen back (15), Drift. No Time-sensitive / When you're ready grouping, no section subtitles.
- [high] (basic, advanced) live rows show "Unknown account dev_admin / sam_student / maya_staff / leo_mentor" with "Holds no bundle" (4 rows, all raw usernames; the board names people and groups them "Lena Brandt and Ravi Iyer" with a "Give a bundle" button). Live has no action button on that block at all. Raw handle where a name should be.
- [med] (advanced) board has a status line under the title ("What needs you was read just now" plus "Checked, nothing waiting: Unfinished revocations - Holds due"). Live only has "Tuesday 20 Oct - last checked 14:02". No "checked, nothing waiting" line.
- [med] (advanced) heading: board "8 things need you." solid; live "Afternoon, test. 67 things need you." with a greyed second sentence (greeting variant from earlier board; the count 67 includes the 47 drift and 15 sent-not-seen, so the number is large and un-grouped).
- [med] (advanced) "Sent, and not seen back" on live lists 15 rows, each with initials avatar, name, an em dash, "A grant, accepted 6 days ago" and "Look at their access". Board shows one row with the system/role and a "Read again" button. Live is much heavier; the board's block was a single compact row. Live row has an empty dash column.
- [med] (basic) Basic shows only the bundle block plus the explainer paragraph "In Basic these blocks are your whole queue...". Board has no Basic frame for Home, so no comparison; the paragraph is extra copy.
- [med] "The makerspace" strip: board = 3 tiles (Connected systems, People, Where access lives). Live = 4 tiles (Zitadel, Waiting for you to send, Drift, Ending in the next 14 days), a "Gaps 16" row and a "Where access lives" ranked list with "N unexplained" red text. Different content and denser than the board.
- [low] "Lately": board shows 2 plain lines with times, "Open Audit ->". Live is a full audit table with time, actor, description, and one row in red/bold ("Revoked test user's direct access by hand"); link says "Full audit log ->".
- [low] live loading state shows the greeting "Afternoon, test. Checking." with greyed skeleton rows, which roughly matches the board's loading frame idea.

## route-requests  (live /requests)   frames: default, empty, mine (board needed the quote fix)
- [high] board operator default is a searchable table (Who / What they asked for / Why / When / Decision) with Approve and Decline buttons, a search box, "Select" and the Open / Approved / Declined / Withdrawn / All pills. Live has the pills but no search box and no Select button (nothing to select since 0 open).
- [med] (basic, advanced) live Open tab empty state matches the board's empty frame: lime dot, "No open requests.", "New requests appear here the moment someone submits one." Same copy, similar card. Good.
- [med] live shows an extra "Your requests 8" card below the empty card (the operator's own asks, including "Syndra - Admin", "TrueNAS - Staff" with sub-lines "tester", "test2", "test?", "test"). The board's operator frames never show this; it only appears in the member frame "mine", which has its own "Ask for access" button. Live operator has no "Ask for access" button.
- [low] live "Your requests" rows read "For 64 days, from when it's approved." (board uses "For a month - until 3 Nov" style). Status text is plain ("You withdrew this", "You have this") with a lime or grey dot, matching the board's pattern.
- [low] live sidebar badge on Requests is 0 (outline pill) vs board's violet "2/3".
- Basic vs Advanced: page body identical; only the sidebar differs.

## route-pending  (live /governance/pending)   frame: default
- [high] (advanced) board has a "Send 12 changes" bright violet button, a search box, "They are sent in this order.", and columns WHO / CHANGE / CAUSED BY / WAITING with a per-row amber/red status line ("Tried when it was approved at 10:42. Zitadel didn't answer."), a grouped band "These 6 changes come from one edit..." with a chevron, and handle codes (c_7f2a91). Live has one row, no search, no ordering note, no grouped band, no status line, and an extra empty column "WHY IT'S WAITING" (blank for the one row).
- [med] live "Send 1 change" button is dim/muted violet (looks disabled) vs the board's solid bright violet.
- [med] live intro is two paragraphs with dotted-underline glossary terms ("Zitadel", "outbox" with tooltips) and a footer "Handles: R_ is an automatic rule, b_ a bundle, c_ one edit's set of changes." The board's intro is one paragraph with no jargon terms ("outbox" is absent).
- [low] WAITING column: live "7 min ago", board "1d / 5h / 3h" compact style. Row cell for what: live "TrueNAS account: Their roles changed." vs board "Give Studio - Booking".
- [low] breadcrumb "Changes / Pending changes" (live) vs "Automation / Pending changes" (board).

## route-holds  (live /review/holds)   frames: default, empty
- [high] (basic, advanced) live is empty. It shows an extra card "Holds due 0" with a long explainer paragraph and then a heading "Nothing due" with "No hold has reached its review date. Holds appear here on the date the person who placed them chose." The board's empty frame is a single card: lime dot, "No hold is due for review.", "A hold appears here on the review date the person who placed it chose. Until then it stays in force without asking." Card-in-card with two headings on live vs one flat card on board. Copy differs.
- [med] page intro: live one line with "Holds" as a dotted glossary term ("Holds whose review date has passed. Doing nothing keeps the access blocked."); board has a 3-line intro that also explains lifting and how to keep a hold with a later date.
- [low] title block: board default shows "4 holds - the oldest due 18 Sept" and "This list was read just now"; live has neither (nothing to count).
- [low] could not compare default rows (Who / What is held / Why / Review due / "Lift the hold") since live has none.

## route-expiring  (live /review/expiring-access)   frames: default, empty
- [high] (basic, advanced) live window is 14 days with "14 days / 30 days" tabs; board is a single 30-day window ("Nothing expires in the next 30 days.", no tabs).
- [high] live empty state is a table shell (header row WHO / WHAT / GRANTED / STATE / REMAINING / ACTION) with the message inside it, plus two explainer cards below ("What 'Let it end' does, and what it doesn't" and "When a row comes back"). Board empty frame is one small card with lime dot and one line, no table header, no explainer cards.
- [med] copy: live "Direct grants that end in the next 14 days, soonest first..." vs board "Direct access that ends in the next 30 days. Syndra ends each on its date whether or not you visit this page. Extend gives 90 more days. Let it lapse changes nothing..." Live uses "grants" and "Let it end" where the board uses "access" and "Let it lapse". Wording drift.
- [low] default rows (Extend / Let it lapse, "Extended", floating pill) not comparable; live has none.

## route-revocations  (live /governance/unconfirmed-revocations)   frames: default, empty
- [med] (basic, advanced) empty state: board = card with lime dot "Nothing outstanding." and "Every revocation has reached the system it was sent to." Live = plain heading "Nothing outstanding" and the same sentence but with no card, no lime dot, indented under the intro (reads as bare text on the page).
- [low] intro copy matches the board closely (same two sentences, board wraps to 2 lines). Title block count line ("2 given up - 3 on the way - 5 people") and "Syndra's send queue was read just now" are absent when empty; fine.
- [low] could not compare the "Given up / Still on the way" sections, chips, search or "Revoke again on Tomas's page ->" links since live has 0.

## route-drift  (live /governance/drift)   frames: default, many
- [high] (advanced) board is grouped: sections "In Zitadel, not given by Syndra 3" and "Gone from Zitadel", each foldable with its own header and "They keep it until you decide", sorted via chips ("Not given by Syndra 3", "Gone from Zitadel 1"), a search box, Filters, Select, a "Check Zitadel again" button, "What Zitadel held was read 6 min ago" line. Live is one flat table of 47 rows: no sections, no fold, no search box, no sort chips, no "was read X ago" line. It has tabs "Needs a decision 47" and "Side by side" (not on the board).
- [high] (advanced) row layout: board puts the explanation wide in the middle column and the Adopt / Revoke / Owned elsewhere buttons under it with "Show evidence" and "Select similar rows". Live gives WHY SYNDRA CAN'T EXPLAIN IT a very narrow column (text wraps to about 5 lines) and puts the three buttons in a separate far-right RESOLVE column; "Who made it?" and "Select similar rows" sit below. The board's red left-edge accent on danger rows is absent; board's "Safety-gated" amber tag is absent.
- [med] FOUND column: live "4 Aug" absolute dates (oldest found 4 Aug, all rows identical date); board shows relative "2 days / 4 days". Live shows the same long boilerplate sentence on every row (47 times), board varies it and adds amber hints ("Zitadel usually tells Syndra about a change like this on its own...").
- [med] board title block "4 to decide - oldest found 6 days ago" vs live "47 items - oldest found 4 Aug". Intro on live has a dotted glossary term for "drift"; board's intro doesn't.
- [low] role tag: board shows mono role key under the system role ("electronics.operator"); live shows "Netbird / interns" with a small pill "acad". Names are real (no "Unknown account" here).
- [low] row count: live renders all 47 rows up front; board's "many" frames show 10 a section with "Show 25 more". Not verified whether live pages.
- Basic vs Advanced: identical body.

## route-events  (live /operations)   frames: default, partial/stale not shown
- [high] (advanced) board is a table: search box, "The timeline was read just now", columns EVENT / WHAT HAPPENED / OUTCOME / TIME (sortable), date groups ("TODAY 7"), human event titles ("Access given in Zitadel", "Person joined") with the raw ID in small mono underneath, an outcome dot per row (Done / Not acted on) and "Show details" under each description. Live is a "Timeline 19" card with a row per event: mono time, raw event name ("onboarding.trigger", "zitadel.grant_added") as the main label, description, "Show details" at the right. No outcome column, no search, no sortable headers, no "read just now".
- [med] live date group rows are small grey bars ("8 Oct", "18 Sept"); board uses uppercase "TODAY" with a count.
- [med] "Timeline 19" count pill and "newest first - loaded 14:06" in the card header; the board has neither.
- [low] live copy "joined - no default bundle for new members is set, so nothing was given" is longer but similar to the board's "Syndra gave her New members..." variant. One live name renders as "Deepak ." (trailing dot, data issue, not a raw ID).
- [low] breadcrumb "System / Incoming events" matches the board.
- Basic vs Advanced: identical body.
