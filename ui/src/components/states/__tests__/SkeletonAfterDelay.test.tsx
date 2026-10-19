// @vitest-environment jsdom
import { act, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { ListStates, SKELETON_DELAY_MS } from "@/components/states";
import { setMediaQuery } from "@/test-utils/media";
import { motionInTests } from "@/test-utils/motion";

function loading(isLoading: boolean) {
  return (
    <ListStates
      isLoading={isLoading}
      error={null}
      isEmpty={false}
      errorTitle="People couldn't be loaded."
      empty={<p>Nobody yet</p>}
    >
      <p>Aanya Rao</p>
    </ListStates>
  );
}

describe("a skeleton shows only for a read that is slow (A-05 S7)", () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => vi.useRealTimers());

  it("shows nothing for the first 200ms, then the skeleton", () => {
    render(loading(true));
    expect(screen.queryByLabelText("Loading")).not.toBeInTheDocument();
    act(() => {
      vi.advanceTimersByTime(SKELETON_DELAY_MS);
    });
    expect(screen.getByLabelText("Loading")).toBeInTheDocument();
  });

  it("a read that lands inside 200ms never shows one", () => {
    const { rerender } = render(loading(true));
    act(() => {
      vi.advanceTimersByTime(SKELETON_DELAY_MS - 1);
    });
    rerender(loading(false));
    act(() => {
      vi.advanceTimersByTime(SKELETON_DELAY_MS);
    });
    expect(screen.queryByLabelText("Loading")).not.toBeInTheDocument();
    expect(screen.getByText("Aanya Rao")).toBeInTheDocument();
  });
});

describe("a skeleton hands over to its rows (A-05 S7)", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    motionInTests();
  });
  afterEach(() => vi.useRealTimers());

  it("fades first on tint, then the rows arrive: they do not crossfade", () => {
    const { rerender } = render(loading(true));
    act(() => void vi.advanceTimersByTime(SKELETON_DELAY_MS));
    rerender(loading(false));

    const skeleton = screen.getByLabelText("Loading");
    expect(skeleton.parentElement).toHaveClass("tint-out");
    expect(screen.queryByText("Aanya Rao")).not.toBeInTheDocument();

    act(() => void vi.advanceTimersByTime(119));
    expect(screen.queryByText("Aanya Rao")).not.toBeInTheDocument();
    act(() => void vi.advanceTimersByTime(2));
    expect(screen.queryByLabelText("Loading")).not.toBeInTheDocument();
    expect(screen.getByText("Aanya Rao")).toBeInTheDocument();
  });

  it("a read that never showed a skeleton has nothing to fade: its rows are there at once", () => {
    const { rerender } = render(loading(true));
    act(() => void vi.advanceTimersByTime(SKELETON_DELAY_MS - 1));
    rerender(loading(false));
    expect(screen.getByText("Aanya Rao")).toBeInTheDocument();
  });

  it("in reduced motion the skeleton and the rows swap at once", () => {
    setMediaQuery("(prefers-reduced-motion: reduce)", true);
    const { rerender } = render(loading(true));
    act(() => void vi.advanceTimersByTime(SKELETON_DELAY_MS));
    rerender(loading(false));
    expect(screen.queryByLabelText("Loading")).not.toBeInTheDocument();
    expect(screen.getByText("Aanya Rao")).toBeInTheDocument();
  });

  it("a failed read hands over the same way, to its Notice", () => {
    const { rerender } = render(loading(true));
    act(() => void vi.advanceTimersByTime(SKELETON_DELAY_MS));
    rerender(
      <ListStates isLoading={false} error={new Error("boom")} isEmpty={false} errorTitle="People couldn't be loaded." empty={null}>
        <p>Aanya Rao</p>
      </ListStates>,
    );
    expect(screen.queryByText("People couldn't be loaded.")).not.toBeInTheDocument();
    act(() => void vi.advanceTimersByTime(121));
    expect(screen.getByText("People couldn't be loaded.")).toBeInTheDocument();
  });
});
