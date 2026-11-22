# A-27 · The life of a storage account: closing, switching off, deleting

**Asked by the owner, 21 Nov 2026:** "few scenarios for removal/suspension … should there be a forced way for admin to delete an account and let the user make another account for truenas. and what should be the sequence of steps. if i stop tracking on truenas page but the role is still there what happens? and other such edge cases. we need to build it to be fool proof and intuitive."

**Status:** shaped and grilled in two rounds on 21 Nov 2026; staff access to a home decided on 22 Nov 2026 (§5.8); every answer is in §9. How Closing is enforced still waits for the owner's yes (§1a). This supersedes the earlier storage-page Shape (its 9 states and questions 1 to 7 are folded in). Nothing here is built.

**The prototype:** none yet. No look was put to the owner; the staff surfaces in §5 are specified from existing parts only. A prototype at 1440 and 390, both themes, is owed before slice L4 is built (build matches board).

**Checked, and how.** I read two fact surveys (`lifecycle-facts.md`, `lifecycle-needs.md`, in the job's temp directory) and a scout's answers on paths and the copy guide. I did not re-read the code and nothing was run on TrueNAS. Every `file:line` below comes from those surveys, branch `feat/addon-contract`; lines marked **UNVERIFIED** were not confirmed by anyone. The L0 test ran on the real TrueNAS on 21 Nov 2026 (§1a). It settled TBD-A and withdrew the quota floor, so **how Closing is enforced is pending the owner's choice.** One sentence still waits: **TBD-B** (does a leaver keep NetBird).

**Why this shape.** It is the ordinary leaver model: block or limit, keep for a stated window, then delete on a person's confirmation (Google Workspace and Microsoft 365 both suspend, keep, then delete). It meets NIST SP 800-53 AC-2 (accounts are disabled and removed on a defined schedule), ISO 27001 A.5.18 (access rights are removed) and A.8.10 (information is deleted when no longer needed), and GDPR art. 5(1)(e) (storage limitation). One departure from AC-2(3) is the owner's choice and is recorded in §9: a leaver's storage sign-in stays alive through Closing, and the immediate stop is a manual staff action.

---

## 1 · The states

One record per person per storage system, held by the backend. The state is a pure function of the record's cause, its two dates, and today in the configured zone. No new scheduler: the existing drift pass evaluates it each observation interval and queues what follows. What the page shows as *holding* is always what TrueNAS reports; the record only explains it and supplies the dates.

| State | When | On TrueNAS |
|---|---|---|
| **Active** | a role reaches the system and the person is in the directory | enabled, SMB on, groups of held roles, the role's quota |
| **Closing** | a cause exists and today is on or before *files open until* | enabled, SMB on, shared folders ended, the home readable and not writable. **How is pending the owner's choice (§1a).** The storage password keeps working |
| **Switched off, kept** | today is after *files open until* and before *can be deleted from* | locked, SMB off. Home, files, groups and quota row untouched |
| **Due** | today is on or after *can be deleted from* | the same. Nothing is written |
| **Deleted** | staff confirmed | account, personal group and quota row gone. The home folder belongs to root and the staff group, which can read it, and stays at its path. The number is never given out again |
| **Paused** (hold or Revoke, as today) | a staff hold on the account | locked, SMB off. Sits on top of any state above |

**Causes.** `left` (Zitadel reports the person deactivated, locked or deleted), `no_role` (no role they hold reaches the system), `home_only` (§7.6).

**Dates.** *Files open until* = the day the cause is first seen plus the system's "Files stay open for" setting (30 days). *Can be deleted from* = that date plus "Then kept for" (30 days). Both are stamped when the cause is first seen, so changing a setting moves nobody already closing. A date includes its whole day, in the configured zone.

**Transitions.**

| From → to | Trigger | Who sends |
|---|---|---|
| Active → Active, one share fewer | a role is lost, others stay | the runner, within about a minute (§2) |
| Active → Closing | the last role that reaches the system is lost, or the person leaves, or Zitadel no longer returns them | the runner |
| Closing → Switched off, kept | the day after *files open until*; or staff press **Switch off now** | the runner |
| Switched off, kept → Due | *can be deleted from* arrives | nothing is sent |
| Due → Deleted | staff tick, confirm and type the key (§6) | staff, one call per account |
| any → Paused | hold or Revoke, as today | the runner; Revoke also replaces the password |
| Closing, Switched off or Due → Active | the cause ends: a role returns, or the directory has them again | waits in Pending changes for Send (it gives access back) |

Rules that hold across the table:

- **No automatic fast stop.** Every leaver and every last-role loss goes to Closing, whatever Zitadel's word for it. The immediate stop is manual: **Switch off now** on a Closing row, or Revoke.
- **The clock runs whenever the cause stands.** A hold only decides whether the account is on. A leaver on hold still becomes Due. A person on hold who still holds a role and is in the directory has no cause, so no clock.
- **Gone means a successful read without them.** A directory read that fails never counts as someone leaving, and never makes an account deletable (today it can: `backend/internal/services/deps.go:351-354`).
- **At rollout nothing reopens.** An account already switched off for a cause starts as Switched off, kept, with *can be deleted from* = rollout day plus "Then kept for". (Defaulted by me; see the reply's Need.)

## 1a · The L0 test, and how Closing is enforced (pending the owner's choice)

**L0, run 21 Nov 2026, 22:24 to 22:30 IST,** on a disposable test account on the real TrueNAS, over SMB3, its home in the homes folder. With a user quota at the smallest value accepted (1 KiB) and the account over it:

- open and read work, byte for byte;
- a new file and a new folder are refused (disk full);
- delete works;
- **an overwrite is refused and the file is left at 0 bytes.** The client empties the file on open, then the write fails.

So the quota floor makes "open, copy and delete" true, and it destroys any document a member saves again during Closing. **The quota floor is withdrawn.** Part B (does a removed group still work in a connection already open) was not run and stays UNVERIFIED.

**Also learned that night.** Members reach the home share through their member groups' right to pass through it; homes are owned by the member, mode 700. So "no share groups" alone would shut a Closing member out of their own home. Any option that keeps reading must leave them one group that still gives the way in.

**The owner asked, 21 Nov 2026:** "is this the most elegant solution? look up online and for industry standards". What was looked up (fetched that day; the list with quotes is in the job's `readonly-facts.md`):

- **The common norm gives the leaver nothing.** Microsoft 365 keeps a deleted user's OneDrive 30 days by default and gives access to their manager, not to them (https://learn.microsoft.com/en-us/sharepoint/retention-and-deletion). Google Workspace suspends, lets an admin transfer the files, then deletes; a deleted user reaches nothing (https://knowledge.workspace.google.com/admin/users/delete-or-remove-a-user-from-your-organization). CU Boulder ends access at 90 days and deletes (https://oit.colorado.edu/accounts/former-employees).
- **A read-only stage is a recognised pattern too.** Case Western makes a departed person's Drive read-only 90 days after suspension, then deletes later (https://case.edu/utech/help/knowledge-base/google-workspace-education/google-drive-data-retention). The owner's read window is this pattern with shorter, configurable stages; taking Case Western's three long stages literally would add a stage and nothing else.
- **NIST leaves the timing to the organisation.** AC-2(3) and PS-4 say to disable "within [an] organization-defined time period" and to keep access to the information the person controlled (https://csf.tools/reference/nist-sp-800-53/r5/ac/ac-2/ac-2-3/, https://csf.tools/reference/nist-sp-800-53/r5/ps/ps-4/). The two windows are that period, stated.
- **What TrueNAS 25.10 supports.** A share has a documented read-only switch: "SMB clients cannot create or change files and directories in the SMB share" (https://api.truenas.com/v25.10.0/api_methods_sharing.smb.update.html). A share's own permissions are separate from the files' (https://www.truenas.com/docs/scale/25.10/scaletutorials/shares/smb/managesmbshares/). Samba's per-user read list exists only as an auxiliary parameter, which TrueNAS calls unsupported and limits to full administrators (same API page), so it is out.

**On this TrueNAS:** one share; homes in a folder beneath it, each owned by its member, mode 700; members get in through their member groups' right to pass through.

**The options.** The add-on's everyday key holds `ACCOUNT_WRITE`, `SYSTEM_AUDIT_READ` and `DATASET_WRITE`; the deleting key is typed per call.

| | A1 · A read-only share for closing accounts (recommended) | A2 · One share, write denied there (the earlier A) | B · Hand the home to root | C · No Closing |
|---|---|---|---|---|
| How | A second share over the homes folder with TrueNAS's read-only switch on. A Closing member moves from their member groups into one "closing" group, which may pass through to the homes folder and is denied the main share outright | The closing group is denied Change in the main share's own permissions | Every file in the home is re-owned to root with read for the member, in a recursive job; a second job undoes it on return | Switched off at once, files kept; staff copy on request |
| A member can | open and copy through the read-only share, which their computer shows as read-only. The usual share refuses them | open and copy, if a Change denial leaves Read standing. In Windows' model Change includes Read, so this may refuse everything | open and copy, at once | nothing; ask makerspace staff |
| Risk | The main share gains one entry that denies one new group; nobody else's entry changes and no file is touched. If that entry is missing, the home stays writable through the main share, so Syndra must be able to see it. An open connection keeps its old rights until it reconnects (UNVERIFIED) | Rests on what a partial denial means, which no source checked states. The same open-connection caveat | Rewrites owner and mode of every file on a TrueNAS with no backups; per-file permissions are lost for good; the everyday key gains a right not limited to a path | None on TrueNAS. Reverses the owner's round-1 and round-2 answers |
| The add-on must write | Nothing new: group membership, under `ACCOUNT_WRITE` | Nothing new | `filesystem.chown`, `filesystem.setperm` on the everyday key | Nothing new |
| Rests on | TrueNAS's documented read-only share; Case Western's read-only stage; NIST AC-2(3), PS-4 | Share permissions, with a meaning still to be proven | NIST AC-3; ISO 27001 A.8.3 | Microsoft 365, Google Workspace, CU Boulder; NIST AC-2(3) |

**Not offered.** The quota floor with loud warnings: a warning does not give a file back. A read-only ACL on the home with the member still its owner: an owner can always rewrite the permissions of what they own; made to hold, it is option B. Snapshots as Previous Versions: read-only, but they show the whole dataset, not one person's folder.

**Recommended: A1.** The plainest design of all is C, and it is the common norm; the owner has ruled it out twice, and Case Western shows his read window is a recognised pattern. Among designs that keep the window, A1 is the only one that rests on a documented switch, changes nobody else's access, touches no file and gives the add-on no new right. C stays as what **Switch off now** already does.

**The owner's one-time setup under A1** (all on TrueNAS, none of it by Syndra):

1. Make the group for closing accounts, as an SMB group.
2. Let that group pass through the main dataset's top folder and read the homes folder.
3. Add a share over the homes folder with Read-only on.
4. In the main share's own permissions, add one entry: that group, denied Full.

Compared with the earlier A: step 3 is new, and step 4 is a full denial in place of a Change denial.

**The test before L3,** on a disposable account in that group:

- through the read-only share: open, new file, new folder, overwrite, delete. The overwrite must be refused with the file's contents intact;
- through the main share: the connection is refused;
- TrueNAS accepts a second share over a folder inside the first.

Until it passes, nothing in L3 is built. If it fails, the choice is B against C.

**Since 22 Nov 2026.** Syndra's key holds `READONLY_ADMIN`, which reads share permissions, so Syndra can see whether step 4's entry is there and raises a finding when it is not (§5.9). No extra role is asked for. No TrueNAS rule was found against a share inside another share. Whether a read-only share refuses an overwrite before the file is emptied is still UNVERIFIED and is the test's key check.

**If A1 is chosen:** the member's Closing body and the notice each gain one sentence naming the read-only share ("Open "{share}" instead of your usual folder."), read from TrueNAS, never typed into Syndra. A home-only loss (§7.6) has no mechanism, because that member keeps their member groups; it stays gated.

## 2 · What sends itself

A queued change sends itself when it **only takes away**: compared with what was last applied, it adds no group, raises no quota, turns nothing on, and creates no account or home. That now covers a share's group leaving, the move to Closing, and the switch-off. Today only the bare switch does (`backend/internal/services/entitlement.go:507-526`, `backend/internal/db/propagations.go:311`).

Unchanged: a role marked "always waits" still waits for Send; a change that gives anything waits for Send; after TrueNAS refuses a change, the next one waits for Send (tasks 37.7b).

## 3 · What the member reads

Dates follow the copy guide: day and short month with the distance, "21 Dec (in 30 days)". `{system}` is the system's own name. A leaver cannot sign in, so they never read these; they get the notice in §5.2.

| State | Title | Body, ending on the next step |
|---|---|---|
| Closing | Your {system} account is closing | You can open and copy your files until 21 Dec (in 30 days). Nothing can be saved, changed or deleted, and you no longer reach any shared folder here. After 21 Dec the account is switched off. Copy what you want to keep before then. |
| Switched off, kept | Your {system} account is switched off | Your files are kept until 20 Jan (in 30 days). After that makerspace staff can delete the account. To get your files, ask makerspace staff before then. |
| Due | Your {system} account is switched off | Your files were kept until 20 Jan. Makerspace staff can delete the account any day now. To get your files, ask makerspace staff now. |
| Deleted, no role | Your {system} account was deleted | It was deleted on 22 Jan. Makerspace staff hold your folder until they remove it. Nothing for you to do. |
| Deleted, a role again | (today's `waiting`: "Your account is on its way") | unchanged |
| Paused, with a clock | (today's hold card) | today's words, then: Your files are kept until 20 Jan (in 30 days). |
| Home closing (§7.6) | Your home folder is closing | None of your roles gives a home folder any more. You can open and copy it until 20 Jan (in 60 days). Nothing new can be saved there. Your shared folders are unchanged. Copy what you want to keep before then. |
| Staff can read the home (any state; one sentence added to the card) | | Makerspace staff can open your home folder since 22 Nov, to copy files for you. |
| Two people on one account | Your {system} account needs staff | Makerspace staff need to sort out your account. Nothing for you to do. |
| {system} cannot be read | (the §13.1 line) | {system} can't be read right now. Nothing for you to do. |

A share that leaves simply leaves the list of places. No sentence.

## 4 · What staff read on a person's row

"Accounts Syndra manages" (`ui/src/components/targets/PeopleOnTarget.tsx`), Status column. The Since column keeps its meaning.

| State | Status | Row actions |
|---|---|---|
| Active | Active | today's |
| Closing | Closing · files open until 21 Dec | Change dates · Switch off now · Revoke · Give staff access |
| Switched off, kept | Switched off · can be deleted from 20 Jan | Change date · Give staff access |
| Due | Due for deletion since 20 Jan | Give staff access (deleting is in the Due card) |
| Paused | put on hold by {name}; with a clock, followed by "· can be deleted from 20 Jan" | today's, plus Delete and start over when a role still reaches the system |
| Conflict | Two people are mapped to this account | Stop tracking (today's) |

The cause shows as the row's second line, one of: "deactivated in Zitadel", "deleted in Zitadel", "holds no role that reaches {system}".

A step that is late shows what TrueNAS reports, then: "Due to switch off since 22 Dec. Syndra is retrying." (`RetryStatus`.)

## 5 · The staff surfaces

All on the system's own page (`ui/src/components/targets/TargetOverview.tsx`), built from parts that exist: the table and row of `PeopleOnTarget`, the card of `DormantAccounts`, `SettingRow`, `SortHead`, the dialog, `ActionOutcome`, `Notice`, `RetryStatus`. Nothing new is drawn. Names in lists are alphabetical where order means nothing; these three lists have an order that means something and say so in their sort header.

### 5.1 The Overview line

One line among the system's facts: "3 closing, first on 21 Dec · 1 due for deletion · 2 folders to remove". Each part links to its card. A part with zero is left out; with all three at zero the line is absent.

### 5.2 Closing (new card)

- Columns: Person · Why · Files open until · Can be deleted from · Copy notice. Soonest *files open until* first.
- Card note: "Syndra tells nobody. Copy the notice and send it yourself."
- Empty: "Nobody's account is closing. Nothing for you to do."
- **Copy notice** puts this plain text on the clipboard. It leaves Syndra, so its dates carry the year:

  > Your makerspace storage account ({username} on {system}) is closing. You can open and copy your files until 21 Dec 2026. Nothing can be saved, changed or deleted. Connect with your storage password from the makerspace network (on campus or through NetBird **[TBD-B: drop "or through NetBird" for a leaver if they lose it]**). If you can't connect, or you have forgotten the password, reply and ask for a copy of your files. From 20 Jan 2027 the account can be deleted.

- After the click the button reads "Copied" for two seconds in a width reserved for both words, and a polite live region says "Copied. Send it to them yourself."

### 5.3 Change dates (dialog, from a row)

- Two native date fields: "Files open until" and "Can be deleted from". The first cannot be before today; the second cannot be before the first. On a Switched off row only the second shows.
- One line under them: "Each date includes its whole day." A text button "Use this system's settings" puts both back.
- Button: "Save dates". Outcome: "Saved. Files open until 28 Dec, can be deleted from 27 Jan. Nothing for you to do."
- This is the one write place for a person's dates. It needs no reply from TrueNAS.

### 5.4 Switch off now (confirm, from a Closing row)

- Title: "Switch off {name}'s account now?"
- Body: "They can no longer open their files on {system}. A connection already open ends when it next reconnects. The files are kept, and the account can be deleted from 20 Jan."
- Button: "Switch off now". It sets *files open until* to yesterday; *can be deleted from* becomes today plus "Then kept for" unless staff had set it by hand.
- Outcome: "Switched off. Nothing for you to do until 20 Jan." If {system} cannot be reached: "Recorded. Syndra switches it off as soon as {system} can be reached. Nothing for you to do."

### 5.5 Settings (two rows in the system's settings card)

- "Files stay open for" 30 days · "Then kept for" 30 days. Whole days, 0 to 365.
- Help, once for both: "When someone leaves, or loses their last role here, their files stay open for copying, then the account is switched off and kept, then it is due for deletion. A change applies to accounts that start closing from now."
- Held by the backend, not the add-on, so a date never depends on TrueNAS being reachable.
- A third row, "Staff group": the group on {system} that staff accounts belong to, picked from its groups in alphabetical order. Nothing is assumed about its name. Until it is set, Give staff access is not offered, and a deleted home passes to root alone.

### 5.6 Due for deletion (replaces "No longer needed", `ui/src/components/targets/DormantAccounts.tsx`)

- Rows are accounts in Due only. Columns: Person · Account · Why · Due since · Holds. Longest due first. "Holds" is a size, or "can't see how much" (as today).
- Empty: "No account is due for deletion. Nothing for you to do."
- To act: tick rows, tick the acknowledgement, type the key that can delete (per call, as today), press "Delete 2 accounts".
- Acknowledgement: "Delete 2 accounts. This cannot be undone. Each home folder passes to root and the staff group, and stays on {system} until someone removes it there."
- Outcome per row:
  - "Deleted. Its folder is listed under Folders to remove on {system}."
  - "Deleted. It had no home folder. Nothing for you to do."
  - "Stopped at step 2 of 4: the folder could not be handed to root. Nothing after that step was done. Try again."
  - "{system} has not answered. Don't send it again; the result appears here."
  - "Not deleted: {name} holds a role that reaches {system} again. Nothing for you to do."

### 5.7 Folders to remove on {system} (new card)

- Rows are folders Syndra handed to root and TrueNAS still reports. Columns: Folder · Was · Since · Size. Oldest first. A row leaves once TrueNAS reports the folder gone.
- Card note: "Syndra cannot remove a folder. Remove each one on {system} itself."
- Each row shows where staff open the folder, with Copy (§5.8). After a delete the staff group can already read it, so no further step is needed.
- When TrueNAS reports a snapshot task that covers the homes, a row adds "Once removed, it can be brought back from a snapshot until {date}." The date is read from TrueNAS. With no such task the line is absent and §5.9 raises the finding. Syndra only reads snapshots; it is never given the right to write them, because that right can also roll a whole dataset back.
- A row whose person has a new account queued or made adds: "{name} gets a home folder only after this one is removed."
- Empty: "No folder is waiting. Nothing for you to do."
- {system} unreachable: the last known rows, with "{system} can't be read right now, so this list may be out of date. Nothing for you to do."

### 5.8 Give staff access (from a row; owner, 22 Nov 2026)

Staff have no standing way into anyone's home. This gives the staff group **read** on one person's home, for one stated purpose: copying files out for them. Read and not modify, because the job is to hand over a copy, and a slip with modify could change or remove someone's only copy on a TrueNAS with no backups.

- Offered on a Closing, Switched off or Due row. After Deleted nothing is needed (§6 step 2 already gives it).
- Dialog title: "Give staff access to {name}'s home folder?"
- Body: "Everyone in {staff group} can open and copy the files in this one folder. They cannot change them. Syndra records that you did this."
- The operator pastes a {system} key that can change a folder's permissions, for this one action. It is used once and never stored, as with deleting (`backend/internal/handlers/inventory.go:487`, `DormantAccounts.tsx:253`). Syndra's own key cannot do this, on purpose: TrueNAS roles cover every folder, not one.
- Button: "Give staff access". Outcome: "Staff can read this folder. Open it with your own account at the path below. End staff access when you are done."
- The row then shows, as its second line: "Staff can read this home since 22 Nov, given by {name}", the path in both forms (`smb://…` and `\\…`) each with Copy, and the action **End staff access** (the same pasted-key pattern; outcome "Staff access ended. Nothing for you to do."). The address in the path is read from the system's configuration.
- What the row says is what the folder's permissions report on TrueNAS, not Syndra's record. If the person becomes Active again while staff can still read their home, §5.9 raises it.
- Audit: one record per grant and per end, with who, when and which home.
- Unreachable or unknown outcome: as for deleting (§7.7).

### 5.9 What Syndra notices about the setup (small; read-only)

Syndra reads {system}'s setup with the key it has and raises a finding in the system page's existing findings list. Each ends on its next step and leaves once TrueNAS reports it fixed.

| Finding | Words |
|---|---|
| The closing rule is missing | "Closing accounts can still change their files: the main share does not deny {closing group}. Add that entry on {system}." (only once §1a's A1 is chosen) |
| The audit list misses a group | "{system} does not record what {groups} do in shared folders. Add them to the share's audit watch list on {system}." |
| No snapshot protects the homes | "No snapshot task covers the home folders, so a removed folder cannot be brought back. Add a daily snapshot task on {system}." |
| Staff can read an active member's home | "Staff can still read {name}'s home folder. End staff access." (links to the row) |

## 6 · The delete sequence

One call per account, on the key the operator types for that call. The backend checks, the add-on acts in order, and each step is safe to run again.

0. **Backend, fresh read.** Refuse unless: the state is Due (or the call is "Delete and start over", §7.1); TrueNAS reports the account switched off; the binding names one person; no role reaches the system on a fresh resolve. Unreachable: refused at once, nothing sent (§13.4).
1. **Clear the quota row** for the account's number.
2. **Hand the home to root and the staff group** (owner, 22 Nov 2026): owner root, group the staff group with read, nobody else, the whole tree. Staff can then open it through the share until someone removes it on TrueNAS. The path comes from the binding and the home place, never from the request. Skipped when there is no home; with no staff group set, the group is root.
3. **Delete the account** with its personal group (TrueNAS keeps the group if anyone else is in it).
4. **Forget the binding** in the add-on; then the backend forgets its copy and merge base, marks the record Deleted with who and when, and records the folder's path for §5.7.

A failure stops at its step and reports the step; nothing later runs, and nothing retries on its own. An unknown outcome is "unconfirmed" and settles through the call's own read. A second run skips the steps already true.

## 7 · Edge cases

1. **Delete and start over** (forced). Offered only on an account that is switched off while a role still reaches the system, so it is always two deliberate acts: put on hold or revoke, then this.
   - Title: "Delete {name}'s account and start over?"
   - Body: "The account {username} is deleted. This cannot be undone. Its home folder is handed to root and stays on {system} until it is removed there. None of its files move to the new account."
   - Then one of: "A new, empty account is queued. It waits in Pending changes for you to send. Remove the old folder on {system} first, or the new account gets no home folder." or "No new account is made while {name} is on hold. Lift the hold when you want one."
   - The operator types the account's name and the key. Button: "Delete and start over". It runs §6 without the date check and never lifts a hold.
2. **Stop tracking.** It is never a removal. The confirm gains, when it applies: "{name} still holds a role that reaches {system}. Syndra will queue an account for them again and ask you to adopt this one." and, on a Closing, Switched off or Due row: "Syndra forgets this account's dates. It stays on {system} exactly as it is."
3. **Deleting while a role still reaches the system.** Refused everywhere except 7.1 (step 0).
4. **Coming back.** Before Deleted: the same account and home; groups and quota return on Send and the record is cleared. From Closing the password still works. From Switched off or Due the member sets a new storage password (today's `restored`). After Deleted: a new account with a new number; its home is made only once the old folder is gone (§5.7).
5. **An outright Zitadel delete.** The cause is `left`, as for a deactivation, on a successful read (§1).
6. **Only the home's role is lost, others stay.** Saving closes on the home at once (sends itself). The home has one date: *open until* = first seen plus both settings (60 days). Then it is Due, in §5.6 with Why "no role gives a home folder", and staff confirm hands the folder to root. The account and shares are untouched. There is no switched-off phase: a home cannot be locked while its account is on. Not gated on datasets: a user quota is per dataset and is set only on the home place's own dataset (`addons/truenas/home.go`, `quotaOwed`; checked by the session 21 Nov 2026), so closing saving on a home never touches a share.
7. **{system} unreachable mid-step** (`openspec/changes/addon-contract/design.md` §13). Closing and switch-off are queued work: they wait, spend no attempt, and show their retry. Change dates and Switch off now are backend writes and always succeed. Delete and the folders read are halting calls: refused at once with nothing sent, or "unconfirmed" when the outcome is unknown.
8. **{system} paused** (draining or read-only). Queued changes wait until staff set it active, with today's banner.
9. **Two people on one account.** No dates are stamped, nothing is sent and nothing can be deleted for either until staff stop tracking one. The dates start when the conflict clears.
10. **Closing is not enforced for a home.** Whatever §1a's chosen mechanism is, the add-on reports whether it is in place. Where it is not, the row says "Closing · files open until 21 Dec · the files can still be changed" and the member's body drops "Nothing can be saved, changed or deleted".
11. **A role marked "always waits".** Its Closing change waits in Pending changes; the dates run regardless, and the row says "waiting for you to send".

## 8 · Copy that becomes false, and the contract delta

**Becomes false (change in the slice named):**

| Where | Today | Slice |
|---|---|---|
| `ui/src/components/kinds/storage/MyStorage.tsx:219-225` | "no account here for you yet", shown to someone with a switched-off account. Wrong today | L1 |
| `RevokeDialog.tsx`; `openspec/NEXT.md:54` | does not say the account is switched off, not deleted, and the files stay. Missing today | L1 |
| `TargetOverview.tsx:700-760` | Stop tracking confirm omits what follows when a role is held | L1 |
| `MyStorage.tsx:560-564`; `backend/internal/handlers/my_storage.go:448-456`; `addons/truenas/home.go:21-22` | "Syndra never moves or deletes a home folder" | L3 (reword), L7 |
| `MyStorage.tsx:228-231` | hold copy has no date for a person with a clock | L3 |
| `addons/truenas/server.go:359-393` | "It stays off while the person … is on hold or deactivated": a deactivated person is now Closing first | L3 |
| Pending changes, rows that send on their own | a departure row reads as a switch-off; it now closes the account | L3 |
| `PeopleOnTarget.tsx:102` | card note names only hold and revoke | L4 |
| `DormantAccounts.tsx:326-332`; `backend/internal/services/dormant.go:45-49`; the operation's registry text "Remove a disabled account" | "No longer needed", "cannot be brought back. They held X", and still-a-member rows that can never be selected | L5 |

Stale, reported and not edited here: `openspec/changes/addon-platform/design.md:228-230` says the backend injects a stored second key; the code has the operator type it per call. `openspec/NEXT.md:342` (home-folder policy, open) is answered by this document. Tasks 37.1 to 37.7b describe departure as an automatic switch-off; L3 changes that.

**Contract delta (add-on `addons/truenas`, manifest in `capabilities.go:165-212`):**

- **The Closing write is pending §1a.** `home_quota: closed` is withdrawn (L0). Option A1 adds no write: the resolver's group set for a Closing person is the one closing group. A status read reports whether the read-only share exists over the home place and whether the main share denies that group; the key's `READONLY_ADMIN` role reads both. Option B adds `filesystem.chown` and `filesystem.setperm` to the everyday key.
- **A group set that drops every member group on an enabled account** must leave only what it names, plus the personal and built-in groups. **UNVERIFIED** that it does today; L3 proves it with a test.
- **`account.purge` becomes the four ordered steps of §6** and now refuses an account that is not switched off. On the typed key's session only, two TrueNAS methods join the allowed list: `filesystem.chown` and `filesystem.setperm`, confined to the bound account's home under a home place. The everyday key's write list does not change. The reply carries each step's outcome and the folder left.
- **New operation `home.staff_access`** (operator scope, pasted key, §5.8): give or end the staff group's read on the bound account's home, the whole tree. On that key's session only, `filesystem.setacl` joins the allowed list, confined as above. Step 2 of §6 uses the same write for the group's read.
- **New read `setup.get`** (operator scope, §5.9): the share's permissions, its audit watch list, the snapshot tasks over the home place, and each managed home's staff-group entry. Reads only, all within `READONLY_ADMIN`.
- **New read `folders.get`** (operator scope): for paths the backend recorded, whether each still exists, its owner and its size. Built on the stat read already allowed.
- **New operation `home.set_aside`** (operator scope, typed key): step 2 of §6 alone, for §7.6. Last slice.
- **No date crosses the contract.** The add-on receives desired state only; the record, the two settings and the clock are the backend's (next migration: 000104).

---

## For the build

- **Builder:** Opus for L2 to L7 (resolver, runner gate, add-on steps, destructive path). Sonnet for L1 (copy only).
- **Branch:** `feat/addon-contract-storage-lifecycle` · **replaces:** departure as an automatic switch-off (tasks 37.1 to 37.7b), the "No longer needed" card and its still-a-member refusal, and the sentence "Syndra never moves or deletes a home folder".
- **The comp:** none. Owed before L4: one prototype of §5.2 to §5.7 from existing parts, captured at 1440 and 390 in both themes.
- **Slices, smallest first. Each is its own PR and review.**
  - **L0 · the staging test.** Done 21 Nov 2026; results in §1a. Owed before L3: the owner's choice in §1a and, for option A1, its one-time setup and the test listed there. Part B is still unrun.
  - **L1 · what is already wrong** (copy): the three L1 rows of §8.
  - **L2 · a share that leaves sends itself:** the "only takes away" rule of §2.
  - **L3 · the record, Closing and switch-off** (after the owner's choice in §1a): migration 000104, the two settings, the state function, the chosen Closing write, leavers and last-role loss to Closing, the directory rule, the rollout rule, §3 and §4 words.
  - **L4 · staff controls:** §5.1 to §5.5.
  - **L4a · what Syndra notices about the setup:** §5.9 and `setup.get`. Reads only.
  - **L5 · delete:** §6, §5.6, §5.7, `folders.get`, the Deleted words.
  - **L5a · give staff access:** §5.8, `home.staff_access`, its audit records, the member's sentence in §3.
  - **L6 · delete and start over:** §7.1.
  - **L7 · home-only loss:** §7.6 and `home.set_aside`, once its Need is answered.
- **Files to touch:**
  - `backend/internal/services/entitlement.go` (`applyDirectoryStanding` :264, `WithdrawsOnly` :507): the state function and the rule of §2.
  - `backend/internal/db/propagations.go:311-354`: the runner takes every change that only takes away.
  - `backend/internal/services/deps.go:351`: a failed directory read is never "gone".
  - `backend/internal/services/dormant.go`, `backend/internal/handlers/inventory.go:500`: eligibility is Due; step 0 of §6.
  - `backend/internal/handlers/my_storage.go`, `ui/src/components/kinds/storage/state.ts:31`, `MyStorage.tsx:78`: the states and words of §3.
  - `addons/truenas/capabilities.go`, `home.go`, `operations.go:558-614`, `server.go:359-393`: §8's delta.
  - `ui/src/components/targets/PeopleOnTarget.tsx`, `DormantAccounts.tsx`, `TargetOverview.tsx`, `RevokeDialog.tsx`, `SettingRow.tsx`: §4 and §5.
- **Tokens:** existing only. No new token, no light.
- **Layout:** at 1440 each card is the existing table, dates right-aligned in tabular figures, row actions at the row's end. At 390 each row stacks label over value and its actions sit on their own line below; nothing scrolls sideways. A row's actions never change place when its status changes.
- **States:** rest, empty, unreachable and late are worded in §3 to §5. Loading: the card's existing skeleton rows; no date is drawn before it is known. Partial (some rows unreadable): the readable rows, then "Some accounts could not be read. Nothing for you to do; Syndra reads again shortly." Error on a write: the part's own outcome with "Try again."
- **Keyboard:** Tab order is row, then its actions in reading order. Enter or Space on Copy notice copies and keeps focus on the button. Dialogs open with focus on the first field (Change dates) or on Cancel (Switch off now, both deletes); Esc closes and returns focus to the row's button; after a successful write focus goes to the outcome.
- **Motion:** none new. Outcomes use the shared parts' own. Reduced motion and forced colours: as those parts already behave; status is always a word, never colour alone.
- **Words:** §3, §4, §5 and §7 exactly. `{system}` through `targetLabel`; never "NAS", "target" or "purge" on screen.
- **Never:**
  - never a date before the cause is confirmed by a successful read;
  - never "deleted" for files (the folder is handed to root and kept);
  - never "nothing can be saved" unless TrueNAS reports the chosen mechanism in place;
  - never a quota used to stop saving (L0: it empties a file that is saved again);
  - never a switch-off triggered by Zitadel's "locked" alone;
  - never a delete without the fresh read of step 0, and never retried on its own;
  - never a pasted key stored, logged or reused for a second action;
  - never staff access to more than the one home named, and never modify;
  - never the right to write snapshots on Syndra's key;
  - never a raw number or ID as a person's label; a deleted account's row keeps the name it had.
- **Tests to add:**
  - the state function at each boundary day in the configured zone (the last open day, the first switched-off day, the first due day);
  - a share's group leaving sends itself; a change that adds anything, or a role that always waits, does not;
  - a leaver resolves to Closing with SMB on, no member groups and the chosen Closing write, and to the switch only on the day after;
  - a failed directory read stamps no date and makes nothing deletable;
  - a returning person before Deleted keeps the account and clears the record;
  - `account.purge` refuses an account that is on, runs its steps in order, stops at a failed step, and a second run skips finished steps; the chown path can only be the bound home;
  - the delete call is refused when a role reaches the system again;
  - give and end staff access each write one audit record, touch only the bound home, and leave no trace of the pasted key in any log or row;
  - the row's "Staff can read this home" line follows the folder's permissions as read, not the record;
  - each §5.9 finding appears when its condition is read and leaves when it is not;
  - copy guards for every sentence in §8's table, so the old ones cannot return.
- **Layout check:** extend the system page's rule to the Closing, Due for deletion and Folders to remove cards: no clipped date or action at 390, and Copy notice keeps its width in both labels.

## 9 · Owner decisions this rests on (21 Nov 2026)

**His brief:** share loss "should be taken away at that moment"; with a home folder "they should be able to read it till a specified date. after that maybe delete in a month or a specific time".

**Round 1.**

1. A change that only takes away sends itself; "always waits" roles still wait. **Yes.**
2. Who gets Closing. **Changed by him:** leavers get it too. Departure and an outright Zitadel delete go Active → Closing → Switched off, kept → Due → staff confirm → Deleted.
3. Hold and Revoke stay a full switch-off with no file access. **Yes.**
4. Closing means open and copy, never save; strict read-only waits. **Yes.** L0 then withdrew the quota that this rested on; the mechanism is open again in §1a.
5. 30 days and 30 days, as settings on the system's page, with per-person dates. **Yes.**
6. Deletion always needs a staff tick and confirm. **Yes.**
7. What "deleted" removes. **Yes, as:** the home is handed to root and kept for whoever looks after TrueNAS; homes stay plain folders.
8. No export feature; the window is the copy-out; staff copy for someone who cannot. **Yes.**
9. A forced re-create does not carry the old home over. **Yes;** round 2's question 7 settles what happens to the folder.
10. Notice is on the page and the Overview only; there is no mail path. **Yes.**

**Round 2.**

1. Home-only loss: the home closes for the full 60 days, then is Due. **Yes.**
2. Syndra tells nobody; the Overview lists who is closing and offers Copy notice. **Yes.**
3. The storage password keeps working through Closing; staff never set one. **Yes.**
4. Off campus. **Changed by him:** NetBird already counts as the makerspace network; the notice says "on campus or through NetBird". Whether a leaver keeps NetBird is being checked (TBD-B).
5. A leaver loses every share at once; their files in shared folders stay with the share. **Yes.**
6. Fast stop. **Changed by him:** no automatic stop on Zitadel's "locked". "removing read access to home directory immediately is the rare case and should have a manual button for that to admin".
7. After deletion the folder stays under root at its path, is listed for removal, and a new account's home waits for it to go. **Yes.**
8. One reversible quota test on a disposable account on the real TrueNAS, before the build. **Yes.** Run 21 Nov 2026; results in §1a. He skipped part B.

**22 Nov 2026.**

- **Staff access to a member's home: "option 1, paste per action".** One home at a time, with a key the operator pastes for that action and Syndra never stores; every grant recorded. He rejected standing staff access to every home (privacy) and Syndra holding the stronger key (TrueNAS roles cover every folder).
- **A deleted home passes to root and the staff group with read,** so staff can still reach it through the share until it is removed.
- **Mine, for him to confirm:** read and not modify; an "End staff access" action; one sentence telling the member that staff can open their folder; the staff group as a setting, not a fixed name.
- **Recommended to him by the session, not yet decided:** a daily snapshot task over the homes, kept 30 days. Syndra reads it if it exists.
- **Decided, 22 Nov 2026:** yes to the four additions above (staff access is read, not modify; "End staff access" plus a finding; a member sentence; the staff group as a setting).
- **Decided, 22 Nov 2026:** yes to Closing via the read-only share (§1a A1), with one disposable test session before L3.
- **Decided, 22 Nov 2026:** option 1, paste per action, for staff access.

**Standing (for every screen), in my reading:** the immediate stop is always a manual staff act, never automatic; deletion always needs a staff confirm; Syndra never says files are deleted when a folder is only set aside. **Local to storage:** the two windows, Copy notice, the three cards.
