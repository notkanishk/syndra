# Verdicts, by area

Folded into 00-CONSENSUS.md on 13 Oct 2026; start at README.md.

## Area 1 · Nav, brand and target plane

#5b  KEEP-CODE
     Holds due is the only list of holds that has work in it. A list of every hold in force would be a page with nothing to do, and each hold already shows where it acts: on the person's page and on the system's.
     Boards: route-holds (note), route-home (note)

#11  SYNTHESIS
     "Paused after failures" is an amber Notice, worded as on Connected systems and Holds due, sending the reader to the Syndra server. It is drawn beside "Not answering", which sends them to TrueNAS, so the two states differ only in who to look at. It is amber, not red as the reply said: nothing is lost and Syndra retries by itself.
     Boards: route-system-targets-id (new), route-systems (row links to it)

#13  SYNTHESIS
     Add a mapping is the same compose step every write uses, then the shared preview. The setting stays the built text box prefilled "group", with its reason as the hint. TrueNAS can give two answers, a definite no (red, no Try again) and could-not-ask (amber, allowed through), so the operator always knows whether anything saved.
     Owner, 8 Oct: the group is a list from the add-on, and the setting comes from the add-on's schema. Each system has one Zitadel project, so a mapping is a role and a group. See proposed/Mapping-implementation-notes.md.
     Boards: route-system-targets-id-mappings (new)

Carried onto the new boards (Locked, Phase 1): #6 #7 #8 #9 #10 #12 #69.
#4: route-zitadel now holds the Zitadel pages built from the questions board. The earlier board is retired.

### New differences found

N1  SYNTHESIS  (BOARD-AUDIT, maintenance buttons)
    The buttons name their verb: Start draining…, Set read-only…, Make active…. The current state reads as a dot and a word. A button labelled with a state reads as a state, not an act.
    Boards: route-system-targets-id

N2  SYNTHESIS  (T5 RegionIndex, shipped touch-only)
    Replaced by the read line's section chips and the floating pill, the pattern every page with sections already uses at every width. One pattern for getting to a section.
    Boards: route-system-targets-id

N3  SYNTHESIS  Owner, 8 Oct: one Zitadel project per system, set before anything else can be done. A project serves one system, and lifecycle is exempt.
    3a is picked. A violet Notice-shaped panel gives the one step. Below it the page is whole, with everything that waits greyed inside one grey outline and covered by a single line, so the reason is said once and nothing inert looks active. The panel is seen only until the project is set; after that the page is Default.
    Boards: route-system-targets-id (gate frames), route-system-targets-id-mappings (no-project), route-systems (project line, noproject frame), explore-system-project (new)

N4  SYNTHESIS  Button grammar, made the same on every board
    Buttons carry no trailing ellipsis anywhere: the dialog that follows says the rest. Accent marks the one primary action per region. Outline is used for row and secondary actions. Danger-outline is used for anything that takes access away, labelled "Revoke", as on People and Drift. Ghost is used for Cancel. A refresh (Reconcile now, Check again, Read again) is a quiet outline.
    Boards: route-system-targets-id, route-system-targets-id-mappings, route-zitadel

## Area 2 · Shell and navigation

#17  KEEP-CODE
     A member is looking for their files, not for a product name, so the row is "Network storage" at /storage. It never moves as roles change, and the page, not the rail, answers whether they have storage. A second storage system would become a second section on that page, not a second row: one row for one job.
     Boards: shell-account (member frames)

#18  KEEP-CODE
     Owner, 9 Oct: the sheet as built. One account control at every width, holding appearance, the install offer and sign-out below a hairline with its consequence written underneath, so sign-out is never one mis-tap away in the header.
     Boards: shell-account (new)

Not reconcile, already carried: #19 #20 #21 (Locked, 44px below desktop) #22 #23.

N5  SYNTHESIS  /storage, the member's page, drawn for the first time on these boards (§20, §30, C1–C3)
    The three states are kept. Each opens with the step line and one display line saying where you are. When a password is the next step, its form moves into the violet step panel, as on a system's project step. Shares, use and connection are one row per share instead of three lists, the usage bar is neutral (amber from 90%), and each card has at most one Notice.
    The status beside the system name reads "Online and accepting sign-ins", or "Offline since" a time once Syndra is sure.
    Boards: route-storage (new), shell-account (member frames)

N6  SYNTHESIS  Owner, 9 Oct: no password while TrueNAS is paused or offline
    The form is not offered and one sentence takes its place. Nothing on the page promises a change Syndra cannot deliver: it keeps no copy of the password to send later.
    Boards: route-storage (draining, readonly, quiet). Build list: BUILD-LIST.md

## Area 3 · Home

