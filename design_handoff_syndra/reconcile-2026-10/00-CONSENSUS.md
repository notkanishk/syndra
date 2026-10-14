# Consensus — where the boards and the build disagree

> **Folded back, 13 Oct 2026.** Every row now carries its October verdict and the boards that draw it. *Carried* means the row was settled (Keep, Locked or Owner-ruled) and every board follows it. The reasoning for each verdict is in `verdicts.md`; what to build is in `BUILD-LIST.md`; differences the sweep did not list are under **New differences** at the end. Start at `README.md`.

The boards and the shipped app have drifted apart in both directions. Some of
that is the build correcting the board, some is the build drifting from it, and
some is the board never having drawn something the build needed. This file is
the one list of those differences, and the rule for settling each one.

## The deciding test

**An elegant, cohesive experience is the final test for any change.** When a
difference is settled, the winner is whichever version makes Syndra read as one
product: the same pattern for the same job everywhere, nothing that makes an
operator stop and decode it, and nothing on a screen that has no reason to be
there. Neither "the board said so" nor "the code already does it" is a reason
on its own.

That test decides *presentation* — layout, hierarchy, interaction, wording
shape. It does not overrule the two kinds of fact below, which are not matters
of taste.

## Four classes

| Class | What it means | Who settles it |
|---|---|---|---|---|
| **Keep** | A small change the build made with a recorded reason — a rename, a label, a cap. | Already settled. Adopt it on the boards. Raise it only if it breaks cohesion somewhere else; do not change it unilaterally. |
| **Locked** | The behaviour is a correctness, safety or data-truth fact: what the backend can truthfully return, what an action must require, what it costs. | The behaviour stands. Design is free to redesign how it is *presented*. |
| **Owner-ruled** | The owner already chose between board and code. | Preserved. If the deciding test argues otherwise, write the argument down and it goes to the owner — consensus, not override. |
| **Reconcile** | A real design difference with no recorded reason, or a state the boards never drew. | Design decides, by the deciding test. |

## The ledger

Sources are commits in this repo or sections of the handoff notes. Rows 1–16
came first. Rows 17–111 come from a sweep of **every one** of the 197 commits
that touched `ui/src` from 2026-07-31 to `c940f93` (2026-09-20); every hash
cited was checked to exist in that range, and the claims most likely to be
wrong were checked against the source.

Counts by leading class, 112 rows: **Locked 51 · Keep 32 · Reconcile 22 ·
Owner-ruled 7.** Open design work is every row with *Reconcile* anywhere in
its class — 26 rows, since some rows are Locked in behaviour but Reconcile in
presentation. Everything else is a settled fact the boards should carry.

### Nav, brand and target plane

| # | Area | Board | Shipped | Class | Source | Verdict · Oct 2026 | Boards |
|---|---|---|---|---|---|---|---|
| 1 | Landing name | Today | **Home**. "Today" names a day; the page grew a second zone, *The makerspace*, that is not day-scoped. "Today" now means only the day, never a screen. | Keep | `8750b50` | Carried | — |
| 2 | Accent | Lime `#d8f24e` active row | Violet everywhere; lime is *healthy* only — a dot, a word or a hairline, never a fill. Recorded in the brand board too. | Keep | `fe26d79`, `login/LOGIN.md` | Carried | — |
| 3 | Review › first item | Unexplained access | **Drift** | Keep | `5dc4deb` | Carried | — |
| 4 | System › items | Identity provider · Hardware sync · Event activity | **Zitadel** · **Connected systems** · **Incoming events** | Owner-ruled (copy guide names Zitadel deliberately) | `82b386c`, `25bcb40` | Carried | — |
| 5 | Review › Withdrawn access | §22 *Withdrawn access* (README §IA delta) | **Unfinished revocations**, danger badge | Owner-ruled (copy guide) | `82b386c` | Carried | — |
| 5b | Review › Holds | §25: two items — *Holds* (every hold in force, no badge: a hold is not work outstanding) and *Review due* (amber) | One item, **Holds due**, at `/review/holds`, amber. Placed beside Expiring access, never inside it — inaction means opposite things in each. No nav entry lists every hold in force. | Reconcile — the merge has no recorded reason | `3ef0246`, `ui/src/lib/nav.ts` | **KEEP-CODE** | route-holds, route-home |
| 6 | M3 rehearsal | Rung 3 — type the number | **Rung 2.** Copying digits trains an operator not to look; rung 3 is reserved for taking access from a named person. The step was strengthened into an act about the number. | Owner-ruled | `target-plane/SHIPPED.md` | Carried | — |
| 7 | M3 refusal | `409 · COHORT_LIMIT` | `422 COHORT_ACKNOWLEDGEMENT_REQUIRED`; the screen branches on the code, never the status | Locked (invisible to the operator) | `SHIPPED.md` | Carried | — |
| 8 | M6 census | Shows how many people hold the mapped roles | Not rendered. No endpoint returns distinct people across roles; summing would overstate it. | Locked — redesign the row without the number, or propose what it should say | `SHIPPED.md` | Carried | — |
| 9 | M1 per-row "TrueNAS recognises this group" | Per row | Checked at edit time only; per row is one call per mapping per view | Locked | `SHIPPED.md` | Carried | — |
| 10 | M5 "accounts already on the NAS are unaffected" | Card | Not built — needs an inventory read that screen doesn't take | Locked | `SHIPPED.md` | Carried | — |
| 11 | Version band "backed off" | Described in words only | Exists in code, never seen | Reconcile — draw it | `SHIPPED.md` | **SYNTHESIS** | route-system-targets-id, route-systems |
| 12 | Publish | A control in the band *and* in the history panel | **One control**; a blank note is refused, since the note is the only record of why a set was right | Locked | `SHIPPED.md` | Carried | — |
| 13 | Add a mapping | Not drawn | `AddMappingDialog`; field is a text box prefilled `group`, because the add-on's schema is the authority on which fields exist | Behaviour Locked, presentation Reconcile | `SHIPPED.md` | **SYNTHESIS** | route-system-targets-id-mappings |
| 14 | Requests queue | Renders everything | Rendering capped to a page; selection still spans the whole queue | Keep | `SHIPPED.md` | Carried | — |
| 15 | Board colour values | Hex on the board | Never pasted — the build refuses a hex outside the token file | Locked | `BUILD-NOTES.md` §1 | Carried | — |
| 16 | Board statements of system fact | e.g. §23's reconciliation direction | Where `BUILD-NOTES.md` corrects what a board says the system does, the correction stands | Locked | `BUILD-NOTES.md` §2–§3 | Carried | — |

