# A-17 · Refinement sweep (draft, 4 Nov 2026)

**Owner's ask, 4 Nov:** "do a refinement sweep for consistent, cohesive, elegant, enriching UX and UI."

**Scope.** Every operator and member screen. I read the code (`ui/src/app/**/page.tsx` and the components they use) and the prod screenshots (operator, Advanced, dark, 1482px). I checked both against RULES.md, FINAL.md, the copy guide (`plain-language-copy/design.md`), the design-system parts, and the owner's rulings of 4 Nov:
- a lone button is outline sm;
- ghost appears only as the quieter half of a pair;
- destructive is outline danger at its seat's size;
- there are no bare text buttons;
- names are A to Z;
- calm by default;
- no clutter;
- one pattern per job.

**Excluded.** The claims card on the app page (`apps/ShapeCard.tsx`, `ShapeChooser.tsx`, `ShapeSheets.tsx`, `claim-words.ts`), which is being rebuilt to A-16. The rest of the app page is included: its header, preview, tools card, Debug mode and History. Login's ambient motion is excluded too, as it is allowed by `route-login`.

**Severity.**
- **clash**: visibly inconsistent with the rest of the product.
- **polish**: finer.

Fixes prefer deleting code. No board was needed: every finding maps to a part that already exists. `file:line` is relative to `ui/src`.

---

## 1 · Global rules to add or tighten in RULES.md

Each is one line, with the wording to add. Most findings in §2 are instances of one of these. Fixing the shared part first clears most of them.

| # | Rule (wording for RULES.md) | Fixes at the source |
|---|---|---|
| G1 | **Buttons.** "A button on its own is outline sm. Ghost is only the quieter half of a pair beside a primary (Cancel, Discard) and PaginationRow's own controls. No control is text styled as a button: `link-tint` and the `accentSoft` variant are retired." | Delete `accentSoft` (2 callers) and `link-tint` (10 callers) |
| G2 | **Going vs doing.** "A control that only opens a page is an ArrowLink. ButtonLink stays only where the page it opens starts the act its label names (Give a bundle, Ask to keep it). One link per destination per block: no header 'See all →' beside an 'and N more →'." | `CardHeaderLink` renders `ArrowLink` (it drops its arrow today, and callers type "→", which is read aloud) |
| G3 | **Showing more.** "More of the same row is one control. A row with no buttons of its own opens itself (CardRow onToggle). Otherwise use Button outline sm 'Show X' / 'Hide X', with no chevron. A section folds by the 28px round chevron, one part (FoldChevron). No ›, and no muted text toggles." | Six hand copies of the fold chevron in two border tones; four toggle grammars on Zitadel alone |
| G4 | **Dialog footer.** "Cancel first, ghost; the act last (accent, or dangerConfirm). After the result, one outline Close. The result is ActionOutcome with its word, never the 'Applied' fallback; a dialog never closes onto the row." (Q1) | Make `ActionOutcome`'s `word` required |
| G5 | **Page rhythm lives in one place.** "PageHeader owns the 28 below it; a page never sets its own header gap. Content stacks at 16. The read line is ReadFreshness, 10 under a FilterBar, never placed by a negative margin." (Q5) | Header→content measures 16, 18, 20, 28 and 0 today (§2.1) |
| G6 | **Eyebrow.** "An eyebrow names a parent thing (the project above a role page), never a nav section: the breadcrumb already says that." | Delete 5 eyebrows |
| G7 | **Notice.** "A Notice sits under the PageHeader, never above it. A fact a Notice states is not restated by the card that holds it." | Zitadel, Incoming events |
| G8 | **Dots.** "Every status dot is StatusDot (6px); a dot with its word is StatusSeat. Nav tone dots are the only other dot." | About 25 hand-made dots at 6, 7 and 8px |
| G9 | **Empty.** "Every empty, quiet or all-clear state is EmptyState (resolved = lime dot), inside its card, under no column headings. Its title is a sentence with a full stop. Its action is an ArrowLink or a Button outline sm, never a second copy of the header's action." | Fix `EmptyState`'s action (a bare text button today) once |
| G10 | **Ids.** "A raw id never appears in a heading, a meta line, a sentence or a table cell. In Advanced it lives once, in the lineage or evidence fold, as CopyableValue. An actor or rule Syndra can name is shown by name." | Drift actor ids, grant ids in headings, rule ids as names, `c_` handles |
| G11 | **Em dash.** "On screen, an em dash never joins a sentence or makes 'WORD — fragment'. Use a full stop, comma or colon." Add it to `plain-language.test.ts`. | About 80 instances |
| G12 | **Said once.** "A fact is said once per screen: lede, card note or row, never two. A lede is at most two sentences." | Repeated notes on almost every queue |
| G13 | **No alpha text.** "Quiet text is `--muted` or `--faint`; `text-ink/NN` is not used, whatever its value." (Tightens A-09.) | `Badge`, `RowField`, nav, Home, Requests, AccessSource, BundlePage |
| G14 | **Names link.** "UserName, ProjectName and BundleName link to their page by default (ink, accent on hover); only the page's own subject passes `link={false}`." | Plain names on Requests, Home, Pending, Audit and Change history |
| G15 | **One choice card, one chosen chip.** "A card you choose is ChoiceRow. A chosen thing with × is one Chip part." | 8 hand-made radio/checkbox cards, 3 chip shapes |
| G16 | **Seats.** "Inside a card or row a button is sm; md only in a page header and a dialog footer. A number seat is 110." | Bundle end cards, Trends 72px, rules 130px |
| G17 | **Silence for normal.** "When a count is all as expected, nothing more is written (no 'confirmed', no 'none'); only the exception is." | `lib/holders.ts`, People's "none" |
| G18 | **FilterBar** (amends 9b to match `FilterBar.tsx`). "A pill set that chooses which list you are reading (Requests' status, Expiring's window) leads; search follows; Filters and Select sit right." | No code change |
| G19 | **Boxes.** Add to "A box means something is wrong": "`accent-note`, `bg-tint-1` and `bg-accent-soft` panels never hold explanation. Dashed means NotOpenYet and nothing else." | About 20 tinted or dashed explanation boxes |

---

## 2 · Findings by screen

