# Q-18 · My access's header, and Syndra's character

From Claude Code, 5 Nov 2026. These are the owner's two open questions from A-17 §3, Q3 and Q4, restated with what code knows.

## 1 · My access header (A-17 §3 Q3)

**The owner (5 Nov):** the line "Nine roles across six rooms" is going away. It calls projects "rooms", and the cards already show the count. "Can something replace it, or can we add around that area to elegantly enrich the user experience?"

**Locked facts:**
- The line is `summarySentence()` in `ui/src/components/member/MemberAccess.tsx:435-450`, drawn faint under the title at `:144`. Today it reads, for example, "Four roles across two rooms. One ends on 20 Oct."
- The title is "Hi {first name}. Here's what you can use." (`MemberAccess.tsx:140-143`).
- The page then shows Recent changes (only when the last 30 days hold something, `:157-164`), any holds in force, one card per project, and the catalog.
- What Syndra knows about a member today:
  - the roles Zitadel holds for them (`observed_role_keys`, `observed_role_count`);
  - the end date its records give each role (`useUserGrants`);
  - the last 30 days of changes (`/me/changes`), including approvals still on their way;
  - their own requests (`/requests?mine=1`);
  - their network storage state (`useMyStorage`: entitled, account, credential, health);
  - holds in force.
- What Syndra does **not** know: when a member last visited, or when they first signed in. No table or field records either.

**Asked:**
- 3–4 distinct options for what sits under "Hi Kanishk. Here's what you can use.", or around it. Use real facts: what changed, access ending soon, a request in flight, storage state, a quiet "nothing needs you".
- The rules:
  - every number is observed from Zitadel, and records only explain;
  - structure never moves with data, so a slot's calm state is a sentence, not a gap;
  - plain language, with no em dashes;
  - both themes, and phone width.
- Recommend one.

## 2 · The mythology, and character in general (A-17 §3 Q4)

**The owner (5 Nov):** the Zitadel page's mythology card goes, but "where to move it to. It adds character to the app, part of the story, the user experience. Give more suggestions to make the app feel alive like this, with a character and a soul."

**Locked facts:**
- The card is `TheName()` in `ui/src/app/zitadel/page.tsx:131-142`. Its words, verbatim:
  - Title: "Syn keeps the door. Syndra keeps the list."
  - Body: "Syn keeps the door of Frigg's hall and bars it against those who should not enter. Zitadel is that door: it decides whether you are who you say you are. Syndra decides what that gets you, and keeps the record of why."
- The login line stays, rewritten without its dash: "Syn, the goddess who keeps the door." Today it reads "Syn — the goddess who keeps the door. Syndra keeps the list, and the reason for every name on it." (`ui/src/components/login/LoginDoor.tsx:200`).
- The background: Syndra is named after Syn, the Norse goddess who keeps the door of Frigg's hall (Fensalir) and refuses those who should not enter. Syndra is the makerspace's door-keeper for access.
- Copy guide §2: calm, courteous, exact, and "Never clever. No wordplay, no arch asides, no jokes."

**Asked:**
- **(a)** 2–3 homes for the full story. The candidates:
  - an "About Syndra" in the Account sheet;
  - the sign-in page's fold;
  - a first-run moment;
  - the 404 page;
  - a footer under the rail's mark.
- **(b)** 6–10 "soul" ideas, ranked, that make the app feel alive without ever getting in an operator's way. Each one needs its exact copy, where it lives, when it shows, and why it stays calm. Consider:
  - a consistent voice;
  - moments: first sign-in, all-clear queues, an empty Pending, the end of a cleanup, quiet hours, the day access starts;
  - time of day or season;
  - the existing motion as a signature;
  - the SyndraMark;
  - small easter eggs;
  - error and 404 pages written in the voice.
- **Hard lines.** The voice never appears in:
  - destructive confirms;
  - errors that need action;
  - audit or log text.

  It is never cute, and it never repeats in a way that grates on the 50th view. The tone is quiet, warm, a little mythic, and confident.
- Recommend a first set.

**Answer at** `from-design/A-18-character-and-soul.md`, with the board `design/a18-character-and-soul.dc.html`. Follow A-16 and A-17's shape: findings, then options with recommendations, ending with numbered owner questions where "yes" accepts the recommendation.
