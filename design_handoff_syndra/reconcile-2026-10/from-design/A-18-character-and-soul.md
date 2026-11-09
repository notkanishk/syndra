# A-18 · My access's header, and Syndra's character (draft, 5 Nov 2026)

Answer to `Q-18-character-and-soul.md`, which carries the owner's open questions from A-17 §3 Q3 and Q4. Board: `a18-character-and-soul.dc.html` (Proposed, 5 Nov 2026). The owner's choices are listed at the board's end and in §6.

**Read for this:**
- RULES.md (with A-17's G1–G19) and FINAL.md;
- the copy guide, `plain-language-copy/design.md`, especially §2 Register, §3 and §7;
- `MemberAccess.tsx`, `RecentChanges.tsx`, `AccountSheet.tsx`, `Sidebar.tsx`, `SyndraMark.tsx`, `Home.tsx` (greeting, weekday line, calm line), `not-found.tsx` and `error.tsx`;
- `LoginDoor.tsx`, and `zitadel/page.tsx` `TheName()`;
- the queues' empty states (`UnexplainedAccess.tsx:431`, `WithdrawnAccess.tsx:260`);
- `EmptyState`'s `bloom` and `lift`, and the motion tokens in `globals.css`.

**In one line each:**
- **My access:** the old count line becomes **one line that always says the one thing worth knowing**, picked from a fixed ladder. Its calm state is "Nothing needs you right now.", the same words as the operator's Home.
- **The story:** it lives in **About Syndra at the foot of the Account sheet**, the one place every person reaches at every width. The login line stays as its short form.
- **Character:** it comes from **exactness at the right moment, not from extra words**. The ranked ideas in §4 add only seven pieces of copy and one bloom, and none of them is a joke.

---

## 1 · Findings

### 1.1 My access, as built
- `MemberAccess.tsx:144`: the summary is a **sentence in `--faint`**. A-09 keeps faint for times, ends and captions, so a sentence belongs in `--muted`. It also counts "rooms" (A-17 Q3), and repeats each end date that the row already says (G12). **clash**
- `MemberAccess.tsx:157`: **Recent changes gives up its seat** on a quiet month (`feed.length > 0 &&`). The card comes and goes with the data, which the owner's rule forbids. The route-me board already drew its calm state ("Nothing has changed in the last 30 days."). **clash** (Q4)
- `MemberAccess.tsx:211`: **project cards are in API order**, not A to Z (the owner's rule of 4 Nov). **clash**
- **Syndra keeps no record of a member's visits**, or of their first sign-in. So "what changed since your last visit" and a true first-run moment both need one new fact from the backend (§4, idea 8). The 30-day window in `/me/changes` is the honest substitute, and Recent changes already uses it.
- **The rule for numbers:**
  - The roles a member holds are observed from Zitadel.
  - End dates, requests and changes are Syndra's records. They may name and explain, but they never count access.
  - So every option below names things ("Operator in Laser cutting") rather than counting records. The only number it says is a count of held roles that end soon: held is observed, and the record only supplies the date.

### 1.2 The mythology
- The Zitadel card answers nothing an operator came for, so A-17's ruling to delete it stands.
- The card's best sentence is its last: "Syndra decides what that gets you, and keeps the record of why." The login line says the same thing more briefly ("the reason for every name on it"). Both carry over to the new home.
- **On exactness.** Snorri's *Gylfaginning* says Syn keeps the door of "the hall" and shuts it on those who should not enter. It also says she is set at the assembly to defend cases she means to refuse, which is why "syn" came to mean a denial. It doesn't name Frigg's hall, so placing her at Fensalir among Frigg's company is a later reading. The copy guide's first courtesy is exactness. §3 drafts the story both ways, and Q6 asks which to use.
- **The voice already exists.** Three lines in the product are already in character:
  - Home's "Morning, Kanishk. Nothing needs you right now.";
  - "Everything has reached Zitadel.";
  - the login's door.

  Each is exact, short, and appears at a moment, so the soul in §4 grows from these rather than adding a new register.

---

## 2 · My access: what sits under the greeting

All four options keep the title as it is, "Hi Cody. Here's what you can use." (§4 idea 2 would change "Hi" to the time of day). They differ only in the seat under it, 8px below the title. Content still starts 28px under the header (G5).

### (A) One line that knows · **Recommended**
**One sentence, always there, choosing the most useful thing to say from a fixed ladder.**

The seat: a StatusDot, then a 14.5px sentence in `--muted`, then at most one violet link. It is always one line on desktop and wraps to two at most on a phone. It never changes size between states, so nothing below it moves.

**The ladder.** The first rung that matches speaks. Everything lower on the ladder still shows in its own place on the page or on its own page.

| # | When | Dot | Copy | Link |
|---|---|---|---|---|
| 1 | A role Zitadel holds for you ends within 14 days | warn | "Operator in Laser cutting ends in 7 days." With two or more: "Two of your roles end soon. The first, Operator in Laser cutting, ends in 7 days." | "Laser cutting ↓" (jumps to its card) |
| 2 | A role you were given isn't on your account after a full read (the row's "Not there yet") | warn | "Hand tools in Wood shop isn't on your account yet." | "Wood shop ↓" |
| 3 | Network storage needs a step from you (entitled, with no password set yet) | neutral | "Your TrueNAS account is ready. Set its password to connect." | "Network storage →" |
| 4 | A request of yours is with makerspace staff | neutral | "Makerspace staff have your request for Bandsaw in Wood shop. You asked on 3 Nov." With more than one: "Makerspace staff have your requests. The newest, for Bandsaw in Wood shop, is from 3 Nov." | "Your requests →" |
| 5 | An approval is on its way (approved, not yet sent) | neutral | "Resin printer in 3D printing is on its way to you." | "3D printing ↓" |
| 6 | A role Zitadel holds for you was given today | healthy | "From today, you can use Laser cutting." With two projects: "From today, you can use Laser cutting and Wood shop." | "Laser cutting ↓" |
| 7 | None of the above | healthy | "Nothing needs you right now." | none |
| · | Loading | none | "Checking what you can use." | none |
| · | Zitadel not read | neutral | "Zitadel hasn't answered yet, so your roles aren't shown." | none |

**Notes on the ladder:**
- **Why rung 1 leads.** Ignoring it is the only rung that costs the member something: the role ends, whether or not they look.
- **The jump.** "Laser cutting ↓" uses motion 8d: it glides to the card, and the card's title flashes once. With six cards, the ending row can sit below the fold, especially on a phone. The header names the role and the row keeps the act ("Ask to keep it"), so the date and the button each appear once (G12).
- **Relative here, absolute below.** The line says "in 7 days" and the row says "until 12 Nov · 7 days left". Copy guide §3.9 is met on the row, and the header doesn't repeat the date.
- **No counts from records.** Rung 4 names the newest request rather than counting them. Rung 1 counts only held roles (observed) that have a recorded end.
- **Rung 6 is "the day someone's access starts"** (§4 idea 7), and it lasts that whole day. It never blooms: the member didn't do it (A-05: a bloom is only for something you did).

**Trade-off.**
- **For:**
  - one seat, one sentence;
  - calm on most visits, with the same words as the operator's Home;
  - it surfaces the one thing a twice-a-term visitor might miss.
- **Against:**
  - only one thing at a time. A member with an ending role and a pending request sees only the ending. The request is still one tap away in Requests, and Recent changes shows it.

### (B) Three quiet seats
**A fixed row of three short facts: what ends, what you asked, your storage.**

Each seat is StatusSeat-like (a dot and a sentence). For example: warn "Operator in Laser cutting ends in 7 days." · neutral "Makerspace staff have your request for Bandsaw in Wood shop." · healthy "TrueNAS is ready." On a calm visit: "Nothing ends in the next 14 days." · "Nothing asked." · "TrueNAS is ready."
- **For:** everything at a glance, and the structure is fixed.
- **Against:**
  - On most visits it writes three "all is well" lines, which G17 (silence for normal) forbids.
  - It reads as a dashboard on the one screen a student opens to see what they can use.
  - It competes with the greeting.

### (C) What changed, in one line
**The seat names the newest change, and Recent changes folds into it.**

"Latest: you were given Operator in Laser cutting, on 2 Nov." Then Button outline sm "Show the last 30 days" (G3) unfolds the Recent changes rows in place. On a calm month it reads "Nothing has changed in the last 30 days."
- **For:** it fixes the Recent changes card that comes and goes, because one seat holds both.
- **Against:**
  - The newest change is not always the most important thing: an ending in 5 days is.
  - The news moves behind a click.
  - "Since your last visit" would be better, and Syndra can't know it (§1.1).

### (D) The greeting alone
**A-17's recommendation: delete the line and let the cards speak.**
- **For:** the calmest option. "Here's what you can use." is the lede.
- **Against:**
  - Nothing names an ending role that sits below the fold.
  - It doesn't give the owner the enrichment he asked for.

**Recommendation: (A).** It is one quiet sentence that is right on every visit, and it uses parts Syndra already has: StatusDot, the 8d jump, ArrowLink, and Home's calm words. Recent changes keeps its own card (Q4).

**Phone (390).** The seat wraps to two lines at most, with the link on the second line. The jump lands with the card title under the TopBar. The 44px touch floor applies to the link.

**Motion.** The line arrives with the header (S11). When its rung changes while you're on the page, the words crossfade on tint in place (S4), and the dot changes colour where it sits. It never blooms, never loops and never moves.

---

## 3 · Where the story lives

### Homes, side by side
| | Home | For | Against |
|---|---|---|---|
| **H1 · Recommended** | **About Syndra**, the last row of the Account sheet, above the Sign out hairline. It is closed by default, and the whole row opens it in place (Unfold, with the 28px round chevron). | Every person reaches it, member or operator, at every width, including phones with no rail. The sheet is already where you think about the app rather than a machine. It costs one line at rest. | The sheet grows by one row. |
| H2 | The rail's mark becomes a button that opens the same paragraph in an AnchoredPanel. | The name explains itself where the name is. | The top of the rail is not a control anywhere else. The rail is hidden below tablet, so phones still need H1, and that makes two homes for one story. |
| H3 | The sign-in page: the login line opens to the full story. | It is where the name is first met. | Sign-in is one action. A person signing in wants to get through the door, not read about it, and a second control on the stage dilutes the only one. |
| H4 | The 404 page carries the story under its way home. | It would be a nice surprise. | A lost reader wants the way back. Lore on an error page reads as a joke at their expense. |

**Recommendation: H1, plus the login line as the short form.** That makes one long home and one short one. There is no new route and no new control type.

### The About row, closed and opened
- **Closed.** The SyndraMark (16), "About Syndra" (14.5, ink), and the chevron. It uses the same bordered row shape as Appearance and Sound in that sheet, so it reads as part of the sheet.
- **Opened.** Text indented to the words, in this order:
  1. 14.5 ink, the exact version: "Syndra is named for Syn. In the Norse stories she keeps the door of the hall, and shuts it on those who should not enter. When a case is argued, she speaks for the one who refuses it."
     - Or, Frigg's hall (Q6 no): "Syndra is named for Syn, who keeps the door of Frigg's hall and shuts it on those who should not enter."
  2. 14.5 muted: "In the makerspace, Zitadel is the door. It checks that you are who you say you are. Syndra keeps the list of what that lets you use, and the reason for every name on it."
  3. 12.5 faint: "Built in the lab it runs. Powered by Zitadel." (the login's credit line, carried over word for word).
- It is three short paragraphs, with no heading inside and no link out. The story ends there.

**The login line** becomes, per A-17: "Syn, the goddess who keeps the door. Syndra keeps the list, and the reason for every name on it."

**The Zitadel page** loses `TheName()` and keeps nothing in its place.

---

## 4 · Character: the keeper's voice

### The voice, in one paragraph (for the copy guide, Q7)
Syndra speaks as **the door-keeper who knows everyone**:
- She uses your name and knows the time of day.
- She speaks up only when something has changed for you.
- When everything is in order, she says so in one short sentence and stops.
- She uses the door and the list only where they are literally true. Drift really is "someone inside who isn't on the list". She never uses them to decorate.
- She doesn't joke, doesn't exclaim, and never speaks in a confirm, an error that needs action, or a log.

The character is in *when* she speaks and *how exactly*. There are no extra words.

### The ideas, ranked
Each idea has its exact copy, where it lives, when it shows, and why it stays calm.

1. **Term days on Home's weekday line.**
   - **Copy:** "Thursday 31 Dec · Last day of Monsoon term · last checked 09:14". On a term's first day: "· First day of Spring term".
   - **Where:** the operator's Home, the faint line under the greeting.
   - **When:** only on a term's first and last day, read from Makerspace settings (the one write place).
   - **Why calm:** six days a year. It is a fact Syndra holds, said in the place a date is already said. Nothing moves.
2. **The time of day on My access.**
   - **Copy:** "Morning, Cody. Here's what you can use." (Afternoon and Evening at the same hours as Home: before 12, before 17.)
   - **Where:** the member's title, replacing "Hi".
   - **When:** always.
   - **Why calm:** it is three fixed words, already the operator's greeting, so member and operator are greeted by the same keeper. It is not a rotating line.
3. **About Syndra** (§3, H1).
   - **Copy:** as in §3.
   - **Where:** the foot of the Account sheet.
   - **When:** only when someone opens it.
   - **Why calm:** it is closed at rest, one line, and never pushed at anyone.
4. **The keeper's all-clears.** Two queues where the metaphor is exact. Their guidance lines stay as they are.
   - **Copy:**
     - Drift: "Everyone inside is on the list, and everyone on the list is inside." (was "Everything is explained.")
     - Unfinished revocations: "Everyone whose access was revoked is out." (was "Nothing is unfinished.")
   - **Where:** those queues' EmptyState (resolved, lime dot).
   - **When:** whenever the queue is empty.
   - **Why calm:** each is still one declarative sentence with a full stop, and each says *more* than the old title. Drift's line names both directions the page checks. Home, Pending changes, Expiring and Holds keep their plain titles, because no door metaphor is exact there, and forcing one would make the voice a gimmick.
5. **The last one, cleared.**
   - **Copy:** none new. The queue's own all-clear title is the moment.
   - **Where:** Drift, Unfinished revocations, Requests, Expiring access and Holds due.
   - **When:** your own act empties the queue while you're on it. The EmptyState rises 6px (`lift`) and its lime dot blooms once (`bloom`), as Pending changes already does (S13).
   - **Why calm:** it follows the existing rule: a bloom is only for something that went well, because of you, on screen. It never plays on arrival or on refresh, and never for an empty queue you merely opened. This is "the end of a long cleanup" with no new words.
6. **The mark lights once after sign-in.**
   - **Copy:** none.
   - **Where:** the SyndraMark at the top of the rail (desktop) or the TopBar (phone).
   - **When:** only the first paint after a sign-in. Its dot blooms once (flash, 900ms), carrying on from the login's orb, which is the same drawing.
   - **Why calm:** once per sign-in. It is something that went well because of you (you came through the door). Reduced motion turns it into nothing at all.
7. **The day access starts** (rung 6 of §2 A).
   - **Copy:** "From today, you can use Laser cutting."
   - **Where:** My access's line.
   - **When:** the whole day the role is first held.
   - **Why calm:** it is the ladder's ordinary seat. A role is new on that day only once.
8. **A first welcome.**
   - **Copy:** "Welcome to Syndra. It keeps the makerspace's list of who can use what, and why."
   - **Where:** My access's line (or Home's, for an operator), above every rung.
   - **When:** the first visit only.
   - **Why calm:** once in a lifetime. **Needs a fact Syndra doesn't keep** (the first sign-in), so it waits for the backend.
9. **Quiet hours.**
   - **Copy:** Home's calm guidance gains a sentence between 22:00 and 05:00: "No open requests, and no access ending in the next 14 days. It will keep until morning."
   - **Where:** Home's calm panel only.
   - **When:** late at night, on a calm day.
   - **Why calm:** it only appears when the answer is "nothing needs you", and it gives permission to stop. Optional: it is the closest of these to sentiment.
10. **The 404 in voice.**
    - **Copy:** "There's no door at this address." The guidance stays as it is.
    - **Not recommended:** a route is not a door, so the metaphor stops being exact, and an error page is the wrong place to be charming.

**Rejected, so nobody proposes them again:**
- the mark breathing amber when Zitadel isn't answering (colour alone, and it restates the Notice);
- rotating or random lines;
- anniversaries ("A year ago today…");
- holiday themes;
- confetti, emoji or a mascot;
- any lore in an error, a confirm or the audit log.

### Recommended first set
- **Ideas 1–7.** Idea 7 comes with §2 (A).
- They add **seven pieces of copy** and **one bloom**. They use no new component, no new route and no new loop.
- 8 waits for the backend fact. 9 is the owner's taste (Q8). 10 is no.

---

## 5 · For code (once the owner answers)

1. **My access (A):**
   - Replace `summarySentence` with `headerLine(...)`, which returns `{ tone, text, link? }` from the ladder in §2.
   - It is one pure function in `components/member/`, tested rung by rung, including "no counts from records" and "rung 1 counts only observed roles".
   - Render it in `--muted` with StatusDot. Jumps use the existing 8d glide-and-flash.
   - Sort the project cards with `sortByName`.
2. **Recent changes** keeps its seat (Q4). It always renders, with EmptyState "Nothing has changed in the last 30 days." when the feed is empty.
3. **About Syndra:**
   - A row in `AccountSheet.tsx` above the Sign out block: a CardRow-shaped toggle with FoldChevron and Unfold.
   - Delete `TheName()` from `zitadel/page.tsx`.
   - Fix the login line's dash.
4. **Ideas 1, 2, 4, 5, 6:**
   - **1:** `formatWeekday` + `useTerms` on Home.
   - **2:** `greeting()` moves from `Home.tsx` to a shared `lib/` helper.
   - **4:** two title strings.
   - **5:** `bloom` and `lift` on the five queues' EmptyState, set only by the act that emptied them, as Pending does.
   - **6:** a one-shot `bloom` on SyndraMark's dot, keyed to the session's first paint.
5. **Copy guide:** add "The keeper's voice" (§4's paragraph) under §2, and the new strings. `plain-language.test.ts` already guards dashes and exclamation marks.

---

## 6 · Questions for the owner

1. **My access header: option (A), one line that knows, from the ladder in §2?** *Recommended: yes.*
2. **Should the ladder's order stand (ending soon first, calm last), with the line jumping to the card it names?** *Recommended: yes.*
3. **Should the member's title greet by time of day ("Morning, Cody."), like Home?** *Recommended: yes.*
4. **Should Recent changes keep its seat on a quiet month, with "Nothing has changed in the last 30 days."?** *Recommended: yes. Structure doesn't move with data.*
5. **Should the story live in About Syndra at the foot of the Account sheet, with the login line as its short form, and nowhere else?** *Recommended: yes.*
6. **Should the story be told the exact way, "the door of the hall", with Syn speaking for the one who refuses?** *Recommended: yes. "No" keeps "Frigg's hall".*
7. **Should "The keeper's voice" paragraph (§4) be added to the copy guide, so the voice has a rule as well as examples?** *Recommended: yes.*
8. **Should the first set be ideas 1–7, with quiet hours (9) left out for now?** *Recommended: yes.*
9. **Should the backend record a person's first sign-in, so the one-time welcome (8) can exist?** *Recommended: yes, later. It is one timestamp, written once.*
