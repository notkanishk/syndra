"use client";

import { createContext, useContext, useEffect, useRef } from "react";

/**
 * The dialog primitive. Destructive actions ALWAYS open one of these — never a
 * bare confirm(), never an inline undo-only toast.
 *
 * surface-2, 22px radius, the dialog shadow, focus-trapped, Esc to dismiss
 * unless a mutation is in flight.
 */

interface ModalProps {
  open: boolean;
  onClose: () => void;
  labelledBy?: string;
  describedBy?: string;
  /** Disable Esc and click-outside dismiss while a mutation is in flight. */
  busy?: boolean;
  size?: "sm" | "md" | "lg";
  children: React.ReactNode;
}

/**
 * One dialog, two shapes.
 *
 * Above the tablet breakpoint these are centred cards at their stated widths.
 * Below it every one of them rises from the bottom edge as a sheet, because a
 * centred card at 390px is a sheet with worse manners: it wastes both margins,
 * puts its actions in the middle of the screen where no thumb rests, and gives
 * a keyboard nothing to dock against.
 *
 * The height each size takes on a phone is a judgement about what it holds:
 *
 *  - `sm` is a rename field or a single confirmation. It takes its content's
 *    height and no more — a one-field sheet at full height is a ceremony the
 *    field has not earned.
 *  - `md` acts on one named subject, so it stops 96px short of the top and the
 *    person or target being acted on stays visible behind it. An operator who
 *    cannot see who they are acting on is the failure this margin exists for.
 *  - `lg` is a plan, a payload or a picker — the content *is* the subject, so
 *    it goes full height and its footer stays pinned.
 */
const SIZE_CLASS: Record<NonNullable<ModalProps["size"]>, string> = {
  sm: "max-w-[420px] max-h-[86dvh] tablet:max-h-[calc(100dvh-32px)]",
  md: "max-w-[520px] max-h-[calc(100dvh-96px)] tablet:max-h-[calc(100dvh-32px)]",
  lg: "max-w-[760px] h-[calc(100dvh-24px)] tablet:h-auto tablet:max-h-[calc(100dvh-32px)]",
};

/**
 * Whether a dialog is already open above this point in the tree.
 *
 * Push, never stack. A second sheet rising over the first leaves two scrims,
 * two focus traps competing for Tab, and a dismiss gesture whose meaning
 * depends on which one is on top — and on a phone the pair eat the screen
 * between them. Nothing in the product nests a dialog today; this is what
 * makes that a property rather than a coincidence, because the failure is
 * invisible on a desktop mock and total on a 390px screen.
 *
 * A second dialog still renders — refusing to would replace a layout problem
 * with a missing confirmation, which is worse — but it says so loudly enough
 * that it does not reach a phone. Where two steps are genuinely needed, the
 * second replaces the first's content inside the same panel and the first's
 * title becomes a back-line.
 */
const InsideDialog = createContext(false);

const FOCUSABLE_SELECTOR =
  "button:not([disabled]), [href], input, select, textarea, [tabindex]:not([tabindex='-1'])";

/**
 * The panel's focusable elements, in order, minus anything deliberately taken
 * out of the tab order.
 *
 * The selector alone is not enough: `button:not([disabled])` matches a button
 * carrying `tabindex="-1"`, because the negative-tabindex clause is a separate
 * alternative rather than a filter over the others. Without this the sheet's
 * grabber — a redundant touch affordance and the panel's first child — took
 * the focus every dialog gives on open, so every dialog opened with the cursor
 * on "close it".
 */
function focusableIn(panel: HTMLElement | null): HTMLElement[] {
  if (!panel) return [];
  return Array.from(panel.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTOR)).filter(
    (element) => element.tabIndex >= 0,
  );
}

/**
 * Shared dialog behaviour for Modal and Drawer: focus the first focusable
 * element on open, trap Tab inside the panel, Esc-to-close (unless busy),
 * restore focus to the previously focused element on close.
 */
export function useDialogFocusTrap(
  panelRef: React.RefObject<HTMLDivElement | null>,
  open: boolean,
  busy: boolean,
  onClose: () => void,
) {
  // Held in refs so the trap installs ONCE per open. With `busy` and `onClose`
  // in the dependency list the effect tore down and re-ran every time a
  // mutation started or finished — and its first act is to focus the first
  // focusable element in the panel, which during an apply is the first button
  // still enabled. So starting an apply moved focus onto Cancel: the one
  // control that abandons the operation, under the operator's next keystroke.
  const busyRef = useRef(busy);
  const closeRef = useRef(onClose);
  useEffect(() => {
    busyRef.current = busy;
    closeRef.current = onClose;
  });

  useEffect(() => {
    if (!open) return;
    const previouslyFocused = document.activeElement as HTMLElement | null;

    focusableIn(panelRef.current)[0]?.focus();

    function handleKey(event: KeyboardEvent) {
      if (event.key === "Escape" && !busyRef.current) {
        event.preventDefault();
        closeRef.current();
        return;
      }
      if (event.key !== "Tab") return;
      if (!panelRef.current) return;
      const list = focusableIn(panelRef.current);
      if (list.length === 0) return;
      const first = list[0];
      const last = list[list.length - 1];
      if (event.shiftKey && document.activeElement === first) {
        event.preventDefault();
        last.focus();
      } else if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault();
        first.focus();
      }
    }

    document.addEventListener("keydown", handleKey);
    return () => {
      document.removeEventListener("keydown", handleKey);
      previouslyFocused?.focus();
    };
  }, [panelRef, open]);
}

