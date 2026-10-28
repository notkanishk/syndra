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

  // A keyboard user who opens the panel lands in it and, on leaving, is back on
  // the button they pressed rather than at the top of the page.
  it("is a labelled dialog that takes focus on open and returns it to the button on close", () => {
    render(panel(1));
    const button = screen.getByRole("button", { name: /Filters/ });
    button.focus();
    fireEvent.click(button);
    const dialog = screen.getByRole("dialog", { name: "Filters" });
    expect(dialog).toHaveFocus();
    expect(dialog).toHaveAttribute("tabindex", "-1");

    fireEvent.keyDown(document, { key: "Escape" });
    expect(button).toHaveFocus();

    fireEvent.click(button);
    fireEvent.click(screen.getByRole("button", { name: "Done" }));
    expect(button).toHaveFocus();
  });

  it("does not pull focus back from where the person clicked next", () => {
    render(
      <>
        {panel(1)}
        <input aria-label="Elsewhere" />
      </>,
    );
    fireEvent.click(screen.getByRole("button", { name: /Filters/ }));
    const other = screen.getByLabelText("Elsewhere");
    other.focus();
    fireEvent.mouseDown(other);
    expect(other).toHaveFocus();
  });
});
