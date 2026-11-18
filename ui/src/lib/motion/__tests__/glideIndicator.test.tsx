// @vitest-environment jsdom
import { render } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { GlideIndicator, useGlideIndicator } from "@/lib/motion";

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
      <GlideIndicator box={{ left: 0, top: 0, width: 10, height: 10 }} kind="fill" />
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

/**
 * Transform only, and round ends at every point of the glide: the caps are
 * translated and never scaled; only the square-ended middle is scaled, along x.
 */
describe("the glide indicator's shape", () => {
  const parts = (el: Element) => [...el.children] as HTMLElement[];

  it("places a fill by translating it, as two round caps and a middle tucked under each", () => {
    const { container } = render(<GlideIndicator box={{ left: 10, top: 4, width: 100, height: 30 }} kind="fill" className="bg-accent" />);
    const fill = container.querySelector<HTMLElement>("[data-indicator]")!;
    expect(fill.style.transform).toBe("translate(10px, 4px)");
    expect(fill.style.left + fill.style.top + fill.style.width).toBe("");
    const [start, middle, end] = parts(fill);
    expect(start).toHaveClass("rounded-l-pill", "bg-accent");
    expect([start.style.width, start.style.height, start.style.transform]).toEqual(["15px", "30px", ""]);
    // The middle runs 0.5px under each cap, so no seam shows where they meet.
    expect(middle.style.transform).toBe("translateX(14.5px) scaleX(71)");
    expect(middle.className).not.toMatch(/rounded/);
    expect(end).toHaveClass("rounded-r-pill", "bg-accent");
    expect([end.style.width, end.style.height, end.style.transform]).toEqual(["15px", "30px", "translateX(85px)"]);
  });

  it("draws an underline 2px tall at the bottom of the active segment", () => {
    const { container } = render(<GlideIndicator box={{ left: 10, top: 4, width: 100, height: 30 }} kind="underline" />);
    const line = container.querySelector<HTMLElement>("[data-indicator]")!;
    expect(line.style.transform).toBe("translate(10px, 33px)");
    expect(parts(line).map((p) => p.style.height)).toEqual(["2px", "2px", "2px"]);
    expect(parts(line)[1].style.transform).toBe("translateX(0.5px) scaleX(99)");
  });
});
