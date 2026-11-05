import type { CSSProperties } from "react";

import type { Lane, Tone } from "@/components/insights/model";
import { StatusDot, type StatusTone } from "@/components/ui/Badge";

/** Tokens only. A pin's shades are mixes of the accent, never a second hue. */
export const TONE_COLOUR: Record<Tone, string> = {
  accent: "var(--accent)",
  accent2: "color-mix(in oklch, var(--accent) 50%, var(--ink))",
  accent3: "color-mix(in oklch, var(--accent) 55%, var(--canvas))",
  warn: "var(--warn)",
  danger: "var(--danger)",
  ink: "var(--ink)",
};

const DOT_TONE: Partial<Record<Tone, StatusTone>> = { accent: "accent", warn: "warn", danger: "danger" };

/** A tone's dot is the product's StatusDot; a pin's shade of the accent is its fill. */
export function ToneDot({ tone, className = "" }: { tone: Tone; className?: string }) {
  const named = DOT_TONE[tone];
  return <StatusDot tone={named ?? "neutral"} fill={named ? undefined : TONE_COLOUR[tone]} className={className} />;
}

const SOFT: Partial<Record<Tone, string>> = {
  accent: "var(--accent-soft)",
  warn: "var(--warn-soft)",
  danger: "var(--danger-soft)",
};

/**
 * A node's box: border and fill say its part in the answer. One with no part
 * recedes to the canvas ground with a line-strong edge, never dashed (dashed means NotOpenYet, A-20 P12) and keeps its words at full
 * strength: an opacity took its count line under 4.5:1 (A-09 §2).
 */
export function nodeSurface(tone: Tone | undefined, focused: boolean, unused: boolean, dim: boolean): CSSProperties {
  const border = focused
    ? "var(--accent)"
    : tone === "accent"
      ? "var(--accent-line)"
      : tone === "ink"
        ? "var(--line-strong)"
        : tone
          ? TONE_COLOUR[tone]
          : unused
            ? "var(--line-strong)"
            : "var(--line)";
  return {
    borderColor: dim ? "var(--line)" : border,
    borderStyle: "solid",
    background: (tone && SOFT[tone]) || (dim ? "transparent" : unused && !tone ? "var(--canvas)" : "var(--surface-1)"),
  };
}

/** Ink only: violet means something can be done here. */
export function NodeGlyph({ kind }: { kind: Lane }) {
  const shape =
    kind === "project"
      ? "rounded-[3px] bg-ink"
      : kind === "role"
        ? "rounded-pill bg-ink"
        : kind === "bundle"
          ? "rounded-[3px] border-[1.5px] border-ink"
          : "rounded-pill border-[1.5px] border-ink";
  return <span aria-hidden className={`inline-block h-2.5 w-2.5 flex-none ${shape}`} />;
}