export function Modal({
  open,
  onClose,
  labelledBy,
  describedBy,
  busy = false,
  size = "md",
  children,
}: ModalProps) {
  const panelRef = useRef<HTMLDivElement | null>(null);
  const nested = useContext(InsideDialog);
  useDialogFocusTrap(panelRef, open, busy, onClose);

  if (open && nested && process.env.NODE_ENV !== "production") {
    console.error(
      "Modal: a dialog opened inside another dialog. Sheets push, they do not stack — " +
        "replace the first sheet's content and make its title a back-line instead.",
    );
  }

  if (!open) return null;

  return (
    <InsideDialog.Provider value={true}>
    <div
      role="dialog"
      aria-modal="true"
      aria-labelledby={labelledBy}
      aria-describedby={describedBy}
      className="settle-scrim fixed inset-0 z-50 flex items-end justify-center bg-black/55 tablet:items-center tablet:px-4"
      onClick={(event) => {
        if (event.target === event.currentTarget && !busy) onClose();
      }}
    >
      {/* The scrim fades, then the card rises 10px from 97% a beat behind it.
          It never zooms out of screen centre — the destination is where the
          dialog will live, so the eye lands there and stays. */}
      {/* The panel is bounded by the viewport and scrolls inside that bound.
          It used to be `overflow-hidden` with no height at all, which does not
          shrink a tall dialog — it clips it, and the clipped part is the
          footer, so a long plan or a full role list ended with its confirm
          button unreachable and no scrollbar to say so. `dvh` rather than `vh`
          because a mobile URL bar makes those two different numbers, and the
          one that lies is the one that hides the button. 32px keeps the
          gutter the scrim already sets on the horizontal axis. */}
      <div
        ref={panelRef}
        className={`flex w-full ${SIZE_CLASS[size]} settle-in flex-col overflow-y-auto rounded-t-[24px] border border-line-strong bg-surface-2 pb-[env(safe-area-inset-bottom)] shadow-dialog tablet:rounded-[22px] tablet:pb-0`}
      >
        <SheetGrabber busy={busy} onClose={onClose} />
        {children}
      </div>
    </div>
    </InsideDialog.Provider>
  );
}

/**
 * The sheet's own dismissal, and the one case where it refuses.
 *
 * Only on a phone — a centred dialog has a scrim to click and a visible edge,
 * where a sheet has neither below the fold. While an action is in flight the
 * grabber is replaced rather than merely disabled: a sheet that silently
 * ignores a drag or a tap reads as a frozen application, and the operator's
 * next move is to reload the page in the middle of a mutation.
 */
function SheetGrabber({ busy, onClose }: { busy: boolean; onClose: () => void }) {
  if (busy) {
    return (
      <div className="flex flex-none flex-col items-center gap-1.5 pb-1 pt-3 tablet:hidden">
        <span aria-hidden className="h-1 w-[38px] rounded-pill bg-accent/40" />
        <span className="text-[12.5px] text-faint">Applying your change — this sheet can&apos;t be closed until it finishes.</span>
      </div>
    );
  }

  return (
    <button
      type="button"
      onClick={onClose}
      // "Dismiss", not "Close": a rehearsal's done step carries a named Close
      // control, and two controls answering to one name makes "the dialog is
      // finished" and "the sheet has a handle" indistinguishable to anything
      // querying by accessible name — a screen reader included.
      aria-label="Close this sheet"
      // Out of the tab order deliberately. It is a redundant affordance — Esc
      // dismisses, the scrim dismisses, and every sheet carries a named
      // control in its footer — and as the panel's first focusable element it
      // would take the focus the trap gives on open, so every dialog would
      // open with the cursor on "close it" rather than on the first thing the
      // operator came to do.
      tabIndex={-1}
      // 44px tall, 26px of it visible: the bar is a hairline and the target
      // around it is a thumb. It draws no extra space because the padding
      // eats into the header's own top gap.
      className="-mb-[18px] mx-auto flex h-11 w-full max-w-[120px] flex-none items-center justify-center tablet:hidden"
    >
      <span aria-hidden className="h-1 w-[38px] rounded-pill bg-ink/20" />
    </button>
  );
}

/** Dialog header: an optional source chip above the title, then the title. */
export function ModalHeader({
  chip,
  title,
  lede,
  titleId,
}: {
  chip?: React.ReactNode;
  title: string;
  lede?: React.ReactNode;
  titleId?: string;
}) {
  return (
    <div className="px-6 pt-[22px]">
      {chip && <div className="mb-3.5">{chip}</div>}
      <h2 id={titleId} className="type-dialog-title mb-2.5">
        {title}
      </h2>
      {lede && <p className="mb-4 text-[14.5px] leading-[1.55] text-muted">{lede}</p>}
    </div>
  );
}

export function ModalFooter({
  children,
  note,
}: {
  children: React.ReactNode;
  note?: React.ReactNode;
}) {
  // Sticky to the panel's own bottom edge, and opaque, so a dialog long enough
  // to scroll keeps its actions in reach instead of burying them under the
  // fold. On a dialog short enough not to scroll this is inert — sticky only
  // engages inside a scrolling container — so nothing about the common case
  // moves. `bg-surface-2` is the panel's own colour, which is why the seam is
  // invisible until content passes behind it.
  return (
    <div className="sticky bottom-0 flex flex-none flex-col gap-2.5 bg-surface-2 px-6 pb-[22px] pt-5">
      <div className="flex flex-wrap items-center gap-2.5">{children}</div>
      {note && <div className="text-[12.5px] leading-[1.5] text-faint">{note}</div>}
    </div>
  );
}
