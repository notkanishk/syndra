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
