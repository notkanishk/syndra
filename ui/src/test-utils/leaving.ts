import { act, fireEvent } from "@testing-library/react";

/**
 * A `LeavingRow` whose gap has finished settling: jsdom runs no transitions, so a test says
 * when the fold has ended. The row is gone once it has.
 */
export const gapSettled = (row: Element) =>
  act(() => void fireEvent(row, Object.assign(new Event("transitionend", { bubbles: true }), { propertyName: "grid-template-rows" })));
