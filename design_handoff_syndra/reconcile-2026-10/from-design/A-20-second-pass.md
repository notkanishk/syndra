# A-20 · The second pass (draft, 5 Nov 2026)

Answer to `Q-20-second-pass.md`. Boards: `a20-rules-at-the-edges.dc.html`, `a20-first-visit-and-phone.dc.html`, `a20-system-in-words.dc.html` (Proposed, 5 Nov 2026). New parts: `design/proposed/AdvancedLink.md`, `design/proposed/TechnicalDetail.md`. The owner's choices are in §6.

**Read for this:** RULES.md (with G1 to G19), FINAL.md, A-09 to A-19, the copy guide (§2 the keeper's voice, §5 the forecast table, §6, §8), `globals.css`, the components, the Design System artifact as mirrored in `ui/ds-bundle/` (its README, cards, screenshots, `_ds_bundle.css`, `.design-sync/MOVES.md`) and the Syndra Project canvas (`canvas.json`, the a09 to a19 boards). Every claim below names a file and a line in `ui/src` unless it says otherwise. Nothing was published, edited or commented.

**In one paragraph.** The product now has more rules than any screen breaks, and the rules land unevenly at the edges. The shell's own Notices sit above every page's header, against G7. Sixteen lists draw their column headings over their empty state, against G9. Names still do not link, against G14, and that one miss is why A-19's "the name arrives with you" only half works. The first-visit welcome moves the page on Home and hides the ladder on My access. Go to cannot be reached by touch in Basic. The System group, Insights and the claims page carry raw ids, JSON and three different words for one state. The design system says "nothing on it moves" on twenty cards that move. None of this needs a new idea; it needs the rules already written to be applied where they were not. Nineteen proposals follow, ranked, each with a file. The first wave is eight of them, all S or M, and none adds a part, a loop or a route transition.

---

## 1 · Findings

**clash** means the code contradicts a settled rule; **gap** means no rule covers it; **rough** means the build differs from its own answer's words.

### 1.1 What A-19 and A-18 built, read against their own words

- **P1, the name arrives.** The seeded person frame paints the avatar, name and meta but not the tabs (`components/people/PersonAccess.tsx:116-140`), so the tabs pop in when the read lands; A-19's board showed them on frame one. The app seed reads the Apps list cache only (`components/apps/AppTokenScreen.tsx:55-79`), so an app opened from Go to with no list cached gets no seed. Project and bundle seed the name only. **rough**
- **G14 is not built, and it undercuts P1.** `UserName`, `ProjectName` and `BundleName` (`components/names/`) have no link at all. Of 59 `UserName` uses, only two local `PersonLink` copies link (`app/governance/pending/page.tsx:511`, `components/home/Home.tsx:945`). Every other name on every queue is plain text, so most ways into a person's page are not links and cannot seed it. **clash**
- **P2, the forecast, is wired on five surfaces and not the two with the highest stakes.** Requests, Home, a person's Requests, Expiring's Extend, a person's Extend and Holds' lift forecast (`lib/request-decisions.ts:115-140`, `app/review/expiring-access/page.tsx:653`, `components/people/PersonAccess.tsx:829`, `components/review/HoldsDueForReview.tsx:173`). Drift's `ResolutionDialog` (`components/review/UnexplainedAccess.tsx:1175-1200`), `RemovalDialog` (`components/people/RemovalDialog.tsx:269,414`), Give a role, the bulk dialogs and `CancelQueuedStep` do not. Extend says "Will be extended by 14 days" where A-19 said "to 26 Nov". Approve forecasts "Will be approved" even when the role is one that waits, and then crosses to "Waiting for you to send": the forecast was not Syndra's honest next step. `FORECAST.queued`, `.remove` and `.keep` are used by nothing (`lib/outcome.ts:183`). The rail's count still changes at the refetch, never at the answer (`invalidateQueries` only, `lib/queries/useGovernance.ts`). **rough**
- **P3.** `NextStep` callers that still land on the whole queue or on `?q=`: `components/ui/CancelQueuedStep.tsx:259`, `components/bundles/RolePickerOne.tsx:252`, `components/people/PersonStorage.tsx:199`. The chip on Pending is the bare name and the read line says "N of the M changes match" (`app/governance/pending/page.tsx:187-199`), not "Showing Cody Rhodes's changes". **rough**
- **P4, Go to.** An unspecified Roles group (`lib/go-to.ts:13`). The placeholder names people, apps and bundles to members and to Basic, who get none of them (`components/ui/GoTo.tsx:108`). The footer omits ⌘K, and `modifierKey()` (`components/ui/Kbd.tsx:18`) is dead code. No touch path in Basic or for a member: the sheet renders only above four destinations (`lib/touch-nav.ts:64-68`), and Basic has exactly four. **rough**
- **P7, keys.** The Keys sheet's lede, "These work on every page." (`components/ui/KeysSheet.tsx:14`), is untrue for the arrows: only People, Projects, Apps, Roles and a role's holders carry `data-row`; no queue does. `useRowSelection.ts:49` binds `a` to select all, a letter key, and Insights keeps `t`, `l` and `p` (`components/insights/MapTab.tsx:214-250`). Esc leaves Select by a window listener on People (`app/users/page.tsx:244-253`) and by a container listener on Expiring (`lib/useRowSelection.ts:337`), so it needs focus inside the list there. **rough**
- **P5 and P8.** Back restores the fold, Expiring's sections and scroll; it does not restore a person's project folds (`PersonAccess.tsx:184-191`), the claims editor's open line, Select mode, or a query-only change on the same path (EOY-P5.5). Requests' status pill is remembered across visits, so the rail can open Requests on "Declined" with the pill as the only cue (`components/requests/RequestsScreen.tsx:139,154`). **gap**
- **A-18 idea 8, the welcome, moves structure.** On Home it is a `div` inserted only on the first visit (`components/home/Home.tsx:331`), so the date line sits one row lower that day. On My access it replaces the ladder's line (`components/member/MemberAccess.tsx:226-228`), so a first-visit member with a role that ends, or one given today, is not told. A-18 said "above every rung". **clash**
- **A-18 idea 6.** The mark's one-shot bloom has a seat on the rail only (`components/shell/Sidebar.tsx:63`); `TopBar.tsx` has no mark, so a phone never shows it. **rough**
- **A-18 §2.** The member's greeting always carries the comma (`MemberAccess.tsx:221`); Home drops the address when there is no name (`Home.tsx:322-323`). The plural strings for rungs 2 and 5 are invented, which is fine, and the line crossfades without a bloom, which is right.

### 1.2 Coherence gaps the recent waves opened

- **Two breakpoints.** `FilterBar.tsx:20-26`, `Home.tsx` (7 uses), `PersonAccess.tsx` and `Modal.tsx` switch at Tailwind's `sm:` (640px) while the phone sheet, the tab bar and `useIsPhone` switch at `tablet:` (720px). Between 640 and 720 the page is laid out for a tablet inside a phone shell. **gap**
- **Three mechanisms for one job on Pending.** `?person=`, `?role=` and `?q=` narrow the same list by different means and different words. **gap**
- **The tap floor is a token and a comment.** `--tap` is used in 53 places; `w-11`, `min-h-[52px]` and `min-h-[48px]` survive as literals (`app/users/page.tsx:400,645`, `components/shell/TouchNav.tsx:137`, `components/member/MemberCatalog.tsx:141,158`). **gap**
- **The motion module tells one truth and the design system another** (§1.6).

### 1.3 Areas never deeply designed

**The System group.**
- **Zitadel** (`app/zitadel/page.tsx`): the seat says "Not answering" (`:309`) and the Notice beside it says "Unreachable" (`:327`); the rule is five words with the dot, and "Offline" after three missed checks. There is no Check again (FINAL §2 names it), no missed-check pips, and while Zitadel is down and projects are still loading the meta reads "0 projects, last known" (`:59,76-79`), which is a count that was never read. The key's setup is a block of shell commands and `.env` names on screen (`:200-228`). The Claims delivery card prints "Expected … · found …" with `JSON.stringify` values and falls back to "Syndra found `{kind}`" (`components/system/ClaimsDelivery.tsx:222-226,480`). **clash**
- **A system's page** (`components/targets/TargetOverview.tsx`): every capability row shows its raw operation id in mono (`:918-923`). `UserName` falls back to a raw subject id (`:1047,1049,1117`; `PeopleOnTarget.tsx:146`). The Unconfirmed changes card exists only when it has rows (`UnconfirmedChanges.tsx:44-52`), and the Waiting on you region has no empty state at all although A-09 wrote one ("Nothing is waiting on you."). "Not created by Syndra" is the card's title (`:1354`) and "Not made by Syndra" its rows' words (`UnconfirmedChanges.tsx:101,109`). `.env` names and "Shares → SMB → Edit → Advanced" are on screen (`:770-780`). "accounts" is the count word on the list, the page and the chip (`app/system/targets/page.tsx:179`, `TargetOverview.tsx:527`, `PeopleOnTarget.tsx:75-77`). The connected-systems empty state says "see DEPLOY.md" (`app/system/targets/page.tsx:92-94`). **clash**
- **Mappings** (`components/targets/MappingManagement.tsx`): each row shows the raw `role_key` and `field → value` (`:290-305`); the dialog ledes say "currently gives group = fabrication" (`:361,442`); "Change" and "Remove" are bare verbs (`:325`); the current version row is a `bg-accent-soft` panel (`:546`). **clash**
- **Incoming events** (`app/operations/page.tsx`): "Show details" reveals the raw event type and a `<pre>` of `JSON.stringify(row.details)` (`:434-438`). The FilterBar is drawn only once rows exist (`:183`). **clash**
- **Network storage, both sides.** The member reads "Online" (`components/storage/MyStorage.tsx:387-392`) and the operator reads "Answering" (`components/people/PersonStorage.tsx:139-143`) for the same reading; A-01 says one word. The password button is md inside a card (`MyStorage.tsx:734`). The person's card shows nothing when no system is connected (`PersonStorage.tsx:72-73`) and "and N more" holds is plain text (`:518`). **clash**
- **Service accounts**: the eyebrow repeats the nav leaf (`components/system/ServiceAccountPage.tsx:94-98`, G6); "a bundle or a direct grant" (`:212`) and "Withdraw" for a queued item (`:217`) are against the vocabulary table. **clash**

**Insights** (`components/insights/`): the finding chips come and go with their counts (`MapTab.tsx:269,349-364`); unused nodes and the "N more" button are dashed (`tone.tsx:40`, `LaneMap.tsx:240,255`; dashed means NotOpenYet only); warn and danger answers sit in a tinted box (`AnswerPanel.tsx:30-33`); the dots are hand-built (`AnswerPanel.tsx:112,139`, `LaneMap.tsx:223,263`); "Back" and "Pin" name no object (`AnswerPanel.tsx:124-133`); Trends has no freshness and shows "Reading recent use." as both its loading and its failed state (`TrendsTab.tsx:51,208`); below tablet there is no list at all, only the answer and `RestCards` (`MapTab.tsx:437`), where RULES promise "the answer in sentences (List view, lens as a Select)". **clash**

**The claims page after A-16** (`components/apps/`): Client ID is a tools-card row in both views (`AppTokenScreen.tsx:263`; A-15 keeps it out of Basic). Each claim line draws its own "›" (`ShapeCard.tsx:199`) instead of FoldChevron (G3). Cancel is hand-placed in three sheets (`ShapeSheets.tsx:156,293`, `ShapeCard.tsx:1260`; G4). The TrueNAS offer links a Basic page to `/system/targets` (`ShapeCard.tsx:252`) and Debug mode's copy names Change history (`DebugMode.tsx:508,510`), both Advanced. A preview error with a prior preview draws nothing, no age and no Notice (`ClaimPreview.tsx:123,168`). The page error drops the header, so the app's name goes (`AppTokenScreen.tsx:94-101`). **clash**

### 1.4 Error, offline and empty states

- **The shell's Notices sit above the PageHeader on every page** (`components/shell/AppShell.tsx:94-97`: `OfflineBanner`, `DegradedBanner`, then `children`). G7 says under. **clash**
- **Sixteen lists draw column headings over their empty state** (`app/users/page.tsx:399/426`, `app/projects/page.tsx:115/127`, `app/roles/page.tsx:226/243`, `app/applications/page.tsx:78/90`, `app/bundles/page.tsx:69/75`, `app/policies/page.tsx:87/93`, `app/audit/page.tsx:154/160`, `app/system/targets/page.tsx:72/81`, `app/system/service-accounts/page.tsx:44/53`, `app/changes/roles-that-wait/page.tsx:185/198`, `app/governance/pending/page.tsx:209/229`, `components/review/UnexplainedAccess.tsx:369/397`, `components/requests/RequestsScreen.tsx:265/292`, `app/projects/[id]/page.tsx:165/172`, `app/zitadel/grants/page.tsx:90/97`, `app/zitadel/projects/page.tsx:105/111`). Four pages already do it right (`app/operations/page.tsx`, `WithdrawnAccess.tsx`, `PeopleOnTarget.tsx`, `BundleVersions.tsx`). **clash**
- **ErrorState says half the sentence.** It names the thing and "Nothing was changed." (`components/states/index.tsx:143-185`, `lib/outcome.ts:104-112`) but never which thing did not fail, never a next step beyond Try again, and never "Quote this if you ask for help" (that line exists only at `ActionOutcome.tsx:197` and `app/global-error.tsx:97`). A browser's "Failed to fetch" passes through raw (`outcome.ts:108`). A project's page error drops its header and name (`app/projects/[id]/page.tsx:94`). **clash**
- **Titles without a full stop** (G9): `DormantAccounts.tsx:102`, `MergeFindings.tsx:70`, `PeopleOnTarget.tsx:87`, `TargetOverview.tsx:1373`, `MappingManagement.tsx:119,506`, `PersonAccess.tsx:471`, `zitadel/page.tsx:326`, `MyStorage.tsx:164`. **clash**
- **Eight phrasings for "nothing to do" and seven for "nothing matches"** across Home, Requests, Pending, Holds, Insights, Drift, Expiring, People, Projects, Roles, Go to. Not every queue needs the same words (A-18 kept Drift's and Revocations' lines deliberately), but "Nobody matches that." / "Nothing matches that." / "No project matches that." / "No roles match that." / "Nothing that ends matches that search." / "No requests match that search." / "Nothing by that name." are one fact in seven coats. **gap**
- **The rail knows nothing when Zitadel is down**: no cue in `Sidebar.tsx` or `TouchNav.tsx`; counts simply go undefined. The Notice carries it, which is enough on desktop; on a phone the Notice is below the fold of a long page. **gap**

### 1.5 The phone shell, and Basic against Advanced

- **No tap path to Go to or Keys in Basic or for a member** (`components/shell/GoToHost.tsx:20-32`, `KeysHost.tsx:19`, `TouchNav.tsx:383-405`). **gap**
- **The top bar shows nobody on a phone** and `session.name` in full at tablet and up (`components/shell/AccountSheet.tsx:60-62`); RULES says "Kanishk". On a phone the only sign of who you are is the avatar's initials. **rough**
- **Members are offered "Sound when drift is found"** (`AccountSheet.tsx:129`): an operator's word on a member's screen. **clash**
- **Basic reaches Advanced by plain links, twenty-odd times.** `revealInAdvanced` exists and is used once (`PersonAccess.tsx:1082`). Everything else is an ArrowLink to an Advanced route from a Basic surface: Home's "Expiring access" (`Home.tsx:191`), its "and N more" (`:672`), "Open bundles" (`:785`), service accounts (`:828`); the Makerspace strip's "Connected systems", "N empty bundles" and "Full audit log" in both views (`components/home/Makerspace.tsx:102,342,370`); "Decide on Drift" and "Side by side on Drift" (`PersonAccess.tsx:742,791`); every `NextStep` (`components/ui/NextStep.tsx:36-39`); the storage card's three links (`PersonStorage.tsx:90,158,160`); "Set in Roles that wait" (`app/projects/[id]/page.tsx:265`); "Check a project's roles in Zitadel" (`app/roles/page.tsx:216`). The contract says Basic "names the cause and offers one scoped jump that sets the view". Today the view does not change, so a Basic reader lands on an Advanced page with a Basic rail that has no row for it. Home also says "Pending changes and Drift are in Advanced." as a sentence with no way there (`Home.tsx:304-308`). **clash**
- **110px seats with no phone form** sit beside other fixed seats (`app/changes/roles-that-wait/page.tsx:247,250,396`, `app/bundles/page.tsx:131-132`, `components/member/RecentChanges.tsx:119`). They wrap rather than overflow, so the number lands under the name with no label. **gap**
- **The role page paints a key-derived title and the raw key in meta in both views** (`app/projects/[id]/roles/[key]/page.tsx:78,82`; G10). **clash**

### 1.6 The design system as published

- **Twenty cards say "Nothing on it moves" and move.** Checkbox, Radio, Seat and ChoiceRow (the tick grows from 60% on settle, S15); AnchoredPanel (rises 4px on settle); ConfirmByTyping (the border warms, S16); CopyableValue (S18); ViewSwitch (its indicator glides, S10); SyndraMark (blooms once, A-18 idea 6); Chip and Badge (`chip-in`); SortHead (the arrow turns); FilterChip, WaitsMark, ThemeToggle, StatCard, FormPart, SectionPill, SignInRow, CardRow, Notice (its amber dot breathes). ArriveList says rows rise 10px; the keyframe and FINAL say 8 (`globals.css:203-211`). `ui/.design-sync/moves.json` is one table; the Moves and Phone lines have not reached the published cards yet. **clash**
- **Four cards do not render in the capture**: AppShell's three variants and RoleWordsTable and SignInRow show "This page couldn't render." (`ui/ds-bundle/_screenshots/contact-sheet-1.png`, `-5`, `-6`); TouchNav's three variants render empty. The Modal family (GoTo, KeysSheet, PushedStep, RehearsalDialog) is blank by the capture's own limit, which NOTES.md records. **rough**
- **Eight cards have no preview** (GoToPanel now has one): ReadGate, ArriveList, GlideIndicator, LeavingRow, FormPart, SectionPill, RoleWordsTable, SignInRow. Three of them (ArriveList, GlideIndicator, LeavingRow) are never mounted alone by a board author. **gap**
- **The README's token table lists no `--scrim`, `--tap`, `--layer-*`, `--tint-*`, `--hover`, `--focus-ring`, motion or radius tokens**, and names `--font-sans` as the body face while nothing in `ui/src` references `--font-sans` (`globals.css:90`; body uses `--font-figtree` directly). `--radius-shell` is also unreferenced. `ds-bundle/tokens/` and `guidelines/` are empty folders. **gap**
- **Cards without a "Not for" line or their rule**: Button (shows md buttons in a row, against G16), Notice (no G7, no breathe), Modal (no G4, no phone form), Card. FilterBar's card predates G18 and says tabs lead. **gap**
- **One alpha text survives**: `components/ui/CommandBlock.tsx:159` `text-warn-ink/60` (G13). The StatCard preview shows "Paused" for a sync and "Unreachable" for a connection (`ui/.design-sync/previews/StatCard.tsx:42`), two words the product retired. **clash**
- **Positives, so nobody re-audits them**: both themes declare the same 46 properties; no hex, rgb or raw z-index in any component; reduced motion covers every keyframe but the named survivors; the tap floor is `var(--tap)` everywhere but the five literals above.

### 1.7 What stays alive and personal, and what could

The keeper's lines all stand as written: the time of day on both greetings, the term's first and last day, "It will keep until morning", the two all-clears, the lift and bloom on the queue you cleared, the mark lighting once (on desktop), the About row. Three things would add to it within the voice, and none needs a word the copy guide would cut:

- **"Since you were last here."** Syndra now records a first sign-in (migration 000075). A last sign-in, written the same way and overwritten each time, would let Home's calm panel say "Since Tuesday, nothing came in." and its working headline say "3 things need you, 2 of them since Tuesday." These count requests and joins, which are Syndra's records of events, never access. A-18 wanted it and could not have it; now it is one more timestamp. **gap**
- **The rail's count flashes once when it changed while you were away** (Sidebar's card). It is the right idea, on the rail only. The TopBar on a phone has no such cue.
- **A dialog's before-line already names the person.** The forecast (P2) is the same courtesy one frame earlier. Finishing its coverage (§2 P8) is where "an extension of you" is still thin.

