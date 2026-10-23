"use client";

import { createContext, useContext, useEffect, useRef, useState, type ReactNode } from "react";
import { createPortal } from "react-dom";

import { LIGHT_LEAD_MS, SETTLE_MS, TINT_MS, stillMotion } from "@/lib/motion";

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
    // `[hidden]` is a form kept mounted behind a pushed step (CancelQueuedStep):
    // it is in the DOM but not on screen, so Tab must not land there.
    (element) => element.tabIndex >= 0 && !element.closest("[hidden]"),
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

    // Not an id's copy control sitting in the header: the safe exit must be
    // what takes focus, and the id stays one Tab away.
    const inPanel = focusableIn(panelRef.current);
    (inPanel.find((el) => !el.hasAttribute("data-skip-initial-focus")) ?? inPanel[0])?.focus();

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

/**
 * Puts an overlay under <body>, never where it was written.
 *
 * `position: fixed` is fixed to the viewport only while no ancestor has a
 * transform, filter or backdrop-filter, and an element animating `transform`
 * counts even after the animation ends at `none`. Rendered in place, a dialog
 * opened from a row in an arriving card was fixed to that card: clipped by it,
 * drawn over its rows, with the page behind it uncovered. Every `fixed` overlay
 * goes through this; a guard test holds it to that.
 *
 * Synchronous, not after mount: the focus trap reads the panel in the same
 * commit. Overlays open on a person's action, so none renders on the server.
 */
export function ToBody({ children }: { children: ReactNode }) {
  if (typeof document === "undefined") return null;
  return createPortal(children, document.body);
}

/**
 * A dialog whose state lives only while it is open.
 *
 * Rendered closed with an early `return null`, a dialog kept its state: the
 * request dialog reopened on "Asked for Staff" with Close, the last result,
 * until a reload. Gated here, its body mounts on open and is gone on close, so
 * each opening starts clean. Every dialog with an `open` prop goes through it;
 * a guard test holds it to that.
 */
export function mountedWhileOpen<P extends { open: boolean }>(Body: React.ComponentType<P>) {
  function WhileOpen(props: P) {
    return props.open ? <Body {...props} /> : null;
  }
  WhileOpen.displayName = `mountedWhileOpen(${Body.displayName ?? Body.name})`;
  return WhileOpen;
}

/**
 * What a dialog leaves behind while it goes (S1, closing): its card fades and
 * sinks 4px on tint, then the scrim follows 40ms behind.
 *
 * Nearly every dialog is mounted by its parent only while it is wanted, so
 * closing unmounts the Modal itself and there is no instance left to hold an
 * "exiting" phase. So the leaving is a copy: the scrim's DOM, cloned as the
 * Modal goes, inert and hidden from assistive technology, drawn over the page
 * for 160ms and then removed. The real dialog (its focus trap, its state, its
 * handlers) is gone the instant it is closed; only its picture stays.
 */
function leaveAsCopy(scrim: HTMLElement) {
  if (stillMotion()) return;
  const copy = scrim.cloneNode(true) as HTMLElement;
  copy.removeAttribute("role");
  copy.removeAttribute("aria-modal");
  copy.removeAttribute("aria-labelledby");
  copy.removeAttribute("aria-describedby");
  copy.setAttribute("aria-hidden", "true");
  copy.setAttribute("inert", "");
  copy.querySelectorAll("[id]").forEach((element) => element.removeAttribute("id"));
  copy.classList.remove("settle-scrim");
  copy.classList.add("leave-scrim", "pointer-events-none");
  const card = copy.firstElementChild;
  card?.classList.remove("settle-in");
  card?.classList.add("leave-panel");
  document.body.appendChild(copy);
  setTimeout(() => copy.remove(), TINT_MS + LIGHT_LEAD_MS + 20);
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
  const scrimRef = useRef<HTMLDivElement | null>(null);
  const nested = useContext(InsideDialog);
  // The sheet was dragged off the screen: it has already left, so it leaves no copy.
  const draggedAway = useRef(false);
  useDialogFocusTrap(panelRef, open, busy, onClose);

  const wasDraggedAway = () => draggedAway.current;
  useEffect(() => {
    if (!open) return;
    const scrim = scrimRef.current;
    // Runs after React has taken the dialog out of the page. In strict mode's
    // pretend unmount it is still there, and nothing has left.
    return () => {
      if (scrim && !scrim.isConnected && !wasDraggedAway()) leaveAsCopy(scrim);
    };
  }, [open]);

  const grabber = useSheetDrag({ panelRef, scrimRef, busy, onClose, draggedAway });

  if (open && nested && process.env.NODE_ENV !== "production") {
    console.error(
      "Modal: a dialog opened inside another dialog. Sheets push, they do not stack — " +
        "replace the first sheet's content and make its title a back-line instead.",
    );
  }

  if (!open) return null;

  return (
    <ToBody>
    <InsideDialog.Provider value={true}>
    <div
      ref={scrimRef}
      role="dialog"
      aria-modal="true"
      aria-labelledby={labelledBy}
      aria-describedby={describedBy}
      className="settle-scrim fixed inset-0 z-50 flex items-end justify-center bg-black/55 tablet:items-center tablet:px-4"
      onClick={(event) => {
        if (event.target === event.currentTarget && !busy) onClose();
      }}
    >
      {/* The scrim tints, then the card rises 8px from 98% 40ms behind it.
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
      {/* Long names, emails and ids in a title or row wrap inside the dialog instead of widening it. */}
      <div
        ref={panelRef}
        className={`flex w-full ${SIZE_CLASS[size]} settle-in flex-col overflow-y-auto [overflow-wrap:anywhere] rounded-t-[24px] border border-line-strong bg-surface-2 pb-[env(safe-area-inset-bottom)] shadow-dialog tablet:rounded-[22px] tablet:pb-0`}
      >
        <SheetGrabber busy={busy} onClose={onClose} drag={grabber} />
        {children}
      </div>
    </div>
    </InsideDialog.Provider>
    </ToBody>
  );
}

