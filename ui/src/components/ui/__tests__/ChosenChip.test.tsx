// @vitest-environment jsdom
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { ChosenChip } from "@/components/ui/Badge";

/** A-17 G15: one chosen-with-x part, in every picker. */
describe("ChosenChip", () => {
  it("shows what is chosen and a remove button that says what goes", () => {
    render(
      <ChosenChip removeLabel="Remove Laser cutter" onRemove={() => {}}>
        Laser cutter
      </ChosenChip>,
    );
    expect(screen.getByText("Laser cutter")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Remove Laser cutter" })).toBeInTheDocument();
  });

  it("puts it back when the x is pressed, once", () => {
    const onRemove = vi.fn();
    render(
      <ChosenChip removeLabel="Remove Wood shop" onRemove={onRemove}>
        Wood shop
      </ChosenChip>,
    );
    fireEvent.click(screen.getByRole("button", { name: "Remove Wood shop" }));
    expect(onRemove).toHaveBeenCalledTimes(1);
  });

  it("does nothing while disabled", () => {
    const onRemove = vi.fn();
    render(
      <ChosenChip disabled removeLabel="Remove Wood shop" onRemove={onRemove}>
        Wood shop
      </ChosenChip>,
    );
    const button = screen.getByRole("button", { name: "Remove Wood shop" });
    expect(button).toBeDisabled();
    fireEvent.click(button);
    expect(onRemove).not.toHaveBeenCalled();
  });

  it("reaches the touch floor through the token, not a typed 44px", () => {
    render(
      <ChosenChip removeLabel="Remove Wood shop" onRemove={() => {}}>
        Wood shop
      </ChosenChip>,
    );
    const { className } = screen.getByRole("button", { name: "Remove Wood shop" });
    expect(className).toContain("min-h-[var(--tap)]");
    expect(className).toContain("min-w-[var(--tap)]");
    expect(className).not.toContain("44px");
  });
});
