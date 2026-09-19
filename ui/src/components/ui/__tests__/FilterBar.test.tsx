// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { FilterBar } from "@/components/ui/FilterBar";

describe("FilterBar", () => {
  // Search finds one row; filters narrow to many. Hiding the first behind a
  // disclosure is what makes a search nobody uses.
  it("keeps search on the row and lets the rest be hidden", () => {
    render(
      <FilterBar
        search={<input aria-label="Search" />}
        filters={<button type="button">Filters</button>}
        trailing={<button type="button">Select</button>}
      />,
    );
    expect(screen.getByLabelText("Search")).toBeTruthy();
    expect(screen.getByRole("button", { name: "Filters" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "Select" })).toBeTruthy();
  });

  it("renders nothing it was not given", () => {
    const { container } = render(<FilterBar />);
    expect(container.querySelectorAll("input")).toHaveLength(0);
    expect(container.querySelectorAll("button")).toHaveLength(0);
  });

  // A screen that narrows by search alone still uses the row, so its search box
  // sits where every other screen's does.
  it("works with search and no filters", () => {
    render(<FilterBar search={<input aria-label="Search" />} />);
    expect(screen.getByLabelText("Search")).toBeTruthy();
  });
});
