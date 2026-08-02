# Design — UI Capability Gap Closure

## Decision 1 · A role reference is a pair, composed in the UI

### The problem, stated precisely

`(project_id, role_key)` is the identity of a role. `role_key` alone is not.
Every screen in MkAuth knew this and none of them agreed on how to say it.

Two abstractions existed for it and neither was reachable:

| Mechanism | Where | Call sites before this change |
|---|---|---|
| `<RoleName projectId roleKey>` | `components/names/RoleName.tsx` | 0 (13 before `d8676df`) |
| `formatRoleRef(projectId, roleKey, projects)` | `lib/format.ts` | 0 — only its own test |
| `CatalogRole.display_label` | composed in `services/roles.go` | 1, and that one printed the project twice |

Meanwhile nine surfaces hand-rolled `<ProjectName/> / <Mono>{key}</Mono>`, and
two named a role with no project at all.

### Why `display_label` could not be the answer

`display_label` was `"Printing Lab: admin"`, composed backend-side and attached
to `CatalogRole`. It is only present on rows that come from `GET /roles`.

Grant rows, audit entries, access requests, propagation rows and drift items all
carry `(project_id, role_key)` and no label. So the one field named as the
disambiguation capability could not disambiguate any of the surfaces where two
same-key roles actually collide. Worse, it is a trap: it works on `/roles`, so
the next person to reach for it builds on a foundation that does not generalise.

Removed. The capability moves to the UI, which can resolve any pair through the
name resolver it already runs.

### The shape

Two exports, one rule, split by register — not by screen.

**`<RoleRef projectId roleKey />`** — for rows.

```
Printing Lab / trained
             ^ resolved via <ProjectName>   ^ raw key, monospace
```

The project resolves to its human name because that is what staff recognise.
The role stays as its raw key in monospace because the key is what lands in the
token and what an operator matches against Zitadel. A table is *scanned for
identifiers*, so it shows one.

**`roleLabel(projectName, roleKey, roleDisplayName?)`** — for sentences.

```
"Kanishk now holds Printing Lab / Trained operator."
```

Prose is *read*, so it gets the role's human name; `laser_trained` is not a
word. It is a plain function rather than a hook so a toast handler, a dialog
lede and a warning banner can all call it without a resolver in scope.

The invariant both enforce: **the project is never absent.** `<RoleRef>` with
either half missing renders an em dash rather than half an identity — a bare
`admin` is worse than nothing, because it looks like an answer.

### Where the pair is established by structure instead

`<RoleRef>` is deliberately *not* used inside a container already scoped to one
project. Repeating the project there is a stutter, not a clarification:

- `/roles` — Project is the first column and never collapses.
- `/projects/{id}` and `/projects/{id}/roles/{key}` — the whole page is one project.
- `PersonAccess`, `MemberAccess` — roles are grouped under project cards.
- `Makerspace` "Where access lives" — the project is the trailing column.

`Makerspace` was the one place this was wrong: it rendered `display_label`
("Printing Lab: admin") in the name slot *and* `project_name` in the trailing
slot, printing the project twice on every row. It now renders the role alone,
the same way `/roles` does.

### Surfaces that were already correct, and stay as they are

`policies` and `RemovalDialog` build `"{Project} / {key}"` by hand. They are
correct pairs in the right register — a rule-authoring screen and a destructive
confirmation both want the exact key, not a friendly name. Rewriting them
through the helper would trade precision for uniformity. Left alone.

The audit listed `GrantDirectAccess`'s role `<Select>` as a fifth format. It
isn't a defect: the sibling select fixes the project, so `display_name · key`
inside it is unambiguous. Its *toast* was the defect — it fired
`` `${userName} now holds ${roleKey}` `` after the dialog closed, naming a role
with no project for a write that is meaningful only as a pair.

Similarly, `MemberAccess`'s role rows are fine — they sit under a project card.
Its *expiry warning* sits outside every card, and read "Operator runs out on
12 Aug" to a member who holds Operator in three projects.

Both now name the pair.

---

## Decision 2 · Where a filter lives is a UX question, not a plumbing one

