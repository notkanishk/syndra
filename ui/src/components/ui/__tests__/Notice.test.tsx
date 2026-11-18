// @vitest-environment jsdom
import { act, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { Notice } from "@/components/ui/Notice";
import { FOLD_MS, TINT_MS } from "@/lib/motion";
import { motionInTests } from "@/test-utils/motion";

describe("Notice", () => {
  // A notice about something that can't be used until it clears interrupts;
  // one that only informs is read when reached.
  it("is an alert only when it blocks", () => {
    const { rerender } = render(
      <Notice tone="warn" title="TrueNAS is offline" next="Try again." blocking>
        {"Your shares won't open until it's back."}
      </Notice>,
    );
    expect(screen.getByRole("alert").textContent).toContain("TrueNAS is offline");

    rerender(
      <Notice tone="warn" title="Account changes are paused" next="Try again.">
        {"Your access hasn't changed."}
      </Notice>,
    );
    expect(screen.queryByRole("alert")).toBeNull();
    expect(screen.getByRole("status").textContent).toContain("Your access hasn't changed.");
  });

  it("offers its action as an outline button and the reference as a copyable value", () => {
    const retry = vi.fn();
    render(
      <Notice tone="danger" title="Couldn't load" next="Try again." action={{ label: "Try again", onClick: retry }} reference="req_7f3a91c2">
        Nothing was changed.
      </Notice>,
    );
    fireEvent.click(screen.getByRole("button", { name: "Try again" }));
    expect(retry).toHaveBeenCalledOnce();
    expect(screen.getByText("req_7f3a91c2")).toBeTruthy();
  });
});

// D-1 M3: a Notice whose condition cleared fades (tint), then folds its space (fold); it does not
// vanish at once. jsdom has no Web Animations API, so the fold is stood in for here.
describe("Notice leaving", () => {
  const props = { tone: "danger", title: "Needs a decision.", next: "Decide it." } as const;
  const proto = Element.prototype as { getAnimations?: () => unknown[] };

  afterEach(() => {
    delete proto.getAnimations;
    motionInTests(false);
    vi.useRealTimers();
  });

  it("stays mounted, marked closing, until tint plus fold have passed", () => {
    motionInTests();
    vi.useFakeTimers();
    proto.getAnimations = () => [];
    const { container, rerender } = render(
      <Notice {...props} open>
        Body.
      </Notice>,
    );
    expect(screen.getByRole("status")).toBeTruthy();
    expect(container.querySelector("[data-closing]")).toBeNull();

    rerender(
      <Notice {...props} open={false}>
        Body.
      </Notice>,
    );
    // Still readable while it goes.
    expect(container.querySelector("[data-closing]")).not.toBeNull();
    // (the closing contents are hidden from assistive tech, so asked for as such)
    expect(screen.getByRole("status", { hidden: true }).textContent).toContain("Needs a decision.");

    act(() => void vi.advanceTimersByTime(TINT_MS + FOLD_MS - 1));
    expect(container.querySelector("[data-closing]")).not.toBeNull();
    act(() => void vi.advanceTimersByTime(1));
    expect(container.querySelector("[data-closing]")).toBeNull();
    expect(screen.queryByRole("status")).toBeNull();
  });

  it("is gone at once under still motion", () => {
    const { container, rerender } = render(
      <Notice {...props} open>
        Body.
      </Notice>,
    );
    rerender(
      <Notice {...props} open={false}>
        Body.
      </Notice>,
    );
    expect(container.querySelector("[data-closing]")).toBeNull();
    expect(screen.queryByRole("status")).toBeNull();
  });

  it("is not drawn while it has never been open", () => {
    render(
      <Notice {...props} open={false}>
        Body.
      </Notice>,
    );
    expect(screen.queryByRole("status", { hidden: true })).toBeNull();
  });

  it("opens its space, rather than popping in, when its condition arrives after it was mounted closed", () => {
    motionInTests();
    proto.getAnimations = () => [];
    const { container, rerender } = render(
      <Notice {...props} open={false}>
        Body.
      </Notice>,
    );
    expect(container.querySelector(".unfold")).toBeNull();
    rerender(
      <Notice {...props} open>
        Body.
      </Notice>,
    );
    expect(screen.getByRole("status").closest(".unfold")?.getAttribute("data-phase")).toBe("opening");
  });
});