#24  SYNTHESIS
     Code's permanent makerspace zone is kept, because an operator with an empty queue still learns how things stand. The board's "no counts you can't act on" is kept too: every number in the zone opens the list it counts. The empty state's "Go to People →" is dropped, because the People stat already goes there.
     Boards: route-home (zone in every state; empty agrees "3 of 3 read"). Proposed: StatCard-href.md

#27  KEEP-CODE + one addition (departs from my own B15 reply, re-decided)
     Home is a queue to empty, not a map to read, so a clear block gives up its seat. The system page is the opposite case: it is a reference, so its regions keep their seats. B15's six hollow zeros would be furniture.
     What B15 was protecting, knowing that a missing block was checked and not skipped, is now one quiet line under the headline: "Checked, nothing waiting: Unfinished revocations · Holds due". A block emptied during the visit says its outcome, then folds (Motion 5b).
     Boards: route-home (default, degraded)

Carried, Locked/Keep: #25 #26 #28 #29 #30 #31.

N7  SYNTHESIS  Error and degraded on Home follow the Notice rule
    The error state had a "Couldn't check" title and a matching Notice beneath it. Now the Notice comes first, above a stable "What needs you" title. The degraded age moves into the read line, so there is no freshness strip at the foot.
    Boards: route-home

N8  SYNTHESIS  "Open TrueNAS →" on Home was text styled as a link
    It now links to the system's page.
    Boards: route-home

## Area 4 · People

#32  SYNTHESIS (32a)  Owner, 9 Oct
     Bulk mode stays, revoke included. Each of the five verbs opens the shared review, which names every person and what they lose, so nobody is guessed at. The board's worry is answered by that review, not by withholding the verb. The job is real: "Departed but still holding roles" only ends with a revoke. What the code lacked: the operator could select themselves and revoke their own role. A plan that takes access away now leaves the operator out, and the bar says so ("Includes you · you are left out of anything that takes access away"). Dormant accounts are no longer "the one bulk action in the product"; §29's warning is retired.
     Boards: route-users (bulk-a, bulk-b; 32a picked). Build list: People 1

#33  KEEP-CODE
     One selection model on every list, entered through a named Select, with select-all as a row showing both numbers. It works at every width, where a column-header checkbox had no phone form. Bar verbs name the review they open.
     Boards: route-users (bulk-a)

#34  KEEP-CODE
     The chip carries the pinned version. Two people in Lab Tech v2 and v3 hold different things, and the row is where that difference shows. It is how the stragglers after a publish are found.
     Boards: route-users, route-users-id

Carried, Keep/Owner-ruled: #35 #36.

## Area 5 · Person access

All Locked or Keep, carried on route-users-id: #37 #38 #39 #40 #41.

## Area 6 · Roles, projects, apps, bundles

#43  KEEP-CODE (model Locked; the screen that was never drawn is now drawn)
     Publish is drawn as built: what changes, then whether the people already on it come along, with no default, then the shared review. One difference: the dropped role's minus is amber, matching the working copy and the row's own rule that removal copy is amber until people actually move.
     Boards: route-bundles (publish, new). Build list: Bundles 1
     New bundle is drawn too, as built: name, purpose and the roles for v1 in one step, so a bundle never exists empty.
     Boards: route-bundles (new, new-done)
     SUPERSEDED by the redesign below (owner, 9 Oct).

N9   SYNTHESIS  Bundles redesigned (route-bundles-v2; owner picks 9 Oct: 1c, 2a, 3a, 2e)
     A list like Projects, and a page per bundle laid out like a person's page: header, tabs (What it gives · People · History), a card per project. Unpublished changes are one violet panel above the tabs, with what dropping a role would cost. The role picker folds by project, searches every project and bundle, shows the chosen as chips, and marks a choice with the round seat at the start of the row. Start from copies a bundle or a person once; Include makes a bundle a live part, one level deep. An included bundle is pinned: its new version is offered on the parent's page and reaches the parent's people only when the parent publishes. The included bundle's own page counts people directly, through bundles and behind the latest, and moves both the same way; bundles are drawn with square tiles in a dashed card.
     Boards: route-bundles-v2. Build list: Bundles 2–6

N10  SYNTHESIS  Owner, 9 Oct: the round seat replaces the checkbox in every Select mode
     Boards: route-users, route-drift, route-requests, route-expiring, Motion. Spec: proposed/SelectionSeat.md

## Area 7 · Requests

#51  KEEP-CODE
     A member can withdraw their own waiting request, and the row says "You withdrew this", never "Declined". Withdrawn and declined are different facts, and the member is the one who most needs them kept apart.
     Boards: route-requests (mine, member-phone)

