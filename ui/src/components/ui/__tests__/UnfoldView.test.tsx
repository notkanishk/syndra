// @vitest-environment jsdom
import { act, fireEvent, render, screen } from "@testing-library/react";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { notePage } from "@/components/ui/ArriveList";
import { Notice } from "@/components/ui/Notice";
import { Tinted, Unfold } from "@/components/ui/Unfold";
import { setMediaQuery } from "@/test-utils/media";
import { motionInTests } from "@/test-utils/motion";

/** A-08 §3 and §5: parts that come and go with the view, and a Notice that can leave. */

const animationEnd = (el: Element) => act(() => void fireEvent(el, new Event("webkitAnimationEnd", { bubbles: true })));

beforeEach(() => {
  Object.defineProperty(Element.prototype, "getAnimations", { value: () => [], configurable: true });
});
afterEach(() => {
  delete (Element.prototype as { getAnimations?: unknown }).getAnimations;
});

describe("closing an Unfold (A-08 §3)", () => {
  it("folds the gap in 200ms, which is the settle less half a tint, behind the fade", () => {
    const css = readFileSync(resolve(__dirname, "../../../app/globals.css"), "utf8");
    const unfold = css.slice(css.indexOf("@utility unfold"), css.indexOf("@utility close-gap"));
    expect(unfold).toMatch(
      /\[data-phase="closing"\] \{[^}]*animation: fold calc\(var\(--duration-settle\) - var\(--duration-tint\) \/ 2\) var\(--ease-settle\)\s+var\(--duration-tint\) both/,
    );
    // The row that left on a later read is a different move, and keeps the settle.
    expect(css.slice(css.indexOf("@utility close-gap"))).toContain(
      "animation: fold var(--duration-settle) var(--ease-settle) var(--duration-tint) both",
    );
  });

  it("opens and closes by phase, and is gone once the fold has run", () => {
    const { rerender } = render(<Unfold open={false}>part</Unfold>);
    expect(screen.queryByText("part")).toBeNull();

    rerender(<Unfold open>part</Unfold>);
    const region = screen.getByText("part").closest(".unfold")!;
    expect(region).toHaveAttribute("data-phase", "opening");
    // The height settling is not the end: the contents' fade runs one tint past it.
    animationEnd(region);
    expect(region).toHaveAttribute("data-phase", "opening");
    animationEnd(region.firstElementChild!);
    expect(region).toHaveAttribute("data-phase", "open");

    rerender(<Unfold open={false}>part</Unfold>);
    expect(region).toHaveAttribute("data-phase", "closing");
    expect(screen.getByText("part")).toBeInTheDocument();
    animationEnd(region);
    expect(screen.queryByText("part")).toBeNull();
  });

  it("is over after its own length when no animation ends to say so (a rail hidden on a phone)", () => {
    vi.useFakeTimers();
    try {
      const { rerender } = render(<Unfold open={false}>part</Unfold>);
      rerender(<Unfold open>part</Unfold>);
      const region = screen.getByText("part").closest(".unfold")!;
      act(() => void vi.advanceTimersByTime(500));
      expect(region).toHaveAttribute("data-phase", "open");
      rerender(<Unfold open={false}>part</Unfold>);
      act(() => void vi.advanceTimersByTime(500));
      expect(screen.queryByText("part")).toBeNull();
    } finally {
      vi.useRealTimers();
    }
  });

  it("is simply there when it is open at first paint", () => {
    render(<Unfold open>part</Unfold>);
    expect(screen.getByText("part").closest(".unfold")).toHaveAttribute("data-phase", "open");
  });

  it("appears and goes at once for a reader who asked for less motion", () => {
    setMediaQuery("(prefers-reduced-motion: reduce)", true);
    const { rerender } = render(<Unfold open={false}>part</Unfold>);
    rerender(<Unfold open>part</Unfold>);
    expect(screen.getByText("part").closest(".unfold")).toHaveAttribute("data-phase", "open");
    rerender(<Unfold open={false}>part</Unfold>);
    expect(screen.queryByText("part")).toBeNull();
  });

  it("carries the gap beside it inside what folds, and hands it back by margin", () => {
    render(
      <Unfold open gap={18}>
        part
      </Unfold>,
    );
    const region = screen.getByText("part").closest(".unfold") as HTMLElement;
    expect(region.style.marginBottom).toBe("-18px");
    expect(screen.getByText("part")).toHaveStyle({ paddingBottom: "18px" });
  });
});

describe("an inline bit of an Advanced line", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    motionInTests();
  });
  afterEach(() => vi.useRealTimers());

  it("is simply there at first paint, fades before it goes, and tints in when the view reaches it", () => {
    const { rerender } = render(<Tinted show>id-1</Tinted>);
    expect(screen.getByText("id-1")).not.toHaveClass("tint-in");

    rerender(<Tinted show={false}>id-1</Tinted>);
    expect(screen.getByText("id-1")).toHaveClass("tint-out", "pointer-events-none");
    act(() => void vi.advanceTimersByTime(121));
    expect(screen.queryByText("id-1")).toBeNull();

    rerender(<Tinted show>id-1</Tinted>);
    expect(screen.getByText("id-1")).toHaveClass("tint-in");
  });
});

describe("a Notice that can leave (A-08 §5)", () => {
  it("opens and folds by the Unfold around it, and is not opened twice", () => {
    const notice = (open: boolean) => (
      <Unfold open={open}>
        <Notice tone="warn" title="Stale">
          Nothing changed.
        </Notice>
      </Unfold>
    );
    const { rerender } = render(notice(false));
    expect(screen.queryByRole("status")).toBeNull();

    rerender(notice(true));
    const box = screen.getByRole("status");
    expect(document.querySelectorAll(".unfold")).toHaveLength(1);
    expect(box.className).not.toContain("tint-in");

    rerender(notice(false));
    expect(screen.getByRole("status").closest(".unfold")).toHaveAttribute("data-phase", "closing");
    animationEnd(box.closest(".unfold")!);
    expect(screen.queryByRole("status")).toBeNull();
  });
});

describe("a Notice and its page's first paint (A-08 §5)", () => {
  // Past the app's first paint, so a Notice can be late at all.
  beforeEach(() => new Promise((resolve) => setTimeout(resolve, 80)));

  it("opens itself when it mounts inside a region that has long been open", () => {
    const block = (stale: boolean) => (
      <Unfold open>
        {stale && (
          <Notice tone="warn" title="Stale">
            Nothing changed.
          </Notice>
        )}
      </Unfold>
    );
    const { rerender } = render(block(false));
    rerender(block(true));
    // The block is open; the Notice's own space opens inside it.
    expect(screen.getByRole("status").closest(".unfold")?.parentElement?.closest(".unfold")).not.toBeNull();
  });

  // Last in the file: it notes a page, which the tests above must not see.
  it("arrives with a page reached by a link: it is part of that page's first paint", () => {
    notePage("/reached-by-a-link");
    render(
      <Notice tone="warn" title="Stale">
        Nothing changed.
      </Notice>,
    );
    expect(screen.getByRole("status").closest(".unfold")).toBeNull();
  });
});
