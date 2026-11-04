# A-16 · The claims editor, simple for the common case


**Owner, 4 Nov 2026: every recommendation accepted.** (1) a, the menu of finished claims; (2) yes, roles claim first, the rest A to Z, and the token in the same order; (3) yes, a lone button is outline sm and ghost only beside a primary, swept across Syndra; (4) keys Zitadel already sends are greyed, saying so; (5) no, Basic cannot rename the roles claim; (6) `groups` is the default; (7) yes, the roles stay folded even when some are renamed or left out.

Answer to `Q-16-claims-editor-simple.md`, 4 Nov 2026. Board: `a16-claims-editor.dc.html` (Proposed, 4 Nov 2026; the owner's choices are listed at its end and in §7). This revises A-15 §2 to §4 (the summary, the roles table and the Claim rows fold) and A-14 §4's row form. Every A-14 behaviour, the API, A-15's page structure (shape and roles beside preview and verdict, with the tools card below) and the owner's rulings all stand.

**What was wrong.** The card said everything twice. First the summary said it in words, then the Claim rows fold said it again as five full forms. Meanwhile the roles table, which is rarely changed, was always open and the tallest thing on the card. The controls came from three grammars: bare-text ghost buttons, 46px outlined fields, and a segmented switch on every roles row. **The fix is one list.** The summary becomes the editor. Each line is a claim, and each opens its own editor in place. The roles claim comes first and is the hero. Everything rare lives inside the line it belongs to.

## 1 · The shape card

**One list of claims, under the card title. Each line has the key in mono, what it carries in words, and one faint line where it helps. The roles claim is first. The rest follow A to Z. Lands in and the template move to the card's foot.**

Top to bottom (Advanced):
1. **"What NetBird is told"** (type-card-title).
2. **The claims**, one line each:
   - **`groups`** · "The roles each person holds in Network, as a list". Faint: "All 7 roles, in Syndra's words."
   - **`custom_claims`** · "An object holding `family_name`, `given_name`, `groups` and `name`". Faint: "Its `groups` is the same list of roles."
3. **Add a claim** (Advanced, §3).
4. **Lands in**: a settings row (Advanced): "The access token and userinfo", with Segmented sm Both | Access token | ID token and userinfo. A-15's explanatory sentence becomes the `title` of the ID token option, so it shows on hover and focus and costs no line at rest.
5. **The foot**: faint "From the Custom template, 5 changes since." Then **See the NetBird template** and **Copy another app's shape**, both outline sm, right-aligned. A-15's suggested-template rule stands.
6. **Save**, which unfolds with the first change, as in A-15.

**The line.** The whole line is the control: CardRow's `onToggle` shape, a button with `aria-expanded`. The 28px round chevron in its 44px seat starts the line. It is the grouped list's chevron, as on PersonAccess and SignInRow, and it turns 90° on tint. Then come the key (Mono 14, in a 132px seat) and the words (14.5 ink), with the faint line (13) under the words. On a phone the key sits above its words.

**Answers to Q-16 §1:**
- **Is the roles claim the hero?** Yes, by place, not by size. It is always first, and it is the only line whose faint line describes a state ("All 7 roles…", "6 of 7 roles…"). It doesn't get a bigger type size or a box, because a different look would make it a one-off.
- **One line?** Yes, with a second faint line for its state. "`groups` · a list of every role this app's people hold" was close. "The roles each person holds in Network, as a list" names the project, and says what each person gets.
- **Where do its settings live?** In its editor (§2): the claim name and Sent as (Advanced), then the roles table, then Roles added later (Advanced).

**The order.** The roles claim comes first, then the others A to Z by key, following the new standing rule. Keys inside an object are also A to Z, both in its words and in its editor. The Preview's token uses the same order, so the two read across (owner's call, §7 Q2).

