# A-10 · A person's storage, and where they sign in

Answer to `Q-10-person-storage-and-sign-ins.md`, 3 Nov 2026. Board: `a10-person-storage-and-sign-ins.dc.html` (Final, 3 Nov 2026). Frames are named as on the board (`?frame=`). Nothing here reopens a verdict. It adds no movement and no token. The component it needs (the settings-card row) is already used on several cards.

---

## Part 1 · Network storage on a person's page

## 1 · Placement: a card per connected system on the Access tab, under the projects (chosen)

- **The Access tab, after the last project card.** The page lede is "Everything this person can use". Storage is something they use, and their roles reach it. A tab of its own would split one question, "what can Aanya reach", across two tabs. Activity is history, and this card is the present.
- **A quiet section label first:** "Network storage" (type-label, inset 20 to align with card content, 14 above the first card). It's the member's own word for it, so operator and member name it the same way.
- **One card per connected system.** Today that's TrueNAS. A second system gets a second card in the same section.
- **The read line gains one chip per system:** "TrueNAS ↓", with no count, after the project chips. The page now has more sections, and this is how you reach them. Folded projects don't hide it.
- **Both views.** Basic gets the whole card: "why can't Aanya open her folder" is an everyday question. Advanced adds one thing, each folder's path in faint mono under its name (`main/projects/renders`), opened by Unfold like every Advanced part (A-08 §3).
- **Operators only.** A member reading their own person page doesn't get the card. Their storage is on Network storage, with the password form.

## 2 · The card

**Header.**
- The system's name in `type-card-title`, linking to its page. It's ink, with accent and underline on hover.
- Beside it, the status: dot and words, 13px. These are the operator's words from Connected systems, not the member's:
  - "Answering · read 2 min ago" (healthy dot)
  - "Answering · their account is on hold" (line-strong dot)
  - "Offline since 08:33 · last read 08:31" (warn dot, warn-text)
- Caption, faint 13.5: "What TrueNAS says about Aanya's account. Limits, holds and folders are set on TrueNAS's page; only Aanya sets their password." This is the card's one explanation, with no box.

**Rows.** The setup card's row: label 170 · value · a 200px seat on the right. The seat holds a link or nothing, and never a button. Rows read in the order a person gets to their files:

| Row | Value (ink 15) | Faint line (13) | Seat |
|---|---|---|---|
| Account | `aanya.rao` (mono 14.5) | "Made by Syndra on 3 Sept" | none |
| Password | "Set, last changed 14 Oct" | "Only Aanya sets it, on their Network storage page." | none |
| Home folder | "18.4 GB of 50 GB", 5px bar (max 360) | the limit and where it comes from (below) | "Set on TrueNAS's page →" |
| Folders | one line per folder (below) | none | "Set in TrueNAS's mappings →" |
| Hold | "None" (muted) | none | "Set on TrueNAS's page →" |

- **Every row is always there** while there's an account: Hold reads "None" rather than disappearing. Structure doesn't move with the data.
- **The Password row has no link.** Nobody but the person can set it, so there's nowhere for an operator to go. The faint line says so, which answers "can I reset it for them?"

**Own limit beside the roles' limit.** The limit in force reads first, in ink. The limit it replaces sits under it, faint, in words, never as a second number of equal weight and never struck through. The copy follows `HomeLine`:
- Own: "Up to 50 GB, their own · ends 28 Nov · “Thesis renders for the final review.”" Under it, faint: "In place of the 20 GB their Fabrication lead role gives."
- Roles only: "Up to 20 GB, from their Fabrication lead role" (one faint line).
- Review due: the own line becomes warn-text with a warn dot: "Review was due 1 Nov. It stays at 50 GB until you keep it or end it."
- Own ended with the folder over: HomeLine's amber line as built.
- **Use:**
  - "18.4 GB of 50 GB", with the bar in `--muted`.
  - At 90% or more, the bar turns `--warn` and the line adds "· nearly full" in warn-text.
  - At or over the limit: "Full. They can open and delete files here, but can't save new ones until they're under 20 GB."
  - Use that couldn't be read says so ("TrueNAS couldn't say how much is used just now"). It is never 0.
- No home: "None" with "None of their roles gives a home folder." Home kept after the role went: the use line, then "No role of theirs gives a home folder any more. It stays, with their files, and only they can open it."

**Folders: a list, not a count.** The list is short, and the names are the point: they're what the person will ask about.
- Each folder is one line: the name (600, `overflow-wrap:anywhere`), then "Can change files" or "Read only" (muted 13), then "from Fabrication lead" (faint 13).
- The home folder isn't repeated here; the Home folder row covers it.
- No share usage: a share's total belongs to the share, not the person, and it's on TrueNAS's page.
- All of them are listed. No cap, because there's no fuller list to link to.