### Shell and navigation

| # | Board | Shipped | Reason recorded | Class | Source | Verdict · Oct 2026 | Boards |
|---|---|---|---|---|---|---|---|
| 17 | README §19 / M18: the member storage row is named **after the add-on** ("TrueNAS"), route `/storage/[target]`, "so a second storage add-on adds a row" | Member row is **"Network storage"** at `/storage`, named for what it does. Ungated, as drawn | Only the reason for leaving it ungated. Nothing on the name, which directly contradicts the board's stated rule | Reconcile | `25bcb40`, `82e4837`, `f4e7299` | **KEEP-CODE** | shell-account |
| 18 | Not drawn. Board header: view pill top-right "beside the account"; on desktop the account shows as a name/avatar (sign-out not drawn) | An **account sheet** at every width: who you are, appearance (theme), sign-out below a hairline, and the **"Install" (PWA) offer**. Header cut to two controls; theme toggle's `title` tooltip removed | "A standing sign-out on a phone sits one mis-tap from a destination"; install offer goes in the sheet, not a first-load banner, because "a banner … gets dismissed by reflex" | Reconcile | `db5701a`, `65a2fc6` | **KEEP-CODE** | shell-account |
| 19 | M01: Basic tab bar = "Today, People, Requests, **Roles**". Advanced = 8 destinations incl. "History" | Tabs come from the rail tree: Basic = Home, People, **Access** (opens its first child, Projects), Requests. A group becomes one tab and keeps the group's name. No More tab | "Renaming the tab after the child would make 'Access' unreachable, and that word is the only thing explaining why Projects, Roles and Apps sit together" | Keep | `3686730` | Carried | — |
| 20 | MOBILE breakpoints: tablet (720–1080) has a **64px rail, icons and badges only, no labels** | Tablet keeps the **full labelled rail** | "This product has no icons … 64px would be fourteen identical dots"; "a label needing a hover is a label nobody can act on" | Keep | `2245c1f` | Carried | — |
| 21 | Board draws the view switch at **34px** | 44px up to the desktop breakpoint. On a phone in Advanced it sits at the top of the nav sheet only, not also in the header | "The one place a drawn value could not be followed": below desktop it is a touch device, and the board's own floor is 44px | Locked | `db5701a` | Carried | — |
| 22 | refines recorded Unexplained access→Drift (nav label only) | Page title and Home tile also say **Drift**. One name for one place | "one destination, three names, which is the exact defect the rename existed to remove" | Keep | `63f7109` | Carried | — |
| 23 | §19: one row per *registered* add-on. Zero add-ons never drawn | **Connected systems** is a fixed System row on every operator deployment. `/system/targets` lists what is registered, or says "No system is connected." and what would connect one. Not registered, no manifest and bad secret are reported as three separate states | With no add-on, the rail looked like the platform "did not ship". "Seat held by configuration rather than by data". BOARD-AUDIT lists this as a deliberate difference | Keep | `5072ea0`, `060a748` | Carried | — |

### Home

| # | Board | Shipped | Reason recorded | Class | Source | Verdict · Oct 2026 | Boards |
|---|---|---|---|---|---|---|---|
| 24 | §04: "Every block is something you can finish. **No counts you can't act on**." When everything is clear, "says so in one line and gets out of the way" + "Go to People →" | An empty queue collapses to one line, then a permanent second zone, **"The makerspace"**: lifecycle gaps, machine health, where access lives, recent activity. Every number links; no charts | "An operator landing on a hero-sized 'Nothing needs you.' learned nothing … so they went hunting through the nav" | Reconcile | `1b3623d` | **SYNTHESIS** | route-home |
| 25 | §04 Advanced appends only Pending changes + Unexplained access | Blocks also count **targets nobody has been able to read** and **disputed target values**, so "Nothing needs you" can't show while either is outstanding | "'Nothing needs you' — the one sentence that must never be said about a system nobody has been able to look at" | Locked | `0fc28c2`, `a1261b2` | Carried | — |
| 26 | §04 Advanced: two extra blocks (Pending changes, Unexplained access) | Advanced Home adds a **Merge findings** block that links to each registered target | The headline already counted merge_findings ("6 things need you", but only five blocks). Findings are kept per target, so it links to targets and does not invent a queue | Keep | `28b0a3f`, `519aaad` | Carried | — |
| 27 | B15 (`mobile/CLAUDE-DESIGN-REPLY.md:358`): all six blocks keep their seats at zero, each with a hollow zero | Blocks with nothing in them **do not** keep a seat. A failed load says "Couldn't check." and never "Nothing needs you". No reassuring empty state while loading | "Deliberate deviation from B15… six hollow cards would be a page of empty furniture" | Reconcile (deliberate, reason given, not owner-ruled) | `28b0a3f`, `c7527e8` | **KEEP-CODE + one addition (departs from my own B15 reply, re-decided)** | route-home |
| 28 | not drawn | New block for changes **sent but not seen back** (accepted by Zitadel, not observed after 20 min). It says what this is NOT: not failed and not still waiting to be sent | `applied` only means Zitadel received it. A read-back is what confirms it | Locked | `981f4ba`, `d86641f` | Carried | — |
| 29 | not drawn | **New people without a bundle** block (joined and got nothing). A missing default bundle counts in the headline but not as a person in this block's badge | Auto-onboarding had never once worked. Only state can answer "who joined and got nothing" | Locked | `bf5d7bb`, `f3d466f` | Carried | — |
| 30 | not drawn (unvouched-targets block came earlier) | Each unvouched target's reason is written as a sentence. Before, the raw `target_unreachable` code was shown | "the next move differs by reason" | Keep | `2018eca` | Carried | — |
| 31 | ia §04 greeting: "Four things need you." | Greeting reads "N things need you" with "here" removed. The headline and the nav badge each say what they count. "Zitadel" and "Waiting to be sent" tiles now go to the right pages | The two totals named one thing and disagreed. The tile links were left over from the rename | Keep | `f3d466f`, `e504187` | Carried | — |

### People

