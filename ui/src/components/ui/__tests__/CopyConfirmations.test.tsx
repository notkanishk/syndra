// @vitest-environment jsdom
import { act, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { Checkbox, Radio } from "@/components/ui/Choice";
import { CommandBlock } from "@/components/ui/CommandBlock";
import { CopyableValue } from "@/components/ui/CopyableValue";
import { COPIED_MS, TINT_MS } from "@/lib/motion";
import { motionInTests } from "@/test-utils/motion";

/**
 * S18, everywhere something copies: the words share one seat, "Copied" arrives
 * with a lime dot that blooms once, and it is Copy again after COPIED_MS. The
 * seat is `CopiedSeat`; these hold the callers to it.
 */

function withClipboard() {
  Object.defineProperty(navigator, "clipboard", {
    value: { writeText: vi.fn().mockResolvedValue(undefined) },
    configurable: true,
  });
}

afterEach(() => {
  Object.defineProperty(navigator, "clipboard", { value: undefined, configurable: true });
  motionInTests(false);
  vi.useRealTimers();
});

describe("CommandBlock's copy button", () => {
  it("crossfades to a blooming dot in one seat and reverts after the shared delay", async () => {
    withClipboard();
    vi.useFakeTimers();
    render(<CommandBlock command="docker compose restart api" />);
    const button = screen.getByRole("button", { name: /Copy command/ });
    const idle = screen.getByText("Copy");
    const done = screen.getByText("Copied");
    expect(idle.parentElement).toBe(done.parentElement);
    expect(idle).toHaveClass("opacity-100");
    expect(button.className).toContain("press-scale");

    await act(async () => {
      fireEvent.click(button);
    });
    expect(done).toHaveClass("opacity-100");
    expect(idle).toHaveClass("opacity-0");
    expect(done.querySelector(".bloom.bg-healthy")).not.toBeNull();

    act(() => void vi.advanceTimersByTime(COPIED_MS - 1));
    expect(done).toHaveClass("opacity-100");
    act(() => void vi.advanceTimersByTime(1));
    expect(idle).toHaveClass("opacity-100");
    expect(done.querySelector(".bloom")).toBeNull();
  });

  it("says Selected without a bloom where the browser cannot copy", () => {
    Object.defineProperty(navigator, "clipboard", { value: undefined, configurable: true });
    render(<CommandBlock command="docker compose restart api" />);
    fireEvent.click(screen.getByRole("button", { name: /Select command/ }));
    expect(screen.getByText("Selected")).toHaveClass("opacity-100");
    expect(screen.getByText("Selected").querySelector(".bloom")).toBeNull();
  });
});

describe("CopyableValue, inline", () => {
  it("keeps the word out of the text until it has copied, then crossfades it in and out", async () => {
    withClipboard();
    motionInTests();
    vi.useFakeTimers();
    render(
      <p>
        <CopyableValue inline value="366421618001445894" label="Account id" />
      </p>,
    );
    const button = screen.getByRole("button", { name: "Copy account id" });
    // A value in a sentence is selected by hand too: it never carries "Copied".
    expect(button).not.toHaveTextContent("Copied");
    expect(button.className).toContain("press-scale");

    await act(async () => {
      fireEvent.click(button);
    });
    expect(screen.getByText("Copied").className).toContain("tint-in");
    expect(screen.getByText("Copied").querySelector(".bloom")).not.toBeNull();
    expect(screen.getByText("366421618001445894")).toHaveClass("opacity-0");

    act(() => void vi.advanceTimersByTime(COPIED_MS));
    expect(screen.getByText("Copied").className).toContain("tint-out");
    expect(screen.getByText("366421618001445894")).toHaveClass("opacity-100");
    act(() => void vi.advanceTimersByTime(TINT_MS));
    expect(screen.queryByText("Copied")).toBeNull();
  });
});

describe("Checkbox and Radio", () => {
  it("are the selection seat, still real inputs", () => {
    const { container } = render(
      <>
        <Checkbox aria-label="Understood" className="mt-1" />
        <Radio aria-label="Stays" name="ending" />
      </>,
    );
    expect(container.querySelectorAll(".seat")).toHaveLength(2);
    const box = screen.getByRole("checkbox", { name: "Understood" });
    expect(box.closest(".seat")).toHaveClass("mt-1");
    fireEvent.click(box);
    expect(box).toBeChecked();
    // A radio's mark is the dot, a checkbox's is the tick.
    expect(screen.getByRole("radio", { name: "Stays" }).closest(".seat")?.querySelector("circle")).not.toBeNull();
    expect(box.closest(".seat")?.querySelector("path")).not.toBeNull();
  });
});
