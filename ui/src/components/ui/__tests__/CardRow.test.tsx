// @vitest-environment jsdom
import { fireEvent, render, screen } from "@testing-library/react";
import { renderHook, act } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { Card, CardColumns, CardHeader, CardRow, RowField } from "@/components/ui/Card";
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

/**
 * The second shape: a row whose own control opens the panel.
 *
 * The row here already holds a button, and a row that is ALSO a button nests
 * one inside the other — invalid, and two overlapping targets. So `onToggle`
 * is omitted and the row stays inert. What must not change is where the panel
 * lands: this branch used to drop the disclosure on the floor, which is why the
 * adopt form rendered at the foot of the account list instead of under the
 * account it was about.
 */
describe("a row opened by a control inside it", () => {
  it("puts the panel under its own row, not somewhere else", () => {
    render(
      <Card>
        <CardRow first expanded disclosure={<span>adopt sai</span>}>
          <span>sai</span>
          <button type="button">Adopt</button>
        </CardRow>
        <CardRow expanded={false} disclosure={null}>
          <span>syndra</span>
        </CardRow>
      </Card>,
    );

    // The wrapper the panel lives in: `settle-in` box, then the row wrapper.
    const owner = screen.getByText("adopt sai").parentElement!.parentElement!;
    expect(owner.textContent, "the panel belongs to the row it is about").toContain("sai");
    expect(
      owner.textContent,
      "and not to the last row in the list, which is where it used to appear",
    ).not.toContain("syndra");
  });

  it("keeps the row inert — no button inside a button", () => {
    render(
      <CardRow expanded disclosure={<span>body</span>}>
        <span>sai</span>
        <button type="button">Adopt</button>
      </CardRow>,
    );

    // Exactly one button: the row's own control. The row itself is not one.
    expect(screen.getAllByRole("button")).toHaveLength(1);
    expect(screen.getByRole("button")).toHaveTextContent("Adopt");
  });

  // Everything in this product that opens rises into place. A panel that simply
  // appears is the one thing on the screen that did not.
  it("settles in like every other thing that opens", () => {
    render(
      <CardRow expanded disclosure={<span>body</span>}>
        <span>sai</span>
      </CardRow>,
    );
    expect(screen.getByText("body").parentElement!.className).toContain("settle-in");
  });

  it("renders nothing while it is closed", () => {
    render(
      <CardRow disclosure={<span>body</span>}>
        <span>sai</span>
      </CardRow>,
    );
    expect(screen.queryByText("body")).toBeNull();
  });
});

/**
 * One count, one rendering of zero.
 *
 * `CountChip` (region headings) and `CardHeader` (card headings) both draw a
 * count, and they agreed everywhere except the case that carries the message:
 * `CountChip` holds a zero hollow, `CardHeader` filled it solid. A solid badge
 * is an alarm, so "Not going to happen · 0" was an alarm about nothing
 * happening.
 */
describe("a count of zero", () => {
  it("is hollow, not a filled alarm", () => {
    render(
      <Card>
        <CardHeader title="Not going to happen" count={0} tone="danger" />
      </Card>,
    );
    const badge = screen.getByText("0");
    expect(badge.className).toContain("border");
    expect(badge.className, "danger fill on a zero says something is wrong").not.toContain(
      "bg-danger",
    );
  });

  it("still fills a real number, in its own tone", () => {
    render(
      <Card>
        <CardHeader title="Not going to happen" count={4} tone="danger" />
      </Card>,
    );
    expect(screen.getByText("4").className).toContain("bg-danger");
  });
});
