# Syndra conventions

Syndra is an access-management console for a university makerspace (who may use the laser cutter, the wood shop, the 3D printers). Dark is home, light is daylight; both themes are complete.

## Setup
Wrap every screen in `SyndraRoot`. It mounts the query client, name resolver, theme (`data-theme` on `<html>`) and error boundary - without it, name components and the shell cannot render.

```jsx
<SyndraRoot pathname="/users">{/* screen */}</SyndraRoot>
```

- `pathname` is the current route; `Sidebar`, `TopBar` and `TouchNav` mark the active nav item from it. Navigation itself comes from `BASIC_NAV` / `ADVANCED_NAV` - never hand-build a nav.
- `AppShell session={...}` is the page frame (sidebar + top bar + scroll area). `session.role: "admin"` is an operator (Basic/Advanced view switch); `"user"` is a member.
- Components that read data (`UserName`, `ProjectName`, `RoleRef`, `UserAvatar`, `DegradedBanner`) take ids, not names. Give them data through `SyndraRoot data={[{ key: ["name-catalog"], value: { users, projects, applications } }]}`; keys also include `["system", "mode"]` and `["governance", "indicators"]`.

## Styling idiom
Colour is semantic, never decorative:
- **accent** (violet) is what you can act on - one primary action per region.
- **healthy** (lime) means nothing is needed here. It is only ever a dot or a word, never a button or a fill.
- **warn** means a deadline or a broken assumption; nothing is lost yet.
- **danger** takes access away or reports a failure. Danger buttons are outlines; the solid fill (`dangerConfirm`) appears only on a dialog's confirming button.

Prefer the components (`Button` variants, `Badge` / `Chip` tones, `StatusDot`, `CardHeader tone`) over styling by hand. For your own layout glue, use CSS variables - never a literal hex:

| Role | Variables |
|---|---|
| Surfaces | `--ground` (page), `--canvas`, `--surface-0` `--surface-1` `--surface-2`, `--rail` |
| Text | `--ink`, `--muted`, `--faint`, `--label` |
| Lines | `--line`, `--line-strong`, `--divider` |
| Roles | `--accent` `--accent-text` `--accent-soft` `--accent-line`, `--healthy`, `--warn` `--warn-text` `--warn-soft`, `--danger` `--danger-text` `--danger-soft` |
| Type | `--font-sans` (Figtree, body), `--font-display` (Bricolage Grotesque, headings), `--font-mono` (JetBrains Mono, role keys / ids) |

Ready-made classes: `type-page-title`, `type-section-title`, `type-card-title`, `type-label`, `type-mono`, `card`, `panel`, `accent-note`, `warn-note`, `danger-note`, `row-divider`. Tailwind classes only exist if the app already uses them, so do not invent utility classes - use the variables above with inline styles.

## Where the truth lives
- `styles.css` and its import `_ds_bundle.css`: the app's compiled `globals.css`, with every token for both themes.
- `components/<group>/<Name>/<Name>.prompt.md` and `.d.ts`: each component's usage and props.

## Example
```jsx
<SyndraRoot pathname="/">
  <Card>
    <CardHeader title="Open requests" count={2}
      action={<CardHeaderLink href="/requests">See all →</CardHeaderLink>} />
    <CardRow first>
      <div style={{ width: 170, fontWeight: 600 }}>Aanya Rao</div>
      <div style={{ flex: 1, color: "var(--muted)" }}>Laser cutter · Operator</div>
      <Button size="sm" variant="accent">Approve</Button>
    </CardRow>
  </Card>
</SyndraRoot>
```

Copy is plain language: say what physically happens ("She loses badge entry to the laser bay"), never the system's terms ("grant deleted").

# Syndra (ui@0.1.0)

This design system is the published ui React library, bundled as a single
browser global. All 76 components are the real upstream code.

## Where things are

- `_ds_bundle.js` — the whole-DS bundle at the project root; loads every component to `window.Syndra`. First line is a `/* @ds-bundle: … */` metadata header.
- `styles.css` — the single stylesheet entry: it `@import`s the tokens, fonts, and component styles (`_ds_bundle.css`). Link this one file.
- `components/<group>/<Name>/<Name>.prompt.md` (example JSX + variants), `<Name>.d.ts` (types), `<Name>.html` (variant grid).
- `tokens/*.css` — CSS custom properties, names verbatim from upstream.
- `fonts/` — `@font-face` files + `fonts.css` (when the package ships fonts).

For a specific component, `read_file("components/<group>/<Name>/<Name>.prompt.md")`.

## Loading

Add these two lines to your page once (React must be on the page first):

```html
<link rel="stylesheet" href="styles.css">
<script src="_ds_bundle.js"></script>
```

Components are then available at `window.Syndra.*`. Mount into a dedicated child node (e.g. `<div id="ds-root">`), not the host page's own React root, so the two trees don't collide:

```jsx
const { AccountSheet } = window.Syndra;
ReactDOM.createRoot(document.getElementById('ds-root')).render(<AccountSheet />);
```

Wrap the tree in the provider — most components read theme/i18n from context:

```jsx
<SyndraRoot>{children}</SyndraRoot>
```

## Tokens

189 CSS custom properties from ui. Names are
preserved verbatim from upstream. They are declared inside `_ds_bundle.css` (this DS ships one compiled stylesheet rather than separate token files).

