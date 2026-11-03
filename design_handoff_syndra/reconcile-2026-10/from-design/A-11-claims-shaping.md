# A-11 · Shaping what an app is told: a clearer flow

Answer to `Q-11-claims-shaping.md`, 3 Nov 2026. Board: `a11-claims-shaping.dc.html` (Final, 3 Nov 2026). It replaces the 23 Oct `route-applications-id` board wherever they disagree. The example world: the project 3D printing has 3 apps (Print queue, Printer kiosk, Filament log) and 41 people. Version 6 sent `roles`; version 7 renames it to `printing.roles` and adds `team`.

The page has one job: make sure this app is told the right thing about people. Every part below either says what the app should be told, shows what it is told, or changes it.

## 1 · The flow

**The page reads top to bottom: what the app reads, then the format and the preview side by side, then test people, History, and Moving off Actions v1. In the preview, the five checks are a trail that ends in the token (1a).**

- **Order.** PageHeader → "What Print queue reads" (§6) → Token format | Preview token (two halves, as today; on a phone the format comes first, #90) → Test people (§3) → History (§5) → Moving off Actions v1 (§7). Gaps: header → content 28, cards 20.
- **Header lede:** "What Print queue is told about a person when they sign in. Come here when it isn't seeing the roles it should, or before you change what it's told." Meta: "OIDC · uses roles from 3D printing →"; the id stays Advanced-only.
- **Preview token card**, top to bottom:
  - Lede: "Exactly what Print queue receives when this person signs in, and each check on the way."
  - The person seat: "for" · avatar and name chip · **Change person** (ghost sm, opens the person form, §3) · faint "Checked 09:54" · **Check again** (outline sm). Choosing a person runs the check, so the old accent "Run preview" goes.
  - An ordered list, one check per row: a 6px StatusDot in a 14px seat, a 1px `--line` joining it to the next dot, then the sentence in ink at 14.5, then one faint detail line at 13.
  - **Check 5 is the token.** Its sentence ("Zitadel receives this:") sits above the token block, then the sentence under the block (#49 and the faded-line sentence carry over), then Copy preview and Show the role list.
- **The five sentences**, all passing:
  1. "Zitadel lists 3D printing in Print queue's tokens." Faint: "Assert roles is on for 3D printing."
  2. "Ada Lovelace holds 2 roles in 3D printing." Faint: "Trained and Maintainer, as Zitadel shows them."
  3. "Her claims are ready." Faint: "Built at 09:52 today, after her last change."
  4. "3D printing's default format applies." Faint: "It makes `printing.roles` and `team`. Print queue has no format of its own."
  5. "Zitadel receives this:" and the token.
- **Every check always runs.** A failure doesn't hide the rest, so the trail points at the one step that broke. The token block shows `{}`, and its sentence says what checks 2 to 4 would give once the fault is fixed.
- **A failed check** has a danger dot, its sentence, a faint detail, then the fix: a plain sentence, then a violet link ending in →. No box: the dot and the words carry it.
  - **The fix is in Zitadel** (frame `check-assert`): "Zitadel doesn't list 3D printing in Print queue's tokens, so Print queue gets nothing from Syndra." Faint: "Assert roles is off for 3D printing, and Print queue doesn't ask for role scopes." Link: "Turn on Assert roles for 3D printing in Zitadel →" (opens Zitadel's console). Faint: "Or have Print queue ask for role scopes when it signs people in. Then check again." A fix in Zitadel always names Zitadel in the link's words.
  - **Refused, ADR-002** (frame `check-refused`): "Print queue asks for two projects when it signs people in: 3D printing and Wood shop. A token carries one project's claims, so Syndra adds none." Faint: "Last refused at 10:12 today, when Aanya Rao signed in." Fix: "Stop Print queue asking for Wood shop's project. That is set in Print queue itself, not in Zitadel. If it needs a Wood shop role, give a matching one in 3D printing with an automatic rule." Link: "Automatic rules →". Check 5: "Zitadel receives no Syndra claims for anyone signing in to Print queue."
  - **No roles** (frame `no-roles`) is not a failure. Check 2 has a **neutral** dot: "Rohan Mehta holds no roles in 3D printing." Faint: "That is not an error: none of the roles he holds belong to 3D printing." Link: "Give him a role on his page →". Check 3, neutral: "Nothing to build for him." Under the token: "Rohan Mehta would sign in to Print queue with no roles." (#49).
- **Tones.** Lime: passes. Neutral: a fact, nothing to fix. Amber: a broken assumption (check 4 when the token doesn't match what the app reads, §6; claims not built yet). Red: the token is wrong or empty because of this step.
- **Degraded** (frame `degraded`): the amber Notice reads "Zitadel isn't answering. This is what it held at 09:40.", then "Everything below is real, read 14 minutes ago, and may have changed since. Previews go by what Zitadel held then." Checks 1 and 2 get an **amber ring** (not a fresh read) and past tense: "Zitadel listed 3D printing in Print queue's tokens at 09:40." Faint: "Its last answer. Check again won't help until Zitadel answers." The preview still runs. This changes the 23 Oct line, which said previews wait.
- **Loading, error, not found:** as on the 23 Oct board.
- **Why not a strip (1b) or a fold (1c):** five sentences don't fit a strip, so each check would shrink to a word and its fix would move into a hover. A fold hides the reason for an empty token, and the reason is what people open this page for. All three are drawn side by side under Options on the board.

## 2 · Previewing an edit before saving

**A draft previews itself. The token block shows now and after in one block, line by line, and a sentence names each difference in words.**

- The dashed amber "save to preview" line is gone. While the format has unsaved edits:
  - Preview lede: "What Print queue receives now, and once you save this draft. Nothing changes until you save."
  - Check 4: "Your draft of 3D printing's default applies." Faint: "Not saved yet. It makes `printing.roles` and `team`, where version 6 made `roles`."
  - Check 5: "Zitadel would receive this once you save:"
- **One block, not two side by side.** A half-width card can't hold two token blocks, and one block keeps unchanged lines in one place.
- **Marks, none by colour alone:**
  - A line that **leaves** starts with "−" in a 1.5ch gutter, is struck through, and is muted.
  - A line that **joins** starts with "+" and is set at weight 600.
  - A line with no mark is unchanged. A renamed key is a − line and a + line.
  - Each marked line carries hidden text for screen readers ("Leaves the token:", "Joins the token:").
- **The sentence under the block:** "Two differences for Ada Lovelace: `roles` becomes `printing.roles`, and `team` is added. A line marked − leaves the token; a line marked + joins it. The faded line is Printer kiosk's own."
- The draft runs through the same explain step as the live token (Locked), every time the form changes and the person is chosen. It shows a skeleton only if slow (S7).
- **Inline validation runs on the draft:** an invalid key or a repeated key shows under its row before Save (§8).

## 3 · Test people

**A card under the format and preview, full width, up to five people. Pinned per app and shared by every operator. Picked with the person form.**

- **Title** "Test people". Lede: "Up to five people every change to Print queue's format is checked against. Choose people who hold their access in different ways." **Add a test person** is outline sm in the card's header seat.
- **The table**, not editing: Person (240: avatar, name linking to their page, and how they hold access in faint 13: "Given by hand", "From the Makers bundle" with the bundle linking, "From an automatic rule", "No roles in 3D printing") · What Print queue receives (mono, wraps, never truncated) · Checks (dot and word: "All pass", "No roles", "Fails check 1") · Remove (outline sm).
- **While editing**, the same rows show: Person · "Now, version 6" · "After this draft" · Difference (words: "`roles` renamed, `team` added"; "None. He holds no roles in 3D printing."). The After column uses the § 2 marks.
- **It only shows the claims this app reads** (the format that applies to it). Other apps' keys belong to the token block and the save summary.
- **The person form** (frame `pick`) is the bundle dialog's "A person" form in an AnchoredPanel:
  - An Input, "Choose a person", then one ChoiceRow per match, single seat.
  - Each row has the name, a person line ("Design · Student") and the email, as in Start from a person.
  - The note seat says what they hold in this project before anyone picks: "Trained, through the Makers bundle", "No roles in 3D printing".
  - A person already pinned is held: ticked grey, "Already a test person".
  - Choosing adds them, the panel closes, and the new row tints in. A faint line under the rows: "4 of 5 chosen. Choosing someone adds them."
  - At five, Add a test person is off, with "Five is the most." beside it.
  - Below tablet, the panel is a Modal sm sheet: you act in it.
- **Change person** in the preview opens the same form, with the test people listed first under a quiet "Test people" label.
- **Why five, and why "how they hold it":** the outside review's matrix (by hand, bundle, rule, none, more than one project) is what makes a test set worth having. The how-line lets an operator see the spread without writing a reason for each person.

## 4 · Saving

**Both: the before-line under Save always says the reach, and Save opens a summary dialog that ends on its own result.**

- **The before-line**, faint 13, beside the accent "Save token format":
  - Nothing changed: "Nothing to save yet." (Save off)
  - With edits: "2 unsaved changes. Reaches 3 apps and 41 people at their next sign-in."
  - Blocked: "Fix `kiosk.roles` to save." (Save off)
- **Why a dialog as well:** a default change reaches every app on the project and can break one of them (§6). History needs a note (§5). And every act ends in its sheet (96b).
- **The dialog** (frame `save`), Modal md. Title "Save token format". Lede: "3D printing's default, used by 3 apps. It is kept in History as version 7."
  - **What changes**, one line each, with a gutter mark: "+ `team` is added: the person's team, as text." "~ `roles` becomes `printing.roles`. Still an `array`." (− for removed.)
  - **The 3 apps on 3D printing**, one row each: the name, then a dot and a sentence.
    - Print queue: lime, "Will match what it reads."
    - Filament log: amber, "Won't match: it reads `roles`." Faint: "Change what Filament log reads, or update it first. Saving doesn't wait for either."
    - Printer kiosk: neutral, "No change: it reads its own `kiosk.roles`."
  - One sentence: "Your 4 test people: 3 change as the page shows, and Rohan Mehta doesn't."
  - **What changed, for History**: a Textarea, required, prefilled from the differences ("Rename for the new print queue; add team."). It is the Publish version note's pattern on mappings: one pattern per job.
  - **Footer note (the before-line):** "41 people get this at their next sign-in. Anyone signed in now keeps the old token until then." Cancel (outline) · Save token format (accent).
- **A mismatch doesn't block the save.** The operator may be updating the app next. The dialog says so plainly and the app's page carries it afterwards.
- **Saved** (frame `saved`): lime dot that blooms once, **Saved**, "3D printing's token format is now version 7." Faint: "41 people get it at their next sign-in. Filament log's page now says it doesn't match what it reads." Close. Reopened, it starts clean.
- **Didn't save:** the red Notice at the top of the form: "The token format didn't save." Then "Syndra's server didn't answer when you saved. Nothing was changed." and the reference. The form keeps what was typed.

## 5 · History

**A card at the foot of the page (5a). A version is the whole project's format, because one token carries all of it. Going back saves the old version again as a new one.**

- **Title** "History". Lede: "Every save of 3D printing's token format, kept as it was, for all 3 apps. Going back saves an old version again, as a new one."
- **Columns:** Version (130, "Version 7") · What changed (the differences in words, mono keys, then the note in faint quotes) · Saved (150, right: "3 Nov", then "Kanishk Singh" linking to his page) · a 210 seat.
  - The version in use shows a lime dot and "In use".
  - Every older one has **Go back to this version** (outline sm; it opens a dialog, so no →).
- Newest first, no search (a short list). Past ten rows, the long-list pattern applies (Showing 10 of N).
- **The same History shows on every app page of the project.** A row about one app's own format names it: "Printer kiosk's own format: `kiosk.roles`, sent as `csv`."
- **Go back** (frame `back`) uses the save dialog's anatomy:
  - Title "Go back to version 6". Lede: "Saves version 6 of 3D printing's format again, as version 8. Version 7 stays in History."
  - What changes: "− `team` is no longer sent." "~ `printing.roles` becomes `roles` again."
  - The apps: Print queue, amber, "Won't match: it reads `printing.roles`."; Filament log, lime, "Will match what it reads."
  - The test people sentence, then the note, prefilled "Back to version 6 until Print queue is updated."
  - Confirm: **Go back to version 6** (accent: nothing is lost). Result: **Saved**, "3D printing's format is version 6's again, saved as version 8."
- **Why not a tab (5b) or a sheet (5c):** a tab hides the preview just when you want to compare a version with it. A sheet costs a dialog every time you only want to read the list.

## 6 · What the app reads

**The first card on the page: one row per claim the app's code expects, each with Matches or Doesn't match. This is its one write place. The apps index names each app's own claim and shows the verdict.**

- **Title** "What Print queue reads". Lede: "The claims Print queue's own code expects. Syndra checks every preview and every save against this note."
- **One row per claim**, the settings-card shape: key (170, Syndra.Mono) · what it is ("An `array` of role keys", "Text") with the verdict under it as dot and word · **Change** (outline sm, 200 seat).
  - Under the rows: **Add a claim it reads** (ghost sm).
  - The type list is the same as § 8, plus the three roles formats (`array`, `csv`, `spaced`).
- **Verdicts:**
  - Lime **Matches**.
  - Amber **Doesn't match**, then why and the way out in one muted sentence: "The token has `printing.roles` instead, since version 7 on 3 Nov. Note `printing.roles` here once Filament log reads it, or go back to version 6 in History." (frame `contract`)
  - While editing: amber **Doesn't match yet**, with "The saved token has `roles`. Your draft makes `printing.roles`, which matches." The draft is checked too.
- **Check 4 turns amber** when the token doesn't match the note: "It makes `printing.roles` and `team`. Filament log reads `roles`, which this token doesn't carry." The note is not a sixth check: it is what the five are measured against.
- **Nothing noted yet:** no rows. One faint sentence: "Nothing noted yet. Note what Print queue reads, and Syndra checks every save against it." Then **Add a claim it reads**. No warning: an app without a note is allowed.
- **Apps index** (frame `index`):
  - The sub-line under each name is the claim that app actually gets: its own when it has one, with "· its own", or else the project default. Example: "OIDC · `kiosk.roles` · its own".
  - The Format cell shows the effective format.
  - Where an app has a note, the verdict replaces the mixed-format guess. The Format cell turns amber with "doesn't match what it reads", in the existing "mixed in Laser Lab" idiom, plus one sentence in the read line: "**Filament log doesn't match what it reads.** Its page says what changed."
  - The mixed-format warning stays only for apps with no note.

## 7 · Ready to move off Actions v1

**On the app page, the last card, drawn as advice. Not one page for all apps: the comparison is per app and per pasted token.**

- **Title** "Moving off Actions v1". Lede: "Advice for the day you turn off Print queue's Actions v1 scripts in Zitadel. Nothing here turns anything on or off."
- **The checklist** (dot and sentence, no boxes):
  - "Zitadel lists 3D printing in Print queue's tokens."
  - "There's a note of what Print queue reads, and the token matches it."
  - "None of the 4 test people fails a check."
  - Compare: neutral, "No Actions v1 token compared yet. Paste one below." After comparing: amber, "2 claims differ from Actions v1 for Ada Lovelace. The comparison is below."
- **Paste and compare** (frame `v1`):
  - The field is "A token Actions v1 issued, decoded" (a mono Textarea). Faint under it: "Syndra doesn't keep what you paste. It keeps only the result."
  - Then **Compare** (outline sm).
  - The token names its person by its `sub`: "This is Ada Lovelace's token, from its `sub`. Compared with what she gets now." With no `sub`, the person form opens.
- **The comparison table:** Claim · Actions v1 · Syndra now · Verdict (dot, word, and what to do).
  - **Same.**
  - **Different:** "Actions v1 sends text with commas; Syndra sends an `array`. Set Sent as to `csv`, or check that Print queue reads an array."
  - **Only in Actions v1:** "A v1 script read this from Zitadel's organisation, which Actions v2 can't read. If Print queue needs it, send it as the same value for everyone."
  - **Only in Syndra** (neutral): "Printer kiosk's own claim. Print queue ignores it."
  - Faint, last: "Left out: `sub`, `email`, `exp` and Zitadel's other standard claims, which it adds either way. When every line reads Same or Only in Syndra, Print queue is ready to move."
- **Owner's call: does it gate anything?** Recommend **7a, advice only**. The v1 scripts live in Zitadel. Syndra can't see them or turn them off, so a gate in Syndra would guard nothing. 7b, holding back Syndra's claims for an app until its list passes, is drawn too: safer during the move, but a second switch per app, and it makes apps that never used v1 wait.
- **Owner's call: does Syndra keep the pasted token?** Recommend **no**. It holds a real person's email and ids. Keep only the result ("Compared 3 Nov: 2 differences") so the checklist line survives a reload.
- When Actions v1 is gone for good, the card is removed in a release. It never hides because of data.

## 8 · Static values

**Choosing "the same value for everyone…" adds a Type Segmented: Text | Number | Yes/no | List. Each row ends with "Sent as" and the exact value in mono, so the type shows as the token will carry it.**

- **The row:**
  - Claim name (mono Input) · Where the value comes from (as built) · Remove.
  - With "the same value for everyone…", a second line: Type (Segmented sm) and the value control for that type.
  - Then the faint "Sent as …" line. Every row has it, attributes too: "Sent as `"Fabrication"` for Ada Lovelace."
- **The value controls:**
  - **Text:** an Input. Sent as `"makerspace"`.
  - **Number:** an Input with a numeric keyboard. Sent as `40`. Field error: "Type a number, like 40."
  - **Yes/no:** a Segmented, Yes | No. Sent as `true`.
  - **List:** an Input. Faint: "Separate items with commas. Sent as `["bay-1", "bay-2"]`." Items are text.
- **Quotes or none, brackets or none:** the "Sent as" line is where the type shows. The draft preview (§2), History and the save summary all show the same literal.
- **A saved value keeps its type.** The form reads the type from the stored JSON value and never guesses: `true` stays Yes/no, `42` stays Number. A value Syndra can't edit (an object) shows read-only: "An object. Syndra keeps it as saved; it can't be edited here." The row keeps Remove.
- **A repeated key is named:** under its row, in danger text: "`kiosk.roles` is already sent by Printer kiosk's own format. A key can be used once in 3D printing." The repeated key is left out of the draft preview and Save stays off until it is fixed. A wrong key reads "Use letters, numbers, dots, dashes and underscores." (frame `static`)
- **The roles claim:** when nobody has named it, it still says so (#50), from a real `configured` flag, not a guess.

## For code

- **One explain step** for the live path and every preview, draft included. It returns the token plus five results, each with a state (`pass` | `fail` | `none` | `stale`), the facts each sentence needs, and when it was read. For example: Assert roles on the project; role scopes; roles held; when the claims were built; which profile applies and its keys; the time of Zitadel's last answer when degraded.
- **Check 1 needs two facts we don't fetch today:**
  - Assert roles is a **project** setting in Zitadel (`projectRoleAssertion`). Confirm, and build the console deep link to it.
  - The last refused sign-in per client from the ADR-002 log line: time, person, the projects asked for. Without it, the refused frame can't say "Last refused at 10:12"; drop that line rather than invent it.
- **Draft preview:** `POST /projects/{id}/claim-shape/preview` with `{ draft, user_ids[] }` through `internal/claims`, one call for the preview person and every test person.
- **Test people:** `GET/PUT /applications/{id}/test-people` (at most 5, shared by every operator). Each person comes back with how they hold access in the project (by hand, bundle with its id, rule, none) for the how-line and the note seat.
- **Save summary:** the reach (apps on the project, people holding roles in it), the key-level differences from the shaper's `Keys()`, and each app's match. `ClaimShape.Default.configured` replaces `looksUnset`.
- **Versions:** each save writes an immutable snapshot of the whole project's claim set (default and every override) with a required note. Endpoints: `GET /projects/{id}/claim-shape/versions` and `POST …/versions/{n}/restore`, which writes a new version. The audit records both.
- **What the app reads:** a per-app list of `{ key, type }` stored in Syndra, with types text, number, yes/no, list, `array`, `csv`, `spaced`. The verdict comes from the explain step's output, for the saved token and for a draft.
- **Apps index:** return each app's effective `claim_name` and `format_type` (its own, else the default), whether it is its own, and the verdict where a note exists. Compute the mixed-format warning from effective formats, only for apps without a note.
- **Static values:** keep `static_claims` as typed JSON end to end (no string coercion in `toExtras`/`fromExtras`). Save errors name the key and which profile already uses it.
- **v1 comparison:** compare in the browser, or in a request that doesn't persist. Store only `{ app, person, compared_at, differences }`. Ignore standard OIDC claims.
- **Degraded:** confirm that saving a format needs nothing from Zitadel. If so, Save stays on while degraded, as drawn. If not, the before-line says "Saving waits until Zitadel answers." and Save is off.
- **Into the design system:** Notice, ChoiceRow, AnchoredPanel (all already proposed). New:
  - **CheckTrail:** dot or ring, the joining line, sentence, detail and fix.
  - **TokenBlock:** mono lines; same, added, removed and faded; the gutter mark and the hidden words; `{}`.
