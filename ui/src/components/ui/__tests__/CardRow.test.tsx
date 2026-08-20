// @vitest-environment jsdom
import { fireEvent, render, screen } from "@testing-library/react";
import { renderHook, act } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { Card, CardColumns, CardRow, RowField } from "@/components/ui/Card";
import { useOpenRow } from "@/lib/useOpenRow";

describe("a row that discloses the rest of itself", () => {
  // Additive by design: every one of the forty-odd existing callers renders
  // exactly as it did, or this could not land in one commit.
  it("renders as it always did when given no disclosure", () => {
    render(
      <Card>
        <CardRow first>
          <span>Aditi Rao</span>
        </CardRow>
      </Card>,
    );
    expect(screen.getByText("Aditi Rao")).toBeTruthy();
    expect(screen.queryByRole("button")).toBeNull();
  });

  // A 16px chevron inside a 60px row is a row that looks tappable and mostly
  // is not.
  it("makes the whole row the control, not a chevron", () => {
    const onToggle = vi.fn();
    render(
      <CardRow disclosure={<RowField label="Granted by">Kabir</RowField>} onToggle={onToggle}>
        <span>Laser cutter</span>
      </CardRow>,
    );

    fireEvent.click(screen.getByText("Laser cutter"));
    expect(onToggle).toHaveBeenCalledTimes(1);
  });

  it("says whether it is open, so a screen reader is not guessing", () => {
    const { rerender } = render(
      <CardRow disclosure={<span>body</span>} onToggle={() => {}}>
        <span>Laser cutter</span>
      </CardRow>,
    );
    expect(screen.getByRole("button")).toHaveAttribute("aria-expanded", "false");
    expect(screen.queryByText("body")).toBeNull();

    rerender(
      <CardRow disclosure={<span>body</span>} expanded onToggle={() => {}}>
        <span>Laser cutter</span>
      </CardRow>,
    );
    expect(screen.getByRole("button")).toHaveAttribute("aria-expanded", "true");
    expect(screen.getByText("body")).toBeTruthy();
  });

  it("labels a disclosed field with what its column used to say", () => {
    render(
      <CardRow disclosure={<RowField label="Granted by">Kabir Rao</RowField>} expanded onToggle={() => {}}>
        <span>Laser cutter</span>
      </CardRow>,
    );
    expect(screen.getByText("Granted by")).toBeTruthy();
    expect(screen.getByText("Kabir Rao")).toBeTruthy();
  });
});

describe("column headings", () => {
  // A heading is a promise that the cells beneath it line up. On a phone the
  // fields stack, so the promise cannot be kept and the heading goes — once,
  // centrally, rather than in forty callers.
  it("are hidden below the tablet breakpoint, from the primitive", () => {
    render(
      <CardColumns>
        <span>Project</span>
      </CardColumns>,
    );
    const columns = screen.getByText("Project").parentElement!;
    expect(columns.className).toContain("hidden");
    expect(columns.className).toContain("tablet:flex");
  });
});

describe("one row open at a time", () => {
  it("closes the previous row when another opens", () => {
    const { result } = renderHook(() => useOpenRow());

    act(() => result.current.toggle("a"));
    expect(result.current.isOpen("a")).toBe(true);

    act(() => result.current.toggle("b"));
    expect(result.current.isOpen("b")).toBe(true);
    expect(result.current.isOpen("a"), "two open rows is a list with two shapes").toBe(false);
  });

  it("closes a row when it is tapped again", () => {
    const { result } = renderHook(() => useOpenRow());
    act(() => result.current.toggle("a"));
    act(() => result.current.toggle("a"));
    expect(result.current.isOpen("a")).toBe(false);
  });
});
