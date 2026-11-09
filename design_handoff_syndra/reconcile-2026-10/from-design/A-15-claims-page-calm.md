# A-15 · The claims page, calm by default

Answer to `Q-15-claims-page-calm.md`, 3 Nov 2026. Board: `a15-claims-page.dc.html` (Final, 3 Nov 2026). It revises A-14 §1's layout only. Every A-14 behaviour, the API and the owner's rulings stand.

Code's reading of the 90% jobs is right, with one change. Job 1 is two questions: what this app gets, and whether it's working. The first screen answers both. Everything on the 10% list is one deliberate step away: a fold, a button or a sheet.

The test I used on every sentence: would someone miss it if it were gone? Eight sentences failed that test and are cut (§1).

## 1 · The page, top to bottom

**The page opens on the answer. The header has no lede. Below it are two halves: what the app is told, which is the summary and then the roles, and the preview, which is the verdict and then the token. Under those is one quiet card of tools. In Advanced, the only extra is one folded line, "Claim rows". At 1440, the first screen holds the summary, all seven roles, the verdict and the token.**

**Order (both views):**
1. **PageHeader.** Eyebrow "Apps", the title, the meta line, and "Look up sign-ins" (outline). **No lede:** the summary under it does that job better.
   - Meta: "OIDC · roles from [[Network →]] · 23 people in 7 days · last sign-in 19:02". In Advanced, the client id follows in Mono (Tinted).
2. **What NetBird is told** | **Preview**, side by side as now (`1 1 460px` | `1 1 440px`, gap 20). The preview stays sticky at 20 from the top.
3. **One card of tools:** Sign-ins, Debug mode, History (§6).

**"What NetBird is told", top to bottom:**
- The card title.
- **The summary** (§2). It's always there and has no heading.
- **Roles NetBird sees**: the type-label, with "7 of 7 included" at its right. Then RoleWordsTable (§3).
- *Advanced only:*
  - **Roles added later**: the settings row, as A-14 has it.
  - **Claim rows**: one folded line (§4).
- **Save**, which unfolds with your first edit (§3).

**Basic never shows** a claim row, Add a claim, the Lands in control, Roles added later or the client id. What is sent still shows in both views, as the summary and the token. This replaces A-14 §1's "a saved row always shows in both views". The rule behind it holds: nothing sent is ever hidden. Only the forms are hidden.

**Cut from the page:**
- the header lede;
- the roles lede ("Which of Network's roles…"), because the column headings say it;
- the Custom template's caption ("Every role, in Syndra's words."), because it repeats the summary (the other templates' captions stay: each says something the summary can't);
- the preview's lede;
- "Nothing to save yet.";
- "A line marked − leaves the token; a line marked + joins it." The marks speak for themselves, and the TokenBlock's sr-only words carry them for screen readers;
- "Sent in NetBird's access token and in userinfo" under the token, because the summary says it;
- the Sign-ins card's lede ("Nobody is named"), because the row names nobody.

**Loading, error, not found, degraded:** as A-14 §1. Loading draws the summary's seat as two Skeleton lines and shows no counts.

## 2 · The summary

**Both: a compact list in words, and the token beside it as literals. There is one summary line per top-level claim, in the token's order, so the two read across: the left says what `groups` is for everyone, and the right says what it is for Charlotte.**

- **Each line:** the key in Syndra.Mono 14 in a 150px seat, then what it carries at 14.5 ink, gap 16. When the words don't fit beside the key (a phone), they wrap under it.
  - Roles: "All 7 of Network's roles, in Syndra's words". When some are renamed, "1 in NetBird's words", and when all are, "in NetBird's words". When some are left out, "4 of Network's 7 roles, …".
  - An object: "An object holding `groups`, `given_name`, `family_name` and `name`".
  - Attribute: the attribute's label ("Given name"). Fixed value: "The same for everyone: `"makerspace"`". If-role: "`true` if they hold Admins, else `false`". Bundles: "Bundles they hold". Role ends: "Each role's end date". Service account: "Is a service account".
  - A roles claim sent as text keeps its warning here, in both views, in warn-text under its line, so it can't hide behind the fold: "As text, with commas. No app Syndra has a template for reads roles as text."
  - None: one line, "Nothing of its own. Zitadel's standard claims do the work."