#52  KEEP-CODE, with the catalogue drawn for the first time. Owner, 10 Oct: 52b (rooms fold, one search) and 52c (recent changes first)
     The how-long picker resolves to a date, and the operator sees the duration asked for. "Ask for this →" in the catalogue opens Ask for access already filled in. The expiry note becomes the role's own row, with "Ask to keep it longer →". "Ready to use" goes: only exceptions speak. The headline says roles, not permissions. Recent changes (30 days, newest first, the decider's note quoted) leads the page and gives up its seat when empty; a member with no access still sees a declined request.
     Boards: route-requests (ask), route-me (new). Build list: My access 1–4, B10

Carried, Locked: #53.

## Area 8 · Review and governance

#57  SYNTHESIS
     Assigning a hand-made account to a person stays on rung 3: typing the account name arms the button, because there is no undo and no way to give the data back. The button stays violet, as built. Red is kept for taking access away; this gives an account to someone, and spending red on it would make the next real revoke read as routine. One change: the built panel asks for the person's Syndra ID, a long code. The person is now found by name or email and chosen with the selection seat, showing email and team, and noting someone who already has an account there. The row verb is "Assign to a person", on the board as in code.
     Boards: route-system-targets-id (adopt, new). Build list: Connected systems 1

Carried, Keep / Locked / Owner-ruled: #54 #55 #56 #58 #59 #60 #61 #62 #63.

N11  SYNTHESIS  Owner, 10 Oct: "team" is Zitadel user metadata, a free-text label with no access meaning
     Board fixtures now use department-style teams (Design, Mechanical Engineering, Operations), never room or project names, so nobody reads a team as access. Pickers show "title · team", then email; an empty team is left out.
     Boards: every route board's fixtures; route-system-targets-id (adopt). Build list: People everywhere 1

## Area 9 · Automation

#64  SUPERSEDED by N12 (Insights)  KEEP-CODE for the model; 64a/64b were drawn before the map became Insights
     Kept: the map opens on everything, browsable; a focused node shows what gives it and what it gives; there is always a way back; search is a shortcut; a rule is a dashed link; Show tracks what is hidden. Changed in both: kind marks are ink-only (violet means actionable), "search for them by name" becomes "Show all N" in place, and only the focused node links to its page. 64b replaces the side panel and tile grids with the FilterBar, a read line with a chip per kind, and folding cards of rows with Connections in the number seat.
     Boards: route-graph (new)

Carried, Locked / Keep: #65 #66 #67 #68 #69 #70.

N12  SYNTHESIS  Owner, 12 Oct: the Access map becomes Insights, with Map and Trends tabs (option B)
     One nav item: the map opens first, and Trends holds what a map cannot draw. Every count on Trends opens the map or a list in the matching state. The Map tab has four seats and nothing else. The question sits in the search seat, built as sentences (Follow, Path, Compare, What if, Who has … without …). The lens sits in the pill seat (People · Recent use · Ending soon · A group). The read line has freshness and an earlier date on the left and findings chips on the right. The panel answers in sentences. Drift, waiting for you to send, single routes and unused things are always-on marks. Left out: 3D or free layouts, editing by dragging, risk rankings, a health score, leaderboards. Name recommended: Insights.
     Boards: route-insights (new; supersedes route-graph-v2 as the destination). Build list: Insights

#45  KEEP-CODE
     No three-way segment on phone. Roles, Bundles and Apps each have their row in TouchNav; a segment would be a second way to do the same job. The stack with the project as an eyebrow stays.
     Boards: route-roles (phone)

#47  Locked, code fix owed: the column heading reads "People", not "Holders" (CLAUDE.md: say people).
     Build list: Roles 1

Carried, Locked/Keep: #42 #44 #46 #48 #49 #50.


## Area 10 · Audit and system

#74  SYNTHESIS
     The sound gets the Segmented Off · On every other two-way choice uses, in place of a violet-filled pill that wore the action colour for a state. Under it, one line speaks only when On is not the whole story: "Can't play yet" (amber, the browser is waiting for a click, and Play it once allows it) or "Silent in this browser" (reduced motion, no Play button). Play it once replaces "That was this.", so the tone is known before it matters. A browser can't tell when a device is muted, so the page doesn't claim to.
     Boards: route-automation-settings (new). Build list: Automation settings 2–3

N13  SYNTHESIS  Automation settings, drawn for the first time (B4)
     Two settings, and the page looks like two settings. The confirmation default had a Segmented and two description cards that lit up to match, which made two controls for one choice. Now the cards are the choice, marked with the selection seat's dot (single choice) and named by their cost. Choosing saves, and the ActionOutcome sits under the control. Each card names its scope in the CardHeader note: "For the whole makerspace", "For this browser only". The amber "This applies to the whole makerspace" box goes, because a scope is neither a deadline nor a broken assumption. A failed read is a red Notice in the card's place, and the sound card still works.
     Boards: route-automation-settings (new). Build list: Automation settings 1
     Revised 13 Oct: the choice cards give way to the rule editor's Segmented with one explaining line, so one pattern sets this choice everywhere (route-automation-settings-v2). The rest of N13 holds.

