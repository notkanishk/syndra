# PROPOSED — Checkbox, Radio, ChoiceRow

Status: proposed 17 Oct 2026 (A-04 §5, §7.7). Board: `Controls`.

## Checkbox and Radio
- The selection seat's round glyph: 16px with a mouse, 24px on touch. The row gives the 44px hit area.
- **On**: an `--accent` fill and border. Checkbox shows a ✓ in `--accent-ink`; Radio shows a 6px dot (8px on touch).
- **Off**: a 1.5px `--line-strong` ring.
- **Focus**: `0 0 0 2px var(--ground), 0 0 0 4px var(--focus-ring)`.
- **Held** (you have it, or it's locked by a bundle): a `--surface-2` fill, a ✓ in `--faint`, muted words, and why in the note seat. Not pickable.
- **Can't be chosen**, not held: an empty `--line` ring, faint words, and why.

## ChoiceRow
The card form, and the picker's row. A 12px radius, 10px 12px padding, 6px between rows. The seat leads, then the name and description, then the note seat on the right (max 170, right-aligned, 12.5px).
- Chosen: `--accent-line` border, `--accent-soft` fill.
- Held: `--surface-0` fill, no border.
- Used by RolePicker, the removal dialog's source chooser, Manage bundles, Publish version and Assign.
