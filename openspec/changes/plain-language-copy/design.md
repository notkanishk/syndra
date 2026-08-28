# Plain-language copy — the writing guide

Every sentence Syndra shows is read by someone who runs a makerspace, or by
someone who wants to use one. Neither has studied identity management, and
neither should have to. This document is the rule for all text in the product:
what it is for, how it sounds, which words it uses, and what every page,
action and message must contain.

It is a contract, not advice. `ui/src/components/ui/__tests__/plain-language.test.ts`
checks the parts of it a machine can check. The rest is checked in review by
reading the sentence aloud to somebody who does not work here.

---

## 1. Who is reading

**Members** — students, faculty, visiting researchers. They come here to see
what they can use, to ask for more, and to reach network storage. They may
open Syndra twice a term. Nothing they see may assume a previous visit.

**Makerspace staff** — the people who decide who may use what. They know the
space, the machines, and the people. They do not know what an identity
provider is, and they should not need to learn in order to approve a request
or take a lapsed member off the laser cutter.

**The person who runs the server** — one or two people who installed Syndra.
They may read a terminal command. Text for them is marked as for them, and
placed where staff can step over it.

Write for the first two. Address the third by name — *"for whoever runs the
Syndra server"* — so everyone else knows the paragraph is not theirs.

## 2. Register

Syndra belongs to a college. Its voice is the voice of a well-run university
office: courteous, unhurried, exact. It respects the reader's time and their
intelligence, and it never performs either.

- **Calm.** No exclamation marks. No urgency the situation does not have. A
  queue of forty is "forty changes waiting", not "40 changes need your
  attention now!"
- **Courteous, not servile.** No "please", no "sorry", no "oops". Courtesy is
  shown by being clear, by saying what will happen before it happens, and by
  never blaming the reader for what the software did.
- **Exact.** A number has a unit and a referent. A time says when. A
  consequence names the person it falls on. "Some of these may not apply" is
  not a sentence Syndra writes; "3 of the 12 already hold this role and will
  not change" is.
- **Plain.** Short words where they exist. The reader should never feel a
  sentence was written to sound authoritative.
- **Never clever.** No wordplay, no arch asides, no jokes. Cleverness costs the
  reader a second reading and gives them nothing for it.
- **Never lecturing.** Design rationale belongs in code comments and in this
  document, not on screen. A sentence that begins "This section keeps its
  seat so that…" is talking to the developer.
- **Second person, present tense.** "You can revoke this" rather than "The
  operator may revoke this". "Syndra sends the change" rather than "the change
  will be dispatched".
- **Whole sentences.** A label may be a noun phrase. Anything longer is a
  sentence with a full stop, and reads as one.

## 3. Shape of a sentence

1. **Consequence before mechanism.** Say what happens to the person first;
   say how Syndra does it only if the reader needs it to decide.
   *Not:* "The row is enqueued to the outbox and dispatched on the next drain."
   *But:* "Their access ends within about a minute."
2. **Name who acts.** "Syndra creates the account." "You decide." "TrueNAS
   refused it." Passive voice hides the actor, and the actor is usually the
   thing the reader most wants to know.
3. **One idea per sentence.** Two consequences are two sentences.
4. **No stacked negatives.** "Cannot be undone" is one negative and fine.
   "Not unless nobody has not…" is not written.
5. **Every pronoun has a visible referent.** "It" and "this" refer to a noun
   in the same sentence or the one before. When in doubt, repeat the noun.
6. **Say what did *not* happen.** Every refusal and failure ends with the
   state of the world: "Nothing was changed." Every partial outcome says
   which part. The reader must never have to guess whether to try again.
7. **Say what to do next**, when there is something to do — and say where.
   "Ask makerspace staff" is a next step. "Contact an administrator" is not.
8. **Numbers carry their unit and their whole.** "3 of 12 people", "2.4 GB",
   "in 14 days", never a bare "3".
9. **Times are absolute and relative together** where the gap matters:
   "12 Sep (in 14 days)". A relative time alone rots; an absolute one alone
   makes the reader do arithmetic.
10. **A system is named by its name.** TrueNAS, Zitadel, Google. Never "the
    target", "the provider", "upstream". If the name is not known, say "the
    connected system".

