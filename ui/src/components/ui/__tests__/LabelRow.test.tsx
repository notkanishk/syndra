// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { Button } from "@/components/ui/Button";
import { Card, LabelRow } from "@/components/ui/Card";

/**
 * A setting's row (staging, 20 Nov 2026: at 768 a label held at 170 from `tablet:` beside a 200px seat
 * left its value 5px). jsdom lays nothing out, so this holds the structure the browser lays out from:
 * the label's switch (`row-label`) is a part of the row itself, so its `100%` is the row's width; it
 * reads the seat it sits beside; and the value has a floor a long verb cannot take.
 */
function draw(seat?: "default" | "wide", action: React.ReactNode = <Button size="sm">Change purpose</Button>) {
  render(
    <Card>
      <LabelRow first label="Purpose" seat={seat} action={action}>
        Backups for the laser cutter
      </LabelRow>
    </Card>,
  );
  const label = screen.getByText("Purpose");
  return { label, row: label.parentElement!, value: screen.getByText("Backups for the laser cutter") };
}

describe("LabelRow", () => {
  it("switches the label by the row's width: a part of the row, beside the value and the seat", () => {
    const { label, row, value } = draw();
    expect(row).toHaveAttribute("data-card-row");
    expect(label).toHaveClass("row-label");
    expect(label).not.toHaveAttribute("data-seat");
    expect(Array.from(row.children)).toEqual([label, value, row.querySelector("[data-action-seat]")]);
  });

  it("gives the value a floor, so a long verb wraps the seat instead of crushing it", () => {
    expect(draw().value).toHaveClass("min-w-[min(var(--seat-value),100%)]", "flex-1", "[overflow-wrap:anywhere]");
  });

  it("reads a wide seat's width for the switch, and the seat holds it", () => {
    const { label, row } = draw("wide");
    expect(label).toHaveAttribute("data-seat", "wide");
    expect(row.querySelector("[data-action-seat]")).toHaveAttribute("data-seat", "wide");
  });

  it("keeps an empty seat's place, but takes no line for it on a phone", () => {
    const { row } = draw("default", null);
    expect(row.querySelector("[data-action-seat]")).toBeEmptyDOMElement();
    expect(row).toHaveClass("max-tablet:[&>[data-action-seat]:empty]:hidden");
  });
});
