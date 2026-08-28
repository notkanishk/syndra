# Plain-language copy

## Why

Syndra is used by makerspace staff and members of a college. Seven independent
readings of every user-facing string (2026-08-27) found **729** places where
the text assumed a reader who knew what an identity provider, a cascade, a
drain, a rehearsal or a binding was — and a staff member approving a request
does not. The product had rules for colour, motion, structure and touch, each
with a guard; it had no rule for words, and no guard.

The findings were not only vocabulary. Zitadel was named six ways across the
product; ending a person's access was named six ways; the preview step four.
Eleven pages had no sentence saying what they were for. Three sentences were
untrue as rendered: a stale list said it was "read null", a hold rendered as
the word `true`, and the Pending changes page told the reader to "resume"
with no such button anywhere. The login door said "Sign in with Zitadel" to
every member.

## What changes

- **A writing guide** — `design.md` — that fixes the register (a well-run
  university office), the shape of a sentence, what every page and every
  action must carry, and one vocabulary with one word per thing and the
  gloss used on first appearance per page. It is the contract for all future
  text.
- **Every user-facing string rewritten to it**, across every screen, for both
  audiences. Zitadel is called Zitadel; the preview step is Preview; ending
  access is revoke, glossed once per page; members are told to ask makerspace
  staff.
- **`PageHeader` gains a `lede`** — the page's purpose sentence — separate
  from `meta`, which returns to metadata only. Every page has one.
- **Additive explanation** where a reader could not otherwise act: a
  consequence sentence before every button that changes something; a titled
  *What happens* list before every destructive one; a one-line legend where a
  screen cannot avoid its terms; an audience line on the one panel written
  for the person who runs the server.
- **A guard** — `plain-language.test.ts` — that reads every component's copy
  and fails on any word from the never-on-screen list, a page without a lede,
  a sentence in `meta`, an exclamation mark, and a button whose label does not
  name its object. Exceptions are argued by file, one reason each.

## What does not change

Structure, colour, motion, routes, the backend, any contract. Nav labels
change in three places (Identity provider → Zitadel; Withdrawn access →
Unfinished revocations; Event activity → Incoming events) and nowhere in
order, grouping or href.

## Decisions taken with the owner

| Question | Decision |
|---|---|
| What to call Zitadel | **Zitadel**, glossed once per page as "the service everyone signs in through" |
| The preview step | **Preview → Apply** |
| Ending a person's access | **Revoke**, glossed once per page; *withdraw* is a member's own request, *remove* is out of a set, *delete* is an object, *retire* is a bundle |
| What members call staff | **makerspace staff** |