**What's gone:**
- A-15's separate summary block. The list is the summary.
- The **Claim rows** fold, along with its closed note and A-15's 4a/4b naming question. Each line now opens its own editor, so nothing needs a fold name. `CLAIM_ROWS` and `foldNote` go.
- The roles type-label, "Roles NetBird sees · 7 of 7 included". The roles line says it.
- The per-row "Sent as `…` for Ada Lovelace." line on every closed claim. It appears once, inside an opened editor.
- "Put it inside another claim" as a bare button. Nesting is the **Inside** field (§3).

**A text-format warning stays visible** (A-15 §2): if a saved roles claim is sent as text, its line carries the warn-text line in both views, so it can't hide behind a closed line.

**No roles claim** (Sign-in only, or one removed). The first line reads "No roles. NetBird gets none from Syndra." It has no chevron. In Advanced, the menu's "Roles NetBird sees" adds one (§3).

## 2 · The roles, folded

**The roles line opens the roles. It is closed on every arrival, and its faint line says the state, so the common case never needs opening.**

**The faint line:**
- Nothing changed: "All 7 roles, in Syndra's words." With two roles: "Both roles, in Syndra's words." With one: "Its one role, in Syndra's words."
- Some renamed or left out, up to two named: "6 of 7 roles. Staff as `netbird-staff`; Guests left out."
- More than two: "5 of 7 roles. 2 renamed, 1 left out."
- All renamed: "All 7 roles, in NetBird's words."
- With a draft, the line describes the draft, as A-15 §2 did.

**Opened (Advanced), indented to the key:**
1. **Claim name**: mono Input, dense, max 240. Faint under it, from the template's "Set in NetBird": "NetBird reads the name set in its Settings › Groups › JWT claim." On Custom there is no faint line.
2. **Sent as**: "A list, and `[]` for anyone who holds none.", with Segmented sm A list | Text, with commas | Text, with spaces. A-14's warning shows in warn-text when text is chosen. This is where text formats live (Q-16 §3).
3. **The roles table**, dense (below).
4. **Roles added later**: "Included. NetBird sees every role added to Network later.", with Segmented sm Left out | Included.
5. **Remove groups** (danger sm), at the end and right-aligned.

**In Basic**, the opened roles show only the table. The claim name, Sent as and Roles added later stay in Advanced (§7 Q5).