## 4. What every page carries

Every page has a **title** and a **lede** — one or two sentences under the
title that answer, in this order:

1. what this page shows,
2. when you would come here,
3. for a queue or review list: what happens if you do nothing.

The third is the sentence most often missing and most often needed. Ignoring
*Expiring access* ends someone's access; ignoring *Holds due* keeps it
blocked; ignoring *Pending changes* means nothing is sent. Each of those pages
says so in its lede, in the same breath as what the page is.

The lede lives in `PageHeader`'s `lede` prop. `meta` is for metadata — a
count, an email, an identifier — and never for a sentence. A page with no lede
fails the guard.

Every **section** has a heading of five words or fewer that names the thing
listed, not the mechanism that produced it. "Not created by Syndra", not
"Unvouched bindings".

Every **empty state** says what would put something here. "No requests
waiting. Requests appear here when a member asks for access." Never "Nothing
here" alone.

Every **term this document lists as glossed** is glossed on its first
appearance on the page, in a parenthesis or a short following clause, in the
words section 6 gives. Once per page, not once per product: a member who
opens *Network storage* has not read *Bundles*.

## 5. What every action carries

**The button names its object.** "Revoke this role", "Send 12 changes",
"Approve for Priya". Never "OK", "Go", "Submit", "Confirm" alone. Out of
context — read by a screen reader from a list of buttons — the label must
still say what it does.

**The consequence comes before the click.** A line above or beside every
button that changes something, stating what will be true afterwards and for
whom. Destructive actions get a short titled list, *What happens*, with one
bullet per consequence and the alternative named beneath it.

**Confirmation follows the three rungs** (`one-control-surface`), and the copy
of each rung is fixed:

- *Rung 1* — the button carries the number: "Apply to 12 people".
- *Rung 2* — a sentence to tick, with the number inside it:
  "I understand this revokes access for 12 people."
- *Rung 3* — type the name. The label says what unlocks: "Type **Laser
  Cutter** to confirm. The button below unlocks when it matches."

**A preview is called a preview.** Every dialog that previews before applying
opens with the same sentence: *"Syndra first shows exactly what would change,
person by person. Nothing changes until you press Apply."*

**The outcome is a sentence, not a status.** What happened, to how many, and
what did not. "Applied to 9 people. 3 already held this role and were left as
they were." A failure ends with "Nothing was changed." A partial outcome names
both halves.

**A queued outcome says where it went and what moves it.** "Recorded in
Syndra and waiting to be sent to Zitadel. Nothing has changed there yet; send
it from *Pending changes*."

## 6. Vocabulary

One word for one thing, everywhere. The table is the product's glossary; the
gloss column is the sentence used on first appearance on a page.

### Systems