| # | Board | Shipped | Reason recorded | Class | Source | Verdict · Oct 2026 | Boards |
|---|---|---|---|---|---|---|---|
| 32 | README decision #1 "**No bulk revoke**". §29: dormant is "**the one bulk action in the product** … Do not read it as permission to add bulk actions elsewhere." §05 has only one filter ("Any project") | People has a **bulk mode** (a toggle, off by default) that can assign or remove a role and assign or remove a bundle. Each opens a rehearsal. Select-all covers everything the filter matches. Filters live in the URL, and two cohorts are added: **"no access at all"** and **"departed but still holding roles"**. Role detail gains **"Add people to this role"**, which opens People in bulk mode | "Assigning a role to six new members meant six trips"; "There is no path from a selection to a write" | **Reconcile ⚠** — reverses a board *safety* decision; needs the owner's explicit yes | `68b9b8e`, `7e1e195`, `a554ca1` | **SYNTHESIS (32a)** | route-users |
| 33 | M25 / MOBILE state table: `selectionMode` is "**M25 only**". Dormant has a named Select in the header, the title becomes the count, and the header's left control becomes select-all. §14 drew bulk adopt / mark-external only | One selection model on **five lists** (People, drift, requests, expiring, automatic rules) plus dormant. Shift-click ranges, drag-paint, Space and Shift+Arrow. The bar docks at the bottom. **"Select similar"** on drift, and the bar says what the selection is made of. Selection is entered through a named **Select**. Select-all is **a row in the list** showing both counts. Over 500 is a sentence and an offer. Bar verbs name the rehearsal ("Remove role" opens a review). Acknowledged rows show dashed and can't be ticked | "No amount of shift-clicking finds a cluster as reliably as asking for one". Select-all moved because the column header "does not exist below the tablet breakpoint — phones simply had no select-all" | Reconcile | `6295dd2`, `3635012`, `10ddc67`, `bdb8d4d`, `4095d1d` | **KEEP-CODE** | route-users |
| 34 | §05: bundle chips "state membership and **nothing else**" | Bundle chips show the person's **pinned version**. The People filter narrows by bundle, then by version | Bundle versioning: it's how you "find the eleven people a publish left behind" | Reconcile | `db5befc` | **KEEP-CODE** | route-users, route-users-id |
| 35 | §11: "Ids resolve asynchronously — **never flash a raw UUID**" | An id nothing can name shows **"Unknown account"** with the raw id visible beside it, never as the label (first only in a `title`) | "The id is the forensic half of that row"; hover doesn't exist on touch | Keep | `16a049f`, `adc4717` | Carried | — |
| 36 | ia §05: header meta "248 accounts · …" | Says **people**, not accounts | The copy guide bans the second name ("counting its own rows as accounts under a title that calls them people") | Owner-ruled (copy guide) | `388acbf` | Carried | — |

### Person access

| # | Board | Shipped | Reason recorded | Class | Source | Verdict · Oct 2026 | Boards |
|---|---|---|---|---|---|---|---|
| 37 | §05 draws tabs Access / Requests / Activity / Bundles for everyone | Requests = full history with inline approve/deny. **Activity is operator-only**, shown "by"/"to", grouped by day, and says when it hits the 200-row cap | /audit is operator-only, so for a member Activity would be "controls whose only possible outcome is a 403" | Locked | `7e1e195` | Carried | — |
| 38 | ia E3: role groups under **GRANTED** / AUTOMATIC. No delivery state | Each role row gives a verdict from Zitadel: the access, "Waiting to be sent", "Not in Zitadel" or "Could not check Zitadel". The project heading carries "In Zitadel · read HH:MM" once, in Basic as well as Advanced. "Granted" became **"Given"**. The grant id is now a suffix, not a label. The grantor shows as a name | "the row answers from Zitadel, not from Syndra's records". A gated Basic view showed the false row with nothing checked | Locked | `cd40fc6`, `c7527e8`, `1f7495b`, `d86641f` | Carried | — |
| 39 | not drawn | A project Zitadel holds that no record explains shows its roles, says nothing explains them, and links to Drift (it used to read "0 roles"). The recorded and unexplained counts are never added together | "the one sentence that is flatly untrue about access somebody can use right now" | Locked | `1f7495b` | Carried | — |
| 40 | §27/§31: removing access is a red, rung-gated confirm | If the delivery is still queued, removing a bundle or grant **withdraws** it: the label says withdraw, there is no red, the pending add is cancelled and no revoke is queued. Queued sources are marked "recorded, not delivered". The "Queues for confirmation" status that looked like a button is gone | "recorded is not delivered, and a revoke is not a cancellation". Six operations netting to nothing | Locked | `06e213c`, `1ee95ea`, `2782ef3` | Carried | — |
| 41 | ia E3: tabs Access / Requests / Activity (contents not detailed) | Activity uses passive voice for things done to the person ("Given a bundle — by Sai Khurana"). It uses the shared event sentence when the person acted | The page is already about them, so naming them twice would bury who did it | Keep | `f09ba7c` | Carried | — |

### Roles, projects, apps, bundles

| # | Board | Shipped | Reason recorded | Class | Source | Verdict · Oct 2026 | Boards |
|---|---|---|---|---|---|---|---|
| 42 | §07: "A project is a boundary … an app is a thing that receives a token. **They are not one-to-one**". Board shows Badge Reader reading four projects | **One app in one project**, matching Zitadel and the schema's UNIQUE constraint | "C7 — settled … The design diagram was the thing that was wrong" | Locked | `27412a8` | Carried | — |
| 43 | §16: editing a bundle's **contents** reaches every holder now ("Removing Laser Lab / trained affects 8 people now", rung-2 "I understand"); impact panel above the fold | **Bundle versioning.** Edits go to a working copy that **reaches nobody**. **Publish** is the step with consequences: it asks whether current holders come along, is rehearsed, and holds the destructive confirm. Removal copy is conditional and **amber, not red**. Also: a diff strip, version history with per-version holder counts and **move holders**, an "unpublished changes / stale holders" note on the list, and a searchable multi-select add-roles picker. Adding roles says **"No change"** to access | "The edit and its consequence were the same click, and the only way to be careful was not to edit"; old copy "became a lie the moment it stopped" | Locked (model) / Reconcile (version screens never drawn) | `db5befc`, `6ce7016`, `87c6bd6` | **KEEP-CODE (model Locked; the screen that was never drawn is now drawn)** | route-bundles |
| 44 | Not drawn: §16 and §17 show no way to delete a bundle or a rule | A mapping rule and a bundle can be **retired**. Deleting either reports **Queued** unless the rule fired unattended. Deleting the welcome bundle folds its "new members receive nothing" warning into the same report | "Nothing MkAuth creates is permanent any more"; "'applied' about a door that still opens is the one sentence this product exists to avoid" | Keep | `27412a8`, `2f394fb` | Carried | — |
| 45 | M08: Roles / Bundles / Apps become a **three-way segment** on phone | Roles stacks with the **project as an eyebrow** (it never collapses), group folds away, the count reads "N members". **No segment built** (none at 4095d1d) | Project eyebrow comes from README decision #9; "a bare number in a stacked row is a number with no idea what it is". No reason given for skipping the segment | Keep (stack) / Reconcile (segment missing) | `718349c` | **KEEP-CODE** | route-roles |
| 46 | §07 Projects row shows a bare Roles count | A project with no roles **says so in words** instead of "0" | "Nothing in that project can be granted to anybody, and a bare zero … reads as a project that is merely quiet" | Keep | `dd5bd43` | Carried | — |
| 47 | ia /roles: "Members" column. ia E1: projects index | Holder numbers are what **Zitadel shows**, with a note (unexplained, not in Zitadel yet, confirmed) and how fresh the read is. The column header reads "Holders". The column went "Members"→"Direct"→observed along the way. "Unused" requires a complete, current read **Code fix owed:** the header word *Holders* breaks the copy guide's ban on "holder" as a noun (`plain-language-copy/design.md:215`). | "holding is observed, explanation is recorded". Six screens disagreed about the same number | Locked | `8328b0b`, `d7f23cd`, `0bc2ac3`, `c7527e8`, `9625207` | **Locked, code fix owed: the column heading reads "People", not "Holders" (CLAUDE.md: say people).** | — |
| 48 | ia E1 projects table | "No roles yet — nothing here can be granted" sits beside the project name, not in the 60px count column. The role count means roles that **exist** (from three sources) on both the list and the detail page | It wrapped to six lines. The two pages counted different things under one word | Keep | `5072ea0`, `0bc2ac3` | Carried | — |
| 49 | ia E6 token simulator: `//` comments explain the silences | "Would issue nothing for this person" is a full sentence, not a faint comment. While the shape editor has unsaved edits, a dashed rule says "Behind your edits. Save the shape to preview it…" | The preview reads the *saved* shape from the one shaper, so it cannot show a draft. An empty result looked like a broken preview | Locked | `740c1bc`, `83623d9` | Carried | — |
| 50 | ia Apps: claim name per project | The built-in default claim name is not shown as a configured value | The backend cannot tell "set to roles" from "never touched" | Locked | `e504187` | Carried | — |

