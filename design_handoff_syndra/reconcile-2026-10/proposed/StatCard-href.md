# StatCard: `href` (proposed prop)

Ledger #24 says every number on Home links to the list it counts. StatCard has no link prop, so the board wraps each card in an `<a>`. Code should own this instead.

## Prop
- `href?: string`. When set, the whole card is the link. The accessible name is the label. On hover a 1px `--accent-line` ring appears, and the value does not change colour. There is no trailing arrow, because the card itself is the target.

## Rule
A StatCard without `href` is allowed only where its number is not a list, such as a percentage of the whole.

## Boards
route-home (The makerspace).
