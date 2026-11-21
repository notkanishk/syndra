// @vitest-environment jsdom
import { act, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { Reads } from "@/components/results/ReadResult";
import { MEMBER, cooldownLine } from "@/components/results/copy";
import { ReadAgainProvider } from "@/components/ui/ReadFreshness";
import type { ReadView } from "@/lib/queries/useMyStorage";

/** The wait is said once in its own status region too; these read the line on screen. */
const STATUS = '[role="status"]';

/** The renderers carry the page's one Read again, and a read with nothing to show says why (a cooldown counts), never an error. */

const NAME = "Parts shelf";
const taken = new Date(Date.now() - 4 * 60_000).toISOString();
/** A kept copy (design §13.3), with its reason and wait. */
const status = (why: { reason: string; retryAfter?: number; since?: number } = { reason: "cached" }): ReadView => ({
  operation: "storage.status",
  shape: "status",
  read: { state: "kept", takenAt: taken, why, data: { usable: true, standing: { usable: true, waiting_on: "none" }, meters: [] } },
});
const show = (reads: ReadView[], press = vi.fn(), skip: string[] = []) =>
  render(
    <ReadAgainProvider value={{ press }}>
      <Reads reads={reads} skip={skip} name={NAME} reader={MEMBER} />
    </ReadAgainProvider>,
  );

describe("a read served from the copy", () => {
  it("presses the page's one Read again, once", () => {
    const press = vi.fn();
    show([status()], press);
    fireEvent.click(screen.getByRole("button", { name: "Read again" }));
    expect(press).toHaveBeenCalledTimes(1);
  });

  it("keeps its freshness line, with the button, when a kind draws the body itself", () => {
    const press = vi.fn();
    const { container } = show([status()], press, ["storage.status"]);
    expect(container.querySelector("[data-read]")).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "Read again" }));
    expect(press).toHaveBeenCalledTimes(1);
  });

  it("draws nothing for a skipped raw answer, which has no age", () => {
    const { container } = show([{ operation: "storage.status", shape: "", read: { state: "now", takenAt: taken, data: "raw" } }], vi.fn(), ["storage.status"]);
    expect(container.querySelector("[data-read]")).toBeNull();
    expect(container.textContent).toBe("");
  });
});

describe("a read refused inside the cooldown with nothing kept", () => {
  const cooling = (since?: number): ReadView => ({
    operation: "holdings.list",
    shape: "records",
    read: { state: "none", why: { reason: "cooldown", retryAfter: 20, ...(since ? { since } : {}) } },
  });
  const nothing = cooling();

  // The page's one region speaks a wait only while its button is there to wait.
  it("clears the page's region when the counting button goes", () => {
    const view = show([nothing]);
    const region = () => document.querySelector(STATUS)!;
    expect(region().textContent).toBe(cooldownLine(NAME, 20));
    view.rerender(
      <ReadAgainProvider value={{ press: vi.fn() }}>
        <Reads reads={[]} name={NAME} reader={MEMBER} />
      </ReadAgainProvider>,
    );
    expect(region().textContent).toBe("");
  });

  it("says so with the countdown, and is not an outage", () => {
    const { container } = show([nothing]);
    expect(screen.getByText(cooldownLine(NAME, 20), { ignore: STATUS })).toBeInTheDocument();
    expect(container.textContent).not.toMatch(/can't be read/);
    expect(screen.getByRole("button", { name: "Read again" })).toHaveAttribute("aria-disabled", "true");
  });

  // Leaving the page and coming back must not restart the full count: it runs from when the page's read answered.
  it("counts from when the page's read answered, not from when it was drawn", () => {
    render(
      <ReadAgainProvider value={{ press: vi.fn() }}>
        <Reads reads={[cooling(Date.now() - 6000)]} name={NAME} reader={MEMBER} />
      </ReadAgainProvider>,
    );
    expect(screen.getByText(/read again in 1[345] s/, { ignore: STATUS })).toBeInTheDocument();
  });

  it("does the same for a read the kind draws itself", () => {
    show([{ ...nothing, operation: "storage.status", shape: "status" }], vi.fn(), ["storage.status"]);
    expect(screen.getByText(cooldownLine(NAME, 20), { ignore: STATUS })).toBeInTheDocument();
  });
});

// A press answered 200 carries its cooldown with the copy, and a press is the only way it gets there.
describe("a cooldown carried with a read's copy", () => {
  afterEach(() => vi.useRealTimers());
  const dot = () => document.querySelector("[data-freshness-dot]") as HTMLElement;
  const since = Date.now();
  const cooled = status({ reason: "cooldown", retryAfter: 30, since });
  const activity: ReadView = {
    operation: "storage.activity",
    shape: "activity",
    read: { state: "kept", takenAt: taken, why: { reason: "cooldown", retryAfter: 30, since }, data: { events: [], uncovered: [], truncated: false } },
  };

  it("is the reader's own press, so it is amber, in a status and an activity card", () => {
    const first = show([cooled]);
    expect(dot().className).toContain("bg-warn");
    first.unmount();
    show([activity]);
    expect(dot().className).toContain("bg-warn");
  });

  it("counts from when the page's read answered, so leaving and returning does not restart it", () => {
    vi.useFakeTimers({ now: since });
    const draw = () =>
      render(
        <ReadAgainProvider value={{ press: vi.fn() }}>
          <Reads reads={[cooled, activity]} name={NAME} reader={MEMBER} />
        </ReadAgainProvider>,
      );
    draw().unmount();
    act(() => void vi.advanceTimersByTime(12_000));
    draw();
    expect(screen.getAllByText(/you can read again in 18 s/, { ignore: STATUS })).toHaveLength(2);
  });
});

// PR 248's leftover: one status region per page's Read again, not one per button, so a page of
// several waiting reads announces once.
describe("the page's one status region", () => {
  it("holds every wait under one provider, with no region of each button's own", () => {
    const cool = (op: string): ReadView => ({ operation: op, shape: "records", read: { state: "none", why: { reason: "cooldown", retryAfter: 20 } } });
    const { container } = show([cool("a.list"), cool("b.list")]);
    expect(screen.getAllByRole("button", { name: "Read again" })).toHaveLength(2);
    const regions = container.querySelectorAll(STATUS);
    expect(regions).toHaveLength(1);
    expect(regions[0].textContent).toBe(cooldownLine(NAME, 20));
  });

  it("is quiet while nothing waits", () => {
    const { container } = show([status()]);
    expect(container.querySelectorAll(STATUS)).toHaveLength(1);
    expect(container.querySelector(STATUS)?.textContent).toBe("");
  });
});
