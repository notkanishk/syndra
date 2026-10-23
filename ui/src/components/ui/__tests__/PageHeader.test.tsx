// @vitest-environment jsdom
import { act, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { ARRIVE_SETTLED_MS } from "@/components/ui/ArriveList";
import { PageHeader } from "@/components/ui/PageHeader";
import { TINT_MS } from "@/lib/motion";

/**
 * `/users/<id>` paired a `flex-1` (basis 0%) title block against `actions`
 * on the same un-wrapped flex line: the title reported zero size to the
 * wrap decision, so `actions` claimed the line and the title was squeezed
 * into whatever sliver was left — "Sam Patel" wrapped into "Sam" / "Patel"
 * with the rest of the row empty. `flex-auto` (basis auto) makes the title
 * count its real size, so a tight line wraps `actions` below instead.
 */
describe("PageHeader title block sizing", () => {
  it("gives the title its content-based size instead of starving it next to actions", () => {
    render(<PageHeader title="Sam Patel" actions={<button>Grant direct access</button>} />);

    const heading = screen.getByRole("heading", { level: 1, name: "Sam Patel" });
    const titleBlock = heading.parentElement;

    expect(titleBlock?.className).toMatch(/\bflex-auto\b/);
    expect(titleBlock?.className).not.toMatch(/\bflex-1\b/);
  });
});

/** S11: the header tints in, then what follows arrives; once, and only on navigation. */
describe("PageHeader first paint", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    window.history.replaceState({}, "", "/roles");
  });
  afterEach(() => vi.useRealTimers());

  const head = () => screen.getByRole("heading", { level: 1 }).parentElement!.parentElement!;

  it("wears the arrival until its stagger has run, then holds still", () => {
    render(<PageHeader title="Roles" />);
    expect(head()).toHaveClass("page-head");
    expect(head()).not.toHaveAttribute("data-settled");
    act(() => void vi.advanceTimersByTime(TINT_MS + ARRIVE_SETTLED_MS));
    expect(head()).toHaveAttribute("data-settled");
  });

  it("is already settled when the reader came Back to the page", () => {
    act(() => void window.dispatchEvent(new PopStateEvent("popstate")));
    render(<PageHeader title="Roles" />);
    expect(head()).toHaveAttribute("data-settled");
  });
});
