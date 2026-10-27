// @vitest-environment jsdom
import { fireEvent, render, screen } from "@testing-library/react";
import { useState } from "react";
import { describe, expect, it } from "vitest";

import { FilterPills, Segmented } from "@/components/ui/Select";
import { Tabs } from "@/components/ui/Tabs";

const OPTIONS = [
  { value: "a", label: "Alpha" },
  { value: "b", label: "Bravo", disabled: true },
  { value: "c", label: "Charlie" },
  { value: "d", label: "Delta" },
];

function Segment({ initial = "a" }: { initial?: string }) {
  const [value, setValue] = useState(initial);
  return <Segmented label="Pick" options={OPTIONS} value={value} onChange={setValue} />;
}
function Pills() {
  const [value, setValue] = useState("a");
  return <FilterPills label="Pick" options={OPTIONS} value={value} onChange={setValue} />;
}

describe.each([
  ["Segmented", Segment],
  ["FilterPills", Pills],
])("%s keyboard", (_name, Group) => {
  const radio = (name: string) => screen.getByRole("radio", { name });

  it("has one tab stop: the checked option", () => {
    render(<Group />);
    const stops = screen.getAllByRole("radio").filter((r) => r.tabIndex === 0);
    expect(stops).toEqual([radio("Alpha")]);
  });

  it("arrows move to the next enabled option, select it and take focus, wrapping at the ends", () => {
    render(<Group />);
    radio("Alpha").focus();
    fireEvent.keyDown(radio("Alpha"), { key: "ArrowRight" });
    // Bravo is disabled: skipped.
    expect(radio("Charlie")).toHaveAttribute("aria-checked", "true");
    expect(radio("Charlie")).toHaveFocus();
    expect(radio("Charlie").tabIndex).toBe(0);
    expect(radio("Alpha").tabIndex).toBe(-1);

    fireEvent.keyDown(radio("Charlie"), { key: "ArrowDown" });
    fireEvent.keyDown(radio("Delta"), { key: "ArrowRight" });
    expect(radio("Alpha")).toHaveAttribute("aria-checked", "true");
    fireEvent.keyDown(radio("Alpha"), { key: "ArrowLeft" });
    expect(radio("Delta")).toHaveFocus();
  });

  it("Home and End jump to the ends; other keys are left alone", () => {
    render(<Group />);
    fireEvent.keyDown(radio("Alpha"), { key: "End" });
    expect(radio("Delta")).toHaveAttribute("aria-checked", "true");
    fireEvent.keyDown(radio("Delta"), { key: "Home" });
    expect(radio("Alpha")).toHaveAttribute("aria-checked", "true");
    expect(fireEvent.keyDown(radio("Alpha"), { key: "a" })).toBe(true);
  });
});

describe("Segmented with nothing checked", () => {
  it("puts the tab stop on the first enabled option", () => {
    render(<Segment initial="zzz" />);
    expect(screen.getByRole("radio", { name: "Alpha" }).tabIndex).toBe(0);
    expect(screen.getByRole("radio", { name: "Delta" }).tabIndex).toBe(-1);
  });
});

describe("Tabs", () => {
  it("is a named group, so its label is announced, and keeps aria-current", () => {
    render(
      <Tabs
        label="Views of this person"
        value="a"
        onSelect={() => {}}
        options={[
          { value: "a", label: "One" },
          { value: "b", label: "Two" },
        ]}
      />,
    );
    expect(screen.getByRole("group", { name: "Views of this person" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "One" })).toHaveAttribute("aria-current", "page");
  });
});
