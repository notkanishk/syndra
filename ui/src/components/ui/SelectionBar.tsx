"use client";

import { useEffect, useState } from "react";

import { Button, buttonClasses } from "@/components/ui/Button";
import { Seat } from "@/components/ui/Choice";
import { Modal, ModalFooter, ModalHeader } from "@/components/ui/Modal";
import { LIGHT_LEAD_MS, TINT_MS, stillMotion, useExitPhase } from "@/lib/motion";
import { useIsPhone } from "@/lib/useViewport";

/**
 * The one selection bar.
 *
 * Docked at the bottom rather than inserted above the list: a bar that appears
 * in the flow pushes every row down the moment you tick the first checkbox,
 * which moves the row you were about to tick next out from under the cursor.
 * Structure does not move in response to data here either.
 *
 * It states the selection in words before offering any verb, because the count
 * is the thing an operator is about to be wrong about — "47" read at a glance
 * is indistinguishable from "4" or "470", and the difference is dozens of
 * people's access. `composition` is the second half of that: what the selection
 * is made of, not just how much of it there is.
 */

/** A verb on the bar, as data, so one list draws both the desktop row and the phone's two rows. */
export interface SelectionVerb {
  key: string;
  label: string;
  /** `danger` for anything that takes access away. */
  tone?: "neutral" | "danger";
  onClick: () => void;
}

interface SelectionBarProps {
  count: number;
  /** Singular/plural noun for what is selected: "person"/"people", "item"/"items". */
  noun: [string, string];
  /** Where the selection came from — "in Laser Lab", "matching 'ada'". */
  scope?: string;
  /** What it is made of — "8 safety-gated · 3 people". */
  composition?: string;
  /** True when the selection came from select-all rather than individual clicks. */
  wholeScope?: boolean;
  /** Rows currently rendered; the escape hatch offers to narrow to these. */
  visibleCount?: number;
  onSelectVisibleOnly?: () => void;
  /** The most this surface's bulk endpoint will accept in one run. */
  ceiling?: number;
  /**
   * What the ceiling counts, when that is not what the bar counts.
   *
   * Usually they are the same thing and this stays unset. Expiring access is
   * the exception in this product: it selects GRANTS and the endpoint caps
   * distinct PEOPLE, so 600 grants held by 300 people is legal and 500 grants
   * held by 500 people is already at the limit. Gating on the selection count
   * there would be wrong in both directions.
   */
  ceilingCount?: number;
  /** Singular/plural for `ceilingCount`. Defaults to `noun`. */
  ceilingNoun?: [string, string];
  /** Narrows the selection to the first `ceiling` rows, in the order shown. */
  onTakeCeiling?: () => void;
  onClear: () => void;
  /**
   * The verbs as data. Below tablet, three or more become one primary verb and
   * "More changes", a sheet of the rest (A-12 §2); above it, one row as always.
   */
  verbs?: SelectionVerb[];
  /** The key of the verb the phone bar offers first. Defaults to the first verb. */
  primary?: string;
  /** The verbs, for a bar with one or two: rendered right-aligned, unchanged at every width. */
  children?: React.ReactNode;
}

/**
 * The bar tints in when the first row is ticked and tints out when the last is
 * cleared, rather than popping: it stays mounted for one tint, showing what it
 * last said, and ignores the pointer meanwhile.
 */
export function SelectionBar(props: SelectionBarProps) {
  const { rendered, leaving } = useExitPhase(props.count > 0, TINT_MS);
  const [last, setLast] = useState(props);
  if (props.count > 0 && last !== props) setLast(props);
  if (!rendered) return null;
  return <SelectionBarBody {...(leaving ? last : props)} leaving={leaving} />;
}