---

## 2 · Proposals, ranked by impact

S is under a day, M a few days, L a week or more. "Rule" says which standing rule each serves.

### P1 · Names link, everywhere · **M**
- **What.** `UserName`, `ProjectName` and `BundleName` link to their page by default (ink, accent and underline on hover, A-08 2a); only a page's own subject passes `link={false}`. The two local `PersonLink`s go. Service accounts link to their own page.
- **Where.** `components/names/UserName.tsx`, `ProjectName.tsx`, `BundleName.tsx`; the 59 callers mostly change nothing; `app/governance/pending/page.tsx:511` and `components/home/Home.tsx:945` drop their copies; the subject passes `link={false}` on the person, project and bundle pages.
- **Why.** It is G14, settled 4 Nov and not built, and it is what makes A-19 P1 whole: a seeded page needs a link to be opened from, and today most names are not one. Nothing moves; the reader gains a way in from every row.
- **Rule.** G14; RULES "every name that refers to something with its own page links there".

### P2 · The welcome keeps its seat · **S**
- **What.** The welcome never adds a row and never hides a rung. On My access the title's second sentence carries it on the first visit: "Morning, Cody. Welcome to Syndra." in place of "Here's what you can use."; the line below keeps the ladder, and on a first visit with nothing on it the calm rung reads "It keeps the makerspace's list of who can use what, and why." instead of "Nothing needs you right now." On Home the same: "Morning, Kanishk. Welcome to Syndra." takes the headline's seat only when the day is calm; when work is waiting the headline wins, and the calm panel is not shown anyway. The date line never moves.
- **Where.** `components/member/MemberAccess.tsx:221-228`, `components/member/headerLine.ts`, `components/home/Home.tsx:322-331`.
- **Why.** Structure never moves in response to data, and A-18 put the welcome "above every rung", not instead of them. A member whose welcome roles were given at their first sign-in is told "From today, you can use Laser cutting." on the same visit, which is the better welcome.
- **Rule.** `basic-advanced-ia` structure; A-18 §4 idea 8.

