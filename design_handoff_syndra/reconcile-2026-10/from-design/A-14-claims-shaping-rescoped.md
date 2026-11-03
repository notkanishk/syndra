# A-14 · Shaping what each app is told, rescoped

Answer to `Q-14-claims-shaping-rescoped.md`, 3 Nov 2026. Board: `a14-claims-shaping.dc.html` (Final, 3 Nov 2026). It replaces A-11's board as the final one; A-11's stays for the record. Where they disagree, this one wins.

The example world is fake but the apps are real:
- **NetBird** on the project Network, which has 7 roles. NetBird sees 4 of them, in its own words.
- **Supabase** on Data, through the custom OIDC provider.
- **GitLab** on Code, with no shape yet.
- **Inventory**, an in-house app on Custom.
- The preview person is Ada Lovelace. The test people are Aanya Rao, Rohan Mehta and Grace Hopper.

Every template below follows `docs/research/2026-11-03-app-claims.md`: the same claim keys, value shapes and token placement.

The page still has one job: make sure this app is told the right thing about people.

## 1 · The page, redrawn

**Each app now has its own shape. The page reads top to bottom: the header; what the app is told and the preview, side by side; Debug mode; test people; what the app reads; History. Basic shows the shape and its effect. Advanced adds only the ways to make unusual shapes.**