/** What the sheet's grabber needs to be dragged (S3): handlers, and whether a drag just ended. */
interface SheetDrag {
  handlers: {
    onPointerDown: (event: React.PointerEvent<HTMLElement>) => void;
    onPointerMove: (event: React.PointerEvent<HTMLElement>) => void;
    onPointerUp: (event: React.PointerEvent<HTMLElement>) => void;
    onPointerCancel: (event: React.PointerEvent<HTMLElement>) => void;
  };
  /** The pointer travelled: the click that follows is the end of a drag, not a tap. */
  wasDrag: () => boolean;
  /** While busy and a drag was refused under reduced motion: the sentence answers with a tint. */
  answered: boolean;
}

/** A third of the sheet's height, or this fast (px per ms), and it leaves (S3). */
const LEAVE_FRACTION = 1 / 3;
const FLICK_PX_PER_MS = 0.5;
/** Busy (7a): 15% of the finger's travel, capped at the travel cap. */
const RESIST = 0.15;
const RESIST_CAP_PX = 10;
const SCRIM_ALPHA = 0.55;

/**
 * Drag the sheet down by its grabber (S3), phone only: the grabber is hidden
 * from the tablet breakpoint up, so a desktop pointer never reaches it.
 *
 * Only the grabber captures the pointer, so the sheet's own scrolling is never
 * taken from a thumb. The motion is written straight to the elements rather
 * than through state, so a drag costs no renders.
 *
 *  - Held, it follows the finger 1:1 and the scrim thins with it.
 *  - Let go short of a third of its height, it settles back. Past a third, or
 *    on a quick flick, it leaves on settle's duration and closes.
 *  - Busy (7a), it follows at 15% of the travel, capped at 10px, and settles
 *    back. It never closes. Under reduced motion it does not move at all and
 *    the busy sentence tints instead.
 */