**Pronouns.** Syndra doesn't know them. Copy uses the name, "they" and "their", as HomeLine does.

## 3 · Where each link lands

- **"Set on TrueNAS's page →" on Home folder and Hold:** System › TrueNAS › Accounts Syndra manages › **their row**. The page glides there and the row flashes once (Motion 8d's jump). That row is the one write place for both: Set a limit, Change limit or Review limit, then Put on hold and Revoke.
- **"Set in TrueNAS's mappings →" on Folders:** the TrueNAS mappings page. A folder opens to a person when a role gives a group, which is set in Mappings, and that group can open the folder, which is set on TrueNAS itself. The link goes to Syndra's half. The words name the real place, like "Set in Roles that wait →".
- **Not the setup card.** The setup card holds settings for the whole system (project, mappings, where homes go). A person's limit and hold belong to their row.
- **The system's name** opens TrueNAS's page at the top.
- **"Finish setting it up →"** (being set up) opens TrueNAS's page at its project step.
- **"Waiting for you to send →"** (no account yet) opens Pending changes, filtered to this person.

## 4 · Empty and edge states

- **No account yet, waiting for you to send** (`noaccount`):
  - The Account row is the step's status seat: amber dot, "Waiting for you to send →". Faint under it: "Their Fabrication lead role reaches TrueNAS. Syndra makes the account once you send it."
  - Password, Home folder and Folders become **NotOpenYet**:
    - Lead line: neutral dot, "Opens once their account is made".
    - Why, in muted: "Password, home folder and folders are read from the account itself. This is what they'll get."
  - The preview, at 0.45 and inert, shows what's true already: "Not set yet", "Up to 20 GB, from their Fabrication lead role", and the folders their groups open.
  - The lead line has no link: the step is the row right above it, and two links to Pending changes in one card would be one too many.
  - Hold stays at full strength.
- **No role reaches it** (`norole`):
  - One row: Account "None", faint "None of their roles reaches TrueNAS. Fabrication lead and Archive admin do."
  - No NotOpenYet, because nothing is about to open.
  - The card stays, so the section never moves.
- **An account Syndra didn't make** (`adopted`):
  - Account: `arao`, faint "Made by hand on TrueNAS before Syndra managed it. Kanishk Singh assigned it to Aanya on 12 Sept, with everything it held." Kanishk Singh links to his page.
  - **The Password row follows TrueNAS, not Syndra's record.** Syndra has no record of a password, but TrueNAS accepts one. It reads "Set before Syndra managed this account", with faint "TrueNAS accepts it, so it works. Syndra has no date for it, and only Aanya can change it."
  - Never "Not set" for an account that works.
- **Needs a new password** (`newpw`):
  - Password: neutral dot, 600, "Needs a new password". Faint: "Their access was given back on 20 Oct. TrueNAS opens their folders again only once Aanya sets a new one on their Network storage page. Nobody else can."
  - Hold: "None", faint "The last one ended on 20 Oct", which is the reason.
  - Folders stay at full strength. The operator can read them, though Aanya can't open them yet.
- **First password not set:** the same row, with "Not set yet" and "Their account turns every sign-in away until Aanya sets one on their Network storage page."
- **Old password gone (re-enrolment):** the same row, with "Needs a new password" and "TrueNAS was rebuilt since they set it, and passwords didn't carry over."
- **Not mapped to any folder** (`nofolders`):
  - Folders: "None yet". Faint: "Their Fab visitor role reaches TrueNAS, but no folder there is open to it. Folders open to a group on TrueNAS itself; this fills in when one does."
  - No dashed box and no hollow dot. That's the member's pattern for an empty list; for the operator, nothing is wrong.
- **System offline** (`offline`):
  - **One amber Notice in the card**, live, under the header. Title: "TrueNAS is offline". Body: "It stopped answering at 08:33. Home folder and Folders below are what it said at 08:31, 2 hours ago. Aanya's access hasn't changed, and their folders open again when it's back."
  - Home folder and Folders are TrueNAS's reads, so they sit at **0.62**: stale, still readable.
  - **Account, Password and Hold are Syndra's own records** and stay at full strength.
  - With no last read, the Notice stands alone, those two rows aren't drawn, and there are no numbers.
  - Header status: "Offline since 08:33 · last read 08:31".
  - One or two missed checks show nothing here. That stays on TrueNAS's page (Q-01).
- **One slow answer** (not offline, read failed): Account, Password and Hold as usual. Home folder and Folders each say "TrueNAS didn't answer when this page asked. This page reads it again on your next visit." No Notice: nothing is down.
- **Being set up** (`setup`):
  - **NotOpenYet over the whole card body.** Lead line: "Opens once TrueNAS is set up". Why: "TrueNAS has no Zitadel project yet, so no role reaches it and nobody has an account there." Link at the right: "Finish setting it up →".
  - The five labels are previewed with an em dash each, because nothing has been read.
  - With a project but no mapping yet, the why becomes "No role is mapped to TrueNAS yet, so nobody has an account there."
- **Loading:** the name and the caption (both known), then RowSkeleton 4. No status, no age, no values.
- **Error** (Syndra's server):
  - The red Notice takes the card's place, under the section label: "Couldn't load Aanya's storage on TrueNAS." · "Syndra's server did not answer when this page asked for it. Nothing was changed." · Try again · reference.
  - The projects above keep their own state.
- **Whole account on hold** (`held`):
  - Hold row: neutral dot, 600, "Their whole account, since 20 Oct". Faint: "“Lost their key card. Until it's found.” · put on hold by Kanishk Singh · review on 17 Nov". The status beside the name reads "Answering · their account is on hold".
  - Folders carries one faint line first: "None of these opens while the hold is on."
  - One folder held back: the Hold row reads "renders, since 20 Oct", with the same faint line.
- **Zitadel degraded** (whole page): the page's own amber Notice as built. This card is TrueNAS's, so it keeps its own state.
- **Phone 390:** each row wraps to the label, then the value, then the seat on its own line, still at the right (`margin-left:auto`). The bar keeps 100% of the value width.

## 5 · The 6 Oct "Put on hold" row: dropped (confirmed)

- **Dropped, per the 15 Oct read-only ruling, which is later.** I see no strong experience reason to overturn it. The operator is one tap from the row that already holds Put on hold and Revoke, side by side, which is the comparison that row was built for. A second Put on hold here would be a second write place for the same hold.
- **The Withheld band above the projects stays as built.** It's the page's warning that the roles listed below aren't all usable. The Hold row is the same fact said in the system's own card, read-only. They are one object said twice in two roles, not two controls.
- **Found while checking:** holding **one folder** has no write place today.
  - `PeopleOnTarget.tsx` says a folder hold "is authored from the share's own row on the person's page", and that row never shipped.
  - If folder holds are wanted, they go into the Put on hold dialog on the person's row on TrueNAS's page, as a choice between "Their whole account" and "One folder". Not drawn: nobody has asked for it since 6 Oct.

---

## Part 2 · Where someone signs in

### Owner's call · whether Syndra keeps sign-in counts

Three choices, smallest first. Each frame says which one it needs.

- **A · Last sign-in only, per person per app. Recommended.**
  - One date per person per app, overwritten on every sign-in. No history and no counts.
  - It's what Syndra keeps today (`usage_last_seen`), keyed by app instead of project. Same size, same kind of data.
  - **It answers every question in Q-10:**
    - the person page's line (6);
    - the app page's people in 7 days, which is everyone whose last sign-in falls inside the 7 days (7);
    - Held but not used (8).
  - **Kept** until 13 months pass without a sign-in, or the person is removed from Zitadel.
  - Why 13 months and not 90 days: a student who used the laser in April and comes back in August should read "Last sign-in 14 Apr", not "No sign-in in 90 days". That's the difference between an operator revoking a returning student's access and leaving it.
  - Sign-ins Syndra can't match to an app are kept the same way, as "app unknown". They explain no access, so no page shows them per person.
- **B · A, plus a daily total per app.** One number per app per day, kept 13 months, with nobody named. It adds one frame, the app page's 30-day strip (`app-b`). It's cheap and impersonal, but no decision in Syndra reads it. Add it the day someone asks how an app is trending.
- **C · Counts per person per app per day, kept 13 months** (Q-10's proposal). It adds a person's 30-day strip (`sign-b`). It's a day-by-day diary of when each student uses the makerspace's apps, and nothing an operator decides on needs it. At a university, it's also the record most likely to be asked about. **Not recommended.**

**Recommendation: A.** It's the minimum that serves, and it is no more data than Syndra holds today.

## 6 · Person page: a last-sign-in line in each project's heading (A, recommended)

- **Where:** the project card's heading, after the role count, faint 13.5, `text-wrap:pretty`: "Fabrication · 1 role · Last sign-in 10 min ago, to Fab queue". It shows whether the card is folded or open.
- **Why there:** use sits beside the access it uses, which is where Revoke is decided. No new card, and it works in Basic.
- **The words:**
  - Newest app named: "Last sign-in 3 days ago, to Laser bookings". The project is the unit access is given in, so one line per project, naming the app used last.
  - Under a day, relative ("10 min ago", "3 hours ago"); after that, a date ("2 Oct", with the year only when it isn't this year).
  - A project with apps and none in 90 days uses A-06's line: "No sign-in to its app in 90 days" ("its apps" for two or more).
  - When Syndra started reading sign-ins less than 90 days ago: "No sign-in to its app since 3 Sept".
  - A project with no app (a door, a locker) shows nothing.
  - Sign-ins not reaching Syndra at all: no line anywhere. Never show what hasn't been read.
- **Phone:** the line wraps under the project's name.
- **Not a count over 30 days, and not a strip** (`sign-b`, drawn for comparison). Both need C, and both judge how much someone works, which no decision here needs.
- **The Activity tab doesn't list sign-ins.** route-users-id's Activity frame draws "Signed in · through Zitadel". That line is the board's alone: it isn't built, and it shouldn't be. Syndra keeps no sign-in history under A.
- Operators only, like `GET /users/{id}/usage`.

## 7 · App page: the meta row (A, recommended)

- **In the PageHeader meta, after "uses roles from Laser Lab →":** "9 people in 7 days · last sign-in 10 min ago". Faint, separated by "·". It's a fact about the app, so it goes in the header: no card and no chart.
- No sign-in in 7 days: "Nobody in 7 days · last sign-in 2 Oct". No sign-in ever: "No sign-in yet". Sign-ins not arriving: nothing.
- "9 people" doesn't link. People can't filter by sign-in, and a link to an unfiltered list would lie.
- It's Syndra's own record, so it stays when Zitadel is degraded, with no age of its own.
- **With B only:** a "Sign-ins" card under the two cards. A 30-day strip of daily totals, then "142 sign-ins in 30 days, by 23 people." Nobody is named in it. Not needed (`app-b`).

## 8 · Insights, Held but not used: stays a list, no counts (confirmed)

- **As built:** role · project (the project links) · people, most first. The header note is A-06's "No sign-in to its app in 90 days." The caption reads "Read from app sign-ins. Holding access you don't use is not wrong; it is a place to tidy."
- **Counts add nothing.** "Not used" is zero by definition.
- **The route-insights board is older than A-06 here.** Its "Last used" column and its "No tap or sign-in" note go. A badge source would bring taps back with its own wording (A-06 §7).
- Keying by app doesn't change the rule: "its app" is any app on the role's project.

---

## For code

**Part 1**
- **`GET /users/{id}/targets`** (operator), one view per registered target. It runs `describeMyTarget` for the subject and adds:
  - **binding:** `bound_by`, plus **`adopted`** (bound by the inventory's assign at `inventory.go:124`, not by a drain). The person, the operator and the date come from the existing `bound_by` and `bound_at`.
  - **home:** the `/targets/{t}/homes` fields for this account: `roles_limit`, `roles_limit_from`, `own` (bytes, reason, actor, ends or review, review_due), `own_ended`, `used_bytes`, `quota_bytes`, `gives_home`, `home_kept`.
  - **hold:** each suppressed term in force for this person on this target, with field, value, reason, actor, since and review date.
  - **credential:** what `myCredentialStatus` already carries, plus `storage.usable`. The page reads "works" from TrueNAS's answer, not from Syndra's record: adopted accounts.
  - **last known storage:** the last successful `addonsMyStorage` read for this person, with `read_at`. Without it, the offline frame shows the Notice alone.
  - **setup:** no project, or no mapping yet.
  - **preview:** for an entitled person with no account, the folders their roles' groups open, from `addonsReadReach`, as in `attributePlaces`. Used only inside NotOpenYet.
  - `recorded_at` and the queued change's id, for "Waiting for you to send →" to open Pending changes filtered to this person.
- **TrueNAS's page:** give each `PersonRow` an anchor, and land `?person=<id>#accounts` with the jump's glide and one flash (Motion 8d). Update the stale comment on HoldDialog in `PeopleOnTarget.tsx`: no folder hold is authored from the person's page.
- **No new component.** The rows are the setup card's row as in `ProjectSetup.tsx`, `ServiceAccountPage.tsx` and `AccountKind.tsx`. Notice and NotOpenYet are the existing `ui/src/components/ui` ones. The card reuses HomeLine's copy and the bar from `PlaceBlock`.
- The person page's `ReadLine` gains one section per system, labelled with the system's name and no count.

**Part 2** (once the owner picks; A assumed)
- Key `usage_last_seen` by **app** (`app_id`, empty for unknown) instead of project. `recordSignIn` already resolves the app before mapping it to a project (`resolveSignInProject`), so keep the app id. Migrate existing rows as "app unknown" for their project, or drop them: they're at most 90 days of value.
- Prune rows older than 13 months.
- `GET /users/{id}/usage` returns per app, with its project, `last_at`, and when Syndra began receiving sign-ins (for "since 3 Sept").
- **New** `GET /applications/{id}/usage`: `{ people_7d, last_at }`, computed from the same table.
- Insights' held-not-used: unchanged semantics. A role is used if any app on its project has a sign-in from the holder in the window.
- With B: a daily per-app total table and a `days` array on the app endpoint. With C: nothing is drawn for build.
