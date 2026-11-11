// @vitest-environment jsdom
import { act, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { Reads } from "@/components/results/ReadResult";
import { MEMBER, cooldownLine, notReadableTitle } from "@/components/results/copy";
import { ReadAgainProvider } from "@/components/ui/ReadFreshness";
import type { ReadView } from "@/lib/queries/useMyStorage";

/** The renderers carry the page's one Read again, and a read with nothing to show is a cooldown line, never an error. */

const NAME = "Parts shelf";
const taken = new Date(Date.now() - 4 * 60_000).toISOString();
const status = (over: Record<string, unknown>): ReadView => ({
  operation: "storage.status",
  shape: "status",
  readable: true,
  result: { usable: true, standing: { usable: true, waiting_on: "none" }, meters: [], taken_at: taken, current: false, stale: { reason: "cached" }, ...over },
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
    show([status({})], press);
    fireEvent.click(screen.getByRole("button", { name: "Read again" }));
    expect(press).toHaveBeenCalledTimes(1);
  });

  it("keeps its freshness line, with the button, when a kind draws the body itself", () => {
    const press = vi.fn();
    const { container } = show([status({})], press, ["storage.status"]);
    expect(container.querySelector("[data-read]")).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "Read again" }));
    expect(press).toHaveBeenCalledTimes(1);
  });

  it("draws nothing for a skipped read that said no age", () => {
    const { container } = show([{ operation: "storage.status", shape: "status", readable: false }], vi.fn(), ["storage.status"]);
    expect(container).toBeEmptyDOMElement();
  });
});

describe("a read refused inside the cooldown with nothing kept", () => {
  const nothing: ReadView = { operation: "holdings.list", shape: "records", readable: false, retry_after: 20 };

  it("says so with the countdown, and is not the could-not-be-read warning", () => {
    show([nothing]);
    expect(screen.getByText(cooldownLine(NAME, 20))).toBeInTheDocument();
    expect(screen.queryByText(notReadableTitle(NAME))).toBeNull();
    expect(screen.getByRole("button", { name: "Read again" })).toBeDisabled();
  });

  // Leaving the page and coming back must not restart the full count: it runs from when the page's read answered.
  it("counts from when the page's read answered, not from when it was drawn", () => {
    render(
      <ReadAgainProvider value={{ press: vi.fn(), since: Date.now() - 6000 }}>
        <Reads reads={[nothing]} name={NAME} reader={MEMBER} />
      </ReadAgainProvider>,
    );
    expect(screen.getByText(/read again in 1[345] s/)).toBeInTheDocument();
  });

  it("does the same for a read the kind draws itself", () => {
    show([{ ...nothing, operation: "storage.status", shape: "status" }], vi.fn(), ["storage.status"]);
    expect(screen.getByText(cooldownLine(NAME, 20))).toBeInTheDocument();
  });
});

// A press answered 200 carries its cooldown with the copy, and a press is the only way it gets there.
describe("a cooldown carried with a read's copy", () => {
  afterEach(() => vi.useRealTimers());
  const dot = () => document.querySelector("[data-freshness-dot]") as HTMLElement;
  const cooled = status({ stale: { reason: "cooldown", retry_after: 30 } });
  const activity: ReadView = {
    operation: "storage.activity",
    shape: "activity",
    readable: true,
    result: { events: [], uncovered: [], truncated: false, taken_at: taken, current: false, stale: { reason: "cooldown", retry_after: 30 } },
  };

  it("is the reader's own press, so it is amber, in a status and an activity card", () => {
    const first = show([cooled]);
    expect(dot().className).toContain("bg-warn");
    first.unmount();
    show([activity]);
    expect(dot().className).toContain("bg-warn");
  });

  it("counts from when the page's read answered, so leaving and returning does not restart it", () => {
    vi.useFakeTimers();
    const since = Date.now();
    const draw = () =>
      render(
        <ReadAgainProvider value={{ press: vi.fn(), since }}>
          <Reads reads={[cooled, activity]} name={NAME} reader={MEMBER} />
        </ReadAgainProvider>,
      );
    draw().unmount();
    act(() => void vi.advanceTimersByTime(12_000));
    draw();
    expect(screen.getAllByText(/you can read again in 18 s/)).toHaveLength(2);
  });
});
