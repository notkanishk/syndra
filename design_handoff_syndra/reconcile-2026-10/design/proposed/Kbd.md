# Kbd (proposed, A-19, 5 Nov 2026)

A drawn key, for naming a keyboard key in running text. Drawn inline on both A-19 boards (`.a19-kbd`).

## What it is
A `<kbd>` styled as a small cap: 20px tall, at least 22px wide, 6px side padding, `1px solid var(--line-strong)` edge, `--radius-sm`, `--tint-1` fill, `--font-mono` 11.5px in `--muted`. Both themes are the tokens, so nothing is theme-specific.

## Where it appears
- The Keys sheet (A-19 P7), one or two per row.
- Go to's footer line.
- Nowhere else. A button never shows its key: the shortcut is not the button's label, and a hint on every button is clutter on the 50th view.

## Copy
A key is written as it is printed on the key: `/`, `?`, `Esc`, `Enter`, `Tab`, `↑`, `↓`. Modifier keys are named by platform at render time: `⌘` on a Mac, `Ctrl` elsewhere, with `K` as a second cap.

## Motion
None. It is text.

## Not for
- Inline instructions in a lede or a Notice ("press / to…"): if a key must be named in prose, it is still a Kbd, but prose that names keys is a sign the sheet is missing.
- Drawing a button that looks like a key.

## Props
```ts
type KbdProps = { children: string; className?: string };
```
