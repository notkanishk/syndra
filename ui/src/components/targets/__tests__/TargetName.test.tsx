// @vitest-environment jsdom
import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { TargetNameDialog, TargetNameRow } from "@/components/targets/TargetName";
import type { TargetSummary } from "@/lib/queries/useTargets";

const saved: string[] = [];
vi.mock("@/lib/queries/useTargets", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/queries/useTargets")>()),
  useSetTargetName: () => ({ mutate: (name: string) => saved.push(name), isPending: false, error: null }),
}));

beforeEach(() => {
  saved.length = 0;
});

const row = { target: "laser", name: "Ruby", name_from: "product", kind: "machine" } as TargetSummary;

// D-1 Q4 (owner, 18 Nov 2026; part 2 frame 6): the name is a row of the settings card, its way on in the seat.
describe("the Name row", () => {
  it("says the name and what members see, with Change name in the seat and no field on the page", () => {
    const { container } = render(<TargetNameRow registered={row} />);
    const line = container.querySelector("[data-card-row]") as HTMLElement;
    expect(line).toHaveTextContent("Name");
    expect(line).toHaveTextContent("Ruby");
    expect(line).toHaveTextContent("Members see it as Ruby, the product's own name.");
    // What the field does is the dialog's to say, not the row's.
    expect(line).not.toHaveTextContent("Name it here");
    const change = screen.getByRole("button", { name: "Change name" });
    expect(change.closest("[data-action-seat]")).not.toBeNull();
    expect(screen.queryByRole("textbox")).toBeNull();
  });

  it("opens the one rename dialog", () => {
    render(<TargetNameRow registered={row} />);
    fireEvent.click(screen.getByRole("button", { name: "Change name" }));
    expect(screen.getByRole("dialog")).toHaveTextContent("What members see it called");
    expect(screen.getByLabelText("Name")).toBeInTheDocument();
  });
});

describe("the rename dialog", () => {
  it("says why Save name is off under the button, and offers the default name only once named", () => {
    const { unmount } = render(<TargetNameDialog registered={row} onClose={() => {}} />);
    expect(screen.queryByRole("button", { name: "Use the default name" })).toBeNull();
    unmount();

    render(<TargetNameDialog registered={{ ...row, name: "Big laser", name_from: "operator" }} onClose={() => {}} />);
    const save = screen.getByRole("button", { name: "Save name" });
    expect(save).toBeDisabled();
    expect(save.parentElement).toHaveTextContent("Type a new name first.");
    expect(screen.getByRole("button", { name: "Use the default name" })).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText("Name"), { target: { value: "Laser" } });
    expect(save).toBeEnabled();
  });

  it("saves on Enter in the field, as Save name does, and never an unchanged name", () => {
    render(<TargetNameDialog registered={row} onClose={() => {}} />);
    const field = screen.getByLabelText("Name");
    fireEvent.submit(field);
    expect(saved).toEqual([]);
    fireEvent.change(field, { target: { value: "  Big laser  " } });
    fireEvent.submit(field);
    expect(saved).toEqual(["Big laser"]);
  });
});
