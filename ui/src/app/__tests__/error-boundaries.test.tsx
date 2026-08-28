// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import AppError from "@/app/error";
import NotFound from "@/app/not-found";

afterEach(() => vi.restoreAllMocks());

function renderError(digest?: string) {
  vi.spyOn(console, "error").mockImplementation(() => {});
  const error = Object.assign(new Error("boom"), digest ? { digest } : {});
  render(<AppError error={error} />);
}

/**
 * There was nothing here at all: a render that threw produced a blank screen
 * with no identifier and no way back, which on a phone is indistinguishable
 * from the app being gone.
 */
describe("the render-error boundary", () => {
  it("says nothing was changed, because nothing was", () => {
    renderError();
    expect(screen.getByText(/Nothing was changed/)).toBeTruthy();
  });

  // Next hands this boundary a reset() and the convention is a "Try again"
  // button. A render that threw on the data it was given throws again on the
  // same data, and a button that repeats a failure while looking like a
  // remedy is worse than no button.
  it("does not offer to try again", () => {
    renderError();
    expect(screen.queryByRole("button", { name: /try again|retry/i })).toBeNull();
  });

  it("offers the way out", () => {
    renderError();
    expect(screen.getByRole("link", { name: /Go to the home page/ }).getAttribute("href")).toBe("/");
  });

  // The digest is the only handle that survives into production, where the
  // message is stripped — and copying it beats transcribing a hash by eye.
  it("carries the identifier when there is one, and says nothing when there is not", () => {
    renderError("a1b2c3");
    expect(screen.getByText("a1b2c3")).toBeTruthy();

    vi.restoreAllMocks();
    renderError();
    expect(screen.queryAllByText(/Reference/)).toHaveLength(1);
  });
});

describe("the not-found boundary", () => {
  // Two different 404 sentences were asked for. This boundary only ever sees
  // one of the two cases, so it says that one rather than hedging.
  it("says the address is wrong, not that something was removed", () => {
    render(<NotFound />);
    expect(screen.getByText(/nothing at this address/i)).toBeTruthy();
    expect(screen.getByRole("link", { name: /Go to the home page/ })).toBeTruthy();
  });
});