function useSheetDrag({
  panelRef,
  scrimRef,
  busy,
  onClose,
  draggedAway,
}: {
  panelRef: React.RefObject<HTMLDivElement | null>;
  scrimRef: React.RefObject<HTMLDivElement | null>;
  busy: boolean;
  onClose: () => void;
  draggedAway: React.RefObject<boolean>;
}): SheetDrag {
  const hold = useRef<{ y: number; at: number } | null>(null);
  const travelled = useRef(false);
  const [answered, setAnswered] = useState(false);
  const busyRef = useRef(busy);
  const closeRef = useRef(onClose);
  useEffect(() => {
    busyRef.current = busy;
    closeRef.current = onClose;
  });

  useEffect(() => {
    if (!answered) return;
    const timer = setTimeout(() => setAnswered(false), 600);
    return () => clearTimeout(timer);
  }, [answered]);

  const place = (offset: number, height: number) => {
    const panel = panelRef.current;
    const scrim = scrimRef.current;
    if (panel) panel.style.transform = offset ? `translateY(${offset}px)` : "";
    if (scrim) {
      scrim.style.backgroundColor = offset
        ? `rgba(0, 0, 0, ${SCRIM_ALPHA * (1 - Math.min(1, offset / (height || 1)))})`
        : "";
    }
  };

  const settleBack = () => {
    const panel = panelRef.current;
    const scrim = scrimRef.current;
    if (panel) panel.style.transition = `transform ${SETTLE_MS}ms var(--ease-settle)`;
    if (scrim) scrim.style.transition = `background-color ${SETTLE_MS}ms var(--ease-settle)`;
    place(0, 1);
    setTimeout(() => {
      if (panelRef.current) {
        panelRef.current.style.transition = "";
        panelRef.current.style.animation = "";
      }
      if (scrimRef.current) scrimRef.current.style.transition = "";
    }, SETTLE_MS);
  };

  const heightOf = () => panelRef.current?.getBoundingClientRect().height || 1;

  return {
    wasDrag: () => {
      const was = travelled.current;
      travelled.current = false;
      return was;
    },
    answered,
    handlers: {
      onPointerDown(event) {
        hold.current = { y: event.clientY, at: Date.now() };
        travelled.current = false;
        event.currentTarget.setPointerCapture?.(event.pointerId);
        // Opening's animation holds the card at its end state and would win over
        // anything written here.
        if (panelRef.current && !stillMotion()) {
          panelRef.current.style.animation = "none";
          panelRef.current.style.transition = "none";
        }
        if (scrimRef.current) scrimRef.current.style.transition = "none";
      },
      onPointerMove(event) {
        if (!hold.current) return;
        const travel = Math.max(0, event.clientY - hold.current.y);
        if (travel > 4) travelled.current = true;
        if (stillMotion()) {
          if (busyRef.current && travel > 4) setAnswered(true);
          return;
        }
        if (busyRef.current) place(Math.min(RESIST_CAP_PX, travel * RESIST), heightOf());
        else place(travel, heightOf());
      },
      onPointerUp(event) {
        const start = hold.current;
        hold.current = null;
        if (!start) return;
        const travel = Math.max(0, event.clientY - start.y);
        const height = heightOf();
        const speed = travel / Math.max(1, Date.now() - start.at);
        const leaves = !busyRef.current && travel > 4 && (travel > height * LEAVE_FRACTION || speed > FLICK_PX_PER_MS);
        if (!leaves) {
          if (!stillMotion()) settleBack();
          return;
        }
        if (stillMotion()) {
          closeRef.current();
          return;
        }
        // Settle's duration on tint's curve: it is going, not arriving.
        const panel = panelRef.current;
        const scrim = scrimRef.current;
        if (panel) {
          panel.style.transition = `transform ${SETTLE_MS}ms var(--ease-tint)`;
          panel.style.transform = "translateY(100%)";
        }
        if (scrim) {
          scrim.style.transition = `background-color ${SETTLE_MS}ms var(--ease-tint)`;
          scrim.style.backgroundColor = "rgba(0, 0, 0, 0)";
        }
        setTimeout(() => {
          draggedAway.current = true;
          closeRef.current();
          // If the parent kept it open, it comes back rather than sitting
          // invisible over the page.
          setTimeout(() => {
            if (!panelRef.current) return;
            draggedAway.current = false;
            settleBack();
          }, 100);
        }, SETTLE_MS);
      },
      onPointerCancel() {
        hold.current = null;
        if (!stillMotion()) settleBack();
      },
    },
  };
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
function SheetGrabber({ busy, onClose, drag }: { busy: boolean; onClose: () => void; drag: SheetDrag }) {
  if (busy) {
    return (
      <div {...drag.handlers} className="flex flex-none touch-none flex-col items-center gap-1.5 pb-1 pt-3 tablet:hidden">
        <span aria-hidden className="h-1 w-[38px] rounded-pill bg-accent/40" />
        <span className={`motion-tint text-[12.5px] ${drag.answered ? "text-ink" : "text-faint"}`}>
          Applying your change — this sheet can&apos;t be closed until it finishes.
        </span>
      </div>
    );
  }

  return (
    <button
      type="button"
      {...drag.handlers}
      onClick={() => {
        // The click that ends a drag is not a tap.
        if (!drag.wasDrag()) onClose();
      }}
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
      className="-mb-[18px] mx-auto flex h-11 w-full max-w-[120px] flex-none touch-none items-center justify-center tablet:hidden"
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
  //
  // `mt-2` is the focus ring's room. The ring sits 6px outside its field (2px
  // at a 4px offset), and the opaque footer began right under the last field,
  // so it painted over the bottom of that field's ring: the "Why?" box looked
  // clipped while typing. The margin is transparent, so the ring shows.
  return (
    <div className="sticky bottom-0 mt-2 flex flex-none flex-col gap-2.5 bg-surface-2 px-6 pb-[22px] pt-5">
      <div className="flex flex-wrap items-center gap-2.5">{children}</div>
      {note && <div className="text-[12.5px] leading-[1.5] text-faint">{note}</div>}
    </div>
  );
}
