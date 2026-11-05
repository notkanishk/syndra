// @vitest-environment jsdom
import { act, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { SelectionBar, type SelectionVerb } from "@/components/ui/SelectionBar";
import { LIGHT_LEAD_MS, TINT_MS } from "@/lib/motion";
import { setMediaQuery } from "@/test-utils/media";
import { motionInTests } from "@/test-utils/motion";

/**
 * The bar below tablet (A-12 §2, 2a): one primary verb and "More changes", a
 * Modal sm sheet of the rest. Sheets don't stack: the sheet is gone before the
 * chosen verb's own dialog opens.
 */
const PHONE = "(max-width: 44.99rem)";

function verbs(onClick: (key: string) => void = () => {}): SelectionVerb[] {
  return [
    ["assign_role", "Give a role", "neutral"],
    ["assign_bundle", "Give a bundle", "neutral"],
    ["extend", "Extend what ends", "neutral"],
    ["remove_bundle", "Remove from a bundle", "danger"],
    ["remove_role", "Revoke a role", "danger"],
  ].map(([key, label, tone]) => ({ key, label, tone: tone as "neutral" | "danger", onClick: () => onClick(key) }));
}

function bar(props: Partial<React.ComponentProps<typeof SelectionBar>> = {}) {
  return (
    <SelectionBar count={3} noun={["person", "people"]} onClear={() => {}} verbs={verbs()} {...props} />
  );
}

const region = () => screen.getByRole("region", { name: "Selection" });

describe("the selection bar below tablet", () => {
  beforeEach(() => setMediaQuery(PHONE, true));

  it("offers one verb and More changes, not all five", () => {
    render(bar());
    const buttons = within(region()).getAllByRole("button").map((button) => button.textContent);
    expect(buttons).toEqual(["Clear the selection", "Give a role", "More changes"]);
    // It opens a sheet, so it says so, and never ends in an arrow.
    expect(within(region()).getByRole("button", { name: "More changes" })).toHaveAttribute("aria-haspopup", "dialog");
  });

  it("leads with the verb it is told is primary, in that verb's own tone", () => {
    render(bar({ primary: "remove_role" }));
    const lead = within(region()).getByRole("button", { name: "Revoke a role" });
    expect(lead.className).toContain("border-danger-line");
    expect(lead.className).toContain("min-h-[var(--tap)]");
    expect(within(region()).queryByRole("button", { name: "Give a role" })).toBeNull();
  });

  it("opens the rest in a sheet, in the desktop order, neutral first and the danger outlines after", () => {
    render(bar({ primary: "extend" }));
    fireEvent.click(within(region()).getByRole("button", { name: "More changes" }));
    const sheet = screen.getByRole("dialog", { name: "More changes" });
    const labels = within(sheet)
      .getAllByRole("button")
      .map((button) => button.textContent)
      .filter((label) => label !== "Cancel" && label !== "" && !/close|drag/i.test(label ?? ""));
    expect(labels).toEqual(["Give a role", "Give a bundle", "Remove from a bundle", "Revoke a role"]);
    // The 16px gap sits before the first danger outline only.
    expect(within(sheet).getByRole("button", { name: "Remove from a bundle" }).className).toContain("mt-2");
    expect(within(sheet).getByRole("button", { name: "Give a bundle" }).className).not.toContain("mt-2");
    expect(within(sheet).getByText("Each one shows who would change, person by person, before anything happens.")).toBeTruthy();
  });

  it("Cancel closes the sheet and runs nothing", () => {
    const chose = vi.fn();
    render(bar({ verbs: verbs(chose) }));
    fireEvent.click(within(region()).getByRole("button", { name: "More changes" }));
    fireEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: "Cancel" }));
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(chose).not.toHaveBeenCalled();
  });

  it("over the ceiling, the one narrowing move takes the row and More changes goes", () => {
    render(bar({ count: 640, ceiling: 500, onTakeCeiling: () => {} }));
    const buttons = within(region()).getAllByRole("button").map((button) => button.textContent);
    expect(buttons).toEqual(["Clear the selection", "Select the first 500 people in the order shown"]);
  });

  it("keeps a bar of one or two verbs as it is", () => {
    render(bar({ verbs: verbs().slice(0, 2) }));
    expect(within(region()).queryByRole("button", { name: "More changes" })).toBeNull();
    expect(within(region()).getByRole("button", { name: "Give a bundle" })).toBeTruthy();
  });
});

describe("a verb chosen in More changes", () => {
  beforeEach(() => setMediaQuery(PHONE, true));
  afterEach(() => vi.useRealTimers());

  // Sheets don't stack: the verb's dialog may only open once More changes has gone.
  it("waits for the sheet to leave (S1) before its own dialog opens", () => {
    vi.useFakeTimers();
    motionInTests();
    const sheetOpenWhenChosen: boolean[] = [];
    render(bar({ verbs: verbs(() => sheetOpenWhenChosen.push(screen.queryByRole("dialog") !== null)) }));

    fireEvent.click(within(region()).getByRole("button", { name: "More changes" }));
    fireEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: "Revoke a role" }));
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(sheetOpenWhenChosen).toEqual([]);

    act(() => vi.advanceTimersByTime(TINT_MS + LIGHT_LEAD_MS - 1));
    expect(sheetOpenWhenChosen).toEqual([]);
    act(() => vi.advanceTimersByTime(1));
    expect(sheetOpenWhenChosen).toEqual([false]);
  });

  it("still closes first under reduced motion, then opens with nothing to wait for", async () => {
    const chosen: string[] = [];
    render(bar({ verbs: verbs((key) => chosen.push(`${key}:${screen.queryByRole("dialog") ? "sheet" : "gone"}`)) }));
    fireEvent.click(within(region()).getByRole("button", { name: "More changes" }));
    fireEvent.click(within(screen.getByRole("dialog")).getByRole("button", { name: "Give a bundle" }));
    await act(async () => {
      await new Promise((resolve) => setTimeout(resolve, 0));
    });
    expect(chosen).toEqual(["assign_bundle:gone"]);
  });
});

describe("the selection bar from tablet up", () => {
  it("keeps all five verbs in one row, in their order", () => {
    setMediaQuery(PHONE, false);
    render(bar({ primary: "remove_role" }));
    const buttons = within(region()).getAllByRole("button").map((button) => button.textContent);
    expect(buttons).toEqual([
      "Give a role",
      "Give a bundle",
      "Extend what ends",
      "Remove from a bundle",
      "Revoke a role",
      "Clear the selection",
    ]);
  });
});