- **Then one faint line, 13.5:** where it lands and the template. "In the access token and userinfo. From the Custom template, 5 changes since."
  - Then the template buttons (ghost sm), as now: **See the template** and **Copy another app's shape**.
  - **New:** when the app's suggested template isn't the one it's on, the first button names the suggestion: "See the NetBird template". The seeded NetBird is on Custom, and its own template is one click away. No new sentence.
- **A failing check 1** adds danger text, 13, under the faint line: "Doesn't reach the access token. Check 1 in the preview says why." This replaces A-14's line on the Lands in row, which is now inside the fold.
- **With a draft,** the summary describes the draft (it describes the card). The save row and the preview say what changes.
- The summary is not a sentence that paraphrases the token. A paraphrase goes stale. These lines are the shape's own rows, said in words.

## 3 · The roles table

**RoleWordsTable is the first thing to edit and is always open. Its word fields are quiet at rest. They read as mono text; they show their line on hover and their ring on focus. One click edits. There is no Edit mode.**

- **At rest:**
  - Each row is about 50px: the name at 14.5/600 with WaitsMark, the key under it in faint mono, the word, and Included in the 90px seat.
  - The word field has a transparent border and background at rest. Syndra's word shows as the placeholder, at faint strength.
  - A role that's left out is muted and can't be edited, as A-14 has it.
- **While editing:**
  - An edited word keeps its border while it differs from what's saved, with "Was `staff`." under it in faint 12.5.
  - A role you untick that was sent reads "Was sent as `guest`."
  - The first edit unfolds **the save row**: accent Save, ghost "Discard the draft", and the before-line.
  - The before-line turns warn-text with an amber dot only for a breaking change: "1 change NetBird will act on. 12 people move from `staff` to a new group, `netbird-staff`, at their next sign-in."
  - The summary's roles line counts the rename, and the preview shows − and +. That is three places before the save summary, as A-14 §3 has it.
- **Discarding** folds the save row away (closing fades first) and puts back the quiet fields.
- Row padding goes from 10 to 8. Nothing else in the table changes.

## 4 · Claim rows, folded

**Advanced only, and closed on arrival. One line: a 28px round chevron in a 44px touch seat, "Claim rows" at 14.5/600, and a faint 13 note with the count and the keys. Open, it holds every ClaimRow form as built, Add a claim, and then Lands in. It is fully technical.**

- **Name: "Claim rows".** **Owner's call**, drawn as 4a and 4b. "Custom claims", your word, collides with the `custom_claims` key that the seeded shape and both Supabase templates send. The fold also holds the plain `groups` row. **Recommendation: 4a, "Claim rows".**
- **Closed note:** "5 rows · `groups` · `custom_claims` holding 4 · both places". The count, then the top-level keys in order, with a nested object's leaf count, then where they land, in short.
- **Open, in order:**
  1. Every ClaimRow, as built: Inside › name · Carries · Remove; line 2 where the kind needs it; "Sent as …" for Charlotte Flair; "Put it inside another claim" under a top-level row.
  2. Add a claim (ghost sm).
  3. **Lands in**: the settings row with its Segmented (Both | Access token | ID token and userinfo) and one faint line: "ID token and userinfo is one choice: the ID token carries it only while User Info inside ID Token is on."
- **Text formats** stay where A-14 put them: line 2 of a roles row, "Sent as" A list | Text, with commas | Text, with spaces, with the warning. That is inside the fold.
- **It opens itself** only when a row inside stops Save. That happens when a copied `is_admin` has no role, or the backend refuses a row (400 `details.row`). Otherwise it stays as you left it while you're on the page, and it is closed on every arrival.
- When built, "What NetBird reads" (tasks 7.2, 8.11) is the fold's last row, after Lands in.

## 5 · The preview and its checks

