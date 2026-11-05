# An extension of you (A-19)

## Why

Owner, 5 Nov 2026, answered "all yes" to every question in `design_handoff_syndra/reconcile-2026-10/from-design/A-19-extension-of-you.md` §6, including the first wave: P1, P2, P3, P4, P11, P12, P14. Syndra's rules already describe a product that feels like an extension of its reader; what was missing is the connective tissue between screens. The name you clicked should be on the next page when it paints, a link should finish the sentence it started, and there should be one way to get anywhere by typing its name.

This change is written in labelled sections, one per build, so two builds can land beside each other.

## Section A. Wave 1 navigation (P1, P3, P4)

Built on `feat/a19-wave1-nav`. No backend change and no new endpoint: every read already exists.

### P1. The name arrives with you
- `lib/queries/seed.ts` (`useSeededUser`, `useSeededApp`) reads what the row the reader clicked already knew, from the People list cache (`["users","list",*]`), the Apps list cache (`["applications"]`), and, for a person, the name catalog the resolver holds. No request is made.
- `PersonAccess` and `AppTokenScreen` paint the header (avatar, name, email, title, team; app kind and project) and set the breadcrumb on the first frame. The header is outside the 200ms skeleton delay; only the access rows below it wait behind `SkeletonAfterDelay`. With no row to seed from, behaviour is today's (no name, no crumb until the read).
- The person page's tab is in the address as `?tab=requests|activity`. Access is the bare URL. A write is `history.replaceState` (not a push), so Back is not spent on a tab; a link or reload opens on the tab it names; an unknown or operator-only value falls back to Access.
- Not in this build: project and bundle pages (P1 names them; their lists seed the same way when they are next touched).

### P3. "Waiting for you to send →" lands on you
- `NextStep` takes `person` and links to `/governance/pending?person=<id>`; `ActionOutcome` and `NotSentYet` pass it through. Wired on the person page (Extend, queued roles), Give a role, and the person's Requests tab.
- Pending changes reads `?person=` beside `?q=` and shows an in-force `FilterChip` (the person's name, cleared by its own ✕, which also removes the parameter). The read line says "N of the M changes match. Sending sends all M, in order." Send is never narrowed.
- Names on Pending changes (Who, and Caused by for a connected system's change) are links to the person (G14).
- Not wired yet: the Removal dialog's outcome and Roles that wait still link to the whole queue.

### P4. Go to
- `/` (when no field has the focus) and Cmd/Ctrl+K open `GoTo` (`components/ui/GoTo.tsx`, a Modal `sm` with a `combobox` and a `listbox`, `aria-activedescendant`), mounted once in `AppShell` by `GoToHost`. It never fires while typing in an input, textarea, select or editable region, never with another modifier, and never over another dialog.
- Matching (`lib/go-to.ts`, pure): case and accents aside, contains. Pages come from `lib/nav.ts` for the session's own view (Basic, Advanced, Member). People (service accounts after them), Projects, Roles (a project's roles, under their project), Apps and Bundles come from the name catalog already in the query cache (`useNameCatalog`, `enabled: false`, so typing makes no request). Fixed group order; inside a group, A to Z (`sortByName`), never by score or recency. A member's list is pages only; Bundles appear only in Advanced. Nothing typed lists Pages only. No match says "Nothing by that name."
- Arrow keys move (wrapping), Enter presses the selected link, Esc closes.
- Phone: the existing "Go to a page" sheet (`TouchNav`'s `NavSheet`) gains the same panel (`GoToPanel embedded`) at its top; the page list stays beneath it until something is typed. Below tablet every Modal is already a bottom sheet.
- `Kbd` (`components/ui/Kbd.tsx`) draws the three keys in Go to's footer.
- Both parts and previews are in `ui/.design-sync` (entry, config, `previews/GoTo.tsx`, `previews/Kbd.tsx`); not yet re-synced or published to the canvas.

### Deviations from the proposed parts
- `Kbd` is 12.5px, not 11.5: the type floor ("never below 12.5px") wins over the sketch. The corner is `rounded-[6px]` because `globals.css` has no `--radius-sm` and this build does not touch tokens.
- The catalog key in `GoTo.md` is `["name-catalog"]` and that is what is read (the resolver's own); `useCatalogUsers` uses a different key and is not involved.
- A "Roles" group sits between Projects and Apps (the owner's scope asked for roles; `GoTo.md` lists five groups).
- Basic on a phone has a tab bar, not the sheet, so the phone Input is reached through the sheet (Advanced, and a member never needs it). A tab-bar entry would move structure and is left to the owner.

## Section B. First moment and system tokens (P2, P11, P12, P14)

Built on `feat/a19-wave1-feel`. Another section ("Wave 1 · flow", P1/P3/P4) is added by its own branch.

- **P2, the first 100ms in the future tense.** While a write is in flight the seat forecasts in the plan's tense: an amber breathing dot and "Will be approved", "Will be declined", "Will be extended by 90 days", "Will be lifted". It crossfades to the true word on the server's answer; only the true word blooms. A failure shows the honest error with the buttons back; there is nothing to roll back. Forecasts are words, not counts: no cache is written ahead of the server, so "observed from Zitadel" stays true. The shared seam is `ActionForecast` (`components/ui/ActionOutcome.tsx`), the `FORECAST` table (`lib/outcome.ts`) and `forecasts` in `useRequestDecisions` (Requests, Home, a person's Requests tab). Expiring Extend, a person's Extend and Holds' Lift use React Query's `isPending`.
- **P11, Appearance "Device".** A third option that clears the stored theme and follows `prefers-color-scheme` live, crossfading S14 at a device change. `useTheme()` gains `choice` (`"device" | "light" | "dark"`); `setTheme` accepts `"device"`. One write place: `lib/theme.tsx`.
- **P12, tokens.** `--scrim` (dark `rgba(0,0,0,.55)`, light `rgba(60,40,120,.35)`), `--layer-bar/pill/popover/dialog` (20/30/60/50), `--tap: 44px`, all in `globals.css`. The sheet drag thins the scrim with `--scrim-keep`. The navigation sheet joins the one scrim (it was a separate 0.70). A guard test fails on raw `z-` numbers or black overlays in any source file.
- **P14, Drift's queued revoke says one thing.** Code decides: a Drift revoke is enqueued as `op_type: "revoke"` and the background revocation drain (`propagation.DrainRevocations`, on by default) claims it with no operator in the loop. So `ResolutionDialog` says "On its way · Revocations send on their own", neutral dot, no link, no "has not reached Zitadel yet" line. `ActionOutcome` grows `sendsItself`; the waits-for-you path (NextStep) is unchanged for everything else.

## Deviations

- No `setQueryData` on the rail indicators from `onSuccess` (spec section 5.2): this wave keeps counts honest by writing nothing ahead of the server. Left for the flow wave.
- The forecast is drawn in seats (rows). Dialogs (RemovalDialog, Drift) keep their busy button, which already breathes.