Three screens gained filters in this change and they do not all filter the same
way. That is deliberate; the rule is the reader's experience, not symmetry.

**Event activity filters in the client.** The backend offers `?status=`, and it
is the wrong tool twice over: it covers webhook events only, so a filtered view
would be half server-filtered and half not, and it takes one exact status, so it
cannot express a bucket. The two tables do not share a vocabulary — a webhook
event is `processed`, an onboarding trigger is `completed`, and neither word is
what a reader has in mind. `outcomeOf()` maps both into *did it work / is it
still going / did it break / did we choose not to act*. The page also polls
every five seconds; a server-side filter would drop it into a loading state on
every pill press.

An unrecognised status returns itself rather than falling into `done`. A new
backend status appearing as a success on a forensic log is precisely the silence
this page exists to prevent.

**Drift triage filters on the server.** No polling, the endpoint already
supports it, and the ordering is computed server-side and must not be re-derived.
`useRowSelection` already prunes a selection to live ids, so narrowing the queue
cannot leave a bulk action aimed at rows that are no longer on screen.

`user_id` is accepted by the backend and deliberately not offered here: "select
everything else for this person" is already on every row, works from the row you
are looking at, and does not ask anyone to find a name among three hundred. A
select would be a worse version of a control that exists.

**Every filter says when it is the reason a list is empty.** "Nothing here" and
"nothing here that matches" are different facts, and on the drift queue the
difference is whether unexplained access exists somewhere nobody is looking.

## Decision 3 · A drain has five outcomes, and four of them were silent

`DrainResult` carries `applied`, `failed`, `requeued`, `errored`, `halted` +
`reason`. Only the first two were ever spoken.

The two that were missing are the two that mean *do this again*:

- `requeued` — a transient error; the row will be retried on the next pass.
- `errored` — the Zitadel outcome was decided but MkAuth could not write it
  down. The row stays `in_flight` and the next drain reclaims it. The write may
  well have landed; MkAuth does not know that it did.

Reporting only the terminal pair turns both into silence on an HTTP 200. An
operator who resumes a queue of eight and reads "0 applied, 0 failed" concludes
the queue is idle. `describeDrain()` returns `{tone, message, detail}` from the
whole result, and only a pass with nothing left to do returns `success`.

Halting is three distinct facts and gets three sentences: another drain holds
the lock, Zitadel is unreachable, or a row has exhausted its retry budget and
everything behind it is untouched.

## Decision 4 · The audit cursor is a tuple, because timestamps collide

`/audit` capped at 200 with no offset and no cursor, so anything older than the
most recent 200 mutations org-wide was unreachable.

Offset paging would be wrong here on a list that grows at the head, but the
sharper reason is that `created_at` is the **transaction** timestamp: a cascade
that writes eight audit rows in one transaction writes eight rows at the
identical instant. A `before=<created_at>` cursor would return the rest of that
batch forever, or skip it entirely.

So the cursor is `(created_at, id)` and the ordering is
`ORDER BY created_at DESC, id DESC` — the id tiebreak is not cosmetic, it is
what makes the cursor mean anything. `buildAuditQuery` is split out from
execution so the placeholder arithmetic can be tested without a database: a
mis-numbered `LIMIT` does not error, it returns the wrong page.

The response stays a bare array; the client sends `before_at` + `before_id` from
the last row it holds, and a short page is the end. Adding an envelope would
have been a breaking change to buy a fact the client can already see.

## Decision 5 · The member catalogue is projects, not applications

The audit read `A7` as "the `applications` slice of `/catalog` is fetched and
read by nobody". Re-checking the shape, `ApplicationCatalog` is a token consumer
— a claim name and a format type. Nobody requests an application. What a member
asks for is a role in a project, which is the `projects` slice.

So the catalogue lists every project and its roles, held ones marked rather than
hidden: a list that rearranges itself per person is harder to talk about, and
"you already have everything here" is often the most useful thing the page can
say about a space.

It sits under My access rather than behind a third nav item, because "what do I
have" and "what else is there" are the same question asked twice, and the
contrast between them is the point.
