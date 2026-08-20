"use client";

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
  /** Narrows the selection to the first `ceiling` rows, in the order shown. */
  onTakeCeiling?: () => void;
  onClear: () => void;
  /** The verbs. Rendered right-aligned, destructive ones marked. */
  children: React.ReactNode;
}

export function SelectionBar({
  count,
  noun,
  scope,
  composition,
  wholeScope = false,
  visibleCount,
  onSelectVisibleOnly,
  ceiling,
  onTakeCeiling,
  onClear,
  children,
}: SelectionBarProps) {
  if (count === 0) return null;

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
  const overCeiling = ceiling !== undefined && count > ceiling;

  return (
    <div
      role="region"
      aria-label="Selection"
      className={`sticky bottom-4 z-20 mt-2 flex flex-wrap items-center gap-x-3 gap-y-2 rounded-[18px] border px-5 py-3.5 shadow-dialog ${
        overCeiling ? "border-warn-line bg-warn-soft" : "border-line-strong bg-surface-2"
      }`}
    >
      <span className="text-[14.5px]">
        <strong className="font-semibold">
          {wholeScope ? `All ${things}` : things} selected
        </strong>
        {overCeiling ? (
          <span className="text-warn-text">
            {" "}
            · {ceiling} is the most that can run at once.
          </span>
        ) : (
          <>
            {scope ? <span className="text-muted"> {scope}</span> : null}
            {composition ? <span className="text-faint"> · {composition}</span> : null}
          </>
        )}
        {canNarrow && !overCeiling ? (
          <>
            {" — "}
            <button
              type="button"
              onClick={onSelectVisibleOnly}
              className="font-semibold text-accent-text underline-offset-2 hover:underline"
            >
              select only the {visibleCount} shown
            </button>
          </>
        ) : null}
      </span>

      <span className="flex-1" />

      <div className="flex flex-wrap items-center gap-2">
        {overCeiling && onTakeCeiling ? (
          <SelectionAction onClick={onTakeCeiling}>
            Select the first {ceiling} in the order shown
          </SelectionAction>
        ) : (
          children
        )}
        <button
          type="button"
          onClick={onClear}
          className="min-h-[44px] rounded-pill px-3 text-[13px] font-semibold text-muted motion-tint hover:text-ink desktop:min-h-0 desktop:py-1.5"
        >
          Clear
        </button>
      </div>
    </div>
  );
}

/** A verb in the selection bar. `tone="danger"` for anything that takes access away. */
export function SelectionAction({
  onClick,
  tone = "neutral",
  disabled = false,
  children,
}: {
  onClick: () => void;
  tone?: "neutral" | "danger";
  disabled?: boolean;
  children: React.ReactNode;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      className={`min-h-[44px] rounded-pill border px-3.5 text-[13px] font-semibold motion-tint disabled:cursor-not-allowed disabled:opacity-50 desktop:min-h-0 desktop:py-1.5 ${
        tone === "danger"
          ? "border-danger-line text-danger-text hover:bg-danger-soft"
          : "border-line-strong hover:bg-[var(--hover)]"
      }`}
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
const GLYPH = "h-6 w-6 accent-[var(--accent)] desktop:h-4 desktop:w-4";

/** A row checkbox. Spread `selection.checkboxProps(id)` onto it. */
export function RowCheckbox({
  label,
  ...props
}: {
  label: string;
} & Record<string, unknown>) {
  return (
    <span className={GLYPH_BOX}>
      <input type="checkbox" aria-label={label} className={GLYPH} {...props} />
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
      className={`min-h-[44px] rounded-pill border px-4 text-[13.5px] font-semibold motion-tint desktop:min-h-0 desktop:py-[7px] ${
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
  const wider = total !== undefined && total > inScope;

  return (
    <label className="row-divider flex min-h-[44px] cursor-pointer select-none items-center gap-2 px-5 py-2">
      <span className={GLYPH_BOX}>
        <input type="checkbox" className={GLYPH} {...props} />
      </span>
      <span className="flex flex-col">
        <span className="text-[14px] font-semibold">{label}</span>
        {wider ? (
          <span className="text-[12.5px] text-faint">
            {total} {total === 1 ? noun[0] : noun[1]} match no filter.
          </span>
        ) : null}
      </span>
    </label>
  );
}