function SelectionBarBody({
  count,
  noun,
  scope,
  composition,
  wholeScope = false,
  visibleCount,
  onSelectVisibleOnly,
  ceiling,
  ceilingCount,
  ceilingNoun,
  onTakeCeiling,
  onClear,
  verbs,
  primary,
  children,
  leaving = false,
}: SelectionBarProps & { leaving?: boolean }) {
  const phone = useIsPhone();
  const [more, setMore] = useState(false);
  // Sheets don't stack (A-12 §2): a verb chosen in More changes waits for that
  // sheet to leave (S1: the card on tint, the scrim 40ms behind) before its own
  // dialog opens.
  const [chosen, setChosen] = useState<SelectionVerb | null>(null);
  useEffect(() => {
    if (!chosen) return;
    const timer = setTimeout(
      () => {
        setChosen(null);
        chosen.onClick();
      },
      stillMotion() ? 0 : TINT_MS + LIGHT_LEAD_MS,
    );
    return () => clearTimeout(timer);
  }, [chosen]);

  const things = `${count} ${count === 1 ? noun[0] : noun[1]}`;
  const canNarrow =
    wholeScope && visibleCount !== undefined && onSelectVisibleOnly && count > visibleCount;

  // Over the ceiling the bar changes what it says and what it offers, and does
  // neither of the two easy wrong things: it does not quietly run the first N
  // and report success for a number nobody chose, and it does not disable the
  // only control on screen and leave the operator with a dead bar and no
  // stated reason. It says the number, says the limit, and offers the one move
  // that gets under it — explicitly, so the cohort is chosen rather than
  // truncated.
  const gated = ceilingCount ?? count;
  const gatedNoun = ceilingNoun ?? noun;
  const overCeiling = ceiling !== undefined && gated > ceiling;
  // Only worth saying when the two differ: "601 grants selected · they cover
  // 520 people" tells the operator why a number they can see is being refused
  // by a number they cannot.
  const coverage =
    ceilingCount !== undefined
      ? `they cover ${gated} ${gated === 1 ? gatedNoun[0] : gatedNoun[1]}, and `
      : "";

  const stacked = phone && verbs !== undefined && verbs.length >= 3;
  const frame = `${leaving ? "tint-out pointer-events-none" : "tint-in"} sticky bottom-4 z-(--layer-bar) mt-2 flex rounded-[18px] border px-5 py-3.5 shadow-dialog ${
    overCeiling ? "border-warn-line bg-warn-soft" : "border-line-strong bg-surface-2"
  }`;
  const takeCeiling =
    overCeiling && onTakeCeiling ? (
      <SelectionAction onClick={onTakeCeiling} wide={stacked ? true : undefined}>
        Select the first {ceiling} {gatedNoun[1]} in the order shown
      </SelectionAction>
    ) : null;

  const summary = (
    <span className={stacked ? "min-w-0 flex-1 pt-[3px] text-[14.5px] leading-[1.45]" : "text-[14.5px]"}>
      <strong className="font-semibold">
        {wholeScope ? `All ${things}` : things} selected
      </strong>
      {overCeiling ? (
        <span className="text-warn-text">
          {" "}
          · {coverage}you can change at most {ceiling} {gatedNoun[1]} at once.
        </span>
      ) : (
        <>
          {scope ? <span className="text-muted"> {scope}</span> : null}
          {composition ? (
            // On a phone it wraps under the count, and the bar grows by that line only.
            <span className={stacked ? "block text-faint" : "text-faint"}>
              {stacked ? composition : ` · ${composition}`}
            </span>
          ) : null}
        </>
      )}
      {canNarrow && !overCeiling ? (
        <span className="ml-2.5 inline-flex align-middle">
          <Button size="sm" onClick={onSelectVisibleOnly}>
            Select only the {visibleCount} shown
          </Button>
        </span>
      ) : null}
    </span>
  );

  if (stacked) {
    const lead = verbs.find((verb) => verb.key === primary) ?? verbs[0];
    const rest = verbs.filter((verb) => verb !== lead);
    return (
      <div role="region" aria-label="Selection" aria-hidden={leaving ? true : undefined} className={`${frame} flex-col gap-2`}>
        <div className="flex min-h-7 items-start gap-3">
          {summary}
          {/* 28px drawn, 44px to the touch. */}
          <button
            type="button"
            onClick={onClear}
            className="-my-2 -mr-2 min-h-[44px] flex-none rounded-pill px-2 text-[13px] font-semibold text-muted control press-scale hover:text-ink"
          >
            Clear the selection
          </button>
        </div>
        {/* Over the ceiling nothing in More changes can run, so it goes and the one move takes the row. */}
        <div className="flex gap-2">
          {takeCeiling ?? (
            <>
              <SelectionAction tone={lead.tone} onClick={lead.onClick} wide>
                {lead.label}
              </SelectionAction>
              <SelectionAction onClick={() => setMore(true)} wide={false} sheet>
                More changes
              </SelectionAction>
            </>
          )}
        </div>
        <MoreChanges
          open={more}
          verbs={rest}
          onClose={() => setMore(false)}
          onChoose={(verb) => {
            setMore(false);
            setChosen(verb);
          }}
        />
      </div>
    );
  }

  return (
    <div role="region" aria-label="Selection" aria-hidden={leaving ? true : undefined} className={`${frame} flex-wrap items-center gap-x-3 gap-y-2`}>
      {summary}

      <span className="flex-1" />

      <div className="flex flex-wrap items-center gap-2">
        {takeCeiling ??
          (verbs
            ? verbs.map((verb) => (
                <SelectionAction key={verb.key} tone={verb.tone} onClick={verb.onClick}>
                  {verb.label}
                </SelectionAction>
              ))
            : children)}
        <button
          type="button"
          onClick={onClear}
          className="min-h-[44px] rounded-pill px-3 text-[13px] font-semibold text-muted control press-scale hover:text-ink desktop:min-h-0 desktop:py-1.5"
        >
          Clear the selection
        </button>
      </div>
    </div>
  );
}

