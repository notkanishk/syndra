// @vitest-environment jsdom
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { RunLabel, SortHead, runStarts, useSort } from "@/components/ui/SortHead";

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
