"use client";

import { useEffect, useRef, useState } from "react";

/**
 * Access source — the signature component.
 *
 * It answers "why does this person have this", and it is the single element
 * that does most to make this product feel calm. Built once, used in role
 * members, person detail, unexplained-access triage and change history.
 *
 * The data model emits exactly three kinds, so the vocabulary is fixed and
 * small. Order is ALWAYS Direct → Via bundle → Automatic, everywhere, so a
 * scanning eye learns one sequence.
 *
 * The dot carries the meaning before the word does:
 *   solid  — a person did it
 *   ring   — a bundle did it
 *   dashed — the system did it on its own. Dashed means nobody clicked it.
 *
 * These chips encode a KIND, not a severity. Direct takes the accent because
 * it is the one source an operator can act on; the other two stay neutral.
 * A source never borrows a severity colour, and severity never rides on a
 * source chip.
 */

export type SourceKind = "direct" | "bundle" | "mapping";

export interface RoleReason {
  kind: string;
  description?: string;
  bundle_id?: string;
  bundle_name?: string;
  trigger_project?: string;
  trigger_role?: string;
}

const ORDER: Record<SourceKind, number> = { direct: 0, bundle: 1, mapping: 2 };

export const SOURCE_LABEL: Record<SourceKind, string> = {
  direct: "Direct",
  bundle: "Via bundle",
  mapping: "Automatic",
};

export function isSourceKind(kind: string): kind is SourceKind {
  return kind === "direct" || kind === "bundle" || kind === "mapping";
}

/** Sorts reasons into the fixed reading order and drops unknown kinds. */
export function orderedSources(reasons: RoleReason[] | undefined | null): RoleReason[] {
  return (reasons ?? [])
    .filter((reason) => isSourceKind(reason.kind))
    .slice()
    .sort((a, b) => ORDER[a.kind as SourceKind] - ORDER[b.kind as SourceKind]);
}

/** The qualifier that follows a chip: "Lab Tech", "3D Lab / operator". */
export function sourceQualifier(reason: RoleReason): string | undefined {
  if (reason.kind === "bundle") return reason.bundle_name;
  if (reason.kind === "mapping" && reason.trigger_project && reason.trigger_role) {
    return `${reason.trigger_project} / ${reason.trigger_role}`;
  }
  return undefined;
}

export function AccessSource({
  kind,
  detail,
  className = "",
}: {
  kind: SourceKind;
  detail?: string;
  className?: string;
}) {
  return (
    <span className={`inline-flex items-center gap-[7px] whitespace-nowrap ${className}`}>
      <SourceChip kind={kind} />
      {detail && <span className="text-[12.5px] text-muted">{detail}</span>}
    </span>
  );
}

export function SourceChip({ kind }: { kind: SourceKind }) {
  const base =
    "inline-flex items-center gap-1.5 rounded-pill py-[5px] pl-[9px] pr-[11px] text-[12.5px] font-semibold leading-none whitespace-nowrap";

  if (kind === "direct") {
    return (
      <span className={`${base} bg-accent-soft text-accent-text`}>
        <i aria-hidden className="block h-2 w-2 flex-none rounded-pill bg-accent" />
        {SOURCE_LABEL.direct}
      </span>
    );
  }
  if (kind === "bundle") {
    return (
      <span className={`${base} bg-tint-2 text-ink/[.82]`}>
        <i
          aria-hidden
          className="block h-2 w-2 flex-none rounded-pill border-2 border-ink/80"
        />
        {SOURCE_LABEL.bundle}
      </span>
    );
  }
  return (
    <span className={`${base} border border-dashed border-ink/[.34] text-ink/[.66]`}>
      <i
        aria-hidden
        className="block h-2 w-2 flex-none rounded-pill border border-dashed border-ink/[.66]"
      />
      {SOURCE_LABEL.mapping}
    </span>
  );
}

/**
 * The multiples form. A role can carry more than one source; render the
 * strongest as a full chip and collapse the rest behind a count of the same
 * height. Never a wall of chips.
 */
export function AccessSourceList({
  reasons,
  showQualifier = true,
}: {
  reasons: RoleReason[];
  showQualifier?: boolean;
}) {
  const ordered = orderedSources(reasons);
  if (ordered.length === 0) {
    return <span className="text-[13.5px] text-faint">No recorded source</span>;
  }

  const [strongest, ...rest] = ordered;
  const qualifier = showQualifier ? sourceQualifier(strongest) : undefined;

  return (
    <span className="flex flex-wrap items-center gap-[9px]">
      <SourcePopover reason={strongest} />
      {qualifier && <span className="text-[13.5px] text-muted">{qualifier}</span>}
      {rest.length > 0 && <MoreSources rest={rest} />}
    </span>
  );
}

