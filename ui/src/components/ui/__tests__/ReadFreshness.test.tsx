// @vitest-environment jsdom
import { act, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { ApiError } from "@/lib/api-client";
import {
  blocksIrreversibleAction,
  classifyRead,
  ReadAgainError,
  ReadFreshness,
} from "@/components/ui/ReadFreshness";

/**
 * §31 A — one answer to "how old is this", used everywhere state is read from a
 * target.
 *
 * The rules under test are the ones that were previously decided three separate
 * times on three screens two clicks apart.
 */

// Real wall-clock, because the rendered strip reads the real clock: a fixed
// NOW would test the pure classifier twice and the component not at all.
const NOW = Date.now();
const ago = (minutes: number) => new Date(NOW - minutes * 60_000).toISOString();

describe("classifying a read", () => {
  it("calls a read from the last minute live", () => {
    expect(classifyRead({ readAt: ago(0.5), current: true }, NOW)).toBe("live");
  });

  it("calls a read from a few minutes ago ageing, not stale", () => {
    expect(classifyRead({ readAt: ago(4), current: true }, NOW)).toBe("ageing");
  });

  it("calls a read past the window stale", () => {
    expect(classifyRead({ readAt: ago(11), current: true }, NOW)).toBe("stale");
  });

  // A four-second-old copy of state we could not reach is still a copy. Ageing
  // it into `live` is the one mistake this vocabulary exists to prevent.
  it("calls a mirror provisional however fresh the copy is", () => {
    expect(classifyRead({ readAt: ago(0.1), current: false }, NOW)).toBe("provisional");
  });

  it("treats a read that never happened as stale rather than live", () => {
    expect(classifyRead({}, NOW)).toBe("stale");
  });
});

/**
 * The split §31 A is explicit about: the rule is what the ACTION does, not how
 * old the read is. Adoption binds an identity irreversibly off a list that may
 * have moved; applying a plan joins a queue an operator can still inspect.
 */
describe("what a stale read blocks", () => {
  it("blocks an irreversible action once the read is stale", () => {
    expect(blocksIrreversibleAction({ readAt: ago(11), current: true }, NOW)).toBe(true);
  });

  it("blocks it on a mirror read at any age", () => {
    expect(blocksIrreversibleAction({ readAt: ago(1), current: false }, NOW)).toBe(true);
  });

  it("allows it while the read is merely ageing", () => {
    expect(blocksIrreversibleAction({ readAt: ago(4), current: true }, NOW)).toBe(false);
  });
});

describe("the strip", () => {
  it("always carries an age, never a word alone", () => {
    render(<ReadFreshness state={{ readAt: ago(4), current: true }} subject="The account list" />);
    // "recently" is not something an operator can act on: the number is the
    // whole content of the label.
    expect(screen.getByText(/4 min ago/)).toBeInTheDocument();
  });

  it("says a provisional read is the last state seen, with its age", () => {
    render(<ReadFreshness state={{ readAt: ago(14), current: false }} />);
    expect(screen.getByText(/last state seen/i)).toBeInTheDocument();
    expect(screen.getByText(/14 min ago/)).toBeInTheDocument();
  });

  // Truncation is orthogonal — a complete statement about what was seen and a
  // silence about the rest — so it rides alongside the age rather than
  // replacing it.
  it("reports truncation beside the age rather than instead of it", () => {
    render(<ReadFreshness state={{ readAt: ago(2), current: true, truncated: true }} />);
    expect(screen.getByText(/2 min ago/)).toBeInTheDocument();
    expect(screen.getByText(/not the whole list/i)).toBeInTheDocument();
  });

  it("offers a re-read only once the read is too old to act on", () => {
    const { rerender } = render(
      <ReadFreshness state={{ readAt: ago(2), current: true }} onRefresh={() => {}} />,
    );
    expect(screen.queryByRole("button", { name: /read again/i })).toBeNull();

    rerender(<ReadFreshness state={{ readAt: ago(20), current: true }} onRefresh={() => {}} />);
    expect(screen.getByRole("button", { name: /read again/i })).toBeInTheDocument();
  });

  // A-08 §5: the slot stays zero tall; the pill drawn is the line and a hair, and
  // the 44px thumb target is an unpainted box on touch only.
  it("draws a short pill in a zero-height slot, with the touch target unpainted", () => {
    render(<ReadFreshness state={{ readAt: ago(20), current: true }} onRefresh={() => {}} />);
    const button = screen.getByRole("button", { name: /read again/i });
    expect(button.parentElement?.parentElement).toHaveClass("desktop:h-0");
    // Beats the Button's 44px floor and its desktop padding.
    expect(button).toHaveClass("min-h-0!", "py-px!");
    // The target: a 44px `before` box, centred on the line, gone on desktop.
    expect(button).toHaveClass("before:h-[var(--tap)]", "before:top-1/2", "desktop:before:hidden");
    expect(button.className).not.toMatch(/before:(bg|border)/);
  });
});

describe("how a copy is drawn", () => {
  const dot = () => document.querySelector("[data-freshness-dot]") as HTMLElement;
  const line = (text: RegExp) => screen.getByText(text);

  it("a copy the add-on kept is neutral, at any age and even when it is not current", () => {
    for (const readAt of [ago(0.2), ago(30)]) {
      const { unmount } = render(<ReadFreshness state={{ readAt, current: false, stale: { reason: "cached" } }} subject="The list" />);
      expect(dot().className).toContain("bg-faint");
      expect(dot().className).not.toMatch(/warn|healthy/);
      expect(line(/The list was read/).className).toContain("text-faint");
      expect(line(/The list was read/).className).not.toContain("warn");
      unmount();
    }
  });

  it("a cooldown carried in the data is the reader's own press, so it is amber, and so does a system that cannot be reached", () => {
    const { unmount } = render(<ReadFreshness state={{ readAt: ago(0.2), current: true, stale: { reason: "cooldown", retry_after: 30 } }} subject="The list" />);
    expect(dot().className).toContain("bg-warn");
    expect(line(/you can read again in/).className).toContain("text-warn-text");
    unmount();
    render(<ReadFreshness state={{ readAt: ago(1), current: false, stale: { reason: "unreachable" } }} subject="The list" />);
    expect(dot().className).toContain("border-warn");
    expect(line(/can.t be reached/).className).toContain("text-warn-text");
  });

  // A page load inside somebody else's wait is not the reader's doing, so it is not a warning: only the load sites say so.
  it("a cooldown marked as a page load's is neutral, like a copy", () => {
    render(<ReadFreshness state={{ readAt: ago(0.2), current: true, stale: { reason: "cooldown", retry_after: 30, pressed: false } }} subject="The list" />);
    expect(dot().className).toContain("bg-faint");
    expect(dot().className).not.toMatch(/warn/);
    expect(line(/you can read again in/).className).toContain("text-faint");
  });

  describe("once the count is over", () => {
    afterEach(() => vi.useRealTimers());
    const over = (current: boolean) => {
      vi.useFakeTimers();
      render(<ReadFreshness state={{ readAt: ago(0.2), current, stale: { reason: "cooldown", retry_after: 2 } }} subject="The list" onRefresh={() => {}} />);
      expect(dot().className).toContain("bg-warn");
      act(() => void vi.advanceTimersByTime(3000));
      expect(screen.queryByText(/you can read again in/)).toBeNull();
    };

    it("the colour follows what the line now says: a copy that is not current is neutral", () => {
      over(false);
      expect(dot().className).toContain("bg-faint");
      expect(line(/The list was read/).className).toContain("text-faint");
    });

    it("and a current read is drawn by its age, not amber", () => {
      over(true);
      expect(dot().className).toContain("bg-healthy");
      expect(dot().className).not.toContain("warn");
    });
  });

  it("never reads a since of 0 as the epoch: that is a query that has not answered", () => {
    render(<ReadFreshness state={{ readAt: ago(0.2), stale: { reason: "cooldown", retry_after: 10, since: 0 } }} subject="The list" onRefresh={() => {}} />);
    expect(screen.getByText(/you can read again in 10 s/)).toBeInTheDocument();
  });

  it("counts a cooldown from when it arrived, not from when it was drawn", () => {
    render(<ReadFreshness state={{ readAt: ago(0.2), stale: { reason: "cooldown", retry_after: 10, since: Date.now() - 4000 } }} subject="The list" onRefresh={() => {}} />);
    expect(screen.getByText(/you can read again in [56] s/)).toBeInTheDocument();
  });

  // At 390px the sentence wraps and the button drops to its own row: a zero-tall slot let its edge
  // cut the line above. The slot is a thumb tall on touch and zero only where it sits beside the line.
  it("gives Read again a real-height slot on touch, and a zero-tall one only on desktop", () => {
    render(<ReadFreshness state={{ readAt: ago(0.2), stale: { reason: "cached" } }} subject="The list" onRefresh={() => {}} />);
    const slot = screen.getByRole("button", { name: "Read again" }).parentElement?.parentElement as HTMLElement;
    expect(slot.className).toContain("h-[var(--tap)]");
    expect(slot.className).toContain("desktop:h-0");
    expect(slot.className.split(/\s+/)).not.toContain("h-0");
  });
});

describe("a Read again that failed", () => {
  it("says so in the outcome every refusal wears, and leaves the cooldown to the line", () => {
    const { container, rerender } = render(<ReadAgainError error={new ApiError(503, { error: "TARGET_UNREACHABLE", message: "TrueNAS did not answer" })} />);
    expect(screen.getByText("Not read")).toBeInTheDocument();
    rerender(<ReadAgainError error={new ApiError(429, { error: "FRESH_READ_COOLDOWN", retry_after: 5 })} />);
    expect(container).toBeEmptyDOMElement();
    rerender(<ReadAgainError error={null} />);
    expect(container).toBeEmptyDOMElement();
  });
});
