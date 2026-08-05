# The landing is called Home, and the member view drops the workshop password

## Why

Two small, unrelated corrections to what the interface currently claims.

**"Today" over-promises.** It was the right name when that page was a work
queue and nothing else — the handoff names it that in §04, "Today — work, not
a summary". The page has since deliberately grown a second zone, *The
makerspace*, which is not today-scoped at all; its own docstring records why
(an operator landing on "Nothing needs you." and nothing else learns nothing
about the space they run). A rail item that promises a day while showing the
shape of the place is describing half of itself.

Every other item in that rail names a thing: People, Projects, Roles, Apps,
Requests, Audit. "Today" names a time and "Home" names a position, so neither
names the content — but Home is the one position-word in a rail that is also a
true name, because this page's identity *is* that it is where you land.

**The workshop password card asked for something nothing reads.** Its own copy
admitted this in a paragraph: the door and machine bridge is unbuilt, so a
password set today is stored and used by nothing. That paragraph was the right
call while the card shipped, but the better call is not to ask. A member being
shown a password field on the screen that explains their access invites exactly
one wrong reading — that their institutional login has changed — and the card
spent its first sentence denying that and its second admitting it does nothing.

## What changes

- `nav.ts` renames the operators' first destination from **Today** to **Home**.
  The header breadcrumb derives from the nav tree, so it follows without a
  second edit. Nothing moves, nothing is added or removed, and the route is
  unchanged.
- The component, its file and its directory are renamed with the label:
  `components/today/Today.tsx` becomes `components/home/Home.tsx`. Keeping the
  old component name was considered and rejected — a rail that says Home over
  a component called Today is a question every future reader has to answer
  twice, and the answer is not in the diff. After this, **"Today" means exactly
  one thing in this codebase: the day.** Where the word survives it is the day
  and nothing else — "today's work", what expires today — and it is never the
  name of a screen.
- The audit reason string `"Extended from Today"` becomes `"Extended from
  Home"`, matching the convention its sibling already follows (`"Extended from
  Expiring access"`). It names the screen the operator acted on, so it has to
  name a screen that exists. Rows already written keep the old wording, which
  is correct: they record where the operator actually was.
- `MemberAccess` no longer renders `ShadowCredential`. The component, its
  queries, its tests and the whole backend vault are untouched — this is one
  line, and restoring it is re-adding that line. The comment left in its place
  says when: when the bridge can actually read the password, and not before.

## Impact

- Affected specs: none. No canonical requirement pins the label, and none
  requires the member view to offer a credential.
- Affected code: `ui/src/lib/nav.ts`, `ui/src/components/today/Today.tsx`,
  `ui/src/components/member/MemberAccess.tsx`, and the two test files that
  assert the rail's first label.
- No backend change. No API change. No route change. No data is deleted:
  a member who already set a workshop password still has it.
