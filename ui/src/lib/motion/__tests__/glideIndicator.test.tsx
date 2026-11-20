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
 * One element, round at both ends at every point of the glide: placed by
 * transform, sized by width and height, never scaled. A pill drawn in parts
 * showed two lines where the parts overlapped under a translucent fill
 * (staging, 20 Nov 2026), so it has no parts at all.
 */
describe("the glide indicator's shape", () => {
  it("draws a fill as one round element, translated and sized, with no parts", () => {
    const { container } = render(<GlideIndicator box={{ left: 10, top: 4, width: 100, height: 30 }} kind="fill" className="bg-accent" />);
    expect(container.querySelectorAll("*")).toHaveLength(1);
    const fill = container.querySelector<HTMLElement>("[data-indicator]")!;
    expect(fill.children).toHaveLength(0);
    expect(fill).toHaveClass("rounded-pill", "bg-accent");
    expect(fill.style.transform).toBe("translate(10px, 4px)");
    expect(fill.style.transform).not.toMatch(/scale/);
    expect([fill.style.width, fill.style.height, fill.style.left, fill.style.top]).toEqual(["100px", "30px", "", ""]);
  });

  it("draws an underline 2px tall at the bottom of the active segment", () => {
    const { container } = render(<GlideIndicator box={{ left: 10, top: 4, width: 100, height: 30 }} kind="underline" />);
    const line = container.querySelector<HTMLElement>("[data-indicator]")!;
    expect(line.children).toHaveLength(0);
    expect(line.style.transform).toBe("translate(10px, 33px)");
    expect([line.style.width, line.style.height]).toEqual(["100px", "2px"]);
  });
});