**Order:**
- PageHeader.
- What NetBird is told | Preview. These are two halves, as in A-11. On a phone the shape comes first (#90).
- Debug mode, full width.
- Test people.
- What NetBird reads.
- History.

Gaps: 28 from the header to the content, 20 between cards. On a wide screen the Preview stays in view while the shape card scrolls (sticky, 20 from the top), so every edit is seen next to its effect.

**Header lede:** "What NetBird is told about a person when they sign in. Come here when it isn't seeing the groups it should, or before you change what it's told."
- Meta: "OIDC · roles from Network →".
- In Advanced, the meta adds the client id in Syndra.Mono.

**"What NetBird is told"** is one card, top to bottom:
1. **The template line**, faint 13.5: "From the NetBird template. Same as the template." (or "2 changes since", as on Supabase). Then **See the template** (ghost sm; it opens a sheet, so no →).
   - Under it is one plain caption with the template's main truth: "Each value becomes a NetBird group. NetBird moves people between groups on every sign-in."
   - This is a caption, never a box.
2. **Roles NetBird sees**: the roles table (§3).
3. **Claims**: one row per claim, in the row form (§4). Then **Add a claim** (ghost sm).
4. **Lands in**: one settings row (§5).
5. **Save**, with the before-line beside it (§3, and A-11 §4).

**Preview** keeps A-11's trail (five checks, and check 5 is the token).
- Check 1 is renamed "Zitadel's settings for NetBird". It now also checks where the claims land (§5).
- Check 4 names the shape: "What NetBird is told applies, version 3." Its faint line: "It makes `groups` from 4 of Network's 7 roles, in NetBird's words."
- Check 5's sentence under the token says where the token goes: "Sent in NetBird's access token and in userinfo. Zitadel adds `sub`, `email`, `name` and `preferred_username` itself; NetBird reads those for who she is."
- **The faded "other app's claim" line is gone.** A token now carries only its own app's claims.

**What Basic shows (calm):**
- the template line;
- the roles table;
- every saved claim row, whatever its kind;
- "Lands in" as read-only text with "Set by the NetBird template";
- Save;
- the preview;
- Debug mode;
- test people;
- what the app reads;
- History.

**What waits behind Advanced** (each part opens by Unfold, A-08 §3):
- **Lands in** becomes a Segmented control you can change.
- **Roles added later**, one row under the roles table: "Left out until you include them" | "Included". The template sets it.
- **Put it inside another claim** on every claim row (nesting, §4).
- **"More claims"** in the claim kind list: Yes if they hold a role, Bundles they hold, Each role's end date.
- **Sent as** on a roles claim. Text formats are offered here only, with their warning (§4).
- The client id in the header.

**A saved row always shows in both views.** Advanced adds ways to make more; it never hides what's already sent. The structure doesn't move with the data.

**Debug mode shows in both views.** It sits only here, in the claims part, beside the preview and its checks (owner, 3 Nov). There is no debug page, no setting and no Home or rail entry. If it were Advanced-only, someone could switch to Basic while it was still recording people's sign-ins.

**Loading, error, not found:** as on the 23 Oct board.
**Degraded:** as A-11 §1. The two cards drawn from Zitadel's last answer recede (A-13 (d): canvas ground, line-strong edge, text at full strength, rings for dots).

## 2 · Templates

**A template is a starting point that's copied in. Choosing one makes a draft that previews at once. Nothing changes until you save.**

**An app with no shape** (frame `new`, GitLab):
- The shape card's lede: "GitLab has no shape yet, so Syndra adds nothing to its tokens. Choose where to start. Nothing changes until you save."
- Then one ChoiceRow per template, single choice. Each has a title, a meta line with what it sends and where, and one description.
- **Suggested** sits in the note seat when the app's name in Zitadel contains the template's name ("Suggested: this app is called GitLab"). It is preselected.
- Choosing a row unfolds its short form under it:
  - what it sends;
  - where it lands;
  - its first "no claim can do this" line;
  - **See the whole template** (ghost sm).
- At the same moment, the preview runs that template as a draft: + lines, check 4 "The GitLab template applies, as a draft."
- Accent **Start from GitLab**, with the before-line: "Makes a draft. GitLab gets nothing until you save it."
- The draft fills the card's rows, and Save follows A-11 §4.

**The list of templates, in this order:**

| Template | Meta line | Description |
|---|---|---|
| NetBird | `groups` · access token and userinfo | — |
| GitLab | `groups` · ID token and userinfo | — |
| Supabase, custom OIDC provider | `custom_claims.groups` · ID token and userinfo | — |
| Supabase, Keycloak provider | `groups` · userinfo | — |
| Home Assistant | `groups` · ID token and userinfo | — |
| Sign-in only | Nothing of its own | "For Vaultwarden, Kan and Navidrome. Who may sign in is Zitadel's role check." |
| Custom | `roles` · both | "Every role, in Syndra's words. For in-house apps: SmartDB, Community Slicer, Inventory, Audio-Dash, Printer Config, Syndra." |

- **"Or share another app's shape"** comes last. It appears only when another app on the same project already has a shape. Example: NetBird (clients) offers "NetBird (web)'s shape · edited in one place".
  - Two NetBird apps with different groups would move people back and forth on every sign-in, so sharing is the safe default for one service with two clients.
- **TrueNAS** gets no template. Its page says: "TrueNAS's web interface can't sign in with Zitadel. Access to its shares goes through the TrueNAS add-on." Then "TrueNAS add-on →".

**The template sheet** (frames `tpl-*`) is Modal md. Title: "The NetBird template". Its sections, in order, each a type-label and then dot-and-sentence lines (neutral dots, no boxes):
1. **What it sends.** The key in mono, a sentence, and "Sent as" with the literal.
2. **Where it lands.**
3. **What NetBird does with it.**
4. **What no claim can do.** These are the "not a claim" truths. They are said plainly and never dressed up as settings.
5. **Set in NetBird** / **Set in Zitadel.** The exact setting names, in mono.
6. **Changed since you started from it.** This shows only on an app that has a shape. It uses the gutter marks (+ − ~) and ends on **Start again from the template** (outline). That makes a draft, and the footer note says so.

From the chooser, the footer is Close and accent **Start from …**. From an app that has a shape, it is Close and **Start again from the template**.

**Each template's words.** These come from the research and must stay true to it:

**NetBird**
- *Sends:* `groups`. This is a list of the roles you include, in NetBird's words, for example `netbird-staff`. It is always a list, and `[]` when someone holds none.
- *Lands:* in the access token and in userinfo. That covers NetBird reading its access token, and NetBird's built-in Dex reading the ID token with userinfo.
- *NetBird does:*
  - "Each value becomes a NetBird group."
  - "On every sign-in, NetBird adds the person to the groups in the claim and takes them out of any group it made from a token that the claim no longer names."
  - "A rename makes a new group and moves people out of the old one. Policies on the old group stop matching them."
  - "A group you made by hand in NetBird with the same name isn't joined."
- *No claim can:* "Set NetBird's owner, admin or user role. Syndra found no claim that sets them; they are set in NetBird."
- *Set in NetBird:* Settings › Groups:
  - turn on "Enable JWT group sync";
  - set "JWT claim" to `groups`;
  - "JWT allow groups" turns away anyone in none of them. Never save one you aren't in.
- *Set in Zitadel:*
  - "Auth token type: JWT. A Bearer token carries no claims."
  - "User Info inside ID Token: on."
- *Roles added later:* left out.

**GitLab**
- *Sends:* `groups`. This is a list in the words your `gitlab.rb` uses, for example `gitlab-users`, `gitlab-admins` and `gitlab-external`. It is always a list.
- *Lands:* in the ID token and userinfo. GitLab merges the two.
- *GitLab does:*
  - "If `required_groups` is set, anyone without a match can't sign in."
  - "If `admin_groups` is set, GitLab makes and unmakes admins on every sign-in, including admins made by hand. The same goes for `auditor_groups` and `external_groups`. External cancels admin."
  - "A missing or renamed claim demotes every admin and can lock everyone out."
- *No claim can:* "Do anything on GitLab's free edition. Group sync is Premium or Ultimate only."
- *Set in GitLab:* under `args.client_options.gitlab`:
  - `groups_attribute: groups`;
  - the four lists, which name these words exactly.
- *Renaming:* optional. GitLab's lists can name Syndra's keys directly. If you rename, change `gitlab.rb` in the same step.

**Supabase, custom OIDC provider**
- *Sends:* `custom_claims` › `groups`, a list nested inside one object. It is always sent, `[]` when someone holds none.
- *Lands:* in the ID token, with userinfo too. Supabase reads only the ID token when there is one, and Zitadel always sends one.
- *Supabase does:*
  - "It keeps the top-level `custom_claims` object and drops other unknown keys. A top-level `groups` would be lost unless it is on `custom_claims_allowlist`."
  - "It rewrites the person's metadata on every sign-in, key by key. A missing key keeps its old value. That's why the claim is always sent, even when empty."
  - "Read it as `auth.jwt()->'user_metadata'->'custom_claims'->'groups'`."
- *No claim can:*
  - "Make this safe for row-level security as it arrives. It lands in `user_metadata`, which the signed-in person can edit. Copy it into `app_metadata` with a Custom Access Token Hook first."
  - "Refresh groups without a new sign-in through Zitadel. Supabase's refresh doesn't fetch them."
- *Needs:* Supabase Auth v2.196.0 or later (self-hosted 0.8.1).
- *Names:* `given_name`, `family_name` and `name` already reach `user_metadata` on their own. Put them inside `custom_claims` only if your code reads them there.

**Supabase, Keycloak provider**
- *Sends:* `groups`, top-level, in userinfo. Supabase copies every key it doesn't know into `custom_claims`, so it arrives as `user_metadata.custom_claims.groups`.
- *Lands:* in userinfo. This provider reads only userinfo.
- *No claim can:*
  - "Point this provider at Zitadel by itself. It asks Keycloak's paths, so Zitadel needs a rewrite in front of it."
  - The same `user_metadata` warning as the custom provider.
- *The two Supabase templates' descriptions both say:* "Which one? A `custom:` provider is the custom OIDC one. `GOTRUE_EXTERNAL_KEYCLOAK_*` settings mean the Keycloak one."

**Home Assistant**
- *Sends:* `groups`, a list, in Home Assistant's default words `admins` and `users`. You can rename these, or set `roles.admin` and `roles.user` to Syndra's words.
- *Lands:* in the ID token and userinfo. Home Assistant reads the ID token, and userinfo fills in what's missing.
- *Home Assistant does:*
  - "In `admins`: an administrator."
  - "In `users`, or when no user group is set: a user."
  - "If `roles.user` is set, anyone in neither group is turned away."
- *No claim can:* "Change someone's role once their Home Assistant user exists. Home Assistant sets it only when the user is first made. Promote or demote existing people by hand in Home Assistant."
- *Needs:* the `auth_oidc` integration (hass-oidc-auth). Home Assistant has no OIDC sign-in of its own.

**Sign-in only** (Vaultwarden, Kan, Navidrome)
- *Sends:* nothing of its own. Zitadel's standard `email`, `email_verified`, `name`, `preferred_username` and `picture` do the work.
- *No claim can:*
  - "Decide who may sign in. Vaultwarden and Kan have no groups. Turn on Zitadel's role check for the project, and give the role."
  - Then a violet link, only while the check is off: "Turn on the role check for Vault in Zitadel →".
- *Vaultwarden:* "Needs `email` and `email_verified`. Without them, sign-up is blocked."
- *Navidrome:* "It has no OIDC of its own. It signs in through a proxy, which needs a stable `preferred_username` or `email`. If the proxy checks a group, use Custom instead."
- On an app using this template (frame `vault`), the roles table is **NotOpenYet**: "Roles reach Vaultwarden only through a roles claim." Why: "The Sign-in only shape sends none, on purpose: Vaultwarden has no groups."
- While the role check is off, check 1 is amber: "Zitadel lets anyone in the organisation sign in to Vaultwarden." Faint: "The role check is off for Vault. Vaultwarden has no groups, so nothing else stops them." Then the same violet link. Check 5 is neutral: "Zitadel receives no Syndra claims for her, on purpose."

**Custom**
- *Sends:* `roles`. This is every role in Syndra's words, as a list, in both places. Roles added later are included.
- *No claim can:* "Tell Syndra what your app reads. Note it under What Inventory reads, and Syndra checks every save against it."

**Changed since.** The template line counts the differences between the template and the saved shape: "2 changes since", or "Same as the template". The sheet lists them.
- **Only the shape counts:** claims, where they land, roles added later. Which roles are included and the words they get are always yours, so they never count as changes.
- **A template never changes a saved shape.** If Syndra's template improves, the line says "The NetBird template has changed since 3 Nov." and the sheet shows what changed. It changes nothing for you.

## 3 · Roles for this app

**One table inside the shape card: Role · "NetBird calls it" · Included. A rename or a removal is said three times before it lands: on its row, in the before-line, and in the save summary, each time with what NetBird will do.**

- **Heading:** "Roles NetBird sees". Faint note: "4 of 7 included".
- **Lede:** "Which of [[Network]]'s roles NetBird sees, and the word it sees for each." The project links; the rows don't (a role has no page).
- **Columns** (CardColumns), all 7 project roles:
  - **Role.** The display name at 14.5/600, with WaitsMark where it applies. The key is under it in faint mono. A SortHead orders by name, A to Z, and carries the arrow. This is a short list, so there's no search.
  - **"NetBird calls it".** A mono Input. Empty means Syndra's key, and the placeholder shows it. The heading uses the app's name.
  - **Included.** The Checkbox in a 90px right seat.
- **Not included:** the word field stays readable at muted strength and can't be edited. Ticking turns it back on.
- **Two roles may share a word.** NetBird gets the group once. The second row's faint line says "Also Admins' word."
- **A role added to Network later** comes in with a faint "New. Left out until you include it." (or "Included", per Roles added later).
- **Bundles and rules don't appear here.** They decide who holds a role. This table decides what NetBird calls it.

**The warning, before the save** (frame `rename`):
- **On the row:**
  - A renamed word gets a faint line under it: "Was `netbird-staff`."
  - An unticked role that was sent gets: "Was sent as `netbird-guests`."
- **The before-line**, beside Save. It turns warn-text, with an amber dot, only when a change breaks something for the app:
  - "2 changes NetBird will act on. 12 people move from `netbird-staff` to a new group, `network-staff`, at their next sign-in. 4 people leave `netbird-guests`."
- **The draft preview** shows − `"groups": ["netbird-staff", "netbird-admins"]` and + `"groups": ["network-staff", "netbird-admins"]`. Under it: "One difference for Ada Lovelace: `netbird-staff` becomes `network-staff`, so NetBird moves her to a new group."
- Test people show Now · After this draft · Difference, as in A-11 §3.
- **What makes a change "breaking"** comes from the template, not from a guess:
  - **NetBird:** any rename or removal.
  - **GitLab:** any rename or removal. "If `gitlab.rb` names `gitlab-admins` in `admin_groups`, every admin who signs in is demoted. If it's in `required_groups`, they can't sign in. Change `gitlab.rb` in the same step."
  - **Home Assistant:** renaming `admins` or `users`. "New people get no admin until `roles.admin` matches."
  - **Supabase:** any rename or removal. "Row-level security that checks the old word stops matching."
  - **Custom:** every rename or removal is said plainly, with no app-specific consequence: "Inventory will see `staff` instead of `stf`."

## 4 · Claim rows

**Every claim is one row form: Claim · Carries · Remove. Kind-specific controls sit on a second line, and every row ends with "Sent as" and the exact literal. Text formats for roles are behind Advanced, with a warning.**

**The row:**
- **Line 1:** Claim (mono Input, 180) · Carries (Select, flexible) · Remove (ghost sm, nowrap).
- **Line 2,** only for kinds that need it (below).
- **Line 3:** faint, "Sent as `…` for Ada Lovelace." Every row has it.
- **An error** is danger text under the row, as in A-11 §8.

**Carries**, the Select, in three groups:

| Group | Options | Sent as |
|---|---|---|
| From the person | Roles NetBird sees | a list |
| | Given name · Family name · Full name · Username · Email | text |
| | Is a service account | yes/no |
| The same for everyone | A fixed value | Text, Number, Yes/no or List (A-11 §8) |
| More claims (Advanced) | Yes if they hold a role | yes/no |
| | Bundles they hold | a list of bundle names |
| | Each role's end date | an object |

- **Roles.** No line 2 in Basic. "Sent as `["netbird-staff", "netbird-admins"]` for Ada Lovelace, and `[]` for anyone who holds none." It is always sent, never left out.
- **Is a service account.** "Sent as `false` for Ada Lovelace." Service accounts get `true`. This is the only place a token says so.
- **Yes if they hold a role.**
  - Line 2: "If they hold" · the chosen roles as chips · **Change roles** (ghost sm). That opens the role picker in single-project mode: one part per job.
  - "Otherwise `false`." A missing claim reads differently from false, so it is never left out.
  - Example: `is_admin`, "Sent as `true` for Ada Lovelace (she holds Admins)."
- **Bundles they hold.** "Sent as `["Makers"]`." These are bundle names, not ids, so a rename reaches the app.
- **Each role's end date.** "Sent as `{"staff": "2027-05-31"}`." The keys are the app's words. Roles with no end are left out of the object.
- **Fixed value.** A-11 §8 unchanged: the Type Segmented, the value control, and the type keeping itself.

**Nested paths** (Advanced to make, shown in both views):
- **Put it inside another claim** (ghost sm, under the row's "Sent as" line) adds a mono "Inside" Input before the name. The name seat then reads `custom_claims` › `groups`.
- The "Sent as" line shows the whole object: `{"custom_claims": {"groups": ["analyst", "editor"]}}`.
- **A dot in a name stays a dot.** `syndra.laser.roles` is still one flat key. Nesting is its own field, so no saved key changes meaning.
- Two rows inside the same parent merge into one object.
- A row named the same as a parent is an error: "`custom_claims` is already an object here, holding `groups`."

**Text formats** (`csv`, `space_delimited`). **Owner's call. Recommend 4a: behind Advanced, with a warning.**
- **Where:** in Advanced, a roles row gets line 2 "Sent as": "A list" | "Text, with commas" | "Text, with spaces".
- **The warning:** choosing a text format adds a warn-text line: "No app Syndra has a template for reads roles as text. NetBird would make one group called `netbird-staff,netbird-admins`. Use this only if Inventory's own code splits it."
- **No template ever chooses text.**
- **A saved shape that already sends text** shows line 2 in both views, with the warning, so it can't hide.
- **4b:** remove the text formats entirely. It's cleaner, but an in-house app that splits text (Printer Config today) would have to change first.

**Order:** rows keep the order you add them in, and the token keeps that order. Repeated keys are refused, as in A-11 §8.

## 5 · Where claims land

**Per app, not per claim: one "Lands in" row. Check 1 catches a Zitadel app setting that doesn't match.**

**The row**, in the settings-card shape:
- Label: "Lands in".
- Value: "The access token and userinfo", with "NetBird reads the access token." under it.
- In Advanced, the right seat is a Segmented control: **Both** | **Access token** | **ID token and userinfo**.
- In Basic it is read-only text with "Set by the NetBird template".

**"ID token and userinfo" is one choice, because Zitadel makes it one.**
- Syndra answers Pre Userinfo for both.
- The ID token carries them only when "User Info inside ID Token" is on.
- The Segmented's faint line says so.

**Not per claim.** No app in the research needs one claim in one place and another elsewhere. "Both" works for every template as long as Zitadel's settings are right, so Custom defaults to Both.

**Check 1** is now "Zitadel's settings for NetBird". It reads three facts:
- Network's roles are asserted, or NetBird asks for role scopes;
- the token type, Bearer or JWT;
- whether userinfo goes inside the ID token.

It checks each fact only where the shape lands. The pass line: "Network's roles are asserted, the access token is a JWT, and userinfo goes inside the ID token."

**A mismatch** (frame `lands-fail`) is red:
- Check 1: "Zitadel's settings for NetBird keep its claims out of the access token, where NetBird reads them."
- Faint: "NetBird's token type in Zitadel is Bearer. Only a JWT access token carries claims."
- Link: "Set NetBird's token type to JWT in Zitadel →".
- Faint: "Or have NetBird read its ID token instead, with `NETBIRD_TOKEN_SOURCE=idToken`. Then check again."
- **Check 5** turns red: "Zitadel receives this, but only for userinfo:" then the token. Under it: "NetBird reads its access token, so it sees no groups, and it empties every group it made from a token."
- The Lands in row gets a quiet line under its value: "Doesn't reach the access token. Check 1 in the preview says why." The fix lives in one place, check 1.
- "What NetBird reads" says **Doesn't reach it** (amber), pointing at check 1.

**Other mismatches**, each named the same way:
- Supabase (custom OIDC) or Home Assistant with "User Info inside ID Token" off: "Turn on User Info inside ID Token for Supabase in Zitadel →".
- A shape set to Access token only for an app noted as reading the ID token.

**What the app reads** (A-11 §6) gains a "where" column: "A list of text, in the access token".
- The template fills it in.
- The types drop `csv` and `spaced` as equals. They're offered only as "Text, with commas (rare)" after a divider.

## 6 · Debug mode

**A full-width card under the preview: one quiet row when off, a live list when on. One sign-in opens in place and shows what Zitadel sent, what Syndra answered, the five checks, and a labelled rebuild of the claims. It's off by default, turns itself off after 15 minutes, keeps what it saw in memory for an hour, and Change history records who turned it on.**

**Off** (frame `netbird`). One settings row:
- Label: "Debug mode".
- Value: "Off. See each real sign-in to NetBird as it happens, for 15 minutes."
- Faint: "Last on 2 Nov, for 15 minutes, by Kanishk Singh."
- Seat: **Turn on debug mode** (outline sm; it opens a sheet).

**Turning it on** (frame `debug-turn-on`), Modal md:
- **Title:** "Turn on debug mode". **Lede:** "See every sign-in to NetBird live: what Zitadel asked, what Syndra answered, and the five checks. It turns itself off after 15 minutes."
- **"Whose sign-ins"**, ChoiceRow single:
  - "Only you, Kanishk Singh" (preselected; see below);
  - "Everyone signing in to NetBird";
  - one row per test person: "Only Ada Lovelace", with the note seat "Test person".
- **Faint:** "Syndra keeps what it sees in memory for an hour, never in its database. Names and emails show, because this is for you to debug. Signatures and secrets never do."
- **Footer note (the before-line):** "Ends at 10:21, 15 minutes from now. Change history records that you turned it on." Then Cancel and accent **Turn on debug mode**.
- **Result, in the sheet:**
  - The dot blooms once, and **On**.
  - "Debug mode is on for NetBird until 10:21, for your sign-ins."
  - Faint: "Each sign-in appears under the preview as it happens."
  - Close.
- **Owner's call: who it covers by default. Recommend 6a, "Only you" preselected.** Most debugging starts with signing in yourself, and it records nobody else. "Everyone" is one tap away. 6b preselects Everyone: quicker when one member's NetBird is broken, but it records everyone by default.

**On** (frames `debug-on`, `debug-empty`). The row becomes:
- **StatusSeat, lime:** "On", with the note "Your sign-ins · ends 10:21, 14 min left".
  - The minutes change in place once a minute (Crossfade, S4). It is never a ticking counter.
  - The seat has **Turn off** (outline sm).
- **Beneath, by Unfold:**
  - The read line: a lime dot and "Live · newest on top".
  - Right side, while rows are waiting: faint "2 new sign-ins wait until you move off the list."
- **Columns:** a chevron seat (28) · Time (96, `10:09:14`, with ↓ for newest first) · Person (210: avatar, name linking to their page) · Asked for (120: "Access token" / "Userinfo") · Syndra answered (mono: "`groups`, 2 values" / "Nothing: she asked for two projects") · Checks (120 right: "All pass", "No roles", "Refused", "Fails check 1").
  - It doesn't sort: the order is arrival.
- **Arrival (Motion v2 S5 and A-08 §4):** a new sign-in opens its gap at the top and tints in (the refresh rule). It does not arrive with the 8px rise.
  - **While the pointer or focus is in the list, or a sign-in is open, new rows wait.** The read line counts them. They come in when you move away or close it.
  - What you're reading never moves.
  - Still form: they appear at once.
- **Long lists:** 10 rows, then the PaginationRow.
- **Empty** (`debug-empty`): one sentence with a breathing lime dot, "Waiting for your next sign-in to NetBird." Faint: "Sign in to NetBird in another window to see it here." No column headings.

**One sign-in, opened** (frame `debug-open`). The row's chevron opens it in place by Unfold. Inside, at 1440, two columns, then two rows:
1. **"What Zitadel sent"**:
   - A plain sentence: "Zitadel asked for NetBird's access-token claims for Ada Lovelace, listing 2 roles in Network."
   - Then the raw JSON in Syndra.Mono on `--surface-0`: function, application client id, user (with email, unredacted), org, user grants.
   - Faint under it: "Signed by Zitadel. The signature isn't shown."
2. **"What Syndra answered"**:
   - "Syndra added 1 claim, in 4 ms, from her claims built at 09:52."
   - Then the raw `append_claims` JSON.
3. **"The checks for this sign-in"**: the five-check trail, compact (dot and sentence, details only where a check didn't pass).
4. **"Your claims, as NetBird will see them"**, with a hollow neutral Badge beside it, "Rebuilt, not the signed token":
   - The lede says why: "Zitadel adds its own claims, signs the token and hands it to NetBird after Syndra answers, so Syndra never sees the token. This is rebuilt from what Zitadel sent and what Syndra answered."
   - Zitadel's lines are faded, as in A-11's TokenBlock.
   - `iss`, `aud` and the times are left out, and a faint line says so.
- **Buttons:** **Copy request** and **Copy answer** (outline sm, with CopiedSeat words).
- **Never shown:** signing headers, the signing key, any secret. These are removed before capture, not hidden on screen.

**Ended** (frame `debug-ended`):
- StatusSeat neutral: "Ended", note "At 10:21, after 15 minutes."
- The list stays readable at full strength. It isn't stale; it is a record. A faint line: "What it saw stays here until 11:21, then Syndra forgets it. It's gone sooner if Syndra restarts."
- Seat: **Turn on again** (outline sm).
- Turned off by hand: "Turned off at 10:12 by you."

**Audited.** Change history gets one entry when it is turned on (who, whose sign-ins, until when) and one when it ends ("Ended by itself" or "Turned off by Kanishk Singh"). The captured sign-ins are not audited and are not kept.

**Phone 390** (frame `phone-debug`):
- The row's seat wraps right.
- The turn-on sheet is a Modal sm sheet.
- List rows stack: time and verdict on one line, then the person, then what was asked.
- An opened sign-in stacks its four parts in the order above. The JSON wraps with `overflow-wrap:anywhere` and never scrolls sideways.

## 7 · What changes from A-11

- **Gone:**
  - "Moving off Actions v1" (§7), with its checklist, pasted token and comparison. Its two owner's calls go with it.
  - "Start from a token".
- **A shape belongs to one app, not to a project:**
  - The "What to edit" Segmented control (project default | this app) goes.
  - "Claims in this project's token" goes.
  - The faded other-app line goes.
  - Apps on one project can share a shape (§2).
  - ADR-002 still holds: a token carries one project's roles.
- **History is per shape,** and shared when the shape is shared. Lede: "Every save of what NetBird is told, kept as it was."
  - For apps that exist today, version 1 is "As Actions v1 sent it" (see For code).
- **The save summary's reach** is this app and any app sharing its shape: "Reaches NetBird and 23 people at their next sign-in."
  - "The 3 apps on 3D printing" becomes "**What NetBird will do**": the template's consequences, with amber dots.
  - A breaking save adds an I-understand line before the confirm. **Owner's call. Recommend 7a:** a Checkbox, "I understand NetBird will move 12 people to a new group." Save stays off until it's ticked, and only when the template marks the change as breaking. 7b is the before-line and summary only, with no tick. It is lighter, but a GitLab rename can demote every admin.
  - Saved: "NetBird's claims are now version 4." Faint: "12 people move to `network-staff` as they sign in. Point NetBird's policies at it once it appears there; a group made by hand isn't joined."
- **Check 1** also checks where claims land (§5). **Check 4** names the app's own shape and its template.
- **The roles claim's "Sent as" control** (array | csv | spaced) leaves the front (§4).
- **#50**, "nobody named the claim", becomes **"No shape yet"** with the template chooser. **Owner's call. Recommend 7c:** an app with no shape gets nothing from Syndra. 7d sends the old one-size shape (`groups` with every role, plus `custom_claims`) until one is chosen. 7d is gentler, but it would hand a new NetBird every role as a group. Apps that exist today are seeded with that old shape either way, so turning off Actions v1 changes nothing on the day.
- **"What the app reads"** moves below Test people. The template fills it in, and it gains "where".
- **Apps index:**
  - The Format column becomes **Claims**: the real key in mono, then faint where it lands.
  - The sub-line names the template: "OIDC · NetBird template, changed", "OIDC · Custom", "OIDC · shares NetBird (web)'s".
  - "No shape yet" is neutral, "Sends nothing yet".
  - A mismatch is amber, "Doesn't reach it". The read line adds: "**Home Assistant doesn't get its claims.** Its page says which Zitadel setting to change."
  - The mixed-format warning goes; every template sends a list.
- **Kept as A-11 drew them:**
  - the five-check trail and its tones;
  - the draft preview's − and + marks;
  - test people and the person form;
  - typed static values;
  - degraded (with the A-13 (d) recede);
  - the refused cross-project check;
  - Go back.

## For code

**The shaper (`internal/claims`).** Today it holds `ClaimName` and `FormatType`, and the maps `AttributeClaims` and `StaticClaims`. It must grow these:
- **Per-app profiles.** A shape is keyed by app (or by a shared shape id that apps point to), not by project default plus override.
  - `Shape(profiles, facts)`, which merges every profile on a project, becomes `Shape(profile, facts)` for one app.
  - `Conflicts` and `sortedProfiles` shrink to checking within one profile.
- **Role selection and renaming.**
  - `Roles []RoleMap{ Key, As, Included }` and `IncludeNew bool`.
  - Applied before formatting: filter, rename, then dedupe (two roles may share a word), in stable order.
  - `FormatRoles` already gives `[]` for none. Every roles row must emit even when nothing survives the filter, nested ones included.
- **Ordered claim rows** replace the two maps, because map order loses row order. Each row is `Claim{ Path []string, Kind, Source, Value json.RawMessage, Roles []string, Format }`, where Kind is one of:
  - `roles`;
  - `attribute`;
  - `static`;
  - `if_role` (`true` if they hold any of `Roles`, else `false`);
  - `bundles`;
  - `role_ends`.
- **Nesting.**
  - `Path` builds objects; rows with a shared parent merge.
  - Validation refuses a leaf whose path is a prefix of another.
  - `validClaimKey` stays per segment, so a dotted flat key keeps meaning one key.
- **Facts** must grow, and the cache compiler must persist them, since Facts is "exactly what the cache compiler persists":
  - `GivenName`, `FamilyName`, `Username` (preferred_username);
  - `Bundles []string` (names);
  - `RoleEnds map[string]string` (role key to ISO date);
  - `IsServiceAccount bool`.
  - `Attributes()` gains `given_name`, `family_name`, `username` and `is_service_account`.
  - Title, team, role count, compiled-at and the ids stay as attributes, behind "More claims" in the UI.
- **Placement.**
  - `Lands: both | access_token | userinfo` on the profile.
  - The handler returns that app's claims only when `req.Function` matches (`function/preaccesstoken` or `function/preuserinfo`).
- **Formats.** `csv` and `space_delimited` stay valid. Templates pin `array`.
- **Diff.** `Diff(old, new) []Change{ kind: add | remove | rename | retype | move | lands }`, with role-level changes (renamed word, left out). One function feeds:
  - the draft preview sentence;
  - the before-line;
  - the save summary;
  - History's "What changed";
  - "Changed since the template".
- **Breaking.** `Breaking(template, change)` is a per-template rule (NetBird, GitLab, Supabase: any rename or removal; Home Assistant: `admins` or `users`), plus the reach counts ("12 people hold Staff").
- **Templates.** A static catalog in code (`internal/claims/templates.go`): id, version, rows, lands, include-new, the breaking rule, and the prose keys. The shape stores `template_id` and `template_version`. "Changed since" is `Diff(template, saved)`.

**The data plane:**
- **Which app is signing in.** The handler reads only `user` and `user_grants` today, and ADR-002 says the payload doesn't identify the client. ADR-002 names `application.client_id` (Zitadel v4+); confirm it is in both the preuserinfo and preaccesstoken payloads, and map it to the app.
  - Without it, per-app shaping can't be done for two apps on one project, and the whole design falls back to per project.
  - The debug JSON on the board draws `application.client_id` on that assumption.
- **Check 1's new facts**, from the Management API OIDC app config:
  - `accessTokenType` (Bearer or JWT);
  - `idTokenUserinfoAssertion`;
  - with `projectRoleAssertion` from A-11.
  - Also the console deep links to each.
- **Sign-in only:** read the project's role check (`projectRoleCheck`) for the "Turn on the role check" link.

**Migration.** Seed every existing app with a Custom shape that reproduces Actions v1 exactly:
- `groups` (every role key, flat);
- `custom_claims: { groups, given_name, family_name, name }`;
- lands in both;
- saved as version 1 with the note "As Actions v1 sent it".

Turning off v1 then changes nothing on the day.

**Debug mode:**
- **Endpoints:**
  - `PUT /applications/{id}/debug { scope: "self" | "everyone" | user_id }` returns `{ ends_at }`;
  - `DELETE` turns it off;
  - `GET …/debug/sign-ins?after=` (or SSE) for the live list.
- **Capture** happens in the action handler after the answer is written, off the latency path, as a non-blocking send.
- **Storage:** an in-memory ring per app, an hour's TTL, and a cap of 200 per app.
- **Redaction at capture:** drop the `ZITADEL-Signature` header and any secret. Keep the body, emails included.
- **Each entry:** time, function, client id, user id, the grants, the request body, the response body, the five checks as evaluated at that moment, and the duration.
- **Audit:** an entry for on (who, scope, until) and one for off ("ended by itself" or who).
- **Server-side end.** The 15 minutes is enforced on the server, not by the page.
- **More than one replica:** if the API ever runs more than one, captures split between them. Say so, or pin capture to one.

**Carried from A-11:** the explain step, draft preview, test people, versions (now per shape), and typed static values.

**Supabase:** confirm which provider the deployed Supabase uses. The research couldn't verify it.

**Into the design system:**
- CheckTrail and TokenBlock, from A-11. TokenBlock gains a raw mode (plain JSON lines, wrapping) for debug.
- **ClaimRow:** the row form, with its three lines.
- **RoleWordsTable:** Role · calls it · Included, with the was-lines.
- **SignInRow:** the debug row and its opened body.
- Mounted from the bundle, as they are: Notice, NotOpenYet, StatusSeat, WaitsMark, ChoiceRow, Checkbox, SortHead, Unfold, PaginationRow, ArrowLink.
