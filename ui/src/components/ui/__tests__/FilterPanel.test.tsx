// @vitest-environment jsdom
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { FilterField, FilterPanel } from "@/components/ui/FilterPanel";

function panel(activeCount = 0, onClear = () => {}) {
  return (
    <FilterPanel activeCount={activeCount} onClear={onClear}>
      <FilterField label="Project">
        <select aria-label="Filter by project">
          <option value="">All projects</option>
        </select>
      </FilterField>
    </FilterPanel>
  );
}

describe("FilterPanel", () => {
  it("keeps its contents closed until asked", () => {
    render(panel());
    expect(screen.queryByLabelText("Filter by project")).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: /Filters/ }));
    expect(screen.getByLabelText("Filter by project")).toBeTruthy();
  });

  // The question a hidden filter creates is "why am I seeing so few rows", and
  // the count on the button is the whole of the answer — without it somebody
  // has to open the panel to find out whether anything is narrowing the list.
  it("says how many filters are narrowing the list, without being opened", () => {
    render(panel(2));
    expect(screen.getByRole("button", { name: /Filters/ }).textContent).toContain("2");
  });

  it("offers nothing to clear when nothing is set", () => {
    render(panel(0));
    fireEvent.click(screen.getByRole("button", { name: /Filters/ }));
    expect(screen.getByRole("button", { name: "Clear all" })).toHaveProperty("disabled", true);
  });

  it("clears when there is something to clear", () => {
    const onClear = vi.fn();
    render(panel(1, onClear));
    fireEvent.click(screen.getByRole("button", { name: /Filters/ }));
    fireEvent.click(screen.getByRole("button", { name: "Clear all" }));
    expect(onClear).toHaveBeenCalled();
  });

  it("closes on Escape and on a click outside", () => {
    render(panel(1));
    const open = () => fireEvent.click(screen.getByRole("button", { name: /Filters/ }));

    open();
    fireEvent.keyDown(document, { key: "Escape" });
    expect(screen.queryByLabelText("Filter by project")).toBeNull();

    open();
    fireEvent.mouseDown(document.body);
    expect(screen.queryByLabelText("Filter by project")).toBeNull();
  });
});
