# Selection seat (replaces RowCheckbox and SelectAllRow's checkbox)

Owner, 9 Oct: the round seat from the bundle picker (route-bundles-v2, 3a) is how a choice is marked everywhere, at the start of the row.

## The seat
- 24px circle, `1.5px solid --line-strong`, empty.
- Chosen: fill and border `--accent`, a check in `--accent-ink`.
- Some chosen (select-all only): the same fill with a dash.
- Locked (comes from elsewhere, can't change here): `--surface-2` fill, `--line-strong` border, check in `--faint`, plus a note saying where it comes from.
- Blocked (can't be chosen): empty seat at reduced opacity, with the reason as the row's description.
- Hit area stays 44px below desktop (the seat's span carries it), as RowCheckbox's GLYPH_BOX did.

## The row
- Lists (cards with row dividers): chosen row background is `--accent-soft`, full width, so dividers and row heights never change when Select turns on.
- Pickers in a dialog: rows are inset (12px radius, 6px apart), and a chosen row also gets a 1px `--accent-line` edge.
- The row is the control: a real checkbox (visually hidden input, or `role="checkbox"` + `aria-checked`), labelled with the row's name. Space toggles, and shift-click, drag-paint and Shift+Arrow from #33 keep working.

## Components
- `RowCheckbox`: same props, renders the seat. Pass `checked`, `disabled`, `indeterminate`.
- `SelectAllRow`: the seat replaces the input. The copy is unchanged ("Select these N …", "N … in all.", "Clear the selection").
- New `ChoiceRow` for pickers: `selected`, `locked`, `lockedNote`, `blocked`, `blockedReason`, `onToggle`, children. Used by the role picker for New bundle, Add roles, Grant direct access and Give a role. Single choice uses the same seat with a dot instead of a check.