N14  SYNTHESIS (B)  Owner picked B, 13 Oct · A bundle's confirmation mode is stored and never shown
     Every bundle and rule has its own mode; the global setting only seeds new ones. Rules show theirs (Automatic rules badge, rule editor Segmented, bulk verbs). Bundles store one, take the default silently at creation, and no screen shows or changes it. Two answers drawn side by side:
     A. Keep the mode on each bundle and rule and show it: the default with who set it and a never-set state, every bundle and rule listed by mode with a link to each, an offer to bring a few existing ones in line, and the rule editor's control on each bundle's page. Changing a mode makes no new version; it is recorded in the audit log.
     B (recommended). The pause moves onto roles. Bundles and rules always apply at once. Roles the operator marks ("Roles that wait for you") wait in Pending changes whenever a bundle or rule would give or take them. Under B a waiting role waits on every path, including grants by hand and approved requests (N15). The default and per-container modes go. A one-time move suggests the roles today's waiting bundles and rules give.
     Why B: what deserves a look is what is given, not what carried it; a waiting rule is a reminder, not a rule; one list instead of three places.
     Boards: route-automation-settings-v4 (supersedes v3; v2 keeps A in full). Build list: Confirmation (A or B)

N15  SYNTHESIS (B)  Owner picked B, 13 Oct · Grants by hand and approved requests ignore the setting, and differ from each other
     Owner observed both waiting in Pending changes with the default on Wait for you to send. The code says neither reads it. A grant by hand always waits: POST /users/{id}/grants drains only with ?apply=true, and useUsers never sends it. An approved request drains its own row at once (access.go, DrainOne) and waits only when the drain halts (zitadel_offline, drain_in_progress); the decision response doesn't say that it halted. So the approval the owner saw waiting most likely halted. To confirm: set the default to Apply at once and repeat. A grant by hand will still wait.
     Proposed: under B, one rule for every path: a waiting role waits however it arrives, and everything else goes at once. When an approval or grant was meant to go and didn't, its outcome says why, in the warn tone ("Approved. Not in Zitadel yet: Zitadel didn't answer. Waiting for you to send →").
     Boards: route-automation-settings-v4. Build list: Confirmation

N16  SYNTHESIS  Owner, 13 Oct: one write place per setting; a step left is said where you took the first one
     Whether a role waits is changed only on Automation settings. The project page's per-role control (v3 frame 4) goes; the project page, new-role dialog, rule editor, bundle editor and give-a-role dialog show it read-only ("When given: Straight away / Waits for you") with "Set in Automation settings →". After a new role is added, the outcome offers "Make it wait for you →". Every action with a step left keeps a Next step line on the thing it changed until the step is done (Rohan's role row, the rule row, the request row, the bundle header). Rules lose the mode badge entirely. Both rules are now in CLAUDE.md.
     Boards: route-automation-settings-v5. Proposal: proposed/NextStep.md. Build list: Confirmation, Next step

N17  SYNTHESIS  Owner, 13 Oct: one part per job across the flow
     v4 drew three role choosers (native select, search list, two selects). All role choosing now uses the settled role picker (Bundles 2a/3a) in Modal + ModalFooter: marking roles that wait, a new bundle, the role a rule gives (single choice), giving a role by hand. Locked rows say why (On your list · He has it, through Lab Tech · The first role). The step-left signal is the person row's existing status "Waiting for you to send →" with one faint why line, in each thing's own status seat; v4's amber boxes go. The before-line is the ModalFooter note. One new part: "Waits for you", grey dot, read-only. Rule fields are picker triggers showing role, project and the marker. Pending changes keeps its columns; batches carry the why in their sentence.
     Refined 13 Oct: row verbs are outline sm in the last seat (Stop waiting, Mark none); page actions are one accent button in PageHeader actions; card actions outline sm in CardHeader. No source-style symbol for waiting roles: source chips say how someone has a role, "Waits for you" says how changes travel, and the status seat already shows a real wait.
     Boards: route-automation-settings-v5 (supersedes v4). Build list: Confirmation

N18  SYNTHESIS  Owner picked 3a and 2c, 13 Oct · The Automation group, the settings page and Insights in the nav
     "Automation" fits one of its five items. Drawn: 3a (recommended) Bundles · Automatic rules · Insights top-level, then a "Changes" group with Pending changes, Change history and "Roles that wait" (the settings page, renamed after its one list); 3b the same under "Sending"; 3c as built. The drift sound moves to the account sheet's "On this device" rows. Every "Set in Automation settings →" becomes "Set in Roles that wait →". Marker: 2a (recommended) the two words "Waits for you" where roles are chosen; 2b a ring (collides with the Via bundle chip); 2c a half-filled dot (a new glyph in a product without icons).
     Picked: 3a, and 2c with its meaning on hover and focus; the words "Waits for you" beside the mark where a row has room. Applied to every frame on v5.
     Boards: route-automation-settings-v5 Turn 2. Build list: Nav, Waits-for-you mark

