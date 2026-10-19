# PROPOSED — AnchoredPanel

Status: proposed 17 Oct 2026 (A-04 §3). Where every popover draws: the source popover, Filters, Term, the access map's search. Board: `Controls` §3.

## Rules
- Draws in its own layer above the page (portal), never inside what opened it. No card's clip or rise animation can reach it.
- 7px from its trigger. Never closer than 16px to the window's edge.
- Under by default. Above only when there's more room up than down. It never covers its own trigger.
- `align="start"` by default. `align="end"` when the trigger ends its row (Filters).
- `matchWidth` for a search's results: exactly the field's width.
- Escape and an outside click dismiss it, and focus returns to the anchor.

## Props
```ts
interface AnchoredPanelProps {
  anchor: React.RefObject<HTMLElement>;
  open: boolean;
  onDismiss: () => void;
  align?: "start" | "end";
  matchWidth?: boolean;
  maxWidth?: number;            // source popover 380
  children: React.ReactNode;
}
```

## Below tablet
A popover you read stays a popover, at the window's width less 32 (source, Term). A popover you act in, with a choice or a button, opens as a Modal sm sheet (Filters, 87c).

## Surface
`--surface-2`, 1px `--line-strong`, 14px radius, `--shadow-dialog`. Arrives with tint, opacity only, and never travels.