**The dense table:** RoleWordsTable with a new `dense` prop. It keeps the columns (Role ↑ · NetBird calls it · Included), the sort, the quiet word field, the was-lines and the left-out treatment.
- **One line per role.** The name (14/600), WaitsMark and the key in faint Mono 12.5 sit on one line in a 190px seat. Then comes the word field (dense, quiet at rest), then the round seat in a 70px right seat.
- **Row height is about 44px** (A-15's was 58), because the field is 34px with a mouse and the row pads 5. On touch the field keeps its 44px floor.
- **A changed row grows only by its was-line:** "Was `staff`." or "Was sent as `guest`." under the field, faint 12.5.
- Seven roles opened are about 310px. Closed, they are one faint line.

**It opens by itself** only when a row inside it stops Save. No role row can do that today, so in practice it never opens by itself. It stays as you left it while you're on the page.

## 3 · Adding a claim

**Recommended: (a), a menu of finished claims.** Drawn side by side with (b) and (c) at true size on the board.

**(a) The menu.** **Add a claim** (outline sm) opens an AnchoredPanel, 7px below it and 390 wide. Inside is the QuestionSeat listbox's option rows (`role="option"`, 44px on touch, hover tint). The entries are grouped the same way as the Carries Select, and are A to Z within each group:

| From the person | Gets | |
|---|---|---|
| Bundles they hold | `bundles` | |
| Each role's end date | `role_ends` | |
| Email | | Muted: "Zitadel sends `email`" |
| Family name | `family_name` | |
| Full name | | Muted: "Zitadel sends `name`" |
| Given name | `given_name` | |
| Is a service account | `is_service_account` | |
| Roles NetBird sees | `groups` | Muted, "Sent as `groups`", when a top-level roles claim exists |
| Username | | Muted: "Zitadel sends `preferred_username`" |
| Yes if they hold a role | | "Then choose the roles" |
| **The same for everyone** | | |
| A fixed value | | "Then name it and set it" |

- **A finished entry** adds a line that already works. It has the conventional key, sits in its A-to-Z place and opens as a FormPart (its gap opens and it tints in). Its faint line reads "New in this draft." The menu closes, Save unfolds, and the Preview shows the + line.
- **"Yes if they hold a role"** opens the role picker directly, in single-project mode (one part per job). On **Use this role**, the line arrives opened with `is_<role key>` as its name ("`is_admin`" for Admins), with the words "`true` if they hold Admins, otherwise `false`".
- **"A fixed value"** adds a line already opened at its editor, with the name field focused. Save stays off until it has a name and a value, and the editor's error line says which is missing (A-11 §8).
- **A key that's taken** is muted and inert, and says why, in place: Zitadel's own claims, and a key this shape already sends at the top level. They can still be chosen inside an object, through **Add a claim inside**, because Supabase reads names inside `custom_claims`.
- **The attributes behind "More claims"** (title, team, role count, when built, the ids) aren't in the menu. They are in the editor's Carries Select, which is fully technical.
- **Below tablet** the menu is a Modal sm sheet holding the same list (rule: a popover you act in becomes a sheet).

**(b) An inline composer.** Add a claim unfolds one line in its place: a mono name Input, a Carries Select, then **Add** (outline sm) and **Cancel** (ghost sm, the quieter half of the pair).
- Typing a known key picks what it carries, and the composer says so: "Chose "Yes if they hold a role" from the name."
- A second line grows only for a kind that needs one.
- *For:* one form for adding and editing. *Against:* it starts blank, guessing from the name surprises people, and it brings back the field-and-select row the owner called messy.

**(c) A sheet with a live preview.** Modal md, "Add a claim to NetBird":
- ChoiceRows (single) for what it carries. Zitadel's own claims are locked, ticked grey, with "Zitadel sends email already."
- Then the claim name and Inside, side by side.
- Then the new line for Ada Lovelace in a TokenBlock. The footer note says "Adds it to the draft. Nothing changes until you save.", with Cancel and accent **Add given_name**.
- *For:* the most guided. *Against:* a dialog for a one-line change, and its preview repeats the Preview card beside it.

**Why (a).** The common jobs (the roles, then perhaps a name or a yes/no) take two clicks with a key that works. The page's own Preview already shows the + line, so (c)'s preview would be a second place for the same truth. (b) is the form we're removing.

**A claim's editor** (Advanced; opening any line other than roles). It is indented to the key, with one settings row each (label 130 · control):
1. **Claim name**: dense mono Input.
2. **Carries**: Select, with the menu's groups plus the attributes behind them.
3. **The kind's own row**, only where one exists:
   - If they hold: the roles as Chips, with **Change roles** (outline sm);
   - Value: the Type Segmented and the value Input (A-11 §8).
4. **Inside**: a Select of "Nothing: top level", each object already sent, and "A new object", which becomes a mono Input. **This is where nesting lives** (Q-16 §3). A-14's rules stand: a dot stays a dot, rows that share a parent merge, and a row named like a parent is refused.
5. **The faint literal:** "Sent as `true` for Ada Lovelace, who holds Admins, and `false` for anyone who doesn't."
6. **Remove** (danger sm), at the end and right-aligned.

**An object's editor** (`custom_claims`):
- its name;
- then one compact row per key inside it, A to Z: name, Carries, and Remove (danger sm);
- then one faint literal for the whole object;
- then **Add a claim inside** (outline sm, opening the same menu) and **Remove custom_claims** (danger sm, aria-label "…and the 4 claims inside it").

## 4 · Controls, each by its part

| Job | Part |
|---|---|
| Open a claim or the roles | CardRow `onToggle` (the whole line) + the 28px round chevron in a 44px seat |
| Add a claim | Button outline sm → AnchoredPanel + QuestionSeat option rows; Modal sm below tablet |
| Claim name, object name, Inside "A new object" | Input, mono, **dense** |
| Carries, Inside | Select, **dense** |
| Sent as (roles), Roles added later, Lands in, a fixed value's type | Segmented sm, only inside an opened line or a settings row |
| A fixed value | Input, dense |
| Which roles make it true | Chip + Button outline sm "Change roles" → role picker (Modal) |
| A role's word, Included | RoleWordsTable **dense**: quiet Input, Checkbox (round seat) |
| Remove (a claim, a key, the object, the roles claim) | Button danger sm, aria-label naming it |
| Add a claim inside | Button outline sm |
| See the template, Copy another app's shape | Button outline sm (they open sheets, so no →) |
| Show the checks, Check again, Copy preview | Button outline sm |
| Save · Discard the draft | Button accent md · ghost md |

**The grammar, stated once:**
- A button on its own is **outline sm**.
- **Ghost** appears only as the quieter half of a pair (Discard beside Save, Cancel beside Add or a confirm) and inside PaginationRow. A ghost button at rest reads as bare text, and that was most of the "not consistent" (§7 Q3).
- **Fields in the card are the sm control's height**: 34px with a mouse, with the 44px floor below desktop, as PILL does for buttons.
- **Nothing is a bare text button.**

## 5 · Basic

Basic shows the same list:
- the roles line opens to the dense table, with words and Included only;
- the other lines have no chevron (their seat stays, so the keys align) and don't open;
- there is no Add a claim and no Lands in row;
- the foot says "In the access token and userinfo. From the Custom template, 5 changes since.", with the two template buttons.

A-15 §1's rule holds: nothing sent is hidden, and only the forms are.

## 6 · Phone, motion

- **Phone at 390:**
  - one column, the shape first (#90);
  - each key sits above its words;
  - an opened editor's label sits above its control;
  - the dense table stacks a role's name and key over its field, with the seat at the right;
  - every field keeps 44px;
  - Add a claim opens a Modal sm sheet with the same list, ending in no button, because choosing is the act;
  - the foot buttons wrap, right-aligned.
- **Motion:**
  - every editor opens by Unfold (Motion 6a and 8a): the height settles, the contents tint in, and closing fades first;
  - the chevron turns on tint;
  - a claim from the menu arrives as a FormPart in its A-to-Z place, and a removed one leaves as a LeavingRow;
  - the menu uses popover-in and tint-out;
  - nothing travels more than 10px, and the page never scrolls to an editor;
  - reduced motion: everything appears and goes at once.

## 7 · Questions for the owner

1. **Adding a claim: (a) a menu of finished claims, (b) an inline composer, or (c) a sheet with a live preview?**
   *Recommended:* (a). The common case takes two clicks and gets a key that works, and the Preview beside it already shows the + line.
2. **Should the list put the roles claim first and the rest A to Z, with the Preview's token in the same order?** Today the token keeps the order rows were added in.
   *Recommended:* yes. Apps don't read key order, so the order is only for you. The roles claim being first is the one exception to A to Z, because it is the reason the card exists.
3. **Should "a button on its own is outline sm; ghost only as the quieter half of a pair" become a standing rule across Syndra, with a sweep of the other lone ghost buttons (about 15)?**
   *Recommended:* yes. PaginationRow keeps its own controls.
4. **Should the menu show the keys Zitadel already sends (`email`, `name`, `preferred_username`) greyed, with "Zitadel sends email", or leave them out?**
   *Recommended:* greyed. That answers "why can't I add email?" in place, and they stay choosable inside an object.
5. **Should Basic be able to change the roles claim's name?**
   *Recommended:* no. The app's own settings point at that name, so changing it breaks things for the app, and that is Advanced work. Basic keeps role words and Included.
6. **What should a roles claim added to a shape with none be called by default?**
   *Recommended:* `groups`. Five of the six templates that send roles use it. Custom keeps `roles`.
7. **Should the roles stay folded on every arrival, even when some are renamed or left out?**
   *Recommended:* yes. The structure doesn't move with the data, and the line already says what changed.

## For code

All in `ui/`, except the one backend check in item 6. No API change.

1. **`ShapeCard.tsx`**
   - Replace `Summary`, the roles block, the Claim rows fold and the `ClaimEditor` list with **one list of claim lines**, grouped by top-level key:
     - the first roles row's key first, then A to Z;
     - each line is a `CardRow` with `disclosure`/`expanded`/`onToggle` (Advanced: every line; Basic: the roles line only);
     - local state holds which line is open, one at a time, closed on mount.
   - **The roles line's editor:** Claim name, Sent as and Roles added later (Advanced), and `RoleWordsTable dense`.
     - Its faint line comes from a new `rolesState(profile, roles, { app })` in `lib/claims.ts`, with the five phrasings in §2.
     - `summaryLines`' roles words are replaced by "The roles each person holds in {project}, as a list".
   - **A claim's editor** uses the existing `ClaimEditor` logic, re-laid as settings rows (label 130 · control). Its ClaimRow line 1 and footer go:
     - **Inside** replaces "Put it inside another claim";
     - Remove becomes `variant="danger"`.
   - **An object's editor:** the object's name, then compact child rows, then one literal (`sentAs` on the whole object), then Add a claim inside and Remove the object.
   - **Add a claim:** outline sm plus an `AnchoredPanel` listbox (the QuestionSeat markup), and a Modal sm sheet below tablet.
     - A new `ADD_MENU` in `lib/claims.ts` holds each entry's label, the claim it makes and the reason an entry is muted.
     - A finished entry appends and the list sorts. If-role opens `PickRoles` first. A fixed value lands opened.
   - Lands in moves to a settings row after Add a claim (Advanced). The template line moves to the foot, and its buttons become `variant="outline"`.
   - Remove `CLAIM_ROWS`, `foldNote` and `TEMPLATE_CAPTION` use here.
2. **Ordering.**
   - The list's order is a view: roles first, then A to Z.
   - The TokenBlock lines in `ClaimPreview` sort the same way, top-level and inside objects.
   - Saved row order doesn't need to change, because apps don't read key order. If the owner says yes to Q2, add a display-only sort in `lib/claims.ts` and use it in both places.
3. **Design system.**
   - `RoleWordsTable`: a `dense` prop: name, WaitsMark and key on one line; the dense quiet field; a 70px Included seat.
   - `Input` and `Select`: a `dense` prop, `min-h-[44px] py-[5px] text-[13.5px] desktop:min-h-0`, matching PILL sm. The board stands in for it with `.a16-dense`.
   - Then resync the bundle.
4. **Buttons.**
   - On this page, Show the checks becomes outline sm, along with See the template, Copy another app's shape and Change roles.
   - If the owner says yes to Q3, `one-control-surface.test.ts` can pin it: a `ghost` Button must have an `accent` or `dangerConfirm` sibling.
5. **Copy guide.**
   - Add: "The roles each person holds in {project}, as a list"; the roles-state phrasings; "New in this draft."; the menu's labels and its muted reasons ("Zitadel sends `email`", "Sent as `groups`"); "Add a claim inside"; "Nothing: top level"; "A new object".
   - Remove "Claim rows".
   - The guard's `LEDELESS` reason still holds.
6. **Zitadel's own keys (code decides; flagged, not ruled).**
   - The menu mutes `email`, `name` and `preferred_username`, because A-15's token line says Zitadel adds them. Confirm the list for the scopes each app asks for: `profile` may also bring `given_name` and `family_name`. Read the menu from one constant.
   - Also confirm what Zitadel does with an Actions v2 `append_claims` entry for a reserved key. If it is ignored or refused, `internal/claims` validation should refuse a top-level reserved key too, with an error the editor can show. Today the shaper doesn't check.
7. **Tests.**
   - Basic renders no claim editor, no Add a claim and no Lands in control.
   - The roles line's state for none / ≤2 / >2 / all changed.
   - A finished menu entry adds a valid row and opens no editor; if-role opens the picker first; a fixed value lands opened and blocks Save until filled.
   - Muted entries are inert.
   - The list and the token share one order.
   - Only one line is open at a time.
