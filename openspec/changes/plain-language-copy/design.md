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

**Makerspace staff.** They run an academic makerspace at a university. They
know what an identity provider is, they can name every product in their own
rack, and they configured most of it. Write for a competent colleague: they
do not need *token* explained, and a sentence that explains it anyway costs
them a clause and tells them you assumed otherwise.

What they cannot know from experience is the vocabulary **this product
invented** — a cascade, the outbox, a merge finding, a log anchor, what
Syndra means by adopting an account. Those are not difficult, they are
simply local, and the only place they are ever explained is here.

**Members.** Students, faculty, visiting researchers. They come to see what
they can use, to ask for more, and to reach storage. They may open Syndra
twice a term, and they are not administrators of anything. They get the plain
register — but they get the real names of the systems they actually use, TrueNAS
and Zitadel among them, because a student who cannot mount a share is going to
type the product's name into a search box, not "the network storage server".

**The person who runs the server.** One or two people. Text for them is
marked as theirs — *for whoever runs the Syndra server* — so everyone else
knows the paragraph is not.

The register differs by audience. The **facts** never do, and neither does
the vocabulary: one word per thing, for everybody.

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
- **Plain, but not simplified.** Short words where a short word is as exact —
  never a longer paraphrase in place of the right term. "The changes this edit
  set off" is not plainer than "cascade", it is vaguer and four words longer,
  and it leaves the reader without the word the rest of the product uses.
  Precision is the courtesy; §6 says where a term carries its definition with
  it. The reader should never feel a sentence was written to sound
  authoritative, nor that it doubted them.
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

One word for one thing, everywhere, and the precise word in preference to a
paraphrase of it. Where the precise word is a term of art, it carries its
definition with it rather than being replaced by a plainer, vaguer one.

### The mechanism

`lib/glossary.ts` holds every defined term, once. `<Term name="cascade">` sets
the word in the sentence and puts the definition one hover, tap or Tab away.

Mark up a term's **first appearance on each page**, and only the first. A page
that marks up every occurrence is a page of dotted underlines; a page that
marks up none has left a reader with nowhere to go. A member who opens
*Network storage* has not read *Bundles*, so "first use" is per page, not per
product.

Three kinds live in the glossary, and the difference decides how the copy
around them reads:

- **Products** — Zitadel, TrueNAS, Google Workspace. Use the bare name. Never
  gloss inline: no "Zitadel (the service everyone signs in through)". Marked
  up on first use so a new colleague in their first week has somewhere to look.
- **Standard terms** — grant, entitlement, provision, reconciliation, drift,
  claim, token, OIDC, SAML, role key, service account. The field's own
  vocabulary. Use it plainly. The definition is a courtesy, not a lesson.
- **Syndra's own** — bundle, automatic rule, cascade, outbox, drain, hold,
  mapping, merge finding, adopt, baseline, log anchor, unvouched, intent
  ledger. Nobody knows these from experience, however senior. Their
  definitions carry the consequence, not just the meaning, because they are
  the only explanation the product ever offers.

Adding a term to the glossary is how a word becomes permitted. A term of art
that is *not* in the glossary is not licensed by this section — it is
unexplained jargon, and the fix is to define it, not to paraphrase it away.

### One name per thing

These are not difficulty problems, they are consistency problems: the same
thing under two names reads as two things.

| Say | Not |
|---|---|
| **Zitadel** | the identity provider (as a name), the provider, the directory, the catalogue, upstream, downstream, IdP |
| **TrueNAS**, or the system's own name via `targetLabel` | the target, the NAS |
| **connected system** (the class) | target, integration |
| **person / people** | user, subject, principal, holder (as a noun) |
| **makerspace staff** — in member-facing copy | lab manager, steward, operator, whoever runs the makerspace |
| **revoke** — end a person's access | withdraw access, take away, drop, retire access |
| **withdraw** — a member takes back their own request | — |
| **remove** — take a thing out of a set | — |
| **delete** — an object ceases to exist | purge |
| **retire** — a bundle closes to new members | delete (of a bundle) |
| **preview** then **apply** | rehearse, dry run, simulate |
| **send** — dispatch what is waiting | flush |
| **decline** | deny, reject |
| **lift** — end a hold | release, unblock |

