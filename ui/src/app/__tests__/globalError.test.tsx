// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import GlobalError from "@/app/global-error";

/**
 * The boundary for a throw in the root layout itself, where `error.tsx` cannot
 * reach: that throw unmounts the layout, so the boundary living inside it never
 * renders and Next falls back to its own unstyled page.
 *
 * Rendered into a container rather than asserted whole — the component returns
 * `<html><body>`, which React will happily mount inside a div for the purpose
 * of reading what it says.
 */
function renderBoundary(digest?: string) {
  const noise = vi.spyOn(console, "error").mockImplementation(() => {});
  const error = Object.assign(new Error("provider exploded"), { digest });
  render(<GlobalError error={error} />);
  noise.mockRestore();
}

describe("the last-resort boundary", () => {
  it("says nothing was changed, because nothing had started", () => {
    renderBoundary();

    expect(screen.getByRole("heading").textContent).toMatch(/stopped before it could draw/);
    expect(screen.getByText(/Nothing was\s+changed/)).toBeTruthy();
  });

  it("shows the digest, the only handle that survives into production", () => {
    renderBoundary("a1b2c3d4");
    expect(screen.getByText(/a1b2c3d4/)).toBeTruthy();
  });

  it("omits the id line rather than printing an empty one", () => {
    renderBoundary();
    expect(screen.queryByText(/Error id/)).toBeNull();
  });

  // A full document load, not a soft navigation: if what threw was the layout
  // or a provider, a client-side route change re-renders it and throws again.
  it("offers a way out that does not re-enter the broken tree", () => {
    renderBoundary();

    const out = screen.getByRole("link", { name: "Reload from the start" });
    expect(out.getAttribute("href")).toBe("/");
    // Not a Next Link: a plain anchor is what forces the document load.
    expect(out.tagName).toBe("A");
  });

  it("clears the touch floor on the only control it has", () => {
    renderBoundary();
    expect(screen.getByRole("link", { name: "Reload from the start" }).style.minHeight).toBe(
      "44px",
    );
  });
});
