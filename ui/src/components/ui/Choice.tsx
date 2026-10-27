import type { InputHTMLAttributes } from "react";

type ChoiceProps = Omit<InputHTMLAttributes<HTMLInputElement>, "type">;

/**
 * The one checkbox and the one radio.
 *
 * Each screen used to draw its own: 16px everywhere in one picker, 24px on a
 * phone in the selection bar, a top nudge in some labels and not others. One
 * glyph now, the selection bar's `Seat`: the fill tints, the tick or dot grows
 * from 60% behind it, and it presses. 16px with a mouse, 24px on touch (`seat` in
 * globals.css); the row, not the glyph, is what you hit. Inside a label the whole label is the
 * target, so the glyph needs no box of its own; `className` carries only
 * placement (`mt-1` beside a paragraph).
 */
export function Checkbox(props: ChoiceProps) {
  return <Seat {...props} />;
}

export function Radio(props: ChoiceProps) {
  return <Seat single {...props} />;
}

/**
 * The round selection seat (proposed/SelectionSeat.md), replacing the native
 * checkbox wherever a row is chosen. Still a real input: Space toggles it, a
 * label names it, and shift-click and drag-paint read it as before. `single`
 * draws a dot instead of a check, for one-of-many. `locked` is chosen by
 * something elsewhere and can't change here, and is ticked grey in both forms:
 * held is a tick, never a dot (A-04 §5).
 */
export function Seat({
  single = false,
  locked = false,
  className = "",
  ...props
}: ChoiceProps & { single?: boolean; locked?: boolean }) {
  return (
    <span className={`seat ${className}`} data-locked={locked || undefined}>
      <input
        type={single ? "radio" : "checkbox"}
        {...props}
        {...(locked ? { checked: true, disabled: true, readOnly: true } : {})}
      />
      <span aria-hidden>
        {single && !locked ? (
          <svg className="seat-mark" width="12" height="12" viewBox="0 0 12 12">
            <circle cx="6" cy="6" r="4" fill="currentColor" />
          </svg>
        ) : (
          <>
            <svg className="seat-mark" width="12" height="12" viewBox="0 0 12 12" fill="none">
              <path d="M2.5 6.2 5 8.6l4.5-5" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" />
            </svg>
            <svg className="seat-dash absolute" width="12" height="12" viewBox="0 0 12 12" fill="none">
              <path d="M3 6h6" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" />
            </svg>
          </>
        )}
      </span>
    </span>
  );
}
