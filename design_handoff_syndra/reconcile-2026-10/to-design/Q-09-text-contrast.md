# Q-09 · Quiet text is too quiet to read

From Claude Code, 27 Oct 2026. Send this after Q-08. Nothing here reopens a verdict.

An audit measured the text tokens against WCAG AA. The rule is 4.5:1 for text under 18px, which is all of these. Three of them fail in both themes. Nothing is built yet: the change is to tokens, so it's yours.

## What's measured

Ink at its alpha over each ground (`ui/src/app/globals.css:444-460, 515-528`):

| Token | Used for | Dark (ground, canvas, surface-2) | Light (canvas, white) |
|---|---|---|---|
| `--muted` 0.60 | ledes, notes, secondary lines (319 uses, 13–14px) | 6.7, 6.7, 6.3 ✓ | **4.36** ✗, 4.53 ✓ |
| `--faint` 0.42 dark / 0.45 light | timestamps, "Ends 2 Aug", captions (322 uses, 12.5–14px) | **3.8, 3.8, 3.8** ✗ | **2.8, 2.9** ✗ |
| `--label` 0.34 / 0.40 | `type-label` and `type-nav-group`: column headings, rail group names, field labels (12.5px, 600, caps) | **2.8, 2.9, 2.9** ✗ | **2.5, 2.5** ✗ |

The lowest alpha that clears 4.5:1 on every ground is **0.48 in dark** and **0.61 in light**. The accent, warn and danger text tokens already pass (5.4–11.2).

## Options

**A · Every text token clears 4.5 (recommended).** The order stays the same; the steps get closer, and size, weight and caps carry more of the hierarchy.
- Dark: `--muted` 0.64, `--faint` 0.50, `--label` 0.48
- Light: `--muted` 0.70, `--faint` 0.62, `--label` 0.61

**B · Two text tokens, one decoration token.** `--faint` clears 4.5 as in A. `--label` merges into `--faint` for text, and the 0.34/0.40 step stays only for things that aren't text (dividers, empty rings, a disabled seat's outline), where 3:1 is the rule.

**C · Keep the look, lift only the failures on their worst ground.** Light `--muted` 0.62, the rest as in A. This is the least visible change, but it leaves two nearly equal steps in light.

## What I need

Pick A, B or C, or redraw the values. If the steps get too close, say how the hierarchy should be carried instead (weight, size or case).

**Answer in the design project at** `handoff/2026-10-reconcile/from-design/A-09-text-contrast.md`.