Carried, Locked / Keep: #71 #72 #75 on route-zitadel (CommandBlock, not a button; the write half behind its disclosure with the warning beside each control; the tick, the standing line, each surface's own truth). #73 #76 #77 #78 on route-events (outcome filter including Not acted on; the lede names both streams; no welcome bundle set is its own outcome; one sentence renderer).

## Area 11 · Member pages

#79  KEEP-CODE
     Every built state is drawn on route-storage: ready but not switched on (Set a password), usage with "no limit set", re-enrol ("Your old storage password no longer works"), and now a held account with its own sentence ("Your account is switched off for now"), no addresses, no guide, and the password row saying why it can't change.
     Boards: route-storage (held, new)

#80  KEEP-CODE
     No host, no addresses: the shares list without them, the guide is left out, and one sentence says to ask makerspace staff for the address. Nothing guessed.
     Boards: route-storage (nohost, new)

#82  SYNTHESIS (decided on route-me, 10 Oct; recorded here)
     The headline counts what Zitadel holds and a zero shows only after a complete read, as built. "Ready" goes: a role that works says nothing, and only the exceptions speak in the row's second line, "Not there yet" or "Couldn't check".
     Boards: route-me. Build list: My access

N19  SYNTHESIS  The member's side of a step left (N15, N16)
     When a member's approved request waits (the role waits for staff, or Zitadel didn't answer), My access says so where they look: the approval leads Recent changes with an amber dot, "It isn't on your account yet: makerspace staff check this role before it reaches you, and nothing more is needed from you", and the role sits in its room with "Not there yet" and why. Members never see "Waiting for you to send". It clears when staff send it.
     Boards: route-me (waiting, new). Build list: My access 5

Carried, Locked: #81 (Ask for access, no person named), #83 (a paused target refuses; drawn as N6).

## Area 12 · Mobile

#84  KEEP-CODE
     Three sheet sizes: sm takes its content's height, md stops 96px short, lg is full height with a sticky footer. A busy sheet swaps its grabber for the line saying it can't be closed yet; Cancel goes, Esc and tapping outside do nothing. The role picker (N17) is the lg sheet, unchanged at 390.
     Boards: route-mobile (sm, md, busy; lg from route-automation-settings-v5 give)

#86  SYNTHESIS (86c)  Owner picked 86c, 13 Oct · The freshness dot
     Built: a dot for every reading; live and ageing lime (both can be acted on), stale amber, provisional a grey ring under an amber sentence. The board had ageing grey and provisional filled amber, which made stale and provisional look alike though only one is cured by Refresh.
     86c (picked): as built, the provisional ring in the warn colour. Colour says whether you can act; the ring says it isn't a fresh read from the source. Matches the amber Notice on every degraded page. 86a as built; 86b the board.
     Boards: route-mobile. Build list: Mobile 1

#87  KEEP-CODE (stack) · SYNTHESIS (87c)  Owner picked 87c, 13 Oct
     People stacks and a tap opens the person. Search stays in its seat on every list (a hidden search is a search nobody uses). Filters, never built for a phone, open the desktop popover.
     87c (picked): below tablet, FilterPanel opens as a Modal sm sheet: its fields, Clear all (ghost), and one accent button naming what you'll see, "Show 12 people". Popover from tablet up. Applies to every FilterBar. 87a as built; 87b the board's full-screen search overlay plus the sheet.
     Boards: route-mobile. Build list: Mobile 2

#88  KEEP-CODE
     Drift keeps everything on a phone: the reason at full width, Found with its label, the three answers 12px apart. The board's coloured kind badge is moot: the direction is the group heading.
     Boards: route-drift (phone)

#90  KEEP-CODE
     The app token screen puts the editor first and the preview under it: it composes rather than answers, so the board's "explanation first" (M10, a question-answering screen) doesn't apply. Both Zitadel consoles stack.
     Boards: route-applications-id (phone)

N20  SYNTHESIS (20a)  Owner picked 20a, 13 Oct · Insights on a phone (new; Phase 2 drew none)
     The map needs 1040px; at 390 it can only scroll sideways, and the lines and the answer are never on screen together (20b, the map as drawn).
     20a (picked): below tablet the answer is the page. Map/List collapses to List, the lens becomes a Select. At rest the Map tab lists what is worth a look, each with its link; with a question, the panel's sentences in full, its link, Pin and Copy as sentences. The link keeps the question, so a desktop opens the map as it was left. Trends stacks as is.
     Boards: route-mobile. Build list: Mobile 3