### P3 · The shell's Notices move under the header · **S/M**
- **What.** `OfflineBanner` and `DegradedBanner` render in a slot that `PageHeader` owns, 16 below the lede and above the content, on every page. A page with no PageHeader (none today; the app page has one) would render the slot at its top. Their Unfold stays.
- **Where.** `components/shell/AppShell.tsx:94-97` stops mounting them; `components/ui/PageHeader.tsx` mounts a `ShellNotices` outlet after the header; the two banners stay as they are.
- **Why.** G7 ("a Notice sits under the PageHeader, never above it") was written for pages and the shell was left out. On a phone the banner currently pushes the title off the first screen on every outage.
- **Rule.** G7, G5 (PageHeader owns the space below it).

### P4 · Empty states above no headings · **S/M**
- **What.** `ListStates` takes the column headings as a `head` prop and draws them only with rows. Sixteen call sites move their `<CardColumns>` inside it. The four pages that already do this by hand keep their result.
- **Where.** `components/states/index.tsx` (`ListStates`); the sixteen files in §1.4.
- **Why.** G9. A heading row over "Nobody matches that." is a table with no table.
- **Rule.** G9.

### P5 · One word for one state, across the System group · **M**
- **What.** A copy sweep with one table behind it:
  - Zitadel's seat and Notice use the connected-system words: Answering · Not answering (one or two missed, with the pips) · Offline (three missed) · Waiting to retry · Not connected. The Notice's title is the seat's word plus a full stop. The meta never says a count it has not read: while projects load it says "Answering" and nothing else.
  - The member's "Online" becomes "Answering" (`MyStorage.tsx:387-392`), as A-01 settled.
  - "Not created by Syndra" everywhere (`UnconfirmedChanges.tsx:101,109`).
  - "people" for every count ("12 people have an account", "Accounts Syndra manages" stays as a card title because it names accounts, not people).
  - Full stops on every EmptyState and Notice title in §1.4.
  - The connected-systems empty state names the step, not a file: "No system is connected. Connecting one is done on the Syndra server; the person who runs it has the steps."
  - "Withdraw" on a service account's queued item becomes the queue's own verb (Cancel the change, via CancelQueuedStep); "direct grant" becomes "given by hand".