| Say | Gloss on first use per page | Never say |
|---|---|---|
| **Zitadel** | the service everyone signs in through | identity provider, IdP, the provider, upstream, downstream, the directory, the catalogue |
| **TrueNAS** (or the system's own name via `targetLabel`) | the network storage server | target, the NAS, the add-on (*except* on *Connected systems*, where the add-on — the small program that connects Syndra to the system — is explained once) |
| **connected system** | a system Syndra creates and manages accounts on | target, integration |
| **Syndra** | — | the app, the system, the tool |
| **Google** | the makerspace's Google account | Workspace, the IdP chain |

### People

| Say | Meaning | Never say |
|---|---|---|
| **person / people** | a human, in staff-facing copy | user, subject, principal, holder (as a noun), account (for a human) |
| **you** | the reader, in member-facing copy | the member, the user |
| **member** | someone who belongs to the makerspace; the audience name | — |
| **makerspace staff** | the people who manage access, in member-facing copy | operator, steward, admin, lab manager, whoever runs the makerspace |
| **the person who runs the Syndra server** | the installer, in copy meant for them | administrator, sysadmin, ops |
| **account** | a person's account *on a connected system* ("their TrueNAS account") | — |

The word *operator* names a view and an audience in the code and in the
specs. It does not appear on screen.

### Things a person can have

| Say | Gloss on first use per page | Never say |
|---|---|---|
| **access** | what a person can use | grant (noun), entitlement, permission (staff copy) |
| **role** | one named kind of access, inside a project | grant, claim |
| **project** | a machine or area with its own set of roles — the Laser Cutter, the Studio | org, boundary, Zitadel project |
| **app** | something people sign in to — the booking site, the badge reader | application, client, OIDC client |
| **bundle** | a set of roles given together | — |
| **automatic rule** | if someone holds one role they also get another, with nobody clicking | policy, mapping rule |
| **mapping** | which TrueNAS group a role gives | binding (in this sense) |
| **hold** | a block on someone's access with a date to look at it again | withheld access (as a noun), suspension |
| **direct access** | access somebody gave this person by hand | direct grant, standalone grant |
| **via a bundle / automatic** | how the access arrived | source kind, carrier |

### Verbs

| Say | Meaning | Never say |
|---|---|---|
| **give** (access, a role) | make somebody hold it | grant (as the only verb — *grant* may appear where the noun is *access*: "grant access"), assign, provision |
| **revoke** | end a person's access. Glossed once per page: *revoke (end their access)* | remove access, withdraw access, take away, drop, retire access, deprovision |
| **withdraw** | a member takes back their own request | — |
| **remove** | take a thing out of a set — a role out of a bundle, a person out of a selection | — |
| **delete** | an object ceases to exist — a rule, a mapping, an account on TrueNAS | purge, sweep, drop |
| **retire** | a bundle is closed to new members and kept in history | delete (for bundles) |
| **preview** | see what would change before it does | rehearse, plan, dry run, simulate |
| **apply** | make the previewed change | commit, execute, run, fire |
| **send** | dispatch waiting changes to Zitadel or a connected system | drain, resume, flush, dispatch |
| **check** | compare what Syndra expects with what a system holds | reconcile, sweep, converge, sync |
| **bring accounts in line** | make a connected system match what people's roles say | converge, reconcile |
| **approve / decline** | decide a request | deny, reject |
| **extend** | move an expiry date later | renew, prolong |
| **lift** | end a hold | release, unblock |
| **put on hold** | begin a hold | withhold, suspend, pause |

### States

| Say | Meaning | Never say |
|---|---|---|
| **waiting to be sent** | recorded in Syndra, not yet at the system | queued, pending, in the outbox |
| **sent** | the system accepted it | applied, landed, dispatched |
| **failed** | the system refused it; Syndra will try again | errored, requeued |
| **given up** | Syndra tried its limit and stopped; a person must act | terminal, exhausted, out of retries |
| **no change** | already in that state | no-op, untouched |
| **refused** | Syndra declined to do it, and says why | blocked, rejected |
| **expires on / expired** | access with an end date | lapses, TTL |
| **on hold** | blocked by a hold | withheld, suspended, held |
| **unexplained** | access a person has that Syndra did not give | drift, out-of-band, unvouched |
| **not created by Syndra** | an account on a system that arrived some other way | orphan, foreign, unbound |
| **could not be read** | Syndra asked and got no answer | degraded, stale, unreachable (as a state word) |
| **read at 09:14** | when Syndra last looked | freshness, cached |

### Pages

| Nav label | What the lede says it is |
|---|---|
| **Home** | what needs you today |
| **People** | everyone Syndra knows, and what each can use |
| **Projects / Roles / Apps** | the things access is made of |
| **Requests** | what members have asked for |
| **Bundles** | sets of roles given together |
| **Automatic rules** | roles that follow from other roles |
| **Pending changes** | changes waiting for you to send |
| **Change history** | what each edit to a bundle or rule set off |
| **Access map** | how roles, bundles and rules connect |
| **Unexplained access** | access Syndra did not give |
| **Unfinished revocations** | access someone revoked that is still there |
| **Expiring access** | access that ends soon, and ends whether or not you look |
| **Holds due** | holds past their review date, which stay in force until you act |
| **Audit** | what people did in Syndra |
| **Zitadel** | the service everyone signs in through, and whether Syndra can reach it |
| **Connected systems** | the systems Syndra creates accounts on |
| **Zitadel events** | what Zitadel told Syndra, in order |

### Words that never appear on screen

Mechanism words. Each has a plain replacement above or is simply omitted:

`drain`, `resume` (of a queue), `write` (as a noun), `outbox`, `ledger`,
`cascade`, `propagate`, `propagation`, `reconcile`, `reconciliation`,
`converge`, `convergence`, `sweep`, `drift`, `rehearse`, `rehearsal`, `plan`
(as the preview), `target`, `upstream`, `downstream`, `identity provider`,
`IdP`, `subject`, `principal`, `entitlement`, `binding` (for ownership),
`cache compile`, `hydrate`, `payload`, `mutation`, `idempotent`, `fixture`,
`seeder`, `manifest`, `capability`, `truncated`, `degraded`, `terminal`,
`exhausted`, `fire` (of a rule), `hop`, `node`, `edge`.

A few technical words are allowed **with their gloss**, because the thing has
no plain name: `share` (a storage folder), `group` (on TrueNAS, what decides
which folders an account can open), `token` (what an app is handed when
someone signs in, listing their roles), `claim` (one field inside a token),
`role key` (the short name other systems see for a role), `API key` (a
password for a program rather than a person).

## 7. Members and staff read differently

The same fact is written twice when both audiences see it.

| Fact | To staff | To a member |
|---|---|---|
| a role held via a bundle | "Via Lab Tech (bundle)" | "Because you're in Lab Tech" |
| access expiring | "Expires 12 Sep" | "Ends 12 Sep — ask before then if you still need it" |
| a hold | "On hold until 3 Oct: safety refresher" | "Paused. Makerspace staff will look again on 3 Oct. If you think this is a mistake, ask them and mention the reason shown here." |
| a queued change | "Waiting to be sent to TrueNAS" | "Your storage access is on its way. It usually takes a few minutes." |
| Zitadel | "Zitadel (the service everyone signs in through)" | "your makerspace sign-in" |

Member copy names a person to ask and never a mechanism to understand. Staff
copy may name the mechanism *after* the consequence, when it helps them decide.

## 8. Errors and system failures

- Say which thing failed and which did not: "TrueNAS did not answer. Syndra
  itself is fine, and nothing was changed."
- Say the state of the world: "Nothing was changed." "Your request was
  saved; the confirmation did not send."
- Say the next step, and where: "Try again in a minute. If it keeps failing,
  ask the person who runs the Syndra server and quote the reference below."
- The reference is called a **reference**, is shown in full, and carries the
  line "Quote this if you ask for help."
- Never show a stack, a code path, an HTTP status, or a raw identifier as the
  message. Those may sit under a "Technical detail" disclosure for the person
  who runs the server.

## 9. Accessibility copy

- A button's visible text is its accessible name. `aria-label` is used only
  for an icon-only control, and then says the whole action: "Copy your
  account name", not "Copy".
- Colour never carries a meaning alone. Every dot, tint and badge has a word
  beside it or a visually-hidden one inside it.
- Status regions (`role="status"`, `role="alert"`) always contain a sentence.
- A field's label is its name; its hint is a separate element (`FieldHint`),
  never inside the label. Errors are associated with the field they concern
  and say what to change.
- Links say where they go: "Open Pending changes", never "here" or "→" alone.
- Counts in links say what they count: "12 people", never "12 →".
- Placeholders never carry instructions. Instructions go in the hint.

## 10. What the guard checks

`plain-language.test.ts` reads every component's user-facing strings — JSX
text, and the string values of `title`, `label`, `lede`, `meta`, `guidance`,
`placeholder`, `aria-label`, `note`, `hint`, `consequence`, `subject` — and
fails on:

1. any word from the *never appear on screen* list;
2. a `PageHeader` with no `lede`;
3. a sentence in `meta`;
4. an exclamation mark;
5. "please", "sorry", "oops";
6. a bare "Dismiss", "OK", "Submit", "Confirm", "Go" as a whole button label;
7. `aria-label="Copy"` or any aria-label that is a single verb.

Exceptions are argued in the test's `ARGUED` map, one line of reason each,
and the map is the record of every place the product chose to break its own
rule.

The guard cannot check tone, referents, or whether the consequence is stated.
Those are checked by the one test that matters: read the sentence to somebody
who has never seen the product, and watch whether they ask a question.
