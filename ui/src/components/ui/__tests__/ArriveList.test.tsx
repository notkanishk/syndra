// @vitest-environment jsdom
import { act, render } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  ARRIVE_MEMORY_MS,
  ARRIVE_SETTLED_MS,
  ArriveList,
  armArrive,
} from "@/components/ui/ArriveList";

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

const settledOf = (container: HTMLElement) =>
  (container.firstElementChild as HTMLElement).hasAttribute("data-settled");

describe("ArriveList never replays", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-10-23T10:00:00Z"));
    window.history.replaceState({}, "", "/people");
  });
  afterEach(() => vi.useRealTimers());

  it("a page reached by Back or Forward is already settled; a link there still arrives", () => {
    expect(settledOf(render(<ArriveList><div>a</div></ArriveList>).container)).toBe(false);

    act(() => void window.dispatchEvent(new PopStateEvent("popstate")));
    expect(settledOf(render(<ArriveList><div>a</div></ArriveList>).container)).toBe(true);

    // Going somewhere else by a link arrives, even right after a Back.
    window.history.pushState({}, "", "/roles");
    expect(settledOf(render(<ArriveList><div>a</div></ArriveList>).container)).toBe(false);
  });

  it("forgets the Back once it is long past", () => {
    act(() => void window.dispatchEvent(new PopStateEvent("popstate")));
    vi.advanceTimersByTime(ARRIVE_MEMORY_MS + 1);
    expect(settledOf(render(<ArriveList><div>a</div></ArriveList>).container)).toBe(false);
  });

  it("startSettled holds a list still from its first paint", () => {
    expect(
      settledOf(
        render(
          <ArriveList startSettled>
            <div>a</div>
          </ArriveList>,
        ).container,
      ),
    ).toBe(true);
  });
});

describe("ArriveList after Show 25 more", () => {
  beforeEach(() => {
    vi.useFakeTimers({ toFake: ["Date", "setTimeout", "clearTimeout"] });
  });
  afterEach(() => vi.useRealTimers());

  const list = (n: number) => (
    <ArriveList startSettled>
      {Array.from({ length: n }, (_, i) => (
        <div key={i}>row {i}</div>
      ))}
    </ArriveList>
  );
  const arriving = (c: HTMLElement) =>
    Array.from(c.querySelectorAll<HTMLElement>("[data-arriving]")).map((el) => [
      el.textContent,
      el.style.getPropertyValue("--i"),
    ]);

  it("only the new rows arrive, staggered from the first new one", async () => {
    const { container, rerender } = render(list(2));
    armArrive();
    rerender(list(5));
    await act(async () => {});
    expect(arriving(container)).toEqual([
      ["row 2", "0"],
      ["row 3", "1"],
      ["row 4", "2"],
    ]);
  });

  it("rows that appear without being asked for just appear", async () => {
    vi.advanceTimersByTime(ARRIVE_MEMORY_MS + 1);
    const { container, rerender } = render(list(2));
    rerender(list(3));
    await act(async () => {});
    expect(arriving(container)).toEqual([]);
  });
});
