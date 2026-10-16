"use client";

import { useEffect, useState } from "react";

import type { Tone } from "@/components/insights/model";
import { TONE_COLOUR } from "@/components/insights/tone";
import { ArrowLink } from "@/components/ui/ArrowLink";
import { Button } from "@/components/ui/Button";
import { FilterChip } from "@/components/ui/FilterChip";
import { useClipboardAvailable } from "@/lib/clipboard";

export interface Panel {
  tone: "accent" | "warn" | "danger" | "neutral";
  eyebrow: string;
  title: string;
  meta?: string;
  /** A line may open with a bold lead: "By hand:", a person's name. */
  lines: Array<{ tone: Tone; lead?: string; text: string }>;
  open?: { label: string; href: string };
}

const BOX = {
  accent: "border-accent-line bg-accent-soft",
  warn: "border-warn-line bg-warn-soft",
  danger: "border-danger-line bg-danger-soft",
  neutral: "border-line bg-surface-1",
} as const;

/** The panel's words as plain text: its title, then one sentence a line. */
export const panelText = (panel: Panel) =>
  [panel.title, ...panel.lines.map((l) => (l.lead ? `${l.lead} ${l.text}` : l.text).trim())].join("\n");

/**
 * The answer, in sentences. The map draws the thread; this says it, and links
 * to the page where it is changed. Its footer keeps the pins and copies what
 * it says, so an answer can leave the page as words.
 */
export function AnswerPanel({
  panel,
  onBack,
  pins = [],
  onUnpin,
  onPin,
  onlyPinned,
  phone = false,
}: {
  panel: Panel;
  onBack?: () => void;
  pins?: Array<{ id: string; name: string; tone: Tone }>;
  onUnpin?: (id: string) => void;
  /** Present only when the followed thing can be pinned. */
  onPin?: () => void;
  onlyPinned?: () => void;
  /** Below tablet the panel is the page, and the pin says what it pins. */
  phone?: boolean;
}) {
  const clipboard = useClipboardAvailable();
  const [copied, setCopied] = useState(false);
  useEffect(() => {
    if (!copied) return;
    const t = setTimeout(() => setCopied(false), 1600);
    return () => clearTimeout(t);
  }, [copied]);

  async function copy() {
    try {
      await navigator.clipboard.writeText(panelText(panel));
      setCopied(true);
    } catch {
      // Refused (a permission, an insecure page): the button stays as it was.
    }
  }

  return (
    <section
      aria-label={panel.title}
      className={`tint-in flex flex-col gap-4 rounded-panel border px-5 py-[18px] tablet:flex-row tablet:flex-wrap tablet:items-start tablet:gap-x-6 ${BOX[panel.tone]}`}
    >
      <div className="flex min-w-0 flex-col gap-1.5 tablet:min-w-[240px] tablet:flex-[0_1_320px]">
        <span className="type-label text-label">{panel.eyebrow}</span>
        <h2 className="font-display text-[24px] font-semibold leading-[1.15] [text-wrap:pretty]">{panel.title}</h2>
        {panel.meta && <span className="text-[13.5px] text-muted">{panel.meta}</span>}
      </div>

      <div className="flex min-w-0 flex-col gap-[7px] tablet:flex-[1_1_380px]">
        {panel.lines.map((line, i) => (
          <p key={i} className="flex items-baseline gap-2.5 text-[14px] leading-[1.55] [text-wrap:pretty]">
            <span
              aria-hidden
              className="size-[7px] flex-none rounded-pill"
              style={{ background: TONE_COLOUR[line.tone] }}
            />
            <span>
              {line.lead && <strong className="font-semibold">{line.lead} </strong>}
              {line.text}
            </span>
          </p>
        ))}
      </div>

      <div className="flex flex-wrap items-center gap-x-3 gap-y-1 tablet:flex-col tablet:items-end">
        {onBack && (
          <Button variant="ghost" size="sm" onClick={onBack}>
            Back
          </Button>
        )}
        {panel.open && <ArrowLink href={panel.open.href}>{panel.open.label}</ArrowLink>}
      </div>

      <div className="flex flex-wrap items-center gap-2 border-t border-divider pt-3 tablet:basis-full">
        {pins.length > 0 && (
          <>
            <span className="type-label text-label">Pinned</span>
            {pins.map((pin) => (
              <FilterChip key={pin.id} onClear={() => onUnpin?.(pin.id)} clearLabel={`Unpin ${pin.name}`}>
                <span className="inline-flex items-center gap-2">
                  <span aria-hidden className="size-2 rounded-pill" style={{ background: TONE_COLOUR[pin.tone] }} />
                  {pin.name}
                </span>
              </FilterChip>
            ))}
          </>
        )}
        <span className="flex-1" />
        {onPin && (
          <Button variant={phone ? "outline" : "ghost"} size="sm" onClick={onPin}>
            {phone ? "Pin this" : "Pin"}
          </Button>
        )}
        {onlyPinned && (
          <Button variant={phone ? "outline" : "ghost"} size="sm" onClick={onlyPinned}>
            Show pinned only
          </Button>
        )}
        {clipboard && (
          <Button variant={phone ? "outline" : "ghost"} size="sm" onClick={copy}>
            {copied ? "Copied" : "Copy as sentences"}
          </Button>
        )}
      </div>
    </section>
  );
}
