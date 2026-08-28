// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { Region } from "@/components/targets/Region";
import { CountChip } from "@/components/ui/Badge";

/**
 * The four regions of the target page, and the rule they exist to keep.
 *
 * The page carried eleven answers as eleven peers, in the order they were
 * added. Four regions replaced that — but the structural rule is what makes the
 * grouping worth anything: a region keeps its seat whatever its count, so the
 * page reads the same shape whether or not somebody's access is disputed.
 */

describe("a region keeps its seat", () => {
  it("renders at zero, with a hollow count rather than nothing", () => {
    render(
      <Region id="waiting" title="Waiting on a person" count={0} lede="…">
        <p>nothing here</p>
      </Region>,
    );

    expect(screen.getByRole("heading", { name: "Waiting on a person" })).toBeTruthy();
    expect(screen.getByText("0")).toBeTruthy();
  });

  it("is reachable by anchor, so the touch index can jump to it", () => {
    const { container } = render(
      <Region id="people" title="People and their access here">
        <p>rows</p>
      </Region>,
    );

    expect(container.querySelector("#people")).toBeTruthy();
  });
});

/**
 * A failed read and an empty one are different facts, and `0` for the first is
 * a lie: it says the region is empty when nobody could look.
 */
describe("the count chip", () => {
  it("says nothing was read, rather than saying zero", () => {
    render(<CountChip n={null} />);
    expect(screen.getByLabelText("count could not be read")).toBeTruthy();
    expect(screen.queryByText("0")).toBeNull();
  });

  it("distinguishes an empty region from a populated one", () => {
    const { rerender } = render(<CountChip n={0} />);
    const hollow = screen.getByText("0").className;

    rerender(<CountChip n={3} />);
    const filled = screen.getByText("3").className;

    expect(hollow).not.toEqual(filled);
  });
});