- **color** (47): `--color-black`, `--text-sm`, `--text-sm--line-height`, …
- **spacing** (7): `--tw-space-y-reverse`, `--door-arch-inset`, `--door-stage-inset`, …
- **typography** (14): `--font-sans`, `--font-mono`, `--font-weight-normal`, …
- **radius** (7): `--radius-lg`, `--radius-card`, `--radius-panel`, …
- **shadow** (12): `--tw-shadow`, `--tw-ring-shadow`, `--shadow-dialog`, …
- **other** (102): `--spacing`, `--default-transition-duration`, `--default-transition-timing-function`, …

## Components

### shell
- `AccountSheet` — Who you are signed in as, and the two things that belong to this device.
- `AppShell` — The operator shell.
- `Sidebar` — The navigation rail. 252px, its own background, one border on the right.
- `SyndraMark` — The mark: a contained orb, and a miniature of the login's arch-and-orb.
- `ThemeToggle` — Dark is home light is daylight. The toggle is a quiet neighbour of the
- `TopBar` — The 66px header: where you are on the left, what you're looking at on the
- `TouchNav` — Navigation for a thumb.
- `ViewSwitch`

### general
- `AcknowledgeCount` — The acknowledgement ladder (design 31 B).
- `ActionOutcome`
- `Avatar`
- `Badge`
- `Button`
- `ButtonLink` — A navigation that looks like a button.
- `Card` — The container surface: 20px radius, one hairline, no padding of its own so
- `CardColumns` — Column headings for a table-shaped card.
- `CardHeader`
- `CardHeaderLink` — The quiet way out of a card  See all, Full audit log.
- `CardRow` — One item in a card. The first row omits its divider.
- `Chip` — A neutral outline pill  bundle membership, an app served, a role group.
- `ClipboardUnavailableNote` — The sentence that goes under a copy row when the browser cannot copy.
- `ClockTime`
- `CommandBlock` — A command the operator runs in their own terminal, and the steps around it.
- `ConfirmByTyping` — Rung 3  typing the name is what unlocks the control.
- `CopyableValue` — A value somebody retypes into another application.
- `CountChip` — The count in a region heading. One seat, three things it can say.
- `FieldHint` — The helper line beneath a field  plain language, never a spec.
- `FieldLabel` — A field label  12.5px/600, quiet, sitting directly above its control.
- `FilterBar` — The one row every list screen narrows itself from.
- `FilterChip` — A filter that is in force, with the way out attached to it.
- `FilterField` — One labelled row inside the panel. The label is a real label when the
- `FilterPanel` — A page's filters, behind one button.
- `FilterPills` — Filter pills  like Segmented, but neutral, because a filter is not an action.
- `Input` — Text input. 14px radius rather than a pill: a field the operator types a
- `LogTime` — The wall-clock time this page last looked. Client-only: on the server it is
- `MetaDot` — Dot separator for a metadata row.
- `MetaRow` — A metadata row: children separated by 3px dots.
- `Modal`
- `ModalFooter`
- `ModalHeader` — Dialog header: an optional source chip above the title, then the title.
- `Mono` — A role key, grant id or claim name. Always mono, never the body face.
- `PageHeader` — The page's own title block. The breadcrumb says where you are this says
- `PlanReview` — The rehearsal, rendered.
- `ReadFreshness` — The strip itself.
- `RehearsalDialog`
- `Relative` — Anything whose value depends on now is rendered client-side only.
- `RowCheckbox` — A row checkbox. Spread selection.checkboxProps(id) onto it.
- `RowField` — The label/value pair a disclosed row is built from.
- `Segmented` — A segmented pill  two or more mutually exclusive options, all legible at
- `Select` — Native select, styled to match Input. Native rather than a custom listbox:
- `SelectAllRow` — Select-all, in words, with both numbers.
- `SelectionAction` — A verb in the selection bar. tonedanger for anything that takes access away.
- `SelectionBar`
- `SelectModeToggle` — The named control that turns selection on, and the same control that turns
- `Skeleton` — Pulse-animated placeholder block. Use in space-y-3 stacks alongside
- `SkeletonCardList` — Convenience: a stack of card-sized skeletons.
- `StatCard` — The 3-tile stat row: label caps / big value / one-line sub.
- `StatusDot`
- `Tabs` — The page-level tab row: two or three views of one subject, switched in place.
- `Term` — A word that carries its own definition.
- `Textarea` — A short reason, two rows tall rather than one line stretched to a wall.
- `Withheld` — The panel, for the whole-target case: one or more things held, each with why.
- `WithheldInline` — The one-line form, for a row inside a dense list.
- `WithheldPill` — The inline pill, for a row inside a longer list  a door, a share.

### names
- `BundleName`
- `ProjectName`
- `ResolvedProjectName` — For a project name a server view already resolved (or fell back on) 
- `RoleRef` — A role reference  always the PAIR, never the key on its own.
- `UserAvatar` — An avatar for a user id rather than a name.
- `UserName` — Renders the resolved display name for a Zitadel user id. While loading,

### states
- `DegradedBanner` — Degraded  the fourth state, and the one that matters.
- `EmptyState` — Empty  a sentence naming what is absent, one line of guidance, one link to
- `ErrorState` — Error  names the failed thing, confirms nothing was changed, offers a
- `ListStates` — The standard list wrapper: one component so no view can quietly skip a
- `OfflineBanner` — No network, said as its own thing.
- `RowSkeleton` — Four states, every list view. Not three  degraded is real and specific and