Carried, Keep / Locked: #85 Select on an insecure origin (route-storage) · #89 each list keeps what it's scanned for, numbers carry their noun ("12 people", never "holders"), every list's phone frame on its board (index on route-mobile) · #91 landscape name gate refuses (route-mobile, drawn for the first time) · #92 apple-icon.png.

## Area 13 · Login

Carried, Locked / Owner-ruled, drawn for the first time on route-login: #93 (ten codes, three sentences: refused, handshake, nobody answered; the code never shown; Try again clears ?error=) · #94 (sign-in returns you where you were; a write that meets a 401 is answered in place) · #95 (the door is authored copy and not swept; error lines end "ask makerspace staff"; "Opening…" keeps its ellipsis, exempt from N4).

N21  SYNTHESIS (21b)  Owner picked 21b, 13 Oct · Signing in again without losing the dialog (#94)
     A write that meets a 401 says "Sign in again, then press the same button once more", but nothing on the page lets you sign in without leaving it.
     21b (picked): one violet link under the outcome, "Sign in again in a new tab →", with a faint line "This tab stays as it is. Come back here when you're signed in." It opens /login?then=close; after Zitadel the door shows open with its handoff message, "You're signed in again. Go back to the tab you were working in and press the same button. You can close this one." No new page. 21a as built.
     Found while drawing: the shipped outcome says "Nothing was changed." twice, because ActionOutcome appends it to every refused outcome and the SESSION_ENDED detail opens with it too. The detail becomes "What you were doing is still here. Sign in again, then press the same button once more." (code fix).
     Boards: route-login. Build list: Login 1, 3

N22  KEEP-CODE  Owner, 13 Oct: the shipped Syn line stays (#95)
     Board: "Syn — the goddess who keeps the door, and the defence called upon at trial." Shipped: "Syn — the goddess who keeps the door. Syndra keeps the list, and the reason for every name on it." The code's reason: what follows the gloss should be Syndra's job, not more of Syn's story. Kept.
     Boards: route-login

## Area 14 · Cross-cutting

#96  SYNTHESIS (96b)  Owner picked 96b, 13 Oct · behaviour Locked
     Toasts gone; a six-word ActionOutcome reports where the action happened; queued is amber, present tense; dialogs stay open on their result (#104). The way out reads "Done", which contradicts a queued outcome above it ("Waiting for you to send" over "Done"), and the old board barred the word.
     96b (picked): "Close", in every dialog, whatever the outcome. The FilterPanel's own "Done" stays (nothing is sent there). 96a as built.
     Found while drawing: ActionOutcome's queued state says the wait twice in its own words, the label "Waiting to be sent" (banned by the standing rule; it reads as automatic) and QUEUED_GLOSS "Recorded in Syndra, not yet sent. See Pending changes.", with a stray space before the label's period. The frames show it as built; the fix is in the build list.
     Boards: route-states. Build list: States 1, 4

#97  SYNTHESIS (Notice, 6a)
     Sample data over a live directory keeps its conditions, count and command, and moves from the amber band under the top bar into the content column as an amber Notice: title "38 things on these screens are sample data.", one sentence for the operator, then "For whoever runs the Syndra server ↓", a disclosure holding the switched-on/off line and the CommandBlock (warn tone). Degraded still wins when both apply. The "!" glyph goes.
     Boards: route-states. Build list: States 2

#98  SYNTHESIS (Notice, 6a)  The offline state, drawn for the first time
     Neutral Notice in the content column: hollow dot, surface-2, line-strong, title "No network connection.", the built sentence. Above every other Notice; when Zitadel was already down, both show, 16 apart. Writes refused before the wire, as built.
     Boards: route-states. Build list: States 2

#101  SYNTHESIS
     Behaviour Locked (no Try again; a full reload of / is the way out). The app error becomes a red Notice inside the shell, ending "Nothing was changed.", the digest as a CopyableValue, and "Go to the home page →" as a violet link (a plain <a>, still a full document load). The 404 becomes an EmptyState: "There's nothing at this address." with its guidance and the same link. global-error keeps its token-free copy of the error.
     Boards: route-states. Build list: States 3

#106  KEEP-CODE (drawn for the first time)
     Term: a button in the sentence, definition one hover, tap or Tab away, sticky, portalled. Only for Syndra's own words.
     Boards: route-states

#107  KEEP-CODE
     Nothing readable under 12.5px, product-wide. Every reconcile board already follows it.

#110  KEEP-CODE (settled as CLAUDE.md 9b)

N23  SYNTHESIS  Owner, 13 Oct: never sample data in place of Zitadel
     When Zitadel stops answering, every page shows the last known state, labelled with its age, and giving and revoking pause (proposed/DegradedBanner.md). Syndra never falls back to the demo directory. With no last read, nothing is shown: the amber Notice "Zitadel isn't answering, and Syndra hasn't read it yet.", headers without counts, no rows; search keeps its seat. Retires the board's "These numbers are not real." banner and the backend's demo fallback. Seed residue in a live database (#97) is a separate, named condition and stays.
     Boards: route-states (degraded, degraded-never); every board's degraded frame already shows last known. Build list: B11, States 5

Carried, Locked / Keep / Owner-ruled, followed on every board: #99 #100 #102 #103 #104 #105 #108 #109 #111.

## Area 15 · Handoff

README.md written; 00-CONSENSUS.md folded back (every row carries its verdict and boards; N1–N23 tabled). N14 and N15 resolved as option B; #64 superseded by N12. No row is open.

### Cohesion pass, 13 Oct (every current board against the standing rules)
- Close, never Done, as the way out of a dialog with a result (96b): route-requests, route-expiring, route-zitadel, route-storage, route-automation-settings-v5, Motion. Where it was accent it is now outline: once the result is shown there is no primary action left.
- Roles that wait (N14 B – N18) reached the boards drawn before it. route-pending: every change says why it waited (a role that always waits, or Zitadel didn't answer), fixtures now only hold waiting roles, sends that didn't get through, a failed send and one revocation. route-projects-id: a read-only When given column and "Set in Roles that wait →". route-requests: the mark on Asked for, an approval that waits for its role, and the degraded approval saying Zitadel didn't answer. route-users-id, route-roles, route-bundles-v2: the mark after waiting roles; bundles-v2 gains Turn 5 pointing to the bundle frames on v5.
- One round seat (N10): route-zitadel's role list in the delete dialog. The "I understand" acknowledgement keeps its checkbox, as a consent rather than a selection.
- Nothing readable under 12.5px (#107): 40 sizes raised across 15 boards; only aria-hidden arrows and glyphs stay smaller.
- Roles are never truncated (9b): route-roles role names wrap.
- One date style (#111): "Sep" → "Sept" on Insights, v5 and Mobile.
- Never sample data (N23): Motion's frames no longer mount a degraded demo directory.
- #87c noted on route-users.

## Additions · 14 Oct · BENCHED (owner, 14 Oct: not in this handoff; picks so far kept for when we return)

N24  SYNTHESIS (1a)  Owner picked 1a, 14 Oct · A role's sheet
     A role still has no page. Wherever a role is named and leads nowhere else (Roles, a project's roles, Insights, Jump to), it opens a Modal lg over the page, kept in the link (?role=…). Header: project Badge, the role, its description; then In {project} (link), key, the Waits-for-you mark with words and "Set in Roles that wait →", the apps that read it. One sentence counts who has it and how. Search (lists can outgrow a screen) and the read line. 1a (recommended): a fold per route (Through {bundle} · v3, Given by hand, By rule from {role}), people with title · team and end date, a waiting change in the status seat; then what it gives by rule; footer Close and "Follow it on the map →". 1b: the same as two tabs with a flat list and a route chip per row.
     Boards: route-additions (role-a, role-b, role-phone). Build list: Additions 1

N25  SYNTHESIS (3b) · ASK-OWNER (look)  Owner picked 3b, 14 Oct; its look 4a/4b/4c pending · Jump to anything (2a/2b superseded)
     ⌘K or / anywhere opens a Modal md: one field, results grouped by kind (People, Roles, Bundles, Projects, Apps, Pages), a few per kind, the mark on waiting roles. Empty, it lists the last five places. Enter opens a page; a role opens its sheet (N24). 2a (recommended): it only goes somewhere. 2b: adds a Do row of verbs for the top result; turned down by default because a verb opened from a search is a verb nobody previewed, and it blurs going and doing. Phone: a bottom sheet from a search icon in the top bar (proposed TopBar slot).
     Redrawn as three directions, from Spotlight and Raycast (a big quiet field, keyboard-only), Linear and Arc (typed scopes, a live preview) and Syndra's own Insights (questions as sentences). All keep the product's grammar: tokens only, the door's orb as the field's mark, violet only for what Enter will do, the mark on waiting roles; they go somewhere, never act.
     3a: Spotlight with a preview. Display-face field; results grouped by kind with matched letters in ink, a kind tile (R, B, P, A, →) or the person's avatar; the highlighted result previewed on the right (a person's alerts and roles).
     3b (recommended): Ask. 3a plus questions. Empty, it offers four questions Insights can answer and the recent list; a sentence returns an answer panel from Insights ("9 people can open Laser bay · Door.") with "Follow it on the map →" (⌘↵); @ # / turn into scope chips (People, Roles, Pages). The orb lights the top of the sheet.
     3c: In the rail. The sidebar becomes the search; no sheet and no dimmed page; no room for a preview or an answer.
     Owner, 14 Oct: 3b stays; three looks drawn for it (4a recommended). 4a Glass: translucent panel (surface-2 at 72%, blur 28 + saturate), violet hairline edge with an outer glow, light from the top, every inner part a rounded well (30 · 22 · 16). 4b Halo: the door's orb above a centred field, stronger light, a slow breathing edge (still under reduced motion). 4c Edge-lit: opaque, the edge lit from the top, the highlighted row and answer carrying the light. All color-mix on tokens; 4a drawn in daylight too.
     How questions are read, with no language model: a fixed grammar of sentences (Who has X · Who has X without Y · What does P have, and why · Path from P to X · What changed since D · What ends before D · What's waiting for me · Who left but still has access). Words are matched to the name catalogue (people, roles, bundles, projects, apps) plus a small synonym table kept in Syndra ("get into", "open", "enter" → door roles; "the induction" → Safety basics, as Insights already reads it). The answer always shows how it was read ("Read as: Who can open [Laser bay · Door]?"), each part a chip you can change. If nothing fits, it says so and shows the sentences. Answers come from the reads Insights already makes, with their freshness.
     Boards: route-additions (jx-a, jx-b-empty, jx-b-ask, jx-b-scope, jx-b-phone, jx-c; gl-a-ask, gl-a-empty, gl-a-cant, gl-a-light, gl-b-ask, gl-c-ask). Build list: Additions 2

N26  SYNTHESIS (5b)  Owner picked 5b, 14 Oct · Send one batch
     Send still delivers everything in order. A batch from one edit is sent whole, never part of it. 5b (recommended): the Select every list has; a seat on each batch and each single change; the bar names what goes and that the rest keep their order ("Send 7 changes"). 5a: an outline Send on every batch and single row. After either: the outcome under the header, the rest in their order, the page's Send counting what's left.
     Boards: route-additions (send-a, send-b, send-done). Build list: Additions 3

N27  KEEP-CODE  Daylight
     Nine of the busiest frames in light: every board is tokens only, so each switches whole. One finding, already owed: the provisional ring (86c). The door stays dark by design.
     Boards: route-additions (Daylight)

## From Claude Code · Q-01 (14 Oct)

Q-01  SYNTHESIS (B recommended)  TrueNAS states between answering and offline; "paused" split; picker with no groups of its own. Answer: from-design/A-01-truenas-states.md. Boards: route-systems (missed-a, missed, missed-c, offline, checking, paused), route-system-targets-id (missed, missed-c, checking, quiet, paused), route-system-targets-id-mappings (add-nogroups, add-unread, add-unchecked, paused).

Q-02  SYNTHESIS (naming A recommended)  Places per folder, access given back, what a group opens, a group that opens nothing, home folders. Answer: from-design/A-02-where-to-connect.md. Boards: route-storage (ready, ready-path, restored, home-kept, opens-nothing), route-system-targets-id-mappings (default, opens-nothing, add, add-home, add-home-none, paused), route-system-targets-id (homes-disagree).

Q-03  SYNTHESIS (Account sheet B recommended)  One home location per system, set on the system page; mapping gives a home on/off with a limit; one person's own limit on Accounts on TrueNAS; operator's Account sheet gains Network storage (and My access, B). Retires A-02 §7 homes-disagree. Answer: from-design/A-03-one-home-location.md. Boards: route-system-targets-id (home-*, limit-*), route-system-targets-id-mappings (add-home, add-home-unset), shell-account (proposed-storage, proposed-both).
Q-04  SYNTHESIS (4a, 7.1a)  Owner accepted every recommendation, 17 Oct.  One RolePicker for asking and giving, Ask to keep it, one dialog result, the cancel step pushed not stacked, AnchoredPanel, Checkbox/Radio/ChoiceRow, verbs and dates. Answer: from-design/A-04-asking-and-popovers.md. Boards: route-asking, flow-cancel-queued, Controls.
Q-05  SYNTHESIS  Six movements confirmed with three named curves, glide and the 10px cap as tokens; twelve surfaces drawn (S1–S12); one loop inside the product. Answer: from-design/A-05-motion.md. Board: Motion v2.
Q-06  KEEP-CODE, mostly  The copy guide's "revoke" wins; Ask to keep it drawn end to end; plan rows say "Will wait for you to send"; the member's never-read Notice is amber; Ending soon uses 14 | 30; term dates become one setting. Answer: from-design/A-06-from-the-build.md. Boards: route-asking (keep-*), route-me, route-users, route-insights.
