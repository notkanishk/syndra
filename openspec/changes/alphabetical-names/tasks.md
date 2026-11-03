# Tasks — alphabetical names

## 1. One comparison
- [x] 1.1 `ui/src/lib/sort.ts`: `compareNames` (Intl.Collator, `sensitivity: "base"`, `numeric: true`), `sortByName(items, name, id?)` with an id tie-break, `sortNames`, `sortAccounts`. `orderBy` (SortHead) uses `compareNames`, so a column sort and a picker agree.
- [x] 1.2 Unit tests: case, accents, numbers, ties, no mutation, people before service accounts (`lib/__tests__/sort.test.ts`).

## 2. Sources sorted once, in the list hooks (`select`)
- [x] 2.1 `useProjects` (projects, and each project's roles by label), `useCatalogUsers` (people, then service accounts), `useCatalogProjects` (projects and roles), `useBundles`, `useApplications`, `useGlobalRoleCatalog` (by shown name, ties by project then key), `useServiceAccounts`, `useClaimTemplates`.
- [x] 2.2 Systems by shown label: `useTargets`, `usePersonTargets`, `useMyStorage` (the sidebar, Home, a person's Access, Storage and Activity, My storage). `useRoleMembers` (holders, observed-only), `useUserAccess` (bundles, projects), `useBundleIncluders`, `useDormantAccounts`.
- [x] 2.3 Insights `listOf` sentences and "Who has" (sorted before the 8-person cut); `orderedSources` A to Z within a kind; a bundle page's roles inside each project group; the People list's bundle chips.
- [x] 2.4 Lists named by lookup sort at render by the resolved name (`components/names/shownNames.ts`, `useShownNames`), an unresolved one by its id or key so the list settles at most once: roles inside a person's project cards and unexplained roles (PersonAccess, MemberAccess, ServiceAccountPage), a bundle version's roles and holders (BundleVersions, RetiredBundle, ManageBundles' preview), a system's role rows (MappingManagement) and its people (PeopleOnTarget), TargetOverview's pools, unaudited shares, stale owners and unmanaged accounts, Home's role chips, holds (Withheld), the automatic rules list (by "Project / role ⇒ Project / role"), the raw Zitadel grants (by person, then project) and projects pages (projects and roles), Insights' Unused panel. Guard: `components/__tests__/alphabetical-resolved-names.test.tsx` (PeopleOnTarget with an unresolved name, Withheld).

## 3. Pickers and lists that build their own order
- [x] 3.1 Claims Preview "Preview for": A to Z, no "you first"; service accounts after the people under a "Service accounts" group (a preview checks access, so they stay).
- [x] 3.2 One-role picker (Ask for access, Give access): places A to Z, roles A to Z inside each; the place a link named opens in its seat instead of moving to the top.
- [x] 3.3 Many-role picker (bundles): bundles and project groups through the helper.
- [x] 3.4 Filters: Roles (project, group), Unexplained access (project, person, role), Withdrawn access (system, by its shown label).
- [x] 3.5 Dialog pickers: Clone from (by "project / key"), add-on group and home-folder choices (by shown label), the storage guide's folder choice, a project's claim attribute picker, Zitadel's raw people list, project picker and role keys, Copy shape's app list.
- [x] 3.6 Connected systems list, Home's "not confirmed" systems (by shown label), Roles that wait and a bundle's project groups, Insights map lanes and question tie-breaks, through `compareNames`.

## 4. Guard
- [x] 4.1 `components/__tests__/alphabetical-names.test.tsx`: every source hook above from shuffled fixtures; ClaimPreview and the one-role picker rendered from shuffled props, asserting order, grouping and that the chosen one is not moved.

## 5. Rule
- [x] 5.1 RULES.md: every list or picker of names is alphabetical by its shown name; order-carrying lists keep their order.

Kept in their own order: Pending changes (send order), feeds by time, search results by relevance (the service-account owner search, debug mode's account search), rankings by count (Home's top roles, Insights findings), SortHead tables (their default sort applies), curated menus (a claim's Carries chooser, lenses, devices, kinds, attention values). Debug mode's "You" row is a separate scope choice, not a list position.