### Requests

| # | Board | Shipped | Reason recorded | Class | Source | Verdict · Oct 2026 | Boards |
|---|---|---|---|---|---|---|---|
| 51 | §10 member side: Waiting / Granted / Declined only; no way to withdraw | A member can **withdraw** their own request; the row reads **"You withdrew this"**, never "Denied" | "Withdrawn and declined are different facts … and the member is the one who most needs them kept apart" | Reconcile | `27412a8`, `8165778` | **KEEP-CODE** | route-requests |
| 52 | §10 member form: "asks in verbs", one free-text note, Send | A member **catalogue** on My access with "Ask for this" links that prefill the request; a **how-long picker** (term or days, with the resolved date shown). The operator side shows the requested duration | A6: "duration_days travelled end to end and NO operator surface displayed it"; A7: the member catalogue is projects and roles | Reconcile | `44c0ce0` | **KEEP-CODE, with the catalogue drawn for the first time. Owner, 10 Oct: 52b (rooms fold, one search) and 52c (recent changes first)** | route-requests, route-me |
| 53 | Design (per commit): a free-text escape when the wanted thing is not listed | No free-text request. The escape points at the **Why** field. A project with nothing defined in it shows a sentence, not an empty menu | "A request with no project and no role is not something this API can carry" | Locked | `740c1bc` | Carried | — |

### Review and governance

| # | Board | Shipped | Reason recorded | Class | Source | Verdict · Oct 2026 | Boards |
|---|---|---|---|---|---|---|---|
| 54 | §15 Expiring: "**one action only** … no 'let it lapse'". Flagged a future "acknowledged state with its own column and endpoint", never faked by hiding rows | **"Leave to lapse"** acknowledgement, per row and never bulk, with its own endpoint. It reopens when the grant's date changes. Acknowledged rows sit under a **counted heading** and keep Extend but lose the checkbox. **Bulk extend** works on the ticked grants. Phone: no "granted by", "5 days left", "expires today" | C4: "'nobody has looked at this row' and 'somebody looked and decided' were indistinguishable"; built the way the board's flag asked | Keep | `4c29188`, `ddc2857`, `0b32701` | Carried | — |
| 55 | §14 drift row: who / what / why / found, three resolutions, "Role not in catalogue" badge. "A write that never landed" is only a §15 Reconciliation sentence with Re-push | `syndra_only` rows carry **provenance**: who granted it and why, when Zitadel last held it, and "Syndra applied this at 12:04 … somebody removed it". Such grants are triaged rather than replayed. A **target nobody could read** shows above the queue with its age and reason. The catalogue badge is dropped where no catalogue applies (non-Zitadel targets). Filters are added (no user filter). "Compare again" reports **No change** | "Every one of those changes what they should do about it"; "a week of blindness and a week of good behaviour render identically"; a scan "applies nothing" | Locked | `251ddd9`, `0120463`, `1ab57df`, `0fc28c2`, `778a098`, `44c0ce0`, `0b32701` | Carried | — |
| 56 | §29: the acknowledgement names the data: "*I understand 41.2 GB of their files goes with the accounts*" | The sentence says the files go and that **Syndra cannot see how much**. An unusable read is **refused** rather than shown as an empty list | `bytes_held` "is a pointer and nothing fills it yet … a zero computed from a read that never happened" would be untrue | Locked | `3ef0246`, `9aee6f4` | Carried | — |
| 57 | §31 B ladder: adoption is on no rung. Red is for destructive actions only | Adopting an unmanaged target account is **rung 3**: type the account name to arm the solid red fill | "The one action that hands one person's data to another with no undo had a free-text field and a plain button" (target page; may overlap SHIPPED T-regions) | Reconcile (unsure) | `2404698` | **SYNTHESIS** | route-system-targets-id |
| 58 | ia S6 drift triage. No attribution affordance drawn | Each row gets an on-demand **"Who made it?"**, one row at a time, read from Zitadel's event log. Three distinct answers: lookup failed, too old, names nobody | Doing it for every row on load would be a burst against the identity provider | Locked | `32d2a8b` | Carried | — |
| 59 | not drawn | A drift row explains that the scheduled check cannot see who made the change. A separate sentence says "an event was probably missed" when the log reaches back far enough | Code comment: "Two separate facts, said separately (owner's requirement)" | Owner-ruled | `75c111a` | Carried | — |
| 60 | ia S6: two tabs (Triage queue / Reconciliation), no filter set drawn | Drift filters live in the URL (shareable). Adds person, role, age and **origin**. All sit in one panel behind a "Filters · N" button with a sticky Clear/Done footer. Each option list ignores its own filter | Six header controls wrapped into five rows and pushed the queue below the fold. Shareable links. Picking a person emptied the person control | Keep | `f3d466f`, `6efe798`, `9ed453c`, `a1c0bc7` | Carried | — |
| 61 | not drawn | The drift count bar says revoking is **one row at a time** | Leaving it out "reads as a broken bar to anyone who selected nine rows" | Locked | `2b11eba` | Carried | — |
| 62 | §22 / M21: two populations, never one count | Both buckets keep their seat at every count. An empty one says what that means ("nothing has given up") | They only rendered when non-empty, so a red card jumped in above the queue being read | Keep (refines §22) | `a297bf3` | Carried | — |
| 63 | §25 Holds: lifting a hold | Copy says the block is cleared here and access returns **at the next pass**. It no longer says "gives the access back straight away" | The handler updates one column and queues no convergence | Locked | `5ff419c` | Carried | — |

