// @vitest-environment jsdom
import { act, render } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { Segmented } from "@/components/ui/Select";
import { GlideIndicator, SETTLE_MS, TINT_MS, Unfold, useGlideIndicator } from "@/lib/motion";
import { motionInTests } from "@/test-utils/motion";

/** The fill is measured from the active segment, so it must re-measure when any segment resizes. */

const observed: Element[] = [];

beforeEach(() => {
  observed.length = 0;
  vi.stubGlobal(
    "ResizeObserver",
    class {
      observe(el: Element) {
        observed.push(el);
      }
      disconnect() {}
    },
  );
});
afterEach(() => vi.unstubAllGlobals());

function Group({ tag }: { tag: "a" | "button" }) {
  const { ref, box } = useGlideIndicator("one");
  const Segment = tag;
  return (
    <div ref={ref} data-testid="group" className="relative">
      <GlideIndicator box={{ left: 0, top: 0, width: 10, height: 10, radius: "0px" }} kind="fill" />
      <Segment aria-current="location" href={tag === "a" ? "#one" : undefined}>
        One {box ? "measured" : ""}
      </Segment>
      <Segment href={tag === "a" ? "#two" : undefined}>Two</Segment>
    </div>
  );
}

describe("the glide indicator's observer", () => {
  it.each(["a", "button"] as const)("watches the group and each %s segment, but never its own fill", (tag) => {
    const { getByTestId } = render(<Group tag={tag} />);
    expect(observed).toContain(getByTestId("group"));
    expect(observed.filter((el) => el.tagName.toLowerCase() === tag)).toHaveLength(2);
    expect(observed.some((el) => el.hasAttribute("data-indicator"))).toBe(false);
  });
});

function Shaped({ corner }: { corner?: string }) {
  const { ref, box } = useGlideIndicator("one");
  return (
    <div ref={ref} className="relative">
      <GlideIndicator box={box} kind="fill" />
      <button type="button" aria-current="location" style={corner ? { borderTopLeftRadius: corner } : undefined}>
        One
      </button>
    </div>
  );
}

describe("the measured corners", () => {
  it("takes the option's own corner, and a pill's where none is read", () => {
    const fill = (corner?: string) => render(<Shaped corner={corner} />).container.querySelector<HTMLElement>("[data-indicator]")!;
    expect(fill("12px").style.borderRadius).toBe("12px");
    // An engine that reads no corner at all (as the shorthand reads "" in some).
    const read = vi.spyOn(window, "getComputedStyle").mockReturnValue({ borderTopLeftRadius: "" } as CSSStyleDeclaration);
    try {
      expect(fill().style.borderRadius).toBe("var(--radius-pill)");
    } finally {
      read.mockRestore();
    }
  });
});

describe("a fold around the whole group", () => {
  it("does not hide the choice: a Segmented in an opening fold has its box, and keeps it once the fold is open", () => {
    // jsdom runs no animations; give Unfold an engine that does, so the fold really opens on its timer.
    const engine = Element.prototype as unknown as { getAnimations?: () => Animation[] };
    const had = "getAnimations" in engine;
    engine.getAnimations = () => [];
    motionInTests();
    vi.useFakeTimers();
    try {
      const shape = (open: boolean) => (
        <Unfold open={open}>
          <Segmented label="Shape" options={[{ value: "one", label: "One" }, { value: "two", label: "Two" }]} value="one" onChange={() => {}} />
        </Unfold>
      );
      const view = render(shape(false));
      view.rerender(shape(true));
      expect(view.container.querySelector('.unfold[data-phase="opening"]')).not.toBeNull();
      act(() => {
        vi.advanceTimersByTime(TINT_MS + SETTLE_MS + 100);
      });
      expect(view.container.querySelector('.unfold[data-phase="open"]')).not.toBeNull();
      expect(view.container.querySelector('[data-indicator="fill"]')).not.toBeNull();
    } finally {
      vi.useRealTimers();
      if (!had) delete engine.getAnimations;
    }
  });
});

/**
 * One element, round at both ends at every point of the glide: placed by
 * transform, sized by width and height, never scaled. A pill drawn in parts
 * showed two lines where the parts overlapped under a translucent fill
 * (staging, 20 Nov 2026), so it has no parts at all.
 */
describe("the glide indicator's shape", () => {
  it("draws a fill as one element shaped as the option (its own corners), translated and sized, with no parts", () => {
    const { container } = render(<GlideIndicator box={{ left: 10, top: 4, width: 100, height: 30, radius: "999px" }} kind="fill" className="bg-accent" />);
    expect(container.querySelectorAll("*")).toHaveLength(1);
    const fill = container.querySelector<HTMLElement>("[data-indicator]")!;
    expect(fill.children).toHaveLength(0);
    expect(fill).toHaveClass("bg-accent");
    // The option's own corners: a pill's round ends here, a rail row's 12px there (A-25 moment 4).
    expect(fill.style.borderRadius).toBe("999px");
    expect(fill.style.transform).toBe("translate(10px, 4px)");
    expect(fill.style.transform).not.toMatch(/scale/);
    expect([fill.style.width, fill.style.height, fill.style.left, fill.style.top]).toEqual(["100px", "30px", "", ""]);
  });

  it("draws an underline 2px tall at the bottom of the active segment", () => {
    const { container } = render(<GlideIndicator box={{ left: 10, top: 4, width: 100, height: 30, radius: "0px" }} kind="underline" />);
    const line = container.querySelector<HTMLElement>("[data-indicator]")!;
    expect(line.children).toHaveLength(0);
    expect(line.style.transform).toBe("translate(10px, 33px)");
    expect([line.style.width, line.style.height]).toEqual(["100px", "2px"]);
  });
});
