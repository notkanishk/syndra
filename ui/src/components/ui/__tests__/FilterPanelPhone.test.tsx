// @vitest-environment jsdom
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { FilterPanel } from "@/components/ui/FilterPanel";

vi.mock("@/lib/useViewport", () => ({ useIsPhone: () => true }));

// 87c: below tablet, Filters is a sheet ending in one accent button that names the result.
describe("filters on a phone", () => {
  it("opens as a sheet whose button says how many it shows", () => {
    const clear = vi.fn();
    render(
      <FilterPanel activeCount={1} onClear={clear} resultCount={12} noun={["person", "people"]}>
        <span>Project</span>
      </FilterPanel>,
    );
    fireEvent.click(screen.getByRole("button", { name: /Filters/ }));
    const sheet = screen.getByRole("dialog");
    expect(sheet).toHaveTextContent("Project");
    fireEvent.click(screen.getByRole("button", { name: "Clear all" }));
    expect(clear).toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "Show 12 people" }));
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });
});