### Automation

| # | Board | Shipped | Reason recorded | Class | Source | Verdict · Oct 2026 | Boards |
|---|---|---|---|---|---|---|---|
| 64 | §12 Access map: "Find a node…" search; pick one node and draw its neighbourhood. "Show" legend lists kinds | Opens on a **browsable root**: every node grouped by kind, ordered by how connected it is. A focused node has a **breadcrumb back** to the root. Search becomes a shortcut. The Show legend becomes a filter that tracks *hidden* kinds | "A map you can enter but not leave answers questions you did not need it for" | Reconcile | `ff82400` | **SUPERSEDED by N12 (Insights)** | route-graph |
| 65 | not drawn | The rule editor names the **target** role for its holder count, and says what the "nobody holds it yet" sentence left uncounted | False reassurance in front of a write that would reach 40 people | Locked | `8328b0b`, `d7f23cd` | Carried | — |
| 66 | S3 Pending changes | The handle prefix comes from the source (no longer always "R"). Direct grants show as "Given by hand". Bundles and rules shown by name | A bundle cascade printed a rule handle under the R_ legend | Keep | `d7f23cd`, `d76d6f4`, `c7527e8` | Carried | — |
| 67 | S4 Change history | `/operations/cascades` says what it shows: direct grants left out, capped at 50 | Before, it read as "almost nothing happened" | Locked | `e504187` | Carried | — |
| 68 | §08 E4 / §16 S1 bundles | **Create bundle asks for its roles**, which become v1. The assign panel previews the **published** version and says when unpublished edits are not included | Assignment pins the published version. The panel previewed the working copy and then granted a different set | Locked | `3eb7c3f` | Carried | — |
| 69 | Rehearsal dialog (M3 family) | A publish or save that moves nobody can be applied. The result reads "Done. Nobody's access changed, because nobody holds it yet." The result step names the version that was actually published. The empty-plan sentence names the right noun for each op | "Applied to 0 people" read as a failure. Apply was disabled on the only screen that can cut a version | Locked (refines M3) | `2fb1f89`, `b24f6a3`, `2f58676`, `3a983b1` | Carried | — |
| 70 | copy everywhere | "within about a minute" is gone. Each site says its own timing: revocations send on their own every few minutes, drift runs on a cadence, grants wait for Send | "THE INVENTED MINUTE… wrong in all of them, against three different mechanisms" | Locked | `5ff419c`, `2782ef3` | Carried | — |

### Audit and system

| # | Board | Shipped | Reason recorded | Class | Source | Verdict · Oct 2026 | Boards |
|---|---|---|---|---|---|---|---|
| 71 | §18 IdP: "Action signing key · Rotates in 9 days · Last rotated …"; no action drawn | The key's age against the threshold, as a sentence, plus a **CommandBlock**: the rotate command (or **register** when no key exists), the two env vars, restart, verify, and what happens during the swap. Deliberately **not a button** | "Each has a failure mode where the console reports success and the system sits in the half-state" | Locked | `208d8b0` | Carried | — |
| 72 | §18 IdP: "Read-mostly, with **direct-write escape hatches**" | The upstream write half sits **behind a disclosure** naming three consequences, with the warning **repeated beside each control**. Each write's result says Syndra **"has no record of the change beyond the audit entry"** | "A warning three scroll-lengths above its button is a warning nobody reads"; "these writes go straight to Zitadel with no plan, no queue and no ledger row" | Locked | `2b4ff47`, `f839708` | Carried | — |
| 73 | §18 Event activity: "All sources ⌄ / Today ⌄" filters only | Adds an **outcome filter**: Done / Waiting / Failed / Not acted on, spanning both tables. An unknown status shows as itself | A3: the backend `?status=` "can express neither 'either table' nor a bucket"; "a new backend status can never quietly read as a success" | Keep | `44c0ce0` | Carried | — |
| 74 | Not drawn: no notification sound anywhere on the board | The drift **chime** toggle says when it is **silenced by reduced motion** (on but silent). Also stopped chiming on every page load | "The chime toggle could say 'Sound on' while guaranteeing silence"; "an audible alert on every load" | Reconcile | `dd5bd43`, `fe26d79` | **SYNTHESIS** | route-automation-settings |
| 75 | §18 S9 Identity provider: health plus raw state, mid-fi, no ceremony | The four Zitadel consoles need a tick: "I understand this writes to Zitadel now, with no plan and no record in Syndra". There is also a standing line, "Changes here go straight to Zitadel." Later each surface's warning says its own truth: grant edits are now recorded, project and role edits are not. "Syndra itself is fine" appears only when it is true. The upstream role dialog stays open and offers Done | "the least undoable writes in the product had no rung". A false warning teaches distrust of the path that is recorded | Locked | `a7fb296`, `5ff419c`, `e504187`, `e949b9c` | Carried | — |
| 76 | refines recorded "Incoming events" rename | The lede names both streams (Zitadel webhooks and onboarding triggers) | "Zitadel events" was wrong for manual and system triggers | Keep | `e305887` | Carried | — |
| 77 | not drawn | Incoming events records "no welcome bundle configured" as its own outcome, not `failed` | Five clean rows were sitting in the same bucket as a DB fault | Locked | `bf5d7bb` | Carried | — |
| 78 | §18 Audit: every line names a human or a machine | One renderer for events across Lately, Audit and person Activity. The thing acted on is named inside the sentence. Bundle targets are named, never "Unknown account <uuid>" | The same audit row rendered two ways. The target is sometimes a bundle | Keep | `1f7495b`, `f09ba7c`, `c7527e8` | Carried | — |

### Member pages

