# GoTo (proposed, A-19, 5 Nov 2026)

One way to reach anything in Syndra by typing its name. Drawn on `a19-go-to-and-arriving` §1.

## What it is
A `Modal` size `sm` holding one `Input` and a grouped list of matches. It opens from anywhere by `/` (when no field is focused) or Cmd K (Ctrl K on other keyboards), and from the phone's Go to sheet. It is one part, `components/ui/GoTo.tsx`, mounted once in `AppShell`.

## Anatomy
1. `ModalHeader` title "Go to" (the dialog title; visually it may sit as the Input's label).
2. `Input`, placeholder "A person, a project, an app, a bundle, or a page", autofocused, with the typed text kept while the sheet is open and cleared when it closes (a reopened dialog starts clean, A-04 §11).
3. A `listbox` of results in fixed groups, in this order: **Pages · People · Projects · Apps · Bundles**. Service accounts sit after people inside People, under their quiet label, as every picker does. A group with no match is left out. Inside a group, rows are A to Z by their shown name (`sortByName`), never by score and never by recency.
4. Each row: the name (14.5 semibold, ink), one quiet descriptor (13 muted: a person's kind and team, a project's role count, an app's project, a page's section), and the kind on the right (12.5 faint). Rows are 44px tall, 24px side padding. The selected row has the `--tint-2` fill every chosen thing uses. Rows are `<a>`; the whole row is the hit area.
5. A footer line in `--faint` with three `Kbd`s: ↑ ↓ move · Enter open · Esc close.

## Behaviour
- Matching is prefix-then-contains on the shown name, accents and case aside, over the name catalog Syndra already holds (`["name-catalog"]`: users, projects, applications; bundles from `["name-bundles"]`) and the page titles in `lib/nav.ts`. No request is made while typing; the catalog is already in the query cache. With an empty Input, the list shows Pages only.
- Pages follow the navigation contract: Advanced-only leaves appear only when the view is Advanced. Member sessions see their own pages (My access, Requests, Network storage) and nothing else.
- Enter opens the selected row (the first by default). Arrows move the selection, wrapping at the ends. Esc closes. Typing continues to narrow.
- Opening a row navigates as a link does and seeds the page per A-19 P1 (the name arrives with you).
- Below tablet, the same part renders inside the TouchNav Go to sheet, in both views, with the Input at the top and the page list beneath; the keyboard rises with the sheet, and results stay above it.
- Nothing is pinned and nothing is remembered: no recents, no favourites (RULES: alphabetical, always; nothing is pinned).

## Motion
S1 for the sheet. Rows that leave on narrowing fade and their gap settles at once (A-08 §4). Nothing arrives inside the list. The selected fill tints (120ms). Reduced motion: appears and leaves with its scrim.

## Copy
"Go to" is the title and the verb. No "Search" (that word belongs to the per-page search). With no match: one EmptyState inside the list, "Nothing by that name." and no guidance.

## Not for
- Searching inside a thing (a person's roles, a setting's value): the page has its own search.
- Acting: Go to opens pages, never runs a verb.
- Showing counts or status: a row says the name and what it is, nothing about its state.

## Props
```ts
type GoToProps = {
  open: boolean;
  onClose: () => void;
  /** Which pages may be listed: the current view's nav tree. */
  nav: NavSection[];
};
```
The catalog and bundles come from the query cache through `useNameCatalog()`; no data prop.
