// @vitest-environment jsdom
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { Notice } from "@/components/ui/Notice";

describe("Notice", () => {
  // A notice about something that can't be used until it clears interrupts;
  // one that only informs is read when reached.
  it("is an alert only when it blocks", () => {
    const { rerender } = render(
      <Notice tone="warn" title="TrueNAS is offline" blocking>
        {"Your shares won't open until it's back."}
      </Notice>,
    );
    expect(screen.getByRole("alert").textContent).toContain("TrueNAS is offline");

    rerender(
      <Notice tone="warn" title="Account changes are paused">
        {"Your access hasn't changed."}
      </Notice>,
    );
    expect(screen.queryByRole("alert")).toBeNull();
    expect(screen.getByRole("status").textContent).toContain("Your access hasn't changed.");
  });

  it("offers its action as an outline button and the reference as a copyable value", () => {
    const retry = vi.fn();
    render(
      <Notice tone="danger" title="Couldn't load" action={{ label: "Try again", onClick: retry }} reference="ABCDE12345">
        Nothing was changed.
      </Notice>,
    );
    fireEvent.click(screen.getByRole("button", { name: "Try again" }));
    expect(retry).toHaveBeenCalledOnce();
    expect(screen.getByText("ABCDE12345")).toBeTruthy();
  });
});
