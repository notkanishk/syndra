import type { InputHTMLAttributes } from "react";

/**
 * The one checkbox and the one radio.
 *
 * Each screen used to draw its own: 16px everywhere in one picker, 24px on a
 * phone in the selection bar, a top nudge in some labels and not others. One
 * glyph now, the selection bar's: 24px under a thumb, 16px with a mouse, in the
 * accent. Inside a label the whole label is the target, so the glyph needs no
 * box of its own; `className` carries only placement (`mt-1` beside a
 * paragraph).
 */
const GLYPH = "size-6 shrink-0 accent-[var(--accent)] desktop:size-4";

type ChoiceProps = Omit<InputHTMLAttributes<HTMLInputElement>, "type">;

export function Checkbox({ className = "", ...props }: ChoiceProps) {
  return <input type="checkbox" className={`${GLYPH} ${className}`} {...props} />;
}

export function Radio({ className = "", ...props }: ChoiceProps) {
  return <input type="radio" className={`${GLYPH} ${className}`} {...props} />;
}