### 2.1 Shared parts and shell (fix once, clears many screens)
- `components/states/index.tsx:121-135`: `EmptyState`'s action is a hand-made violet `<Link>` with no arrow, or a bare text `<button>`. Fix: the `href` form becomes `ArrowLink`, the `onClick` form becomes `Button outline sm`. **clash**
- `components/states/index.tsx:114`: hand-made 8px resolved dot. Fix: `StatusDot tone="healthy"`. polish
- `components/ui/Card.tsx:184`: `CardHeaderLink` is `ARROW_LINK` without the arrow, and five callers type "→" into the text (`Home.tsx:581,829`, `Makerspace.tsx:381`, `RecentChanges.tsx:41`, `ProjectSetup.tsx:186`). Fix: render `ArrowLink` and delete the typed arrows. polish
- `components/ui/Button.tsx:63`: `accentSoft` has two callers, both wrong (`PersonAccess.tsx:1105`, `RemovalDialog.tsx:542`). Fix: delete the variant (G1). **clash**
- `app/globals.css:1456`: `link-tint` exists only to style text buttons. Fix: delete it after its 10 callers move (G1). polish
- `components/ui/ActionOutcome.tsx`: with no `word`, it falls back to "Applied" (Pending, Roles that wait, Zitadel users and projects). Fix: make `word` required. **clash**
- `components/ui/SelectionBar.tsx:183`: "— select only the 25 people shown" is a bare text button inside a sentence. Fix: `Button outline sm` "Select only the 25 shown". **clash**
- `components/ui/SelectionBar.tsx:340`: `SelectionAction` hand-styles its own outline and danger. Fix: `Button` with `size="sm"`. polish
- `components/ui/CancelQueuedStep.tsx:343`: "Show all N" is a bare text button. Fix: `PaginationRow`. **clash**
- `components/ui/RehearsalDialog.tsx:570`: the stale-plan warning is a `bg-warn-soft` box. Fix: `Notice tone="warn"`. **clash**
- `components/ui/PlanReview.tsx:100`: rows are in API order. Fix: `sortByName`. **clash**
- `components/ui/PlanReview.tsx:107,150`: names are truncated. Fix: wrap. polish
- `components/ui/PlanReview.tsx:172` and `ActionOutcome` `OutcomeMark`: hand-made pills. Fix: `Badge`. polish
- `components/states/DegradedBanner.tsx:130-143`: "For whoever runs the Syndra server" is a text toggle with ›. Fix: `Button outline sm` (G3). **clash**
- `lib/nav.ts:147`: Incoming events (`/operations`) prefix-matches `/operations/cascades`, so on Change history the rail lights two rows. Fix: `pattern: /^\/operations$/`. The coordinator saw "Incoming events" after clicking Change history. Change history's own href is right, so check that click in the browser after this fix. **clash**
- Fold chevron, hand-copied 6 times in two border tones with no shared part: `governance/pending/page.tsx:254`, `UnexplainedAccess.tsx:814`, `WithdrawnAccess`, `expiring-access`, `PersonAccess.tsx:480`, `zitadel/page.tsx:323`. Fix: one `FoldChevron` (extend `bundles/FoldHeading.tsx`). **clash**
- Header→content gap, measured. The rule is 28 (G5, Q5).
  - `gap-[18px]`: 16 pages.
  - `gap-7`: Bundles, Settings, Roles that wait.
  - `gap-4`: Drift, Zitadel, Insights, the person page.
  - `gap-5`: the app page, TrueNAS.
  - Nothing at all: Network storage, where the card touches the lede in the screenshot.
  - `22px`: the member landing.
  - Fix: PageHeader carries the space; delete the per-page values. **clash**
- Read line under the FilterBar, measured. The rule is 10.
  - 12 (`-mt-1.5`): People, Projects, Roles.
  - 14 (`-mt-1`): Incoming events, Expiring, Drift.
  - 18: Apps.
  - Above the FilterBar: Zitadel grants.
  - 4px from the card: Service accounts.
  - Inside the header: the project page.
  - Missing: Requests, Pending (hand-made text instead), Audit, Automatic rules, Connected systems, Change history, and empty Unfinished revocations.
  - Fix: G5. **clash**
- `components/shell/Sidebar.tsx:68-73`: the "Advanced" pill beside the logo repeats the ViewSwitch in the top bar. Fix: delete it. polish
- `components/shell/Sidebar.tsx:163` and `TouchNav.tsx:144`: inactive rows are `text-ink/75` in one and `/80` in the other. Fix: `text-muted` in both (G13). polish
- `components/shell/TouchNav.tsx:378`: the visible VIEW_EXPLANATION repeats the switch. Fix: delete it. polish
- `components/shell/AccountSheet.tsx:134`: "Install as an app" is a hand-bordered `<button>`. Fix: `Button outline sm w-full`. **clash**
- `components/shell/AccountSheet.tsx:94`: hand-made violet link, and the arrow is read aloud. Fix: `ArrowLink`. polish
- `components/shell/AccountSheet.tsx:49,69`: `text-ink/70`. Fix: `text-muted`. polish
- `components/shell/AccountSheet.tsx:153`: the caption's second sentence. Fix: cut it. polish
- `ui/Badge.tsx:15` (neutral) and `ui/Card.tsx:229` (`RowField`): `text-ink/80`. Fix: `text-ink` (G13). polish
- `app/global-error.tsx`, `app/layout.tsx:67`: hex. These are argued exceptions (they render without CSS, or sit in a meta tag). No action.

### 2.2 Home (`components/home/Home.tsx`, `Makerspace.tsx`)
- `:581` and `:829`: "See all →" and "Open Service accounts →" sit in the header, and the same block ends "and N more →". Fix: drop the `action` (G2). **clash**
- `:925-934` "Sent, and not seen back yet":
  - The paragraph explains what the block is not, with an em-dash aside. Fix: one sentence, "Zitadel accepted these and hasn't shown them back yet."
  - The roles seat prints "—" on every row (screenshot). Fix: render the seat only when keys exist.
  - **clash**
- `:937-964`: the block has no 5/3 cap. Fix: `Capped href="/users"`. **clash**
- `:959`: "Look at their access" is a ButtonLink to the same page as the name link. Fix: delete it. **clash**
- `:776`, `:1007`, `:1047`, `:1092` ("Review them"): ButtonLinks that only open a page. Fix: `ArrowLink` ("Review on Drift"), per G2. **clash**
- `:895`: a dashed amber box, "Sending waits until Zitadel answers", repeats the DegradedBanner. Fix: delete it. **clash**
- `:702`: "No action — expires {date}". Fix: "Ends {date}", once. **clash**
- `:1039`: "the reconciliation found". Fix: "differences Syndra found". **clash**
- `:971`: "A withdrawal" for a revoke. Fix: "A revocation". **clash**
- `:591`, `:693`: requester and expiring names don't link, while sibling blocks do. Fix: G14. **clash**
- `:299-303`: the Basic footnote is rationale. Fix: "Pending changes and Drift are in Advanced.", or delete it. polish
- `:863`, `:932`, `:1001`, `:1043`, `:774`: "— aside" tails. Fix: G11. polish
- `:343-356`: the third status line under the headline ("Checked, nothing waiting: …"); its links have no 44px floor. Fix: `ARROW_LINK` height. polish
- `:345`, `:390`: hand dots. `:387-397`: the calm line is a hand-made EmptyState. Fix: G8, G9. polish
- `:594`, `:696`, `:947`: `text-ink/80`. polish
- `:1069`: a note as a bare `<p>`. Fix: `CardHeader note`. polish
- `:840`: `w-[170px]` with no phone fallback. polish
- `:993`: unvouched targets are unsorted. polish
- `Makerspace.tsx:44-46`: "Not your queue — the shape of the place…" is rationale. Fix: delete it. **clash**
- `Makerspace.tsx:366-413`: Lately shows 8 rows. Fix: cap at 5 (3 on a phone). **clash**
- `Makerspace.tsx:337-352`: hand-made violet links. Fix: `ArrowLink`. **clash**
- `Makerspace.tsx:170`, `:270`, `:371`: hand-made empty and error lines. Fix: G9. polish
- `Makerspace.tsx:137`: "catalogue". Fix: "Reading roles." polish
- `ClaimsLine.tsx:25-35`: "Claims" has no `Term`, and the error line is a lowercase run-on. polish