/**
 * The sources the row did not have room for.
 *
 * This used to carry their names in a `title` attribute and nowhere else,
 * which is a hover tooltip: unreachable by touch, unreachable by keyboard, and
 * unreadable by a screen reader. On the component whose entire job is to
 * answer "why does this person have this", the other half of the answer was
 * available only to somebody holding a mouse.
 *
 * It opens instead. The collapsed state is still the default — "never a wall
 * of chips" is the rule this component is built around — but the wall is one
 * tap away rather than behind a pointer.
 */
function MoreSources({ rest }: { rest: RoleReason[] }) {
  const [open, setOpen] = useState(false);

  if (open) {
    return (
      <>
        {rest.map((reason, at) => (
          <SourcePopover key={`${reason.kind}-${at}`} reason={reason} />
        ))}
        <MoreSourcesButton onClick={() => setOpen(false)} expanded>
          Fewer
        </MoreSourcesButton>
      </>
    );
  }

  return (
    <MoreSourcesButton onClick={() => setOpen(true)} expanded={false}>
      +{rest.length} more
    </MoreSourcesButton>
  );
}

/** The 26px pill, inside a target a thumb can hit. */
function MoreSourcesButton({
  onClick,
  expanded,
  children,
}: {
  onClick: () => void;
  expanded: boolean;
  children: React.ReactNode;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-expanded={expanded}
      className="inline-flex h-11 items-center motion-tint desktop:h-[26px]"
    >
      <span className="inline-flex h-[26px] items-center rounded-pill border border-line-strong px-2.5 text-[12.5px] font-semibold text-muted">
        {children}
      </span>
    </button>
  );
}

/**
 * The expanded form: a popover on the chip. Header is the chip plus a
 * plain-language title; body is a By / When / Expires / Note grid; the footer
 * carries only the action that belongs to THIS source.
 */
export function SourcePopover({
  reason,
  by,
  when,
  expires,
  note,
  footer,
}: {
  reason: RoleReason;
  by?: string;
  when?: string;
  expires?: React.ReactNode;
  note?: string;
  footer?: React.ReactNode;
}) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLSpanElement | null>(null);
  const kind = reason.kind as SourceKind;

  useEffect(() => {
    if (!open) return;
    function onDocument(event: MouseEvent) {
      if (ref.current && !ref.current.contains(event.target as Node)) setOpen(false);
    }
    function onKey(event: KeyboardEvent) {
      if (event.key === "Escape") setOpen(false);
    }
    document.addEventListener("mousedown", onDocument);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("mousedown", onDocument);
      document.removeEventListener("keydown", onKey);
    };
  }, [open]);

  const rows: Array<[string, React.ReactNode]> = [];
  if (by) rows.push(["By", by]);
  if (when) rows.push(["When", when]);
  if (expires) rows.push(["Expires", expires]);
  if (note) rows.push(["Note", <span key="note" className="text-muted">{note}</span>]);
  const hasBody = rows.length > 0 || Boolean(footer);

  return (
    <span ref={ref} className="relative inline-flex">
      <button
        type="button"
        onClick={() => hasBody && setOpen((value) => !value)}
        aria-expanded={hasBody ? open : undefined}
        className={hasBody ? "cursor-pointer" : "cursor-default"}
      >
        <SourceChip kind={kind} />
      </button>

      {open && (
        <span className="absolute left-0 top-[calc(100%+8px)] z-30 w-[380px] settle-in rounded-panel border border-line-strong bg-surface-2 shadow-popover">
          <span className="flex items-center gap-2.5 border-b border-line px-[18px] py-4">
            <SourceChip kind={kind} />
            <span className="font-display text-[19px] font-semibold">{expandedTitle(kind)}</span>
          </span>
          {rows.length > 0 && (
            <span className="grid grid-cols-[80px_1fr] gap-x-3.5 gap-y-2 px-[18px] py-4 text-[14px]">
              {rows.map(([label, value]) => (
                <span key={label} className="contents">
                  <span className="text-faint">{label}</span>
                  <span>{value}</span>
                </span>
              ))}
            </span>
          )}
          {footer && (
            <span className="flex items-center gap-2.5 border-t border-line px-[18px] py-3.5">
              {footer}
            </span>
          )}
        </span>
      )}
    </span>
  );
}

function expandedTitle(kind: SourceKind): string {
  if (kind === "direct") return "Granted directly";
  if (kind === "bundle") return "Carried by a bundle";
  return "Produced by a rule";
}
