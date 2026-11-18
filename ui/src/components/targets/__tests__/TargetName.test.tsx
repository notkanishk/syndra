// @vitest-environment jsdom
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { TargetName } from "@/components/targets/TargetName";
import type { TargetSummary } from "@/lib/queries/useTargets";

vi.mock("@/lib/queries/useTargets", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/queries/useTargets")>()),
  useSetTargetName: () => ({ mutate: vi.fn(), isPending: false, error: null }),
}));

const row = { target: "laser", name: "Ruby", name_from: "product", kind: "machine" } as TargetSummary;

// D-1 §5 #13: the Name card is a form row, a dense field at the page field width with its button on the same line.
describe("the Name form row", () => {
  it("holds the field at its width and Save name beside it, the hint under the field", () => {
    render(<TargetName registered={row} />);
    const input = screen.getByLabelText("Name");
    const line = input.parentElement!.parentElement as HTMLElement;
    expect(input.parentElement).toHaveClass("w-full", "max-w-(--field-max)");
    const save = screen.getByRole("button", { name: "Save name" });
    expect(save.parentElement!.parentElement).toBe(line);
    expect(save).toHaveClass("desktop:min-h-(--control-sm)");
    expect(screen.getByText(/Members see it as Ruby/).previousElementSibling).toBe(line);
  });

  it("says why Save name is off under the button, and keeps Use the default name on the line", () => {
    render(<TargetName registered={{ ...row, name: "Big laser", name_from: "operator" }} />);
    const save = screen.getByRole("button", { name: "Save name" });
    const line = save.parentElement!.parentElement as HTMLElement;
    expect(save).toBeDisabled();
    expect(save.parentElement).toHaveTextContent("Type a new name first.");
    expect(screen.getByRole("button", { name: "Use the default name" }).parentElement!.parentElement).toBe(line);
    fireEvent.change(screen.getByLabelText("Name"), { target: { value: "Laser" } });
    expect(save).toBeEnabled();
  });
});
