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
| Tints and states | `--tint-1` `--tint-2` `--tint-3` (a quiet fill, a chip, a pressed row), `--hover` (a row under the pointer), `--focus-ring` (the ring on every focused control: violet-300 in dark, the accent text tone in light) |
| Scrim and shadows | `--scrim` (the wash behind a dialog or sheet: black in dark, violet-black in light), `--shadow-dialog`, `--shadow-popover`, `--shadow-card` |
| Tap | `--tap` (44px): nothing a thumb must hit is smaller; from the desktop breakpoint (1080px) a control may be denser |
| Layers | `--layer-bar` 20 (sticky bars), `--layer-pill` 30 (a floating pill), `--layer-dialog` 50 (dialogs, sheets and their scrim), `--layer-popover` 60 (over a dialog). Use `z-(--layer-dialog)`, never a number |
| Radius | `--radius-card` 20, `--radius-panel` 18, `--radius-block` 16, `--radius-inner` 14, `--radius-nav` 12, `--radius-pill` 999 |
| Motion | `--duration-tint` 120ms, `-press` 180, `-settle` 260, `-arrive` 420, `-flash` 900, `-breathe` 2400, `-fold` 200 (a closing region, quicker than it opened), `-glide` 520 (scrolling only); `--light-lead` 40ms (a card follows its scrim); curves `--ease-tint` `-press` `-settle` `-arrive` `-breathe` `-flash`; `--travel-cap` 10px (nothing in the page moves further), `--travel-arrive` 8px, `--travel-lift` 6px, `--travel-nudge` 4px; `--arrive-step` 40ms and `--arrive-cap` 5 (rows stagger 40ms apart, capped at five) |
| Page | `--page-max` 1280px (the content's widest), `--page-x` (the side inset that centres it), `--measure` 60ch (a line of sentences), `--field-max` 400px |
| Rows and cards | `--card-x` 20px (a card's one inset), `--row-y` 10px (a row's padding: 50px with a button, 41 without), `--row-gap` 16px, `--region-gap` 40px between regions. A phone keeps its own: `--row-y` 14, `--row-gap` 18, `--region-gap` 32 |
| Seats | A seat's width belongs to the table, not the row: `--seat-label` 170px, `--seat-number` 110px, `--seat-status` 240px, `--seat-action-narrow` 140px, `--seat-action` 200px, `--seat-action-wide` 300px. Minimums, so an empty seat keeps its place |
| Type | `--font-sans` (Figtree, body), `--font-display` (Bricolage Grotesque, headings), `--font-mono` (JetBrains Mono, role keys / ids) |

Ready-made classes: `type-greeting`, `type-page-title`, `type-section-title`, `type-dialog-title`, `type-card-title`, `type-empty-title`, `type-label`, `type-nav-group`, `type-mono`; the type scale `text-body` (14px), `text-small` (13px), `text-fine` (12.5px, the floor), `text-figure` (a stat); `measure`, `card`, `panel`, `warn-note`, `danger-note`, `row-divider`. Tailwind classes only exist if the app already uses them, so do not invent utility classes - use the variables above with inline styles.

## Moves and Phone
Every card opens with two lines: **Moves** (which verb moves, and its still form) and **Phone** (what changes below the 720px tablet breakpoint; `desktop:` is 1080px). Believe them: a card that says a seat is still draws a still seat. The verbs are press (3% scale), tint, settle (a sheet rises 8px from 98%), arrive (rows rise 8px, 40ms apart), unfold, leave, crossfade, glide, flash, bloom, lift (6px) and breathe, the one loop, which means "still happening". Every verb has a still form for reduced motion, and the Moves line gives it. A card marked "Internal: never mounted alone" is a helper a screen uses, not a part to place on a board.

## Where the truth lives
- `styles.css` and its import `_ds_bundle.css`: the app's compiled `globals.css`, with every token for both themes.
- `components/<group>/<Name>/<Name>.prompt.md` and `.d.ts`: each component's usage and props.

## Example
```jsx
<SyndraRoot pathname="/">
  <Card>
    <CardHeader title="Open requests" count={2}
      action={<CardHeaderLink href="/requests">All requests</CardHeaderLink>} />
    <CardRow first>
      <div style={{ width: 170, fontWeight: 600 }}>Roman Reigns</div>
      <div style={{ flex: 1, color: "var(--muted)" }}>Laser cutter · Operator</div>
      <Button size="sm" variant="accent">Approve</Button>
    </CardRow>
  </Card>
</SyndraRoot>
```

Copy is plain language: say what physically happens ("She loses badge entry to the laser bay"), never the system's terms ("grant deleted").

## Talking to Claude Code
This system is built from the Syndra repo by `ui/.design-sync` and published to the Syndra Design System artifact; the Syndra Project canvas installs a copy at `project/ds/syndra/`. Never edit components here; tokens live in `tokens.json`. Questions from code arrive as `Q-NN-<slug>.md`; each answer is `handoff/<handoff>/from-design/A-NN-<slug>.md` on the canvas, with its boards as `<name>.dc.html` artboards. A component this system lacks is drawn as `proposed/<Name>.md`, built in code, then synced back. The full protocol is `design_handoff_syndra/BRIDGE.md` in the repo.
