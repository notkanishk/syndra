# Alphabetical names, always

## Why

Owner, 4 Nov 2026, on the claims Preview's person picker (which listed people in fetch order, the current user pinned first): "alphabetical, always, please". The same was true of most pickers: each showed whatever order the backend sent, and the few that sorted used a case-sensitive `localeCompare` or a bare `.sort()`.

## What changes

- One comparison, `ui/src/lib/sort.ts`: `compareNames` (case and accents aside, numbers as numbers), `sortByName(items, name, id?)` (ties broken on the id), `sortNames`, and `sortAccounts` (people A to Z, then service accounts A to Z).
- The list hooks every picker reads sort with it in `select` (cache-memoised): projects (and each project's roles), the catalog (people, then service accounts; projects and their roles), bundles, apps, the role catalog, service accounts, claim templates.
- Pickers that build their own list sort with it too. Nothing is pinned: not you, not the place a link named; the chosen one is marked, never moved.
- The claims Preview's picker keeps service accounts (a preview checks access, RULES.md) after the people under a quiet "Service accounts" group.
- RULES.md carries the rule.

## Kept in their own order

Pending changes (send order), time-ordered feeds, search results by relevance, rankings by count, tables sorted by a column heading, and curated menus (a claim's Carries chooser, lenses, devices, kinds).
