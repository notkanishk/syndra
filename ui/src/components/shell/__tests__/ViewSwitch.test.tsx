// @vitest-environment jsdom
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { ViewSwitch, VIEW_HINT } from "@/components/shell/ViewSwitch";

const state = vi.hoisted(() => ({ isOperator: true, view: "basic", setView: vi.fn() }));
vi.mock("@/lib/ui-view", () => ({ useUiView: () => state }));

// A-04 §7.8: the Basic/Advanced switch is the same Segmented sm as the chime
// toggle, not a hand-rolled pill.
describe("the Basic / Advanced switch", () => {
  it("is a Segmented sm radiogroup with the current view checked", () => {
    state.isOperator = true;
    state.view = "advanced";
    render(<ViewSwitch />);
    const group = screen.getByRole("radiogroup", { name: "Basic or Advanced view" });
    expect(group.className).toContain("bg-tint-2");
    expect(screen.getByRole("radio", { name: "Advanced" })).toBeChecked();
    expect(screen.getByRole("radio", { name: "Basic" })).not.toBeChecked();
    // Segmented sm's own padding, not the md pill's.
    expect(screen.getByRole("radio", { name: "Basic" }).className).toContain("text-[12.5px]");
  });

  it("says what each view shows, in the segment and for a screen reader", () => {
    state.view = "basic";
    render(<ViewSwitch />);
    expect(screen.getByRole("radio", { name: "Advanced" })).toHaveAttribute("title", VIEW_HINT.advanced);
    expect(screen.getByRole("radiogroup")).toHaveAccessibleDescription(
      `${VIEW_HINT.basic} ${VIEW_HINT.advanced}`,
    );
  });

  it("switches in place", () => {
    state.view = "basic";
    render(<ViewSwitch />);
    fireEvent.click(screen.getByRole("radio", { name: "Advanced" }));
    expect(state.setView).toHaveBeenCalledWith("advanced");
  });

  it("is absent for a member", () => {
    state.isOperator = false;
    const { container } = render(<ViewSwitch />);
    expect(container).toBeEmptyDOMElement();
  });

  it("can fill the phone sheet", () => {
    state.isOperator = true;
    render(<ViewSwitch fill />);
    expect(screen.getByRole("radiogroup").className).toContain("w-full");
  });
});
