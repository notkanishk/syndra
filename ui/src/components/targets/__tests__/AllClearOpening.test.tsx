// @vitest-environment jsdom
import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { AllClear, useEmptied } from "@/components/targets/AllClear";
import { useOneOpening } from "@/lib/useOneOpening";

// MergeFindings' path: `useEmptied(drawn)` says the list emptied in front of the reader, and the one opening
// of its all-clear is spent by the first mount (or by a read that stops answering). A failed refetch keeps
// react-query's old data, so `drawn` stays 0 through it: the retry's all-clear is the same clearing.
type Props = { drawn: number; ready: boolean };
const Page = ({ drawn, ready }: Props) => {
  const opening = useOneOpening(useEmptied(drawn), ready);
  if (!ready) return <p>The findings could not be read</p>;
  if (drawn > 0) return <p>The list</p>;
  return <AllClear title="Nothing disputed." guidance="All matches." emptied={opening.open} byReader onOpened={opening.spend} />;
};
const dot = (c: HTMLElement) => c.querySelector("[data-dot]");
const lifted = (c: HTMLElement) => c.querySelector("[role=status]")?.classList.contains("lift-in");

describe("the findings' all-clear opens once", () => {
  it("lifts and opens when the last finding is resolved in front of the reader", () => {
    const view = render(<Page drawn={1} ready />);
    view.rerender(<Page drawn={0} ready />);
    expect(dot(view.container)).toHaveClass("ember-open");
    expect(lifted(view.container)).toBe(true);
  });

  it("does not reopen when a read fails and the retry lands: the same all-clear, at rest", () => {
    const view = render(<Page drawn={1} ready />);
    view.rerender(<Page drawn={0} ready />);
    view.rerender(<Page drawn={0} ready={false} />);
    expect(dot(view.container)).toBeNull();
    view.rerender(<Page drawn={0} ready />);
    expect(dot(view.container)).toHaveClass("ember");
    expect(dot(view.container)).not.toHaveClass("ember-open");
    expect(lifted(view.container)).toBe(false);
  });

  it("does not replay on a re-render, and a list that was empty on arrival is simply there", () => {
    const view = render(<Page drawn={1} ready />);
    view.rerender(<Page drawn={0} ready />);
    const first = dot(view.container);
    view.rerender(<Page drawn={0} ready />);
    expect(dot(view.container)).toBe(first);
    expect(first).toHaveClass("ember-open");
    const arrived = render(<Page drawn={0} ready />);
    expect(dot(arrived.container)).not.toHaveClass("ember-open");
    expect(lifted(arrived.container)).toBe(false);
  });
});