| # | Board | Shipped | Reason recorded | Class | Source | Verdict · Oct 2026 | Boards |
|---|---|---|---|---|---|---|---|
| 79 | §20: three states (no entitlement / recorded, not created / account present with password form) | Extra states: **"ready but not switched on"** (the target has password login off, with a pointer to the form; a held account gets a different sentence); **usage**, where no quota reads **"no limit set"**, never a full bar; and a re-enrol notice, **"you enrolled before the change"**, per target | "Showed an account name, working mount instructions … to somebody whose every connection attempt was refused"; hashes can't be converted, and "'You enrolled before the change' is a different sentence from 'you have never set one'" | Reconcile | `fc45d55`, `a138e89`, `44edf07` | **KEEP-CODE** | route-storage |
| 80 | §30: connection instructions on the account-present state; host comes from the add-on's registration | When the manifest has no host, the instructions are **omitted** rather than printed with a guessed path | "Unset, the page omits the instructions rather than printing a path that fails" | Locked | `3ef0246` | **KEEP-CODE** | route-storage |
| 81 | Design (per commit): an empty member asks a named person | "Ask for access", with no person named | No project records an owner, so no one can be named | Locked | `740c1bc` | Carried | — |
| 82 | ia My access: sentences per source ("Two memberships and six roles…") | Each role is marked **Ready / Not there yet / Couldn't check**. The headline counts what Zitadel holds. Access nobody recorded a reason for shows on the member's own page. A zero shows only after a complete read | "two permissions" sat above two rows that both said the role had not arrived | Locked | `c7527e8`, `f108635`, `9625207` | **SYNTHESIS (decided on route-me, 10 Oct; recorded here)** | route-me |
| 83 | refines C1–C3 (member's pause) | The password button no longer promises "save it for when changes resume". A paused target refuses the change, and the member is told to wait | `draining`/`read_only` refuse every mutation and nothing is replayed. "Somebody told to try is worse off" | Locked (refines C1–C3) | `b2f8e07` | Carried | — |

### Mobile

| # | Board | Shipped | Reason recorded | Class | Source | Verdict · Oct 2026 | Boards |
|---|---|---|---|---|---|---|---|
| 84 | M09/M22: picker sheet stops 96px short; rehearsal goes full height with a sticky footer. Grabber drawn; busy state not drawn | Three sizes: `sm` takes its content's height, `md` stops 96px short, `lg` is full height. A **busy sheet swaps its grabber** for "Working — this can't be closed yet." The grabber's name is **"Dismiss"** | "Silently ignoring a drag reads as a frozen application"; "Close" collided with the done step's own control | Keep | `d1f29e7` | **KEEP-CODE** | route-mobile, route-automation-settings-v5 |
| 85 | MOBILE "Tap to copy": a 52px row, "Copied" lime for 900ms | On an insecure origin (http on the LAN) the control reads **"Select"**: a tap selects the value, with one note saying the browser is the limit. The token preview shows its payload as selectable text | "On the network most members actually use, every Copy button … did nothing" | Locked | `56cad27`, `87c6bd6` | Carried | — |
| 86 | §31 A freshness: live = lime dot, ageing neutral, stale amber, provisional amber | **A dot for every reading**; provisional is a **ring**, not a fill | "'No dot' meant both ageing and stale — the two readings whose difference decides whether an action is blocked" | Reconcile (unsure) | `729865e` | **SYNTHESIS (86c)** | route-mobile |
| 87 | M05/A2: rows disclose by default. Search is a full-screen overlay; filters sit in a sheet with "Show N people" | People **stacks rather than disclosing** (a tap opens the person); team and bundles fold away below tablet. **No search overlay or filter sheet built** (none at 4095d1d) | "A person row's primary action is opening that person, so putting the row's own tap on a disclosure would bury the thing it is for". Nothing said about the overlay | Keep (stack) / Reconcile (overlay, sheet missing) | `2834c57` | **KEEP-CODE (stack)** | route-mobile |
| 88 | M14 drift on phone: **two lines**, the difference as a sentence with both sides bold; drift kind as a **word**, not a coloured badge | Unexplained access "**keeps everything**": the explanation takes full width, "Found" gains its label, and the three answers sit 12px apart | "On the highest-stakes screen in the product there is nothing to fold away" | Reconcile (unsure) | `ddc2857` | **KEEP-CODE** | route-drift |
| 89 | Dense lists reflow by MOBILE rules; no per-screen choices drawn for rules, projects, apps, role members, pending, events, audit | Each list keeps what it's scanned for. Numbers carry their own noun ("12 holders"). A rule's sentence gets full width. Request justifications wrap in full, with Approve as the wide target and Deny 12px away. Audit rows stack | "A number in a stacked row has no column heading above it any more, so it carries its own noun" | Keep | `718349c`, `ad3840a`, `adc4717`, `60bf702` | Carried | — |
| 90 | M10 token debug: "**Explanation first, evidence second**" on phone | The app token screen and both Zitadel consoles stack. The token screen puts the **editor first** | "That screen is composing rather than answering a question" (it may be a different screen from M10's E6 simulator) | Reconcile (unsure) | `d6a6ab9` | **KEEP-CODE** | route-applications-id |
| 91 | Not drawn | **Landscape type-the-name gate refuses**: with ~190px above the keyboard, the field isn't rendered at all | "The consequence is the most protected element in the ladder and may not be the thing that scrolls away" | Locked | `4901e7e` | Carried | — |
| 92 | not drawn | A 180×180 `apple-icon.png` was added for the installed iOS app | iOS ignores manifest icons. The app launched with a blank tile | Keep | `d122730` | Carried | — |

### Login

| # | Board | Shipped | Reason recorded | Class | Source | Verdict · Oct 2026 | Boards |
|---|---|---|---|---|---|---|---|
| 93 | LOGIN / M28: one failure state, `unreachable`: "Zitadel didn't answer." | The ten `?error=` codes map to **three** messages: "Zitadel didn't let you through." (refused), "The sign-in didn't complete." (handshake), "Zitadel didn't answer." The code is never shown | "Flattening all of them to 'Zitadel didn't answer' would tell a member the provider was silent when it had in fact refused them by name" | Locked | `ec5134d` | Carried | — |
| 94 | M32 covers deep links and back; return-after-sign-in not drawn. Session expiry mid-action not drawn | Signing in **returns you to where you were**: the path goes through the PKCE cookie, not `state`. A **401 during a write is reported in place** ("nothing was changed, and what you were doing is still here") with no redirect. Reads still redirect | "Redirecting at that moment destroys the dialog, the plan they approved and any reason they typed" | Locked | `4901e7e` | Carried | — |
| 95 | LOGIN.md: error sub-lines end "…or find a steward in the space." | The login door keeps its pre-sweep wording. Login error lines say "**ask makerspace staff**" | Owner: the door is authored copy and is not swept. Errors follow the "members ask makerspace staff" decision | Owner-ruled | `c1249ce` | Carried | — |

### Cross-cutting — states, copy, visual language

| # | Board | Shipped | Reason recorded | Class | Source | Verdict · Oct 2026 | Boards |
|---|---|---|---|---|---|---|---|
| 96 | MOBILE forbids toasts and has the row confirm in place; desktop board silent. §23: "After apply, **the word *done* never appears** and neither does a tick" | **Toasts removed everywhere** (desktop too, enforced by a canary). A six-word **ActionOutcome** reports where the action happened. **Queued** is amber, present tense. Refused and Failed are kept apart; Failed shows a labelled request-id copy row. Dialogs **stay open on success**, and the primary control becomes **"Done"**. A row keeps its seat until the next read | "A result never travels"; queued must never wear applied's tone ("would report a door as locked while it still opens"). Note: the "Done" button sits awkwardly with §23's no-"done" rule | Reconcile — the behaviour is Locked (a result never travels; queued never wears applied's tone); the word *Done* conflicts with §23 | `942ff27`, `a0ceb42`, `527c8a6`, `60bf702`, `8165778`, `0b32701`, `87c6bd6`, `df22a2e`, `f839708`, `2f394fb`, `fc5f3ab` | **SYNTHESIS (96b)** | route-states |
| 97 | §11 / README: one Degraded banner ("every number is invented") | A second banner: **demo data seeded under a live directory**. It keys off seed *residue*, gives the count, and carries the reset command (dry-run stated). Kept apart from Degraded; Degraded wins when both apply | "They are different lies: degraded means every number on screen is fiction, this means most of them are real and some are not" | Locked | `d205496`, `cf09d1b` | **SYNTHESIS (Notice, 6a)** | route-states |
| 98 | Not drawn: no offline state | An **offline banner**, neutral, not amber, above Degraded. Writes are **refused before the wire**, nothing is queued on the client, and the copy says "nothing was queued and nothing was sent" | "Degraded means the API answered and answered badly … Offline means nothing answered: what is on screen is true" | Locked (banner visual not drawn) | `e0576ef` | **SYNTHESIS (Notice, 6a)** | route-states |
| 99 | §31 A: "Truncated is not empty". No "could not read" variant drawn for holds or carve-outs | A failed carve-out read is said on the page (`withheld_unavailable`), not rendered as "nobody has one". The same rule appears on drift and dormant (rows above) | "A zero count with no rows is byte-identical to a clean cohort" | Locked | `9a663dc` | Carried | — |
| 100 | §23 / BUILD-NOTES §2: plan, then apply in two steps; a stale plan names who moved | The shared **RehearsalDialog** runs compose → **scope** (a blast-radius refusal becomes its own step) → review → result. **Any** refusal re-plans and raises a per-code banner (role="alert"). Cancel and Back are disabled while a write is out. Rows that **ran out of retries** and **holds in force on the person** are shown. Already in BUILD-NOTES §2; listed only for its extra states | "An operator could then press Apply believing they had already read this diff"; "abandoning the dialog does not abandon the write" | Locked | `a554ca1`, `3e85704`, `f8ce0fc`, `46e7095`, `df22a2e` | Carried | — |
| 101 | §11 list error state: names the failed thing, "nothing was changed", a **Try again** button and a request id | New app-level boundaries (`error.tsx`, `global-error.tsx`, `not-found.tsx`): "nothing was changed", the digest as a copy row, **no Try again**, and a full document reload as the way out. The 404 says one sentence ("nothing at this address") | A render that threw throws again on the same data. A soft navigation can arrive back at a broken layout | Locked | `2b11eba`, `3da1fc4` | **SYNTHESIS** | route-states |
| 102 | §11 error copy | Errors the code catches go through describeFailure ("You don't have permission to do this", "That item no longer exists"). Raw messages are banned except in MergeFindings | Status lines and stack traces were reaching people giving access | Keep | `388acbf` | Carried | — |
| 103 | not drawn | Bulk selection bars state the **500 ceiling** before the tap and offer a narrowing move (Requests, Drift, People, Expiring access). Expiring access counts people, not grants. Select-all says "Clear the selection" without the second clause | `services.BulkMaxUsers` caps three endpoints. Without this, people met a 4xx after tapping | Locked | `c76a92f`, `8f1b32d`, `05efa8c`, `1c1a85c` | Carried | — |
| 104 | not drawn | Dialogs **become their result**: the destructive or primary control retires after success, "Keep it"/Cancel becomes **Done**, and the outcome stays on screen. Cancel gets initial focus | Live red Revoke buttons stayed under "Resolved", so a second click revoked twice. Callbacks were taking unread outcomes with them | Locked | `e949b9c`, `02eb1a1`, `1ee95ea`, `b2f8e07`, `c7527e8` | Carried | — |
| 105 | Board vocabulary (Rehearse, Identity provider, steward, glosses) | Plain-language guide: every page gets a **lede**. Preview → Apply. "revoke". Members ask **makerspace staff**. "Zitadel" stays Zitadel. The bans on "holder" and "Nothing here." are enforced. A guard test fails on violations | `openspec/changes/plain-language-copy/design.md`. "Decisions taken with the owner" | Owner-ruled | `82b386c`, `5ff419c`, `63f7109`, `388acbf` | Carried | — |
| 106 | not drawn | **`<Term>` glossary popovers**: Syndra's own words (cascade, outbox, log anchor, merge finding, baseline) have their definition one hover, tap or Tab away. The inline glosses are gone. The popover renders into the body and flips | The vocabulary was given back to operators who know it. Coined words need a definition on the page that uses them | Reconcile (never drawn) | `e70922c`, `bfd80d2`, `6efe798` | **KEEP-CODE (drawn for the first time)** | route-states |
| 107 | ia type table: micro-label and table header 11.5px, nav group label 10.5px, badge 11.5px (README:907) | Every readable text is at least **12.5px**, including the nav group label and type-label. 14 inline sizes were raised. Only aria-hidden decoration is exempt | MOBILE.md:424 "Never below 12.5px anywhere", applied product-wide. The boards disagree with each other | Reconcile (boards conflict) | `d122730`, `35887aa`, `4a9a067` | **KEEP-CODE** | — |
| 108 | token table (density/type) | The health tile number went 26→24px. A member card title is now a type role, not a raw 22px. Holds spacing set to 18px. Eleven near-duplicate ink opacities collapsed onto the scale | "the product speaking two dialects" | Keep | `388acbf` | Carried | — |
| 109 | README:866 button language: destructive outline in a row | Rule: a row's own action is `outline`, a destructive one `danger`, `ghost` only as the quieter half of a pair. Shared Tabs, Badge and PILL. StatusDot no longer relies on colour alone. The graph search field is no longer a pill | "a borderless control in a table reads as text until it is hovered" | Keep | `ff0d288`, `a297bf3` | Carried | — |
| 110 | ia People header: search plus a filter pill in the header | One **FilterBar** row under the header on every list: search stays visible, two or more filters go behind "Filters · N", list actions trail. A single filter stays on the row (Requests' five-way status, the role page's source pills without the "Filter by source:" label). Page headers hold only page actions | Six screens had six filter layouts. Header filters stacked into tall rows | Reconcile (new pattern, reason given) | `c940f93`, `6efe798` | **KEEP-CODE (settled as CLAUDE.md 9b)** | — |
| 111 | ia "last checked 11:04" (24h) | One date style everywhere ("18 Sept 02:57"). "Last checked" is the query's own time, not the time the screen mounted | Three formats for one event. "12:08 AM" beside "00:08" | Keep | `8f6ce1c`, `f3d466f`, `c7527e8` | Carried | — |

## New differences (found in the October reconcile)

| # | Verdict | What | Area | Boards |
|---|---|---|---|---|
| N1 | **SYNTHESIS** | (BOARD-AUDIT, maintenance buttons) | 1 · Nav, brand and target plane | route-system-targets-id |
| N2 | **SYNTHESIS** | (T5 RegionIndex, shipped touch-only) | 1 · Nav, brand and target plane | route-system-targets-id |
| N3 | **SYNTHESIS** | Owner, 8 Oct: one Zitadel project per system, set before anything else can be done. A project serves one system, and lifecycle is exempt. | 1 · Nav, brand and target plane | route-system-targets-id, route-system-targets-id-mappings, route-systems, explore-system-project |
| N4 | **SYNTHESIS** | Button grammar, made the same on every board | 1 · Nav, brand and target plane | route-system-targets-id, route-system-targets-id-mappings, route-zitadel |
| N5 | **SYNTHESIS** | /storage, the member's page, drawn for the first time on these boards (§20, §30, C1–C3) | 2 · Shell and navigation | route-storage, shell-account |
| N6 | **SYNTHESIS** | Owner, 9 Oct: no password while TrueNAS is paused or offline | 2 · Shell and navigation | route-storage |
| N7 | **SYNTHESIS** | Error and degraded on Home follow the Notice rule | 3 · Home | route-home |
| N8 | **SYNTHESIS** | "Open TrueNAS →" on Home was text styled as a link | 3 · Home | route-home |
| N9 | **SYNTHESIS** | Bundles redesigned (route-bundles-v2; owner picks 9 Oct: 1c, 2a, 3a, 2e) | 6 · Roles, projects, apps, bundles | route-bundles-v2 |
| N10 | **SYNTHESIS** | Owner, 9 Oct: the round seat replaces the checkbox in every Select mode | 6 · Roles, projects, apps, bundles | route-users, route-drift, route-requests, route-expiring, Motion |
| N11 | **SYNTHESIS** | Owner, 10 Oct: "team" is Zitadel user metadata, a free-text label with no access meaning | 8 · Review and governance | route-system-targets-id |
| N12 | **SYNTHESIS** | Owner, 12 Oct: the Access map becomes Insights, with Map and Trends tabs (option B) | 9 · Automation | route-insights, route-graph-v2 |
| N13 | **SYNTHESIS** | Automation settings, drawn for the first time (B4) | 10 · Audit and system | route-automation-settings |
| N14 | **SYNTHESIS (B)** | Owner picked B, 13 Oct · A bundle's confirmation mode is stored and never shown | 10 · Audit and system | route-automation-settings-v4 |
| N15 | **SYNTHESIS (B)** | Owner picked B, 13 Oct · Grants by hand and approved requests ignore the setting, and differ from each other | 10 · Audit and system | route-automation-settings-v4 |
| N16 | **SYNTHESIS** | Owner, 13 Oct: one write place per setting; a step left is said where you took the first one | 10 · Audit and system | route-automation-settings-v5 |
| N17 | **SYNTHESIS** | Owner, 13 Oct: one part per job across the flow | 10 · Audit and system | route-automation-settings-v5 |
| N18 | **SYNTHESIS** | Owner picked 3a and 2c, 13 Oct · The Automation group, the settings page and Insights in the nav | 10 · Audit and system | route-automation-settings-v5 |
| N19 | **SYNTHESIS** | The member's side of a step left (N15, N16) | 11 · Member pages | route-me |
| N20 | **SYNTHESIS (20a)** | Owner picked 20a, 13 Oct · Insights on a phone (new; Phase 2 drew none) | 12 · Mobile | route-mobile |
| N21 | **SYNTHESIS (21b)** | Owner picked 21b, 13 Oct · Signing in again without losing the dialog (#94) | 13 · Login | route-login |
| N22 | **KEEP-CODE** | Owner, 13 Oct: the shipped Syn line stays (#95) | 13 · Login | route-login |
| N23 | **SYNTHESIS** | Owner, 13 Oct: never sample data in place of Zitadel | 14 · Cross-cutting | route-states |

## Read these with the ledger

The reasons above are quoted short. The full reasoning lives in:

- `target-plane/SHIPPED.md`, `target-plane/BUILD-NOTES.md`, `BUILD-NOTES.md`
- `openspec/changes/one-control-surface/BOARD-AUDIT.md` — the add-on screens against the board
- `openspec/changes/plain-language-copy/design.md` — the copy guide; every word decision marked Owner-ruled traces here
- `openspec/changes/mobile-touch-view/design.md`, `openspec/changes/login-doorway/design.md`
- `mobile/CLAUDE-DESIGN-REPLY.md` — your own earlier mobile reply, which row 27 departs from

## Owed by code, not design

These were found during the sweep. They are code defects and need no board change:

- Row 47: the Roles column header says *Holders*, which the copy guide bans as a noun.

## What to return

For every row you touch, and every new difference you find:

```
#n  KEEP-CODE | RESTORE-BOARD | SYNTHESIS | ASK-OWNER
    one sentence: why this is the more elegant, cohesive experience
    which boards changed
```

`ASK-OWNER` is for Owner-ruled rows only. Use only the attached Syndra Design
System; anything it lacks is a proposed component, named as such, not a custom
drawing.
