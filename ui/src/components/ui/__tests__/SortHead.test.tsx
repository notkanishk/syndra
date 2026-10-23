// @vitest-environment jsdom
import { act, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { RunLabel, SortHead, runStarts, useSort } from "@/components/ui/SortHead";
import { motionInTests } from "@/test-utils/motion";

type Key = "who" | "ends";

function Heads() {
  const [sort, flip] = useSort<Key>("ends");
  return (
    <>
      <SortHead label="Who" name="who" sort={sort} onSort={flip} />
      <SortHead label="Ends in" name="ends" by="the clock" sort={sort} onSort={flip} />
      <output>{`${sort.key}:${sort.dir}`}</output>
    </>
  );
}

const arrow = (name: string) => screen.getByRole("button", { name }).querySelector("span")!;

describe("sorting by a heading", () => {
  it("starts on the one asked for, ascending, with the arrow only there", () => {
    render(<Heads />);
    expect(screen.getByRole("status")).toHaveTextContent("ends:1");
    expect(arrow("Sort by the clock")).toHaveClass("opacity-100");
    expect(arrow("Sort by who")).toHaveClass("opacity-0");
  });

  it("reverses on a second press and turns the arrow over", () => {
    render(<Heads />);
    fireEvent.click(screen.getByRole("button", { name: "Sort by the clock" }));
    expect(screen.getByRole("status")).toHaveTextContent("ends:-1");
    expect(arrow("Sort by the clock")).toHaveClass("rotate-180");
  });

  it("starts another heading ascending, whichever way the last one was going", () => {
    render(<Heads />);
    fireEvent.click(screen.getByRole("button", { name: "Sort by the clock" }));
    fireEvent.click(screen.getByRole("button", { name: "Sort by who" }));
    expect(screen.getByRole("status")).toHaveTextContent("who:1");
    expect(arrow("Sort by who")).not.toHaveClass("rotate-180");
    expect(arrow("Sort by the clock")).toHaveClass("opacity-0");
  });

  it("says which way the heading in force is sorted on its header cell, and none on the rest", () => {
    render(<Heads />);
    const cell = (name: string) => screen.getByRole("button", { name }).closest("[role=columnheader]")!;
    expect(cell("Sort by the clock")).toHaveAttribute("aria-sort", "ascending");
    expect(cell("Sort by who")).toHaveAttribute("aria-sort", "none");

    fireEvent.click(screen.getByRole("button", { name: "Sort by the clock" }));
    expect(cell("Sort by the clock")).toHaveAttribute("aria-sort", "descending");
    // Never on the button itself, where it is not valid.
    expect(screen.getByRole("button", { name: "Sort by the clock" })).not.toHaveAttribute("aria-sort");
  });

  it("names itself for a screen reader by its label unless told otherwise", () => {
    render(<Heads />);
    expect(screen.getByRole("button", { name: "Sort by who" })).toBeInTheDocument();
  });
});

describe("runs", () => {
  const rows = ["laser", "laser", "wood", "laser"];

  it("says which rows begin a run and the run's whole size", () => {
    const at = runStarts(rows, (row) => row);
    expect(at(0)).toEqual({ label: "laser", size: 3 });
    expect(at(1)).toBeNull();
    expect(at(2)).toEqual({ label: "wood", size: 1 });
    // Equal values that are not next to each other are two runs; the count is the value's.
    expect(at(3)).toEqual({ label: "laser", size: 3 });
  });

  it("counts the whole run for a paged prefix", () => {
    const at = runStarts(rows, (row) => row);
    expect(rows.slice(0, 2).map((_, index) => at(index))).toEqual([{ label: "laser", size: 3 }, null]);
  });

  it("gathers nothing when the sort does not", () => {
    expect(runStarts(rows, null)(0)).toBeNull();
  });

  it("labels a run with its value and its count", () => {
    render(<RunLabel label="Wood shop · Operator" size={7} />);
    expect(screen.getByTestId("run-label")).toHaveTextContent("Wood shop · Operator7");
  });
});

/** A re-sort (A-05 §2.10): the list fades out, re-orders while unseen, fades back in. Rows never travel. */
describe("a re-sort", () => {
  function Sorted() {
    const [sort, flip, view] = useSort<Key>("ends");
    return (
      <>
        <SortHead label="Who" name="who" sort={sort} onSort={flip} />
        <ul data-testid="list" className={view.className}>
          <li>{view.sort.key}</li>
        </ul>
      </>
    );
  }

  beforeEach(() => {
    vi.useFakeTimers();
    motionInTests();
  });
  afterEach(() => vi.useRealTimers());

  it("turns the arrow at once, fades the list out, re-orders it unseen, then fades it back in", () => {
    render(<Sorted />);
    fireEvent.click(screen.getByRole("button", { name: "Sort by who" }));

    expect(arrow("Sort by who")).toHaveClass("opacity-100");
    expect(screen.getByTestId("list")).toHaveClass("tint-out");
    expect(screen.getByText("ends")).toBeInTheDocument();

    act(() => void vi.advanceTimersByTime(120));
    expect(screen.getByText("who")).toBeInTheDocument();
    expect(screen.getByTestId("list")).toHaveClass("tint-in");

    act(() => void vi.advanceTimersByTime(120));
    expect(screen.getByTestId("list")).not.toHaveClass("tint-in");
    expect(screen.getByTestId("list")).not.toHaveClass("tint-out");
  });

  it("re-orders at once under reduced motion", () => {
    motionInTests(false);
    render(<Sorted />);
    fireEvent.click(screen.getByRole("button", { name: "Sort by who" }));
    expect(screen.getByText("who")).toBeInTheDocument();
    expect(screen.getByTestId("list")).not.toHaveClass("tint-out");
  });
});