/**
 * The phone bar's other verbs (A-12 §2, frame a-more): a Modal sm sheet, title
 * only, because the count is said once, in the bar still readable under the
 * scrim. Desktop order, the neutral verbs first, then the danger outlines after
 * a 16px gap. Choosing one closes this sheet; its own dialog opens after.
 */
function MoreChanges({
  open,
  verbs,
  onClose,
  onChoose,
}: {
  open: boolean;
  verbs: SelectionVerb[];
  onClose: () => void;
  onChoose: (verb: SelectionVerb) => void;
}) {
  const firstDanger = verbs.find((verb) => verb.tone === "danger");
  return (
    <Modal open={open} onClose={onClose} size="sm">
      <ModalHeader title="More changes" />
      <div className="flex flex-col gap-2 px-6 pb-2">
        {verbs.map((verb) => (
          <Button
            key={verb.key}
            variant={verb.tone === "danger" ? "danger" : "outline"}
            className={`w-full ${verb === firstDanger && verb !== verbs[0] ? "mt-2" : ""}`}
            onClick={() => onChoose(verb)}
          >
            {verb.label}
          </Button>
        ))}
      </div>
      <ModalFooter note="Each one shows who would change, person by person, before anything happens.">
        {/* The verbs above are the choice; Cancel is the footer's only button, so outline (G1). */}
        <Button variant="outline" onClick={onClose}>
          Cancel
        </Button>
      </ModalFooter>
    </Modal>
  );
}

/** A verb in the selection bar. `tone="danger"` for anything that takes access away. */
export function SelectionAction({
  onClick,
  tone = "neutral",
  disabled = false,
  wide,
  sheet = false,
  children,
}: {
  onClick: () => void;
  tone?: "neutral" | "danger";
  disabled?: boolean;
  /**
   * The phone bar's second row (A-12 §2): 44px, 14px. `true` takes the row's
   * width (the primary verb, or the ceiling's one move); `false` keeps its own
   * (More changes). Unset is the desktop row's verb.
   */
  wide?: boolean;
  /** Opens a sheet, so it says so to assistive technology. */
  sheet?: boolean;
  children: React.ReactNode;
}) {
  // The phone bar's second row is 14px and shares its width (A-12 §2); the desktop row is the sm pill as it is.
  const phone = wide === undefined ? "" : wide ? "min-w-0 flex-1 text-[14px]" : "flex-none text-[14px]";
  // The button's own surface, not `<Button>`: its reason wrapper would keep `flex-1` from reaching the row.
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      aria-haspopup={sheet ? "dialog" : undefined}
      className={buttonClasses({ variant: tone === "danger" ? "danger" : "outline", size: "sm", disabled, className: phone })}
    >
      {children}
    </button>
  );
}

