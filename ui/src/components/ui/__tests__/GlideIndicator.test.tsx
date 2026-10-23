// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it } from "vitest";

import { FilterPills, Segmented } from "@/components/ui/Select";
import { Tabs } from "@/components/ui/Tabs";
import { motionInTests } from "@/test-utils/motion";

/**
 * S10: the fill (Segmented, FilterPills) or the underline (Tabs) glides to the
 * choice. One indicator for all three, measured from the active button. jsdom
 * has no layout, so the buttons are given places by hand: A at 0, B at 50 (70
 * wide), C at 120.
 */
const AT: Record<string, [number, number]> = { A: [0, 50], B: [50, 70], C: [120, 40] };
const place = (el: HTMLElement) => AT[el.textContent ?? ""] ?? [0, 0];

beforeEach(() => {
  Object.defineProperty(HTMLElement.prototype, "offsetLeft", { configurable: true, get() { return place(this)[0]; } });
  Object.defineProperty(HTMLElement.prototype, "offsetWidth", { configurable: true, get() { return place(this)[1]; } });
});
afterEach(() => {
  cleanup();
  motionInTests(false);
  // @ts-expect-error restoring jsdom's own (absent) layout
  delete HTMLElement.prototype.offsetLeft;
  // @ts-expect-error restoring jsdom's own (absent) layout
  delete HTMLElement.prototype.offsetWidth;
});

const options = [
  { value: "a", label: "A" },
  { value: "b", label: "B" },
  { value: "c", label: "C" },
];
const indicator = () => document.querySelector<HTMLElement>("[data-indicator]");

const controls = {
  Segmented: (value: string) => <Segmented label="Pick" options={options} value={value} onChange={() => {}} />,
  FilterPills: (value: string) => <FilterPills label="Pick" options={options} value={value} onChange={() => {}} />,
  Tabs: (value: string) => <Tabs label="Pick" options={options} value={value} onSelect={() => {}} />,
};

describe.each(Object.entries(controls))("%s indicator", (_name, draw) => {
  it("is drawn once, under the active option", () => {
    render(draw("b"));
    expect(document.querySelectorAll("[data-indicator]")).toHaveLength(1);
    expect(indicator()).toHaveStyle({ left: "50px", width: "70px" });
  });

  it("follows the choice", () => {
    const { rerender } = render(draw("a"));
    expect(indicator()).toHaveStyle({ left: "0px", width: "50px" });
    rerender(draw("c"));
    expect(indicator()).toHaveStyle({ left: "120px", width: "40px" });
  });

  it("is not part of the accessibility tree", () => {
    render(draw("a"));
    expect(indicator()).toHaveAttribute("aria-hidden", "true");
    expect(screen.getAllByRole(_name === "Tabs" ? "button" : "radio")).toHaveLength(3);
  });

  it("has no transition under reduced motion, and glides on settle otherwise", () => {
    render(draw("a"));
    expect(indicator()!.className).not.toContain("indicator-glide");
    cleanup();
    motionInTests();
    render(draw("a"));
    expect(indicator()!.className).toContain("indicator-glide");
  });
});
