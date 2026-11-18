// @vitest-environment jsdom
import { act, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { ARRIVE_SETTLED_MS } from "@/lib/motion";
import { BELOW_HEADER, CONTENT_GAP, HEADER_TO_CONTENT, PageHeader } from "@/components/ui/PageHeader";
import { TINT_MS } from "@/lib/motion";

/**
 * `/users/<id>` paired a `flex-1` (basis 0%) title block against `actions`
 * on the same un-wrapped flex line: the title reported zero size to the
 * wrap decision, so `actions` claimed the line and the title was squeezed
 * into whatever sliver was left — "Rey Mysterio" wrapped into "Rey" / "Mysterio"
 * with the rest of the row empty. `flex-auto` (basis auto) makes the title
 * count its real size, so a tight line wraps `actions` below instead.
 */
describe("PageHeader title block sizing", () => {
  it("gives the title its content-based size instead of starving it next to actions", () => {
    render(<PageHeader title="Rey Mysterio" actions={<button>Give a role</button>} />);

    const heading = screen.getByRole("heading", { level: 1, name: "Rey Mysterio" });
    const titleBlock = heading.parentElement;

    expect(titleBlock?.className).toMatch(/\bflex-auto\b/);
    expect(titleBlock?.className).not.toMatch(/\bflex-1\b/);
  });

  /**
   * D-1's seat: the actions sit at the right end of the header's own width,
   * never just after the text. On `/users/<id>` (staging, 1481px) the
   * caller's `w-full flex-1` landed on the inner row, so the root beside the
   * avatar was only as wide as "Holds no roles." and the buttons sat at x≈686.
   */
  it("sizes its root with the caller's className and ends the row with the actions, after a growing text block", () => {
    const { container } = render(
      <div className="flex">
        <PageHeader className="w-full flex-1" title="Rey Mysterio" lede="Holds no roles." actions={<button>Give a role</button>} />
      </div>,
    );
    const root = container.firstElementChild!.firstElementChild!;
    expect(root.className).toMatch(/\bflex-1\b/);
    expect(root.className).toMatch(/\bw-full\b/);

    const row = screen.getByRole("heading", { level: 1 }).parentElement!.parentElement!;
    expect(row.parentElement).toBe(root);
    // The row is the root's own width (a block child, nothing narrowing it).
    expect(row.className).not.toMatch(/\b(w-|max-w-|inline-flex|flex-none|self-start)/);
    expect(row.className).toMatch(/^flex\b/);
    // Text grows into the line, so the actions' seat is its end, whatever the text's length.
    expect(row.firstElementChild!.className).toMatch(/\bflex-auto\b/);
    expect(row.lastElementChild!.contains(screen.getByRole("button", { name: "Give a role" }))).toBe(true);
  });
});

/** S11: the header tints in, then what follows arrives; once, and only on navigation. */
describe("PageHeader first paint", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    window.history.replaceState({}, "", "/roles");
  });
  afterEach(() => vi.useRealTimers());

  const head = () => screen.getByRole("heading", { level: 1 }).parentElement!.parentElement!.parentElement!;

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

describe("PageHeader owns header to content", () => {
  it("is 28px, made of the page's own gap-4 plus the header's margin", () => {
    expect(HEADER_TO_CONTENT).toBe(28);
    expect(BELOW_HEADER.marginBottom + CONTENT_GAP).toBe(28);
    render(<PageHeader title="Roles" />);
    expect(screen.getByRole("heading", { level: 1 }).parentElement!.parentElement!.parentElement!.style.marginBottom).toBe("12px");
  });
});