**The person picker is the card's title line: "Preview for [Charlotte Flair]". Under it is one verdict line. When everything passes, the checks are one step away. When a check fails, the trail opens by itself in its compact form. The token always sits under the trail, never inside check 5.**

- **Title line:**
  - "Preview" (type-card-title), "for" in faint 13.5, and the pill (Avatar row and Select), as now.
  - At the right end: "Checked 19:06" in faint 12.5, and Check again (sm).
- **Verdict line:** StatusDot, then the verdict at 14.5/600, then a ghost sm "Show the checks" or "Hide the checks".
  - All pass: "All five checks pass".
  - A draft: "All five checks pass on this draft".
  - One fails: "Check 1 fails" (danger dot). With more than one: "Checks 1 and 5 fail".
  - A warning only: "Check 4 needs a look" (amber dot).
  - The verdict crossfades in its seat when it changes (StatusSeat's way, without the bloom).
- **The trail:** CheckTrail inside Unfold.
  - Shown on demand: full (`compact={false}`), every faint line.
  - Opened by a failure or a warning: `compact`, so only the checks that didn't pass carry their detail and fix.
  - The words are A-14's. Check 5 becomes "Syndra answers with `groups` and `custom_claims`, below." On failure it reads "…, but only for userinfo."
- **The token:** TokenBlock, then one faint line at 13 and **Copy preview** (sm) in a nowrap right seat.
  - The faint line at rest: "Zitadel adds `sub`, `email`, `name` and `preferred_username` itself."
  - With a draft: "One difference for Charlotte Flair: `staff` becomes `netbird-staff`, in both claims."
  - On failure: "NetBird reads its access token, so it sees none of this, and empties every group it made from a token."
- **Who's in the picker:** you first, then everyone. When test people are built, they become the Select's first group, "Test people". There is no new section.

## 6 · Debug mode and History

**One card with three settings rows. Each is a label (170, 14/600), a value, and a right seat. Nothing happens without a named button. The card has no title; the labels are the titles.**

1. **Sign-ins.** This is AppSignInStrip as a row.
   - The 30 bars, 178 wide and at most 20 tall. A day with none is a 2px hairline.
   - Then "412 in 30 days, by 23 people" in muted 14.
   - It shows only when the app has had sign-ins, as now.
2. **Debug mode.**
   - Off: StatusSeat neutral, "Off", with the note "See one account's sign-ins as they happen, for 15 minutes." Then **Turn on debug mode** (sm).
   - On or ended: exactly as built (StatusSeat On or Ended, Change account, Turn off). The live list unfolds under this row, inside the card.
3. **History.**
   - One version: "Version 1, as Actions v1 sent it", with "Saved 3 Nov by Syndra." in faint under it. No button: there's nothing to go back to.
   - Two or more: the current version's line, then a ghost sm **Show all 3** with the chevron. It unfolds the version table as built, with Go back to this version.
   - Nothing saved: "Nothing saved for GitLab yet", with "It uses Code's default." in faint under it.

## 7 · Phone at 390

**One column, the shape first (#90).**
- The header, then What NetBird is told:
  - the summary, its keys above their words;
  - the roles stacked, with the word field full width;
  - Save when it is there.
- The preview:
  - the title line wraps, and Checked and Check again go to a second line, right-aligned;
  - then the verdict and the token. The JSON wraps and never scrolls sideways.
- The tools: each row's seat wraps to its own line, right-aligned.
- In Advanced, Claim rows folds the same way. ClaimRow already stacks.

## 8 · Motion

**Every fold is Unfold (Motion v2 6a and 8a; A-08 §3). Nothing travels, and nothing is over 10px.**
- **Opening:** the height settles on the soft curve, and the contents tint in one beat behind. This covers Claim rows, the checks, the save row, History's table and Debug mode's list.
- **Closing:** it fades first, then the gap shuts, quicker.
- **The chevron** turns 90° on tint. It is a control's own indicator, so it isn't page content.
- **The page never scrolls** to a fold. Something you open below the screen pushes down out of sight. The save row is the only part that appears from your own edit. It opens under the table, where you are looking, so nothing above you moves.
- **The verdict** crossfades in its seat when the checks re-run. It never blooms.
- **Arrival** plays none of this. Opened on first paint (a failure), the trail is simply there.
- **Reduced motion:** each part appears and goes at once.

## For code

All in `ui/`. No API change. No new design-system part: each change below is a prop or a layout change on a part that already exists.

1. **`AppTokenScreen.tsx`**
   - Drop PageHeader's `lede`.
   - Drop ClaimPreview's `lede` prop and its computation.
   - Replace the `DebugMode`, `ClaimHistory` and `AppSignInStrip` cards with one `<Card>` holding their three rows (6).
2. **`ShapeCard.tsx`**
   - **Add the summary** under the card title: one line per top-level key, derived from the profile (2). Its words go into `lib/claims.ts` beside `describeShape`. That function stays for check 4.
   - **The template line:**
     - It becomes "In {LANDS}. {tplLine}".
     - When `shape.suggested_template` differs from the template the app is on, the first ghost button reads "See the {suggested} template" and opens that sheet.
     - `TEMPLATE_CAPTION.custom` is removed.
   - **Remove:**
     - the roles lede paragraph;
     - the "Claims" type-label;
     - "Nothing to save yet."
   - **The Claim rows fold, Advanced only:**
     - it holds the ClaimEditor rows, Add a claim and the Lands in `SettingRow`;
     - Basic renders none of them;
     - its header is the 28px chevron in a 44px seat, the same markup as `PersonAccess`'s project fold;
     - it has local state, closed on mount, and is forced open while `firstProblem >= 0 || previewRowError`;
     - its note is "{n} rows · keys · {lands short}".
   - `checkOneFailsReads` moves its danger line from the Lands in row to under the summary.
   - The text-format warning is shown in the summary when a saved roles row isn't `array`.
   - **The save row** goes inside `<Unfold open={Boolean(draft?.committed)}>`.
3. **`RoleWordsTable.tsx`**
   - Add `changed?: boolean` to `RoleWordRow`. ShapeCard sets it when the word differs from the saved one, or when the role was sent and is now unticked.
   - The quiet field:
     - at rest: `border-transparent bg-transparent`;
     - on hover: `hover:border-line`;
     - on focus: the ring, as now;
     - `border-line-strong` while `changed`.
   - Row padding `py-2`.
   - The bundle copy follows when the design system is republished. On the board, a helmet rule stands in for this.
4. **`ClaimPreview.tsx`**
   - Put the picker into the title row, and Checked and Check again at its end.
   - Add the verdict line (5), with local `shown` state.
   - The trail is `<Unfold open={shown || anyNotHealthy}>` and is `compact` when it opened by itself.
   - The TokenBlock (and `would_receive`) moves out of check 5's `children` to under the trail.
   - The faint line drops the marks explainer.
   - Copy preview sits in a right seat beside the faint line.
5. **`claim-words.ts`**
   - `check5`'s sentence becomes "Syndra answers with {keys}, below." (and "…, but only for userinfo." on failure).
   - Add `verdictOf(checks)` for the verdict line: "All five checks pass", "Check 1 fails", "Checks 1 and 5 fail", "Check 4 needs a look", and " on this draft" with a draft.
6. **`DebugMode.tsx`, `ClaimHistory.tsx`, `AppSignInStrip.tsx`**
   - Each renders a settings row, not a Card.
   - Debug's off state is StatusSeat neutral "Off" with its note.
   - History is collapsed to its current version, with "Show all N" (an Unfold) when there are two or more.
   - The strip is 20px bars, 178 wide, without its lede.
7. **Tests.** Pin the behaviour that matters:
   - Basic renders no ClaimRow and no Lands in control;
   - the fold opens itself on a row problem;
   - the trail opens itself, compact, on a fail or warn;
   - the save row appears only with a committed draft;
   - the summary's roles words for renamed, left-out and text-format shapes.
8. **Copy guide.** Add "Claim rows", "Show the checks", "Hide the checks" and the verdict phrases. If the owner chooses 4b, it becomes "Custom claims".