*Operator* names an audience and a view. It is correct on staff screens and
wrong in anything a member reads.

### States

One set of words for a change's life, everywhere it is reported:

**waiting to be sent** → **sent** → (**failed**, and Syndra will retry) →
(**given up**, and a person must act). Plus **no change** (already in that
state) and **refused** (Syndra declined, and says why).

### Pages

| Nav label | What its lede says it is |
|---|---|
| **Home** | what needs you today |
| **People** | everyone Syndra knows, and what each can use |
| **Projects / Roles / Apps** | what access is made of |
| **Requests** | what members have asked for |
| **Bundles** | sets of roles handed out together |
| **Automatic rules** | roles that follow from other roles |
| **Pending changes** | changes waiting for you to send |
| **Change history** | what each edit set off, and whether it landed |
| **Access map** | how roles, bundles and rules connect |
| **Drift** | access with no record explaining it |
| **Unfinished revocations** | access somebody revoked that is still there |
| **Expiring access** | access that ends soon, whether or not you look |
| **Holds due** | holds past their review date, still in force |
| **Audit** | what people did in Syndra |
| **Zitadel** | whether Syndra can reach it, and what it holds |
| **Connected systems** | the systems Syndra maintains accounts on |
| **Incoming events** | what reached Syndra from outside |

A page's title names **everything** the page shows, never one of its parts.
If a filter offers a source the title does not admit, the title is wrong —
that is how *Incoming events* spent a release called *Zitadel events* while
carrying onboarding triggers Zitadel never sent.

### Still never on screen

Not jargon — noise, or the developer talking:

`payload`, `mutation`, `idempotent`, `fixture`, `seeder`, `hydrate`,
`truncated` (say *cut short*), `degraded` (say what could not be read),
`manifest`, `node`, `edge`, `hop`, and any raw identifier where a name
resolves. No `please`, no `sorry`, no `oops`, no exclamation marks.

Design rationale never appears on screen. A sentence explaining why a section
keeps its seat is written for whoever maintains the code, and belongs in the
code.

## 7. Members and staff read differently

Same facts, same vocabulary, different register. A member is not an
administrator, so their copy names a person to ask rather than a mechanism to
understand — but it does not hide which system they are dealing with. A
student whose share will not mount searches for *TrueNAS*, not for "the
network storage server".

| Fact | To staff | To a member |
|---|---|---|
| a role held via a bundle | "Via Lab Tech (bundle)" | "Because you're in Lab Tech" |
| access expiring | "Expires 12 Sep" | "Ends 12 Sep — ask before then if you still need it" |
| a hold | "On hold until 3 Oct: safety refresher" | "Paused. Makerspace staff will look again on 3 Oct. If you think this is a mistake, ask them and mention the reason shown here." |
| a queued change | "Waiting to be sent to TrueNAS" | "Your TrueNAS access is on its way. It usually takes a few minutes." |
| the sign-in service | "Zitadel" | "Zitadel" — marked up, so the definition is one tap away |

Members never meet *operator*, *drift*, *cascade*, *outbox* or *reconcile*:
those name work they do not do. They do meet the products they use, by name.

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

`plain-language.test.ts` reads every component's copy — JSX text and string
literals, through a tokenizer that knows where code and comments are, and that
reads a capitalised one-word literal as a label — and fails on:

1. a name from the *one name per thing* table's right-hand column;
2. a word from *still never on screen*;
3. a `PageHeader` with no `lede`;
4. a sentence in `meta`;
5. an exclamation mark, or `please` / `sorry` / `oops`;
6. a bare `Dismiss`, `OK`, `Submit`, `Confirm`, `Go` as a whole button label;
7. an `aria-label` that is a single verb;
8. a glossary term used on a page that never marks up any term — the
   definition is reachable from somewhere on that page, or the term is
   unexplained jargon again;
9. an `ARGUED` entry whose file no longer exists.

Exceptions are argued in `ARGUED`, one reason each, and that map is the record
of every place the product chose to break its own rule.

What it cannot check is whether a sentence is **true**. Every word in "Zitadel
events" was permitted vocabulary; the claim was false. A title that names part
of its page, a timing that does not match the scheduler, a sentence true in
one branch and false in another — those are found by reading the copy against
the code that produces it, and nothing here substitutes for that.