### 2.3 People list (`app/users/page.tsx`)
- `:273`: `meta` holds "· N with access ending inside 30 days". It is a sentence, and the window is 14. Fix: delete it; the Attention column carries this. **clash**
- `:609`: the Bundles cell says lowercase "none" on most rows. Fix: render nothing (G17). polish
- Attention column: "4 unexplained roles" wraps onto two red lines at 1482 (screenshot). Fix: "4 unexplained" in a nowrap seat. polish
- `:344`: service-account matches are unsorted. polish
- `:604`: the bundle is a Badge here but a Chip on the person page. Fix: Chip. polish
- `:696`: "3 end, the first in 2 days". Fix: "3 ending, first in 2 days". polish

### 2.4 A person's page (`components/people/*`)
- Header (`PersonAccess.tsx:274-301`):
  - `meta` carries the raw Zitadel id and a hand-made violet "Full audit trail".
  - Fix: delete the link (the Activity tab is it). The id becomes a CopyableValue in Advanced only (G10, Q6).
  - **clash**
- `PersonAccess.tsx:383-389`: "Manage bundles" is a bare text button, and the header has the same action. Fix: delete it. **clash**
- `PersonAccess.tsx:798`: "Grant 3827…" sits in every project heading, and `:1067` lineage shows a different "Grant id" (a UUID). Two ids carry one name. Fix: delete the heading id; keep lineage only. **clash**
- `PersonAccess.tsx:242-250`: the section chips read "TrueNAS 1 ↓" (project) and "TrueNAS ↓" (system) side by side. Fix: label the system chip "Network storage", as its section heading already reads. **clash**
- `PersonAccess.tsx:583-595`: cleanup hints draw a "?" glyph, an "Advisory ·" prefix and a faint sentence, and the backend copy says "user… direct platform grants". Fix: a muted caption with no glyph or prefix, and rewrite the backend hint ("No bundle yet: everything they hold was given directly."). **clash**
- `PersonAccess.tsx:706-723`: "NOTHING EXPLAINS THIS" is an uppercase amber label used only here, and the heading already says "1 role Syndra never gave". Fix: keep the heading chip; the row gets `StatusSeat warn "Not given by Syndra"` plus ArrowLink "Decide on Drift". **clash**
- `PersonAccess.tsx:976-1000`: the Zitadel verdict is coloured words with no dot ("In Zitadel"). Fix: `StatusSeat` (2b). **clash**
- `PersonAccess.tsx:777`: "Zitadel couldn't be read" is repeated in every project heading. Fix: one Notice under the header. **clash**
- `PersonAccess.tsx:788`: "No grant in Zitadel — see Drift › Side by side" names a place but isn't a link. Fix: `ArrowLink` "Side by side on Drift". **clash**
- `PersonAccess.tsx:1104-1110`: an accentSoft button reading "This came from an automatic rule — Open automation details →". Fix: caption "This came from an automatic rule." plus `Button outline sm` "Show the rule". **clash**
- `PersonAccess.tsx:1066`: the lineage sits in a `bg-tint-1` box (the screenshot's Grant id box). Fix: a plain `dl` (G19). polish
- `PersonAccess.tsx:1036`: Remove is outline, not danger. Fix: `danger sm` (verbs rule). polish
- `PersonAccess.tsx:399`: em dash. polish
- `PersonStorage.tsx:92-95`: the "Limits, holds and folders are set on…" caption repeats every row's "Set on TrueNAS's page →". Fix: delete it. **clash**
- `PersonStorage.tsx:200-203`: a hand-made "Waiting for you to send". Fix: `NextStep`. **clash**
- `PersonStorage.tsx:523`: "and N more on X's page" is plain text. Fix: `ArrowLink`. **clash**
- `PersonStorage.tsx:477` (folders) and `:206,280` (roles): unsorted. **clash**
- `PersonStorage.tsx:133`: hand-made dot. `:66`: "Syndra's server did not answer". polish
- `PersonRequests.tsx:127`: `tablet:truncate` on the role, which is a decision value. Fix: wrap. **clash**
- `PersonRequests.tsx:63`: When is 110 here but 66 on Requests. Fix: one width. polish
- `PersonRequests.tsx:94-102`: hand-made footer with a dash. polish
- `PersonActivity.tsx:194-208`: the raw target id "truenas". Fix: `targetLabel`. **clash**
- `PersonActivity.tsx:234`: hand-made warn box. Fix: Notice. **clash**
- `PersonActivity.tsx:111`: "Review › Audit" written as a path. Fix: `ArrowLink` "Audit". polish
- `SignInLookup.tsx:61,68,124` and `PersonActivity.tsx:64`: "Syndra keeps only the last sign-in" is said four times. Fix: keep the caption and the footer note (G12). **clash**
- `AccessSource.tsx:318`: the popover trigger renders with no body and is a dead focus stop. Fix: render a bare chip when `!hasBody`. **clash**
- `AccessSource.tsx:117-130`: alpha text and borders. `:359`: "Granted directly". Fix: "Given directly". polish

### 2.5 A person's dialogs
- `ManageBundles.tsx:173-206`: hand-made square 18px checkbox cards. Fix: `ChoiceRow` (round seat). **clash**
- `ManageBundles.tsx:334-346`: three notes in the button row. Fix: one `ModalFooter note`. **clash**
- `ManageBundles.tsx:212`: tinted preview, and `:204` "new members". polish
- `GrantDirectAccess.tsx:158-223`: the end date is said three times. Fix: the date control plus one footer note. **clash**
- `GrantDirectAccess.tsx:218`: a `bg-warn-soft` box for a role that waits. Fix: a plain note with WaitsMark. **clash**
- `GrantDirectAccess.tsx:109,287`: "Grant direct access", while the result says "Given" (Q7). polish
- `RemovalDialog.tsx:93,213,392,406,522`: the raw role key appears in the title and the body. Fix: `RoleRef`. **clash**
- `RemovalDialog.tsx:381-408`: an uppercase label and three tinted boxes (lose, keep, rule). Fix: `type-label` lists with StatusDot. **clash**
- `RemovalDialog.tsx:542`: an accentSoft ButtonLink "Open the rule →" in the footer. Fix: `ArrowLink` in the body; the footer is Close. **clash**
- `RemovalDialog.tsx:106-117`: hand-made source options. Fix: ChoiceRow. `:527`: "⇒". polish
- `BulkDialog.tsx:127-160`: native Selects for project and role. Fix: `RolePickerOne` (RULES: never a native select for roles). **clash**
- `BulkDialog.tsx:100`: "audit log". Fix: "Change history". polish

### 2.6 Requests (`components/requests/*`)
- `RequestsScreen.tsx:318`: `<Avatar name={undefined}>` draws the empty circle (screenshot: Abhinav Hari). Fix: pass the resolved name. **clash**
- `RequestsScreen.tsx:319-324`: requester and role don't link, and the requester is truncated. Fix: G14; wrap. **clash**
- `RequestsScreen.tsx:216-244`: no read line. Fix: `ReadFreshness` (G5). **clash**
- `RequestsScreen.tsx:247`: column headings render only when something is pending, but the rows keep the same layout. Fix: always render them. **clash**
- `RequestsScreen.tsx:566`: the outcome says the same sentence in `message` and in `detail`. Fix: drop `detail`. **clash**
- `RequestsScreen.tsx:209`: the lede says "members", and its third clause is missing. Fix: "What people have asked for… A request stays open until you decide." polish
- `RequestsScreen.tsx:250`: Who isn't a `SortHead`. polish
- `RequestsScreen.tsx:322`: `text-ink/80`. polish
- `RequestsScreen.tsx:134,623`: the reason is curly-quoted in one list and bare in another. The screenshot's `“$$DROP TABLE"$$”` is escaped safely (React text). Fix: one form, unquoted and muted. polish
- `RequestsScreen.tsx:436-444`: "Review approving these". Fix: "Approve these…". `:792`: `lede=""`. polish
- `AskForAccess.tsx:184,177,174,27`: "makerspace staff decide" is said 3 or 4 times. Fix: keep it in the lede and the outcome. **clash**
- `AskForAccess.tsx:302`: "Send request" under the title "Ask for access". Fix: "Ask for access". `:214,346`: `text-ink` on sentences. polish
- `Holding.tsx:99`: a warn dot for "Already has this". Fix: neutral. polish

### 2.7 Projects
- `app/projects/page.tsx:47,187`: "Apps served" chips are unsorted. **clash**
- `app/projects/page.tsx:90`: the lede is three sentences with an em dash. `:178`: "No roles yet — nothing here can be granted", while the Roles column already says 0. Fix: "No roles yet." polish
- `app/projects/[id]/page.tsx:93`: eyebrow "Projects" (G6). **clash**
- `app/projects/[id]/page.tsx:108-141`: `meta` carries "serves syndra →", "2 unexplained", the raw project id and the whole ReadFreshness, wedged between meta and lede (screenshot).
  - meta becomes "4 people · 2 roles".
  - "Read by: syndra →" becomes the lede's second line.
  - The read line moves under the header (G5); the id follows Q6.
  - **clash**
- `app/projects/[id]/page.tsx:198-253`: the role name doesn't link; the violet count "3 →" is the way in. Fix: the name links and the count is plain. **clash**
- `app/projects/[id]/page.tsx:119` (served apps) and `:187` (roles): unsorted. **clash**
- `app/projects/[id]/page.tsx:152,181`: "New role" and "Create a role" show together when the project is empty. Fix: G9. `:143`: New role shows on not-found. polish
- `app/projects/[id]/page.tsx:261`: the "When given says…" caption repeats the WaitsMark. Fix: keep only "Set in Roles that wait →". polish
- `lib/holders.ts`: a "confirmed" line sits under people counts (screenshot "1 → confirmed"). Fix: say nothing when everything is confirmed (G17). polish
- `ProjectClaimDefault.tsx` (Default shape card):
  - This is the claims grammar A-16 retires. Rebuild it from A-16's claim list once ShapeCard lands (one pattern per job). Meanwhile:
  - `:201`: "Add a claim" is a bare text button. Fix: `Button outline sm`. **clash**
  - `:295-318`: Discard is outline md beside Save. Fix: `ghost`. **clash**
  - `:152`: Roles claim sits in a tinted block. **clash**
  - `:48-55`: hand-made header, and the apps list is unsorted. Fix: `CardHeader`; `sortNames`. **clash**
  - `:153-194`: three notes. `:169,225`: 46px fields (A-16 uses the sm height). `:184`: "spaced". Fix: "Space-separated". polish
- `app/projects/[id]/roles/[key]/page.tsx`:
  - `:70`: the eyebrow shows the raw id while loading. Fix: `ProjectName`. **clash**
  - `:108-123`: "Direct 0 · Bundle 0" and "0 people hold this role" show while loading. **clash**
  - `:130-146`: "Holds could not be read — …" is hand-made amber text. Fix: Notice. **clash**
  - `:293`: "Open the rule" is a bare text button. Fix: `Button outline sm`. **clash**
  - `:172,191`: members are unsorted. Fix: `sortAccounts`. **clash**
  - `:75`: the role key is a hand-bordered pill in meta. `:188`: a 6-word heading. `:116-124`: hand-made headers. `:100`: no search. polish

### 2.8 Roles (`app/roles/page.tsx`, `components/roles/*`)
- Project column (screenshot): "Audio-Dash" repeats row after row. RULES: a column of repeated values gathers into runs under a quiet label with its count. Fix: `runStarts` and `RunLabel` by project, as Incoming events does. **clash**
- `:205-210` and `:249,255`: the "made in Zitadel isn't listed" idea appears three times. Fix: the caption only. **clash**
- `:334-337`: "…which is why the project column is first and never collapses" is rationale. Fix: delete it. **clash**
- `:117`: em dash in the lede. `:256`: duplicate action. `:293` vs `:308`: "Used by nothing" vs "Unused". Fix: "Unused". polish
- `CreateRoleDialog.tsx:89`: project options are unsorted. **clash**
- `CreateRoleDialog.tsx:96-189`: six hints, and "goes straight to Zitadel" three times. Fix: the key hint plus one footer note. `:79`: a fake input box. `:161`: "Nothing — start empty". polish

### 2.9 Apps
- `app/applications/page.tsx:71-78`: "X doesn't get its claims." is a hand-made amber paragraph. Fix: `Notice tone="warn"`. **clash**
- Index rows (screenshot): every row carries "OIDC · Custom, changed" plus a two-line Claims cell, "groups / and 1 more · access token, userinfo". Fix:
  - one quiet line, "groups and 1 more";
  - "OIDC" only when the app isn't OIDC;
  - where claims land only when it differs from the usual.
  - This is calm by default. **clash**
- `:55`: a three-sentence lede. `:98-102`: "applications". Fix: "apps". `:185`: hand-made dot. polish
- App page (`AppTokenScreen.tsx`, claims card excluded):
  - `:158`: eyebrow "Apps" (G6). **clash**
  - `:171`: "client 3665…" sits in meta. Fix: a CopyableValue "Client ID" row in the tools card (Q6). **clash**
  - `:155`: a `gap-5 pb-2` wrapper (G5). **clash**
  - `:163`: the meta ArrowLink is forced to 14px. `:224` and `AppSignInStrip.tsx:30`: `px-[22px]`. Fix: `px-5`. `:186,209`: sticky while stacked. Fix: `desktop:sticky`. `:53`: loading has no header seat. polish
- `ClaimPreview.tsx:135`: "Hide the checks" is a lone ghost. Fix: `outline sm` (A-16 §4 table). **clash**
- `ClaimPreview.tsx:84-106`: the person picker is a hand-made pill under 44px. **clash**
- `ClaimPreview.tsx:81`: hand-made header. `:149`: the foot line repeats the verdict. polish
- `ClaimHistory.tsx:96`: "Show all N" is a lone ghost. Fix: `outline sm`. **clash**
- `ClaimHistory.tsx:173-183`: the GoBack result is hand-made. Fix: `ActionOutcome word="Restored"`. **clash**
- `ClaimHistory.tsx:132`: hand-made dot. `:53`: "Actions v1" has no Term. `:108`: a static "↓". `:200`: the confirm is disabled with no reason. polish
- `DebugMode.tsx:435-447`: hand-made result. Fix: `ActionOutcome`. **clash**
- `DebugMode.tsx:406`: matches are unsorted before `slice`. **clash**
- `DebugMode.tsx`: "15 minutes" appears three times (`:86,431,504`); "Rebuilt" three times (`:365-372`). `:204`: hand-made empty. `:216`: a static arrow. polish
- Tools card: three hand-made 170px label seats (`AppSignInStrip`, `ClaimHistory:71`, `DebugMode:83`). Fix: one settings-row part. polish

### 2.10 Bundles (`app/bundles/page.tsx`, `components/bundles/*`)
- `bundles/page.tsx:418`: a tinted "Copied X…" box repeats the hint at `:369`. Fix: one faint caption. **clash**
- `bundles/page.tsx:429`: "Start empty" is a bare text button. Fix: `Button outline sm`. **clash**
- `bundles/page.tsx:70`: a static "Bundle ↑". `:54,86`: "New bundle" vs "Create the first one". `:187`: rationale in a reason. polish
- `RetiredBundle.tsx:193` and `RetiredRules.tsx:48`: unsorted. **clash**
- `RetiredBundle.tsx:107,139-146`: the People tab is always 0, and "everyone lost it" is said four times. Fix: delete the tab. **clash**
- `RetiredBundle.tsx:200`: status drawn two ways. `:152` and `BundlePage.tsx:302`: `TabLabel` is defined twice. polish
- `BundlePage.tsx:185`: with no description there is no lede. Fix: a fallback lede. **clash**
- `BundlePage.tsx:331`: the unpublished panel is a violet tinted box. Fix: `Card` with `CardHeader`. **clash**
- `BundlePage.tsx:336-352`: Discard is outline md beside Publish md. Fix: Discard `ghost sm`, Publish `accent sm` (a card seat). **clash**
- "Nobody's access changes until you publish" appears five times (`:377,449`, AddRolesToBundle `:127-167`, `:681`). Fix: keep `:449` (G12). **clash**
- `BundlePage.tsx:501-505`: a hand-made count in the title. Fix: `CardHeader count`. **clash**
- `BundlePage.tsx:480,597,1142`: includes, included-in and an included bundle's roles are unsorted. **clash**
- `BundlePage.tsx:602-648`: "Given to everyone who joins" and "Retire bundle" are md in card rows. Fix: `sm` (G16). **clash**
- `BundlePage.tsx:951`: the RemovalImpact `warn-soft` box. Fix: an Unfold with a hairline. **clash**
- `BundlePage.tsx:1017-1040`: "Drop", "working copy", "draft" and "next version" are four words for two things. Fix: "Remove", "next version". **clash**
- `BundlePage.tsx:178`: hand-joined meta. Fix: `MetaRow`. `:956`, `:982`: alpha text and ink dots. `:1100/1169`: two links to one place. `:606`: "One bundle at a time." ×3. polish
- `BundleVersions.tsx:70-89`: "who is behind" is said five ways. Fix: the note plus the button. **clash**
- `BundleVersions.tsx:383`, `:437`: a dashed card, and a dashed initials tile (G19). **clash**
- `BundleVersions.tsx:121`: `published_by` is unlinked. `:191`: ReadFreshness on one tab only. `:450`: a "Bundle · " prefix. `:281`: no column heads. polish
- `RolePicker.tsx:205-240` and `RolePickerOne.tsx:134-155`: two hand-made chosen-chip shapes. Fix: one Chip (G15). **clash**
- `RolePicker.tsx:204,223`: chosen chips are in click order. Fix: A to Z. **clash**
- `RolePickerOne.tsx:254-259`: "Extend →" is a hand-made link to `?within=30`. Fix: `ArrowLink`, at the default window (14). **clash**
- `PublishVersionDialog.tsx:263-287`: hand-made radio cards. Fix: ChoiceRow. **clash**
- `PublishVersionDialog.tsx:108,111`: "Both answers are real, so there is no default." Fix: "Choose what happens to the 12 people." **clash**
- `PublishVersionDialog.tsx:189`: a dashed box. **clash**
- `PublishVersionDialog.tsx:130-153`: a third copy of the change lines. Fix: `IncludeChangeLine`. `AddRolesToBundle.tsx:208`: Close shows before anything is added. polish

### 2.11 Automatic rules (`app/policies/page.tsx`)
- `:80,113`, `:337,640,702,712`: the rule's id "R-0a3f" is a column and the dialogs' name. Fix: drop the column; "Edit this rule", "Retire this rule?", "Rule retired" (G10). **clash**
- `:349-429`: four native Selects for project and role. Fix: `RolePickerOne`. **clash**
- Empty state (screenshot): column headings sit over "No automatic rules.", plus a violet "New rule" that duplicates the header's (G9). **clash**
- `:80`: 130px "Given by this rule" wraps. Fix: 110, "People". **clash**
- `:86-102`: no ReadFreshness, unlike Bundles. **clash**
- `:344,392`: `grid-cols-2` with no phone stack. Fix: `tablet:grid-cols-2`. **clash**
- `:498-506`: the disabled-Save reason repeats the footer note, three times in all. **clash**
- `:725`: "Retire rule and revoke access" even with 0 people. Fix: the bundle's logic. **clash**
- `:152-155`: a trailing paragraph ("…Check lists every rule…") with a stray capital. Fix: delete it. `:71`: the lede lacks when you'd come. `:495-562`: footer order. `:685`: "Keep the rule". Fix: "Cancel". `:135`: the count's weight differs from Bundles. polish

### 2.12 Insights (`components/insights/*`)
- `MapTab.tsx:332,339`: "Back to now" and "Earlier date" are lone ghosts. **clash**
- `AnswerPanel.tsx:125-158`: four buttons are ghost on desktop and outline on a phone. Fix: `outline sm` everywhere. **clash**
- `MapTab.tsx:348-357`: the finding chips at the right ("47 drift", "1 single route"…) are hand-made pills and dots. Fix: the read line's `Chip` with `StatusDot`. **clash**
- `TrendsTab.tsx:174-199`: a hand-made table with 72px number columns. Fix: 110. **clash**
- `TrendsTab.tsx:86-91`: Period is Segmented while its siblings are FilterPills. **clash**
- "THE MAKERSPACE AT A GLANCE" (uppercase eyebrow) plus "Two things worth knowing" above the answer (screenshot). Fix: delete both. `AnswerPanel.tsx:30`: the answer is tinted accent. Fix: neutral. polish
- `QuestionSeat.tsx:97`: "a door". Fix: "a project". `:66`: lone ghost ×. polish
- `insights/page.tsx:24`: the tab "Map" and the view "Map | List" share a word. `LaneMap.tsx:237,267`: hand-made pills. `Timeline.tsx:39`: a 42px track. `TrendsTab.tsx:188,220`: unlinked names. `:127-158`: some cards link and some don't. `RestCards` and `Quiet`: hand-made empties. polish

### 2.13 Pending changes (`app/governance/pending/page.tsx`)
- `:103`: the lede is three sentences, and "in order" and "revocations send on their own" repeat at `:150`, `:274` and `:455`. Fix: two sentences; delete `:150`. **clash**
- `:137-148`: a hand-made text read line. Fix: `ReadFreshness`. **clash**
- `:273,526,535`: raw `c_` and rule ids. Fix: `ArrowLink` "What this set off" (G10). **clash**
- `:134`: the outcome has no word, so it reads "Applied". Fix: "Sent". **clash**
- Empty state (screenshot): column headings sit over "Nothing is waiting…", and a disabled accent "Send 0 changes" (Q2). **clash**
- `:357-368`: hand-made EverythingSent. Fix: `EmptyState resolved`. `:283`: a truncated, unlinked name. `:296`: Mono keys. Fix: `RoleRef`. `:513`: raw keys joined by "→". polish

### 2.14 Change history (`app/operations/cascades/page.tsx`)
- `:51`: "Show every edit" is a hand-made violet link with no arrow or 44px. Fix: `ArrowLink`. **clash**
- `:58-73`: a handle legend plus notes that repeat the lede. Fix: delete `:58-59`. **clash**
- `:123`: a raw `c_` id in accent is the card title. Fix: the title names what was edited. **clash**
- `:151-155`: "from rule R_xxxx". Fix: the rule's name. **clash**
- `:129`: a Badge for state where siblings use dot and word. Fix: StatusSeat. `:140`: Mono keys. `:208`: "Pending changes" as plain text. Fix: ArrowLink. `:220`: "read back". polish

### 2.15 Roles that wait (`app/changes/roles-that-wait/page.tsx`)
- `:68`: eyebrow "Changes" (G6). The title "Roles that wait for you" differs from the rail's "Roles that wait". Fix: match the rail. **clash**
- `:177`: a static "Role ↑", and the rows are unsorted. Fix: sort plus `SortHead`. **clash**
- `:206`: a hand-made run label. Fix: `RunLabel`. **clash**
- `:332-333`: a 9-word card title of rationale. Fix: "Roles to keep waiting". **clash**
- `:353-386`: hand-made choice rows, unsorted. Fix: ChoiceRow. **clash**
- `:503-533`: the dialog closes with no result in the sheet. Fix: `ActionOutcome word="Marked"` plus Close. **clash**
- `:73/:200`: "Add roles" twice when empty (screenshot). `:70`: a "For the whole makerspace." fragment. `:264`: two outcome patterns. `:335`: "Until today". `:413`: a filler footer note. `:226`: unlinked names. polish

### 2.16 Incoming events (`app/operations/page.tsx`)
- `:349-356`: "Show details" is a bare text button. Fix: the whole row opens (CardRow onToggle, G3). **clash**
- `:414`: Person links to `/users` even for a service account. Fix: `useAccountHref`. **clash**
- `:334`: the raw `zitadel.grant_added` type on every row. Fix: move it into details. `:313`: hand-made dot and word. `:131/:220`: a Notice above and an error below the header (G7). `:177`: filter options order. `:94`: `localeCompare`. `:246`: empty guidance is a count. `:470,521`: raw `error_message`. polish

### 2.17 Audit (`app/audit/page.tsx`, `components/audit/*`)
- `:113-136`: the window Select, the search and Export sit top-right beside the title, the only list without a FilterBar (screenshot). Fix: `FilterBar` (search, then Select), Export as `trailing`, and a read line. **clash**
- `:108-112`: meta is a sentence and shows "0 loaded" while loading. Fix: delete it. **clash**
- `:150-153`: the handle legend "Follow-on: a c_ handle…". Fix: delete it. **clash**
- `:160` and `TraceCell.tsx:28`: the Follow-on column of `c_` ids. Fix: drop it, or `ArrowLink` "What it set off". **clash**
- `:157-160` vs `:197-200`: heading widths (110) don't match the cells (124), so columns misalign. **clash**
- `:200`: WHO truncates "onboarding (automat…" at 150px. Fix: wrap; `machineName` → "Onboarding", with "Automatic" faint. **clash**
- `:132`: "Export every matching entry (CSV)". Fix: "Export CSV". `:123`: "Everything loaded". Fix: "All time". `:174`: empty copy. `EventLine.tsx:40,45,264`: handles and dashes in sentences. polish

### 2.18 Drift (`components/review/UnexplainedAccess.tsx`)
- `:1426-1428`: "Created in Zitadel on 15 Sept by 365914614526903302". `upstream_actor` is a raw id (screenshot). Fix: resolve it with `names.user`; if it is unknown, drop "by" (G10). **clash**
- `:1428-1430`: "Nothing in Syndra — no direct access, no bundle, no automatic rule — gives it." is on every row. Fix: once, as the section's note; the row says only when and who. **clash**
- `:120,1039-1048,1312`: `QUIET_LINK` text buttons "Show evidence", "Select similar rows" and "Who made it?". Fix: outline sm; "Select similar rows" moves into SelectionBar. **clash**
- `:1022`: Adopt turns ghost when it would do nothing. Fix: accent disabled with its reason. **clash**
- `:904-910`: a repeated line on compared rows, plus a hand-made muted link. Fix: delete the line; `ArrowLink`. **clash**
- `:1102,1183-1200`: "queues the removal" is said four times. Fix: the lede plus the outcome. **clash**
- `:1223`: Cancel is ghost with `order-2` (G4). **clash**
- `:1440`: "Member" under each name. Fix: "Person". `:1471`: "Unexplained access" as a recorded reason. Fix: "Drift". `:899,994`: Mono key on every row. `:229`: sorted by raw key. `:1120`: truncated name. `:1235`: outcome form. `:512`: meta not in `MetaRow`. `:495`: hand-made all-clear. polish

### 2.19 Unfinished revocations (`components/review/WithdrawnAccess.tsx`)
- `:89,96,499`: card hints repeat the lede and every row ("Tries again by itself"). Fix: drop the hints. **clash**
- `:96`: the queued chip has no tone, so no dot. **clash**
- `:427-441`: the who/what seat differs from Drift and Expiring (15px, "Project / Role"). Fix: one seat. **clash**
- Empty (screenshot): no read line, unlike Expiring's. Fix: always render ReadFreshness. **clash**
- `:270`: "Nothing outstanding" has no full stop. Fix: "Nothing is unfinished." `:230`: subject "Syndra's send queue". Fix: "This list". `:454`: a raw id link. `:464`: raw `last_error` in red on every row. `:187`: a three-sentence lede. polish

### 2.20 Expiring access (`app/review/expiring-access/page.tsx`)
- `:354`: the empty state says "appears here a month before it ends" (screenshot), but the window is 14 days. Fix: "Direct access shows here once it ends within 14 days." **clash**
- `:670-716`: Extend is accent only on the soonest row (the variant follows the data), and "Let it lapse" is ghost beside it. Fix: both `outline sm` on every row. **clash**
- `:262`: a four-sentence lede, and hints `:96,102` repeat it. **clash**
- `:749/:829`: the title is "Let this lapse?" and the confirm is "Record the decision". Fix: "Let it lapse" in both. **clash**
- `:760-773`: an `accent-note` box with bullets. Fix: one sentence in the lede (G19). **clash**
- `:814-819`: "Recorded — it will end on its date". Fix: word "Recorded", sentence, faint line. **clash**
- `:99`: the lapse chip has no dot. **clash**
- `:589`: three warn signals on one row. `:783`: rationale. `:315`: "filters" where only search exists. `:854`: `localeCompare`. `:644`: dash. polish

### 2.21 Holds due (`app/review/holds/page.tsx`, `HoldsDueForReview.tsx`, `HoldDialog.tsx`)
- `holds/page.tsx:38-48`: a four-sentence lede that says "place a new one" (FINAL: Put on hold). Fix: two sentences. **clash**
- `HoldDialog.tsx:83,189,200`: "Hold X" and "Hold placed on…". Fix: "Put X on hold", word "On hold". **clash**
- `HoldDialog.tsx:91-121`: hand-made radio cards. Fix: ChoiceRow. **clash**
- `HoldDialog.tsx:159`: confirm first, ghost Cancel last (G4). **clash**
- `HoldDialog.tsx:137`: rationale. `:143`: "Why — shown to…". Fix: "Why (shown to Priya)". `:102`: "Review › Holds due" as a path. `HoldsDueForReview.tsx:50`: no Refresh. `:131`: no column heads, and two near-duplicate rows. `:127`: a nowrap ArrowLink overflows on a phone. polish

### 2.22 Zitadel (`app/zitadel/*`, `components/upstream/*`, `components/system/ClaimsDelivery.tsx`)
- `zitadel/page.tsx:63-69`: the signing-key Notice sits above the title (screenshot), and the key's age is said four times. Fix: the Notice under the header, once (G7). **clash**
- `zitadel/page.tsx:556-569`:
  - `ButtonLink variant="danger"` "… →", with a disabled Button twin, for doors that only navigate. Fix: `ArrowLink`, or faint text when unreachable.
  - `:556`: a hand-made `<a>` "→". Fix: `ArrowLink`.
  - **clash**
- `zitadel/page.tsx:374-405`: "Show checks ›" is a bare toggle, and the fold repeats the line above it. Fix: delete the fold. **clash**
- `zitadel/page.tsx:198`: a lone accent. Fix: outline. **clash**
- `zitadel/page.tsx:455`: "These pages only read" is false. Fix: delete it. **clash**
- `zitadel/page.tsx:539-547`: a `danger-note` box repeats the rows. Fix: delete it. **clash**
- `zitadel/page.tsx:133-137`: the mythology card (Q4). `:189,209,314`: rationale and dashes. `:429-523`: two links per destination. `:182,361`: hand-made dot and word. polish
- `ClaimsDelivery.tsx:235-243`: the Fold is a lone ghost, one of four fold grammars on the page; "Hide each app" is in the screenshot. Fix: G3. **clash**
- `ClaimsDelivery.tsx:553`: `External` has no 44px. Fix: `ARROW_LINK`. **clash**
- `ClaimsDelivery.tsx:310,133`: apps are unsorted. **clash**
- `ClaimsDelivery.tsx:64`: "Read 13:07" as a note. Fix: ReadFreshness. The uppercase "LIVE" label. `:437`: "R1 ·" ids. `:307`: "Answers, 24h". `:87,590`: sentences in faint. polish
- `zitadel/users/page.tsx:237-322`: dialogs close and report in the row. Fix: the result in the sheet (G4). **clash**
- `zitadel/users/page.tsx:282,420,499` and `zitadel/projects/page.tsx:224,339`: no `word`, so they read "Applied". **clash**
- `zitadel/users/page.tsx:484`: EditRoles' confirm is outline danger. Fix: `dangerConfirm`. **clash**
- `zitadel/users/page.tsx:74,161`, `projects/page.tsx:53`, `grants/page.tsx:90`: a count of 0 while loading. **clash**
- `zitadel/users/page.tsx:72`: a hand-made search. Fix: FilterBar. `:104`: a 100-row cut. Fix: PaginationRow. **clash**
- `zitadel/users/page.tsx:193`, `projects/page.tsx:163,166`, `grants/page.tsx:123-135`: role names and keys are truncated. **clash**
- `zitadel/projects/page.tsx:52`: a fixed 280px on a phone. **clash**
- `zitadel/grants/page.tsx:59`: the read line is above the FilterBar. `:42`: search ignores names. `:122`: fixed widths on a phone. **clash**
- `UpstreamShell.tsx:35-64`:
  - A hand-rolled header. Fix: PageHeader with an eyebrow link to Zitadel.
  - `:51`: a permanent amber "Syndra keeps no record", which is false for grants. Fix: delete it.
  - `:57`: a hand-made arrow link.
  - **clash**
- `users:166`, `projects:96`: lone accent sm. `users:296`: "Keep the roles". Fix: "Cancel". Dashes. polish

### 2.23 Connected systems and Service accounts
- `system/targets/page.tsx:219`: the raw "truenas" and "Zitadel project TrueNAS" under the name (screenshot). Fix: delete the id. `:198`: a hand-made row. `:305`: a hand-made dot. `:46`: no read line. `:88`: jargon in empty guidance. polish
- `system/service-accounts/page.tsx:37`: eyebrow "System" (G6). **clash**
- `system/service-accounts/page.tsx:46`: the card title "Service accounts 3" repeats the page title (screenshot). Fix: no CardHeader; meta "3 service accounts", as People does. **clash**
- `system/service-accounts/page.tsx:42`: the read line sits 4px from the card. **clash**
- Read line "Service accounts was read just now". Fix: "This list was read just now" (one subject product-wide). `:86`: a blank purpose. Fix: faint "Nothing written yet". polish
- `ServiceAccountPage.tsx:101`: the lede is the purpose, repeated in the Purpose row. Fix: a fixed lede. **clash**
- `ServiceAccountPage.tsx:194`: projects are unsorted. **clash**
- `ServiceAccountPage.tsx:100`: meta repeats the Kind row. `ServiceAccountOwner.tsx:19`: hand-made dot. polish

### 2.24 TrueNAS (`components/targets/*`)
- `TargetOverview.tsx:99-106` and `MappingScreen.tsx:53`: "← Connected systems" is a hand-made back link, the only one in the product (the breadcrumb has it). Fix: delete it. **clash**
- `TargetOverview.tsx:98`: `gap-5` (G5). `:124`: the raw id in meta. **clash**
- `TargetOverview.tsx:891-942`: hand-made RegionChips. Fix: `ReadLine` plus `SectionPill`; this removes about 50 lines. **clash**
- `TargetOverview.tsx:171`: "Two machines, kept apart…" is rationale under "Whether changes can land". Fix: delete it. **clash**
- `TargetOverview.tsx:198`, `:1044-1094`, `:1119-1147`, `:1979-2001`: a red `<p>`, two red tinted finding boxes (with a bare "Technical detail" button) and an amber failure box. Fix: `Notice`, with Technical detail as an Unfold. **clash**
- `TargetOverview.tsx:1195`, `:1672`: hand-made radios. Fix: ChoiceRow. **clash**
- `TargetOverview.tsx:1771-1809`: hand-made AdoptionOutcome. Fix: ActionOutcome. **clash**
- `TargetOverview.tsx:1420-1461`: MappingCensus repeats the setup strip. Fix: delete it. **clash**
- `TargetOverview.tsx:689,788,914`: the read age is said three times. **clash**
- `TargetOverview.tsx:1961-1964`: "takes away", plus a repeat. Fix: delete it. **clash**
- `TargetOverview.tsx:983`: row padding is 11, 13, 14 and 12 side by side. Fix: CardRow. **clash**
- `TargetOverview.tsx:1869`: "the NAS". `:982`, `:997`, `:1386`, `:699`: sorting, a dotless status, a Reading row that duplicates StatusSeat, and an overloaded "reach". About 25 dashes. polish
- `MappingManagement.tsx:110` and `AddMappingDialog.tsx:434`: Add is accent beside the accent Publish. Fix: Add outline sm. **clash**
- `ConvergeEntitlements.tsx:82,255`: "Reconcile accounts". Fix: "Check for differences". **clash**
- `MappingManagement.tsx:503-509`: repeats VersionBand. `:590`: raw `role_key → value`. **clash**
- `VersionBand.tsx:123-138`: "Nothing to publish" ×3, and the note requirement ×3. **clash**
- `AddMappingDialog.tsx:157`: roles are unsorted. `:178` and `ProjectSetup.tsx:308`: panel boxes. `:417`: the RefusedFields box. **clash**
- `DormantAccounts.tsx:196,224`:
  - a solid `dangerConfirm` outside a dialog, plus a ghost Clear. Fix: `danger sm` plus `outline sm`, or a dialog.
  - `:141`: hand-made rows with a dashed fake checkbox.
  - `:150`: a per-row repeat.
  - `:113`: unsorted.
  - **clash**
- `MergeFindings.tsx:77`: an unlinked name. `:55`: "reconciliation". `:301`: Cancel md, last. `:405`: a violet box. **clash**
- `UnconfirmedChanges.tsx:168`: hand-made status. `:161`: a hard `/users` link. **clash**
- `HomeLimit.tsx:169` and `ProjectSetup.tsx:356`: dialogs close with no result. `ProjectSetup.tsx:64`: a tinted step box with a one-off StepBar. `:92`: projects unsorted. **clash**
- `TakeAwayDialog.tsx:68`: a hand-made result. `:108-178`: consequences said three times. `:187`: "withdrawal". **clash**
- `TargetReading.tsx:128`: an 8px dot. **clash**
- Also polish:
  - `HomeLimit.tsx:237`: "End it" should be danger.
  - `PeopleOnTarget.tsx:78`: the gloss is repeated.
  - `Region.tsx:40`: `gap-3`; use 3.5.
  - `MergeFindings`: placeholder instruction.
  - `takeaway` identifiers.

### 2.25 My access (member: `components/member/*`)
- `MemberAccess.tsx:245` (and `PersonAccess.tsx:712`): "You can use this. Nobody wrote down who gave it to you." is under every role (screenshot). Fix: say nothing when the source is unknown (G17). **clash**
- `MemberAccess.tsx:443`: "Nine roles across six rooms". A project is never a "room" elsewhere (Q3). **clash**
- `RecentChanges.tsx:144-149`: an approval still on its way shows as a warn "isn't on your account yet". Fix: "On its way", neutral, as RoleRow says. **clash**
- `RecentChanges.tsx:59` and `MyStorage.tsx:276,279,334,335,349`: bare "staff". Fix: "makerspace staff". **clash**
- `RecentChanges.tsx:106`: `“note” — Name`. `MemberCatalog.tsx:95,114` and `MemberAccess.tsx:195`: "makerspace staff decide" ×3. `MemberCatalog.tsx:105`: "a machine, a room, a door". `:111`: a hand-made no-match. `MemberAccess.tsx:142,213`: gaps of 22 and 18. `:316`: warn row tint. `:214`: hand-made card heads. polish

### 2.26 Network storage (member: `components/storage/*`)
- `MyStorage.tsx:54-58`: a fragment with no gap, so the TrueNAS card touches the lede (screenshot) (G5). **clash**
- `MyStorage.tsx:397-445`: the three steps are tiny faint text at the card's top-right (screenshot). Fix: one muted 13.5px line under the card title, with StatusDots (ring = not yet). **clash**
- `MyStorage.tsx:501` vs `:548`: "First time? How to connect" is ghost on desktop and outline on a phone. Fix: `outline sm` for both. **clash**
- `MyStorage.tsx:383,426,515`: hand-made dots. **clash**
- `MyStorage.tsx:777-805`: three outcome styles. `:163,359,382,497`: offline said four times. `:185`: a tinted password box. `:514`: a dashed "None yet". `:620`: a card inside a card. `:532`: the home folder pinned first (RULES: nothing is pinned). `:64`: "could not be read". Fix: "Couldn't read…". `ConnectionGuide.tsx:20`: device order. polish

### 2.27 Settings (`components/settings/*`)
- `TermDatesCard.tsx:259-271`: "Add a term" is a lone ghost. **clash**
- `WelcomeClaimsCard.tsx:51`: "Roles that wait for approval". Fix: "Roles that wait for you". **clash**
- `TermDatesCard.tsx:49-52`: the "Trends, 'new this term'…" paragraph repeats the lede. `WelcomeClaimsCard.tsx:45,63`: title, label and lede triple. Two copies of the "Set by" footer. polish

### 2.28 Login
- `LoginDoor.tsx:200`: "Syn — the goddess who keeps the door." (Q4). polish

---

## 3 · Questions for the owner

1. **Dialog footers: Cancel first or last?**
   - Today, 17 dialogs put the act first and 12 put Cancel first.
   - Cancel is outline in most, and ghost in 6.
   - **Recommended:** Cancel first, as the ghost half of the pair (your 4 Nov ruling), with the act last. This is what FINAL already says for Lapse, and Keep it is first for the same reason: the focus trap lands on the safe choice.
   - About 17 footers change order.
2. **Pending changes with nothing waiting:**
   - (a) keep the seat as a disabled "Send changes" with no count;
   - (b) hide the button;
   - (c) leave "Send 0 changes" (today).
   - **Recommended:** (a). Structure doesn't move with data, and the empty state already says why.
3. **The member's count line, "Nine roles across six rooms."** A project is never a room anywhere else, and the cards already show the count.
   - **Recommended:** delete the line ("Hi Kanishk. Here's what you can use." is the answer).
   - The catalog placeholder becomes "Search what's on offer".
4. **The mythology lines** ("Syn keeps the door of Frigg's hall" card on Zitadel; "Syn — the goddess who keeps the door." on login).
   - **Recommended:** keep the login line, written without the dash ("Syn, the goddess who keeps the door."). Delete the Zitadel card, because it answers nothing an operator came for.
5. **Header→content: 28 or 18?** RULES says 28 (9b). 16 pages render 18, and only three render 28.
   - **Recommended:** 28, owned by PageHeader, so no page sets it again. It is the calmer page, and it was the board you chose.
6. **Ids in Advanced meta** (person's Zitadel id, project id, app client id).
   - **Recommended:**
     - The app's client id moves to a "Client ID" CopyableValue row in the app's tools card, since developers need it.
     - The project's and person's ids stay in meta, Advanced only, as CopyableValue.
     - Grant ids leave headings entirely; lineage keeps them.
7. **"Grant direct access" (person page button and dialog) vs "Given" (its result) and "Give a bundle".**
   - **Recommended:** "Give a role". One verb family, matching Give a bundle and the result word.
