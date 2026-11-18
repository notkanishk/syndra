"use client";

import React, { useRef } from "react";

import { TINT_MS, useExitPhase } from "@/lib/motion";

/**
 * A field at the sm control's height (A-16 §4): --control-sm with a mouse, the 44px
 * floor below desktop, as `PILL.sm` is for a button. For fields that sit in a
 * card's rows (the claims editor), where the 46px field was the outsized one.
 */
export const DENSE_FIELD = "min-h-[var(--tap)] py-[5px] text-body desktop:min-h-0 desktop:h-(--control-sm) desktop:py-0";

/**
 * Escape in two stages, in every search (A-24 §11.5): with words in it, Escape
 * clears them and goes no further; empty, Escape goes on to whatever holds the
 * field (Go to closes, a list leaves Select). The browsers disagree on a
 * search field's own Escape (Chrome and Safari clear it, Firefox does not), so
 * this owns it. A controlled field is cleared the way the browser would clear
 * it, through the native value setter and an `input` event, so the caller's
 * own `onChange` hears it and its state stays the only state.
 */
function clearOnEscape(event: React.KeyboardEvent<HTMLInputElement>) {
  const field = event.currentTarget;
  if (event.key !== "Escape" || event.defaultPrevented || event.nativeEvent.isComposing || field.value === "") return;
  event.preventDefault();
  event.stopPropagation();
  Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value")?.set?.call(field, "");
  field.dispatchEvent(new Event("input", { bubbles: true }));
}

/**
 * Text input. 14px radius rather than a pill: a field the operator types a
 * claim name or a date into should read as a container, not as a control.
 * A `type="search"` field clears on its first Escape (`clearOnEscape`).
 */
export const Input = React.forwardRef<HTMLInputElement, React.InputHTMLAttributes<HTMLInputElement> & { dense?: boolean }>(
  function Input({ className = "", dense = false, onKeyDown, ...props }, ref) {
    return (
      <input
        ref={ref}
        onKeyDown={(event) => {
          onKeyDown?.(event);
          if (props.type === "search") clearOnEscape(event);
        }}
        // A dialog field keeps its 15px (D-1 §6 pass 4), the size somebody types a claim name or a
        // date at; the dense field reads the body step. type-scale-exempt
        className={`block w-full rounded-inner border border-line-strong bg-transparent px-[15px] ${dense ? DENSE_FIELD : "py-3 text-[15px]"} text-ink motion-tint placeholder:text-faint focus:border-accent-line disabled:cursor-not-allowed disabled:text-faint ${className}`}
        {...props}
      />
    );
  },
);

/**
 * A short reason, two rows tall rather than one line stretched to a wall.
 * `rows={2}` is a default, not a floor — a caller with a longer reason to
 * capture can still pass a bigger one.
 */
export const Textarea = React.forwardRef<
  HTMLTextAreaElement,
  React.TextareaHTMLAttributes<HTMLTextAreaElement>
>(function Textarea({ className = "", rows = 2, ...props }, ref) {
  return (
    <textarea
      ref={ref}
      rows={rows}
      // The dialog field's 15px, as Input's (D-1 §6 pass 4). type-scale-exempt
      className={`block w-full resize-y rounded-inner border border-line-strong bg-transparent px-[15px] py-3 text-[15px] text-ink motion-tint placeholder:text-faint focus:border-accent-line disabled:cursor-not-allowed disabled:text-faint ${className}`}
      {...props}
    />
  );
});

/** A field label — 12.5px/600, quiet, sitting directly above its control. */
export function FieldLabel({
  children,
  htmlFor,
  // For a field that is a GROUP of controls rather than one input — a role
  // picker, a set of choices. `htmlFor` cannot point at a div, so the div
  // points back at this instead.
  id,
  className = "",
}: {
  children: React.ReactNode;
  htmlFor?: string;
  id?: string;
  className?: string;
}) {
  return (
    <label
      id={id}
      htmlFor={htmlFor}
      className={`mb-[7px] block text-fine font-semibold text-faint ${className}`}
    >
      {children}
    </label>
  );
}

/** The helper line beneath a field — plain language, never a spec. */
export function FieldHint({ children }: { children: React.ReactNode }) {
  return <p className="mt-[7px] text-small text-faint">{children}</p>;
}

/**
 * What is wrong with a field, under it. It appears still, on tint, and goes on
 * tint when the field is edited and the error stops being true; while it goes it
 * still reads what it said. Nothing moves: the page is pushed by the line, not
 * by the line's own motion. Renders nothing for no message.
 */
export function FieldError({
  children,
  className = "",
  ...rest
}: { children?: React.ReactNode } & Omit<React.HTMLAttributes<HTMLParagraphElement>, "children">) {
  const open = Boolean(children);
  const said = useRef(children);
  if (open) said.current = children;
  const { rendered, leaving } = useExitPhase(open, TINT_MS);
  if (!rendered) return null;
  return (
    <p
      role="alert"
      className={`mt-[7px] text-small text-danger-text ${leaving ? "tint-out" : "tint-in"} ${className}`}
      {...rest}
    >
      {said.current}
    </p>
  );
}