- **Where.** `app/zitadel/page.tsx:59,76-79,309,327`, `components/storage/MyStorage.tsx`, `components/targets/*`, `app/system/targets/page.tsx:92-94,179,184`, `components/system/ServiceAccountPage.tsx:212,217`.
- **Why.** One name per thing is the copy guide's first rule, and the System group is where it slipped because nobody designed those screens.
- **Rule.** Copy guide §6; RULES "Zitadel verdict: dot + word"; A-01; G9.

### P6 · Go to and the mark reach the phone · **S**
- **What.** The TopBar at phone width gains two things it lacks: the SyndraMark at its left (16px, the same drawing as the rail's, so the one-shot bloom has a seat on a phone) and a 44px "Go to" button at its right, beside the avatar, for operators in both views. It opens the same sheet the Advanced nav sheet holds, Input at the top. Members do not get it: three pages need no jump. The Keys sheet stays keyboard-only; a phone has no `?`.
- **Where.** `components/shell/TopBar.tsx:32-40`, `components/shell/GoToHost.tsx`, `TouchNav.tsx:383-405` (the sheet is reused, not duplicated).
- **Why.** A3.8 asked whether to add a tab; a tab changes Basic's structure (four tabs become five) for a surface that is not a destination. A bar button is not structure, and it is where a phone user already looks for the account.
- **Rule.** `basic-advanced-ia` structure; A-19 P4.

### P7 · Errors finish the sentence · **S/M**
- **What.** `ErrorState` carries the copy guide's four parts: which thing failed and which did not ("Zitadel didn't answer. Syndra itself is fine."), the state of the world ("Nothing was changed."), the next step and where ("Try again in a minute. If it keeps failing, ask the person who runs the Syndra server and quote the reference below."), and the reference with "Quote this if you ask for help." `describeFailure` maps a network fault to "Syndra's server didn't answer." instead of the browser's words. A page reached by a link keeps its seeded name on error (`app/projects/[id]/page.tsx:94`, `AppTokenScreen.tsx:94-101`).
- **Where.** `components/states/index.tsx:143-185`, `lib/outcome.ts:104-112`, the two pages.
- **Why.** §8 of the copy guide is the rule; one surface (`global-error.tsx`) follows it and sixty do not.
- **Rule.** Copy guide §8; RULES "a page opened from a link already knows the thing's name: show the name while loading and on error".

### P8 · The forecast, everywhere an act is taken · **M**
- **What.** Every act in a row or a dialog forecasts in the first frame, honestly:
  - Drift's three resolutions, Remove (direct and bundle), Give a role, Ask to keep it, the bulk dialogs and Cancel a queued change gain the forecast seat (`ActionForecast`).
  - The forecast is Syndra's own next step: an approve of a role that waits forecasts "Will wait for you to send", not "Will be approved"; Extend says "Will be extended to 26 Nov".
  - The rail's count is set from the answer (`setQueryData(["governance","indicators"])`) and the refetch confirms it; the number simply changes (S4).
- **Where.** `components/review/UnexplainedAccess.tsx:1175-1200`, `components/people/RemovalDialog.tsx:269,414`, `lib/request-decisions.ts:115-140`, `app/review/expiring-access/page.tsx:653`, `PersonAccess.tsx:829`, `lib/queries/useGovernance.ts`.
- **Why.** The two acts with the highest stakes are the two with no forecast. A forecast that is sometimes wrong is worse than none.
- **Rule.** Copy guide §5 ("the first 100ms is said in the future tense"); FINAL S4.

### P9 · The design system tells the truth · **S** (the table) + **M** (the publish)
- **What.** `moves.json` corrected for the twenty cards in §1.6 and ArriveList's 8px; the Moves and Phone lines reach the published cards; the README's token table gains scrim, tap, layers, tints, hover, focus ring, motion and radius; `--font-sans` is either used by `body` or dropped, and `--radius-shell` with it; Button's card shows sm buttons and names G1 and G16; Notice names G7 and the breathing dot; Modal names G4 and its phone form; FilterBar's card is rewritten to G18; StatCard's preview drops "Paused" and "Unreachable"; `CommandBlock.tsx:159` loses its alpha. The four failing captures are fixed (AppShell, RoleWordsTable, SignInRow, TouchNav) and RoleWordsTable, SignInRow, ReadGate, FormPart and SectionPill get previews; ArriveList, GlideIndicator and LeavingRow are marked "internal: never mounted alone" on their floor card rather than given a preview nobody would use.
- **Where.** `ui/.design-sync/moves.json`, `conventions.md`, `previews/*`, then the resync and publish (BRIDGE, the owner's OK in session).
- **Why.** A board author reads the card, not the source. A card that says a seat is still draws a still seat.
- **Rule.** FINAL §4; BRIDGE "components reach the canvas only through `ui/.design-sync`".

### P10 · One jump from Basic to Advanced · **S/M** (new part: `AdvancedLink`)
- **What.** A link from a Basic surface to an Advanced-only route sets the view on the way, so the reader lands with the rail that has the row. `ArrowLink` does it itself: when the view is Basic and `nav.ts` marks the destination `advancedOnly`, the click switches the view (the switch glides, A-08 §3) and then navigates; nothing else changes. Home's "Pending changes and Drift are in Advanced." becomes that link. The Makerspace strip keeps its three tiles in Basic (structure) and their links switch the view.
- **Where.** `components/ui/ArrowLink.tsx`, `components/ui/NextStep.tsx`, `lib/nav.ts` (a lookup), `components/home/Home.tsx:304-308`.
- **Why.** The contract says "one scoped jump that sets the view". `revealInAdvanced` is used once; the other twenty-odd links leave a Basic reader on a page with no rail row and no way back but Back.
- **Rule.** `basic-advanced-ia/design.md` §1 (no dead ends); A-08 §3.

### P11 · The System group designed, part one: no raw ids, no JSON · **L** (new part: `TechnicalDetail`)
- **What.** Where a screen shows the machine's words, one part holds them: `TechnicalDetail`, a fold (FoldChevron, Unfold) titled "Technical detail", for whoever runs the server, holding a `CommandBlock` or `TokenBlock`. Everything outside it is words:
  - Capabilities: "Makes accounts", "Sets passwords", "Removes accounts", "Opens folders" with the StatusSeat; the operation ids inside Technical detail.
  - Mappings: "Everyone with Fabrication lead gets the fabrication group" as the row; the role's name, not its key; "gives group = fabrication" goes.
  - Incoming events: "Show details" becomes a fold of named facts (who, what, when, from where) with the raw event under Technical detail.
  - Claims delivery: "Expected … found …" becomes a sentence per check ("Zitadel still runs the old action syndra-claim-injector. Syndra expected it to be gone."), with the raw values under Technical detail.
  - The Zitadel key's setup commands move under Technical detail; the row says "Rotate the key on the Syndra server. The person who runs it has the command."
  - Unknown subjects read "Unknown account" with the id as CopyableValue (A-07 §7), never the id as the name.
  - The Waiting on you region gets its empty state, "Nothing is waiting on you.", and the Unconfirmed changes card keeps its seat with it.
- **Where.** `components/targets/TargetOverview.tsx:918-923,1047-1117`, `MappingManagement.tsx:290-305,361,442`, `app/operations/page.tsx:434-438`, `components/system/ClaimsDelivery.tsx:222-226,480`, `app/zitadel/page.tsx:200-228`, `UnconfirmedChanges.tsx:44-52`.
- **Why.** These are the screens the owner opens when something is wrong, which is when raw ids cost the most.
- **Rule.** G10; copy guide §8 ("those may sit under a Technical detail disclosure").

### P12 · Insights to the rules · **M**
- **What.** The finding chips keep their seats with a hollow count; unused nodes are receded (canvas ground, line-strong edge) not dashed; a warn or danger answer is a Notice-toned line, not a box; dots are StatusDot; "Back" becomes "Back to the map" and "Pin" becomes "Keep this answer"; Trends gets ReadFreshness and a failed read says so; below tablet the List view RULES promise exists: the lens as a Select, then the nodes as a plain list with their counts, the answer above.
- **Where.** `components/insights/MapTab.tsx:269,349-364,437`, `tone.tsx:40`, `LaneMap.tsx:240,255`, `AnswerPanel.tsx:30-33,112-139`, `TrendsTab.tsx:51,208`.
- **Rule.** Structure; G8; G19; copy guide §5; RULES Phone (87c).

### P13 · The claims page's leftovers · **S**
- Client ID leaves Basic (`AppTokenScreen.tsx:263`); FoldChevron on every claim line (`ShapeCard.tsx:199`); Cancel in the footer's slot (`ShapeSheets.tsx:156,293`, `ShapeCard.tsx:1260`); the TrueNAS offer and Debug mode's copy use `AdvancedLink` or lose the Advanced name; a preview error with a prior preview shows the prior one receded with its age and a Notice (`ClaimPreview.tsx:123`); the page error keeps the seeded header.

### P14 · One breakpoint, and the phone's number seats · **S**
- `sm:` becomes `tablet:` in `FilterBar.tsx`, `Home.tsx`, `PersonAccess.tsx`, `Modal.tsx`; the five literal tap values become `var(--tap)`; the unprefixed 110px seats on Roles that wait, Bundles and Recent changes take the phone form the other lists use (the number as a labelled line under the name).

### P15 · Who you are, on the top bar · **S**
- The chip at tablet and up shows the first name ("Kanishk"), as RULES says. The phone shows the mark (P6) and the avatar. Members lose "Sound when drift is found" (`AccountSheet.tsx:129`: operators only).

### P16 · Pending's three narrowings become one · **S**
- `?q=` stays the search; `?person=` and `?role=` are the one in-force FilterChip with the read line "Showing Cody Rhodes's 2 changes. Sending sends all 7." The three `NextStep` callers that do not pass a person do (`CancelQueuedStep.tsx:259`, `RolePickerOne.tsx:252`, `PersonStorage.tsx:199`).

### P17 · The small sweep · **S**
- A lone dash standing in for a value in cells (`app/system/targets/page.tsx:260,296`, `PersonStorage.tsx:181`, `app/bundles/page.tsx:131`, `app/changes/roles-that-wait/page.tsx:124`) become the quiet word the column means ("None", "Not yet"). "and N more" becomes a violet link on Change history (`app/operations/cascades/page.tsx:148`) and the storage card (`PersonStorage.tsx:518`). The Keys sheet's lede says what is true: "These work on every page. In a list, the arrows move between rows." and gains ⌘K. The Go to placeholder reads "A page" for members and "A person, a project, an app, or a page" in Basic. The role page's meta loses the raw key in Basic (`roles/[key]/page.tsx:82`). The bundles lede loses its third sentence (`app/bundles/page.tsx:47`).

### P18 · Since you were last here · **M** (the owner's, Q9)
- **What.** The backend records each sign-in's time, keeping the previous one (one row, two timestamps). Home's calm panel reads "Since Tuesday, nothing came in." and its working headline "3 things need you, 2 of them new since Tuesday." My access's calm rung reads "Nothing has changed since you were last here, on Tuesday." Relative words under a week, a date beyond it.
- **Why.** It is the keeper knowing when she last saw you. It counts Syndra's records of events (requests, joins, changes), never access.
- **Rule.** A-18 §1.1 (wanted, deferred); the holder rule (records explain, never count access).

### P19 · Back keeps the rest of its place · **S**
- A person's project folds and the claims editor's open line join `usePlace`; a query-only change keeps the page's scroll (EOY-P5.4, P5.5). Requests' remembered status pill is shown as an in-force pill only when it is not "Open", so a remembered "Declined" is never silent.

### Deliberately not proposed
- A route transition, a toast, a letter shortcut for a verb, a recents list in Go to, a density setting, a mascot, a tour.
- Phone arrows between queue rows (C2.6): a phone has no arrows, and on desktop the queues' rows are not links; P1 makes the name the row's link, which is the roving target the queues lacked.
- Rewriting the all-clear titles into one sentence: A-18 chose them one by one, and the search-empty phrasings are the only set worth unifying (P17 leaves it to code's taste: "Nothing matches that." everywhere a search empties a list).

---

## 3 · The first wave

Recommended: **P1, P2, P3, P4, P5, P6, P7, P8**, in that order. About two weeks. They add no part and no loop; they apply G14, G7, G9, §6 and §8 of the copy guide, and A-18 and A-19's own words where the builds fell short, and they give the phone and the two highest-stakes acts what desktop and the queues already have.

Second wave: **P9, P10, P13, P14, P15, P16, P17, P19**. Third: **P11, P12, P18**.

---

## 4 · How the boards read

**`a20-rules-at-the-edges`** (App screens page, right of A-19, 1600 wide)
1. The shell's Notice under the header (P3): People with Zitadel not answering, today beside proposed, dark and light, and the phone.
2. An empty list with no headings (P4): People with a search that matches nobody, today beside proposed.
3. Names link (P1): a Requests row and a Pending changes row with the names as links, at rest and on hover, dark and light.

**`a20-first-visit-and-phone`**
1. The welcome in its seat (P2): My access on a first visit with a role given today, today beside proposed, dark and light, and the phone; Home on a calm first visit.
2. The phone's top bar (P6, P15): the mark at left, Go to at right, Basic's four tabs below, dark and light.
3. An error that finishes the sentence (P7): People failing to load, today beside proposed.
4. The forecast on Drift (P8): a Drift row in four moments, dark and light.

**`a20-system-in-words`**
1. Zitadel's health line, one word (P5): Answering, Not answering with the pips, Offline, each with its Notice, dark and light.
2. A system's Waiting on you with nothing waiting (P5, P11).
3. Incoming events' details as words with Technical detail folded (P11), today beside proposed; the phone.

The boards mount the real parts from `window.Syndra`. `AdvancedLink` and `TechnicalDetail` are drawn inline and say so (BRIDGE).

---

## 5 · For code (once the owner answers)

1. **P1:** `link` prop defaulting to true on the three name parts; `link={false}` on each page's own subject; delete the two `PersonLink`s; a test that every `UserName` outside its own page renders an `<a>`.
2. **P2:** `MemberAccess` title takes `session.firstVisit` into its second sentence; `headerLine` takes `firstVisit` and returns the About sentence for rung 7; `Home` drops the inserted `div` and passes `firstVisit` to the headline when calm. Tests: a first-visit member with an ending role sees rung 1; the date line's offset is equal on both visits.
3. **P3:** `ShellNoticesOutlet` in `PageHeader`; `AppShell` stops rendering the banners; test: the banner's DOM order is header, banner, content.
4. **P4:** `ListStates head={<CardColumns>…}`; sixteen call sites; test: no `CardColumns` in the DOM when the list is empty.
5. **P5:** one `SYSTEM_WORDS` table in `lib/outcome.ts` or beside `TargetReading`; `plain-language.test.ts` gains "Online", "Unreachable", "Not made by Syndra" and "accounts" as a count word.
6. **P6:** `TopBar` mounts `SyndraMark` and the Go to button below tablet; `GoToHost` exposes `open()`.
7. **P7:** `ErrorState` grows `didNot`, `next` and the reference line; `describeFailure` maps `TypeError` to the server sentence; tests per part.
8. **P8:** `ActionForecast` in the six remaining surfaces; `forecastFor(act, role)` returns `queued` for a role that waits; `setQueryData` on indicators in each mutation's `onSuccess`; tests rung by rung.
9. **Copy guide:** add "Technical detail" to §8 as the one disclosure, "Since you were last here" if Q9 is yes, and the System words table to §6.

---

## 6 · Questions for the owner

Each "yes" accepts the recommendation.

1. **Names link by default everywhere (P1), with only a page's own subject unlinked?** *Recommended: yes.*
2. **The welcome never adds a row (P2): on My access it is the title's second sentence, with the ladder still speaking below; on Home it takes the headline's seat only on a calm day?** *Recommended: yes. On a working day the work is the welcome.*
3. **The shell's offline and degraded Notices move under the page header (P3)?** *Recommended: yes.*
4. **Column headings leave with the rows (P4)?** *Recommended: yes.*
5. **One word per state across the System group (P5), with Zitadel on the connected-system words and the member reading "Answering"?** *Recommended: yes.*
6. **The phone's top bar gains the mark at left and a Go to button at right for operators (P6), with no fifth tab?** *Recommended: yes.*
7. **Errors carry the four parts (P7) and a network fault says "Syndra's server didn't answer."?** *Recommended: yes.*
8. **The forecast reaches Drift, Remove, Give, bulk and Cancel (P8), says "Will wait for you to send" for a role that waits, and the rail's count changes at the answer?** *Recommended: yes.*
9. **Record each sign-in's time so Home and My access can say "since Tuesday" (P18)?** *Recommended: yes, third wave. It is one row, two timestamps.*
10. **A link from Basic to an Advanced page switches the view on the way (P10), so the rail always has the row?** *Recommended: yes, second wave.*
11. **Technical detail becomes the one disclosure for the machine's words (P11), and the System group's raw ids, JSON and commands move inside it?** *Recommended: yes, third wave.*
12. **The first wave is P1 to P8?** *Recommended: yes.*