/**
 * Every selection glyph, and the target it sits in.
 *
 * The glyph is 24px and the box around it is 44px, which are two different
 * numbers on purpose: a checkbox drawn at 44px is a slab, and one drawn at
 * 16px — what this was — is a thing you miss, hitting the row behind instead.
 * On a list whose bulk action removes people's access, missing is expensive.
 * Desktop keeps the 16px glyph and its tight cell; nothing about that density
 * was ever wrong for a mouse.
 */
const GLYPH_BOX =
  "flex h-11 w-11 shrink-0 items-center justify-center desktop:h-auto desktop:w-auto";

/** A row checkbox. Spread `selection.checkboxProps(id)` onto it. */
export function RowCheckbox({
  label,
  ...props
}: {
  label: string;
} & Record<string, unknown>) {
  return (
    <span className={GLYPH_BOX}>
      <Seat aria-label={label} {...props} />
    </span>
  );
}

/**
 * The named control that turns selection on, and the same control that turns
 * it off.
 *
 * Selection is a mode rather than a permanent column of checkboxes, and the
 * mode has to be announced somewhere a thumb can reach: a row that is quietly
 * selectable is a row whose tap does one of two things depending on state
 * nobody can see. Long-press is not an option — an invisible gesture is not an
 * affordance, it is a rumour.
 *
 * `Done` rather than `Cancel`, because leaving the mode is not an undo: the
 * selection survives re-entering within the same screen visit, and dies on
 * navigation like every other piece of screen state.
 */
export function SelectModeToggle({
  active,
  onToggle,
  idle = "Select",
  busy = "Done selecting",
}: {
  active: boolean;
  onToggle: () => void;
  idle?: string;
  busy?: string;
}) {
  return (
    <button
      type="button"
      onClick={onToggle}
      aria-pressed={active}
      className={`min-h-[44px] rounded-pill border px-4 text-[13.5px] font-semibold control press-scale desktop:min-h-0 desktop:py-[7px] ${
        active
          ? "border-accent-line bg-accent-soft text-accent-text"
          : "border-line-strong hover:bg-[var(--hover)]"
      }`}
    >
      {active ? busy : idle}
    </button>
  );
}

/**
 * Select-all, in words, with both numbers.
 *
 * A bare "Select all" beside a filtered list is the ambiguity worth removing:
 * all twelve you can see, or all three hundred and forty that exist? The two
 * answers differ by an order of magnitude of other people's access, so the
 * control says which one it means and states the other underneath rather than
 * leaving it to be discovered afterwards.
 *
 * It lives in the list rather than in the column header, because the column
 * header does not exist below the tablet breakpoint — a header-only select-all
 * is a capability phones simply do not have.
 *
 * The input carries no `aria-label`: the wrapping label is its accessible
 * name, so the second number is announced along with the first rather than
 * being visual-only.
 */
export function SelectAllRow({
  inScope,
  total,
  noun,
  allSelected,
  ...props
}: {
  /** How many rows the current filter makes selectable. */
  inScope: number;
  /** How many exist with no filter. Omit, or pass the same number, when unfiltered. */
  total?: number;
  noun: [string, string];
  allSelected: boolean;
  checked: boolean;
  ref: (node: HTMLInputElement | null) => void;
  onChange: () => void;
}) {
  const things = `${inScope} ${inScope === 1 ? noun[0] : noun[1]}`;
  const label = allSelected ? "Clear the selection" : `Select these ${things}`;
  // The second number answers "these, as opposed to what?" — a question only
  // the Select label asks. Left under "Clear the selection" it reads as a
  // claim about what clearing would do.
  const wider = !allSelected && total !== undefined && total > inScope;

  return (
    <label className="row-divider flex min-h-[44px] cursor-pointer select-none items-center gap-2 px-5 py-2">
      <span className={GLYPH_BOX}>
        <Seat {...props} />
      </span>
      <span className="flex flex-col">
        <span className="text-[14px] font-semibold">{label}</span>
        {wider ? (
          <span className="text-[12.5px] text-faint">
            {total} {total === 1 ? noun[0] : noun[1]} in all.
          </span>
        ) : null}
      </span>
    </label>
  );
}
