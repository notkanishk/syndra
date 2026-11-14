// @vitest-environment jsdom
import { act, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { RetryWait } from "@/components/ui/ReadFreshness";
import { useRetryOnce } from "@/lib/queries/fresh";

/**
 * Focus around a press that waits out its cooldown (useRetryOnce, RetryWait): Stop waiting takes
 * focus only from someone still on the button they pressed, and gives it back after.
 */
let retry: ReturnType<typeof useRetryOnce>;
function Card() {
  retry = useRetryOnce();
  return (
    <div>
      <button disabled={retry.waiting} onClick={() => retry.press(() => {})}>
        Check now
      </button>
      <input aria-label="API key" />
      <RetryWait name="Truenas" retry={retry} onStop={retry.stop} />
    </div>
  );
}

beforeEach(() => vi.useFakeTimers());
afterEach(() => vi.useRealTimers());

describe("focus around a press that waits", () => {
  it("is not taken from someone who moved on before the refusal came back", () => {
    render(<Card />);
    const pressed = screen.getByRole("button", { name: "Check now" });
    pressed.focus();
    act(() => pressed.click());
    const key = screen.getByLabelText("API key");
    key.focus();
    act(() => void retry.refused(5, () => {}));
    expect(screen.getByRole("button", { name: "Stop waiting" })).not.toHaveFocus();
    expect(key).toHaveFocus();
  });

  it("moves to Stop waiting from the pressed button, and goes back to it after Stop waiting", () => {
    render(<Card />);
    const pressed = screen.getByRole("button", { name: "Check now" });
    pressed.focus();
    act(() => pressed.click());
    act(() => void retry.refused(5, () => {}));
    const stop = screen.getByRole("button", { name: "Stop waiting" });
    expect(stop).toHaveFocus();
    act(() => stop.click());
    expect(pressed).toHaveFocus();
  });
});
