import { useEffect, type KeyboardEvent as ReactKeyEvent, type RefObject } from "react";

/**
 * Syndra's keys (A-19 P7, Q8): `/` Go to, `?` Keys, Esc closes, arrows move
 * between rows and Enter opens one. No letter is a verb: an accidental Revoke
 * is not calm. Every key here ignores a field, because a key pressed in a
 * field belongs to the field.
 */

/** The Keys sheet's rows, in the order it prints them. `keys` are drawn as Kbd. */
export const KEY_ROWS: ReadonlyArray<{ keys: readonly string[]; does: string }> = [
  { keys: ["/"], does: "Go to a page or a name" },
  { keys: ["?"], does: "Show these keys" },
  { keys: ["Esc"], does: "Close what is open, or leave Select" },
  { keys: ["↑", "↓"], does: "Move between rows in a list" },
  { keys: ["Enter"], does: "Open the row you are on" },
];

/** True when the key was pressed in something the reader types into. */
export function typingIn(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false;
  return target.isContentEditable || /^(INPUT|TEXTAREA|SELECT)$/.test(target.tagName);
}

/** True when a dialog other than the one carrying `own` (a selector) is open: sheets push, they do not stack. */
export function otherDialogOpen(own: string): boolean {
  return Array.from(document.querySelectorAll('[role="dialog"]')).some((el) => !el.querySelector(own));
}

/** What marks a row's link as a stop for the arrow keys. */
export const ROW_STOP = "[data-row]";
const CURRENT = "data-row-current";

const rowsIn = (list: HTMLElement) => Array.from(list.querySelectorAll<HTMLElement>(ROW_STOP));

function rove(list: HTMLElement, current: HTMLElement | undefined) {
  rowsIn(list).forEach((row) => {
    row.tabIndex = row === current ? 0 : -1;
    if (row === current) row.setAttribute(CURRENT, "");
    else row.removeAttribute(CURRENT);
  });
}

/**
 * Roving focus on a list's row links: one row is in the Tab order (the one the
 * reader was last on, else the first), the arrows move between rows, Enter
 * opens (a link does that itself), and the row's own buttons stay on Tab.
 * Runs after every render, because rows come and go with reads.
 */
export function useRovingRows(list: RefObject<HTMLElement | null>) {
  useEffect(() => {
    const node = list.current;
    if (!node) return;
    const rows = rowsIn(node);
    rove(node, rows.find((row) => row.hasAttribute(CURRENT)) ?? rows[0]);
  });
}

/** `onFocus` for the list: the row focused becomes the one in the Tab order. */
export function rowFocused(list: HTMLElement | null, target: EventTarget | null) {
  if (list && target instanceof HTMLElement && target.matches(ROW_STOP)) rove(list, target);
}

/** `onKeyDown` for the list: ↑ and ↓ between its rows. Only from a row, never from a field. */
export function moveBetweenRows(list: HTMLElement | null, event: ReactKeyEvent) {
  if (!list || event.defaultPrevented || event.altKey || event.ctrlKey || event.metaKey || event.shiftKey) return;
  if (event.key !== "ArrowDown" && event.key !== "ArrowUp") return;
  const from = event.target;
  if (!(from instanceof HTMLElement) || !from.matches(ROW_STOP) || typingIn(from)) return;
  const rows = rowsIn(list);
  const next = rows[rows.indexOf(from) + (event.key === "ArrowDown" ? 1 : -1)];
  event.preventDefault();
  next?.focus();
}
