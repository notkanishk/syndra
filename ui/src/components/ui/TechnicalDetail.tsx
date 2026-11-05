"use client";

import { useState, type ReactNode } from "react";

import { FoldChevron } from "@/components/ui/FoldChevron";
import { usePlace } from "@/lib/place";
import { Unfold } from "@/lib/motion";

/**
 * The one fold for the machine's words (A-20 P11, proposed/TechnicalDetail.md):
 * an id, a payload, a command, an expected-and-found pair. Closed by default.
 * Everything outside it is a sentence; nothing inside it is rewritten.
 *
 * It holds one block: a `TokenBlock` for JSON, a `CommandBlock` for commands, or
 * `CopyableValue`s for ids. A `<pre>` for a payload comes through `TechnicalPayload`.
 */
export function TechnicalDetail({ placeKey, ...props }: TechnicalDetailProps) {
  return placeKey ? <PlacedFold name={placeKey} {...props} /> : <Fold {...props} />;
}

function PlacedFold({ name, ...props }: { name: string } & Omit<TechnicalDetailProps, "placeKey">) {
  const [open, setOpen] = usePlace<boolean>(`technical:${name}`, false);
  return <Fold {...props} open={open} onToggle={(next) => (props.onToggle?.(next), setOpen(next))} />;
}

type TechnicalDetailProps = {
  /** Controlled when given; otherwise the fold keeps its own state. */
  open?: boolean;
  onToggle?: (open: boolean) => void;
  /** One block: TokenBlock, CommandBlock, or CopyableValues. */
  children: ReactNode;
  /** Names the fold in the place store when the page has more than one: a Back keeps it open. */
  placeKey?: string;
  className?: string;
};

function Fold({
  open,
  onToggle,
  children,
  className = "",
}: Omit<TechnicalDetailProps, "placeKey">) {
  const [own, setOwn] = useState(false);
  const shown = open ?? own;
  const toggle = () => {
    onToggle?.(!shown);
    if (open === undefined) setOwn(!shown);
  };
  return (
    <div className={`flex flex-col gap-2.5 ${className}`} data-technical-detail>
      <button
        type="button"
        aria-expanded={shown}
        onClick={toggle}
        className="inline-flex min-h-[var(--tap)] items-center gap-2.5 self-start rounded-pill text-left control press-scale tablet:min-h-0"
      >
        <FoldChevron open={shown} />
        <span className="text-[14px] font-semibold text-ink">Technical detail</span>
        <span className="text-[12.5px] text-faint">for whoever runs the Syndra server</span>
      </button>
      <Unfold open={shown}>
        <div className="flex flex-col gap-2 [overflow-wrap:anywhere]">{children}</div>
      </Unfold>
    </div>
  );
}

/** A payload kept exact: mono on surface-2, wrapping rather than scrolling sideways. */
export function TechnicalPayload({ children }: { children: ReactNode }) {
  return (
    <pre className="m-0 whitespace-pre-wrap rounded-inner border border-line bg-surface-2 px-3.5 py-3 font-mono text-[12.5px] leading-[1.55] text-muted [overflow-wrap:anywhere]">
      {children}
    </pre>
  );
}
