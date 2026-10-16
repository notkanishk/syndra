// @vitest-environment jsdom
import { act, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { ListStates, SKELETON_DELAY_MS } from "@/components/states";

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
