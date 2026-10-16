// @vitest-environment jsdom
import { act, render } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { ARRIVE_SETTLED_MS, ArriveList } from "@/components/ui/ArriveList";

describe("ArriveList", () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => vi.useRealTimers());

  it("plays on first paint, then holds still for rows that mount later", () => {
    const { container, rerender } = render(
      <ArriveList>
        <div>one</div>
      </ArriveList>,
    );
    const list = container.firstElementChild as HTMLElement;
    expect(list.className).toContain("arrive-list");
    expect(list.hasAttribute("data-settled")).toBe(false);

    act(() => {
      vi.advanceTimersByTime(ARRIVE_SETTLED_MS);
    });
    expect(list.hasAttribute("data-settled")).toBe(true);

    // A refresh or a filter mounts a new row: the list stays settled, so the
    // CSS gives that row no arrive animation.
    rerender(
      <ArriveList>
        <div>one</div>
        <div>two</div>
      </ArriveList>,
    );
    expect(list.hasAttribute("data-settled")).toBe(true);
  });

  it("a list that mounts again (a fold opening, a new page) arrives again", () => {
    const { container, unmount } = render(
      <ArriveList>
        <div>one</div>
      </ArriveList>,
    );
    act(() => {
      vi.advanceTimersByTime(ARRIVE_SETTLED_MS);
    });
    unmount();
    const again = render(
      <ArriveList>
        <div>one</div>
      </ArriveList>,
    );
    expect((again.container.firstElementChild as HTMLElement).hasAttribute("data-settled")).toBe(
      false,
    );
    expect(container).toBeTruthy();
  });
});
