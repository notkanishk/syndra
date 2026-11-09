// @vitest-environment jsdom
import { fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import NotFound from "@/app/not-found";
import { NotOpenYet } from "@/components/ui/NotOpenYet";
import { Skeleton, useTintAfterSkeleton } from "@/components/ui/Skeleton";

const glide = vi.hoisted(() => ({ to: [] as HTMLElement[] }));
vi.mock("@/lib/motion", async (original) => ({
  ...(await original<typeof import("@/lib/motion")>()),
  glideTo: (target: HTMLElement) => {
    glide.to.push(target);
    return Promise.resolve(true);
  },
}));

/** S11: the screens that are not a `PageHeader` still tint in (the upstream pages are PageHeaders now, A-17). */
describe("a screen's first paint without a PageHeader", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    window.history.replaceState({}, "", "/somewhere");
  });
  afterEach(() => vi.useRealTimers());

  it("the address that matches no page tints in", () => {
    render(<NotFound />);
    const head = screen.getByRole("status").parentElement!;
    expect(head).toHaveClass("page-head");
  });
});

/** 8d: a step that points at somewhere on this page glides there and the place flashes. */
describe("a step that points further down the same page", () => {
  beforeEach(() => {
    glide.to = [];
  });

  it("glides to the target instead of jumping", () => {
    render(
      <>
        <NotOpenYet open={false} title="Set a password" why="Until then, nothing opens." step={{ label: "Set it above", href: "#pw" }}>
          <p>folders</p>
        </NotOpenYet>
        <input id="pw" aria-label="password" />
      </>,
    );
    // false: the native hash jump was prevented.
    expect(fireEvent.click(screen.getByRole("link", { name: "Set it above" }))).toBe(false);
    expect(glide.to.map((el) => el.id)).toEqual(["pw"]);
  });

  it("leaves a link to another page alone", () => {
    render(
      <NotOpenYet open={false} title="Set a password" why="Until then, nothing opens." step={{ label: "Go set it", href: "/me" }}>
        <p>folders</p>
      </NotOpenYet>,
    );
    // Nothing in jsdom can follow a link; the click is all this is about.
    const stop = (event: Event) => event.preventDefault();
    document.addEventListener("click", stop);
    fireEvent.click(screen.getByRole("link", { name: "Go set it" }));
    document.removeEventListener("click", stop);
    expect(glide.to).toEqual([]);
  });
});

/** S7: what lands after a skeleton tints in; what was there to begin with is just there. */
describe("what lands after a skeleton", () => {
  function Name({ loading }: { loading: boolean }) {
    const tint = useTintAfterSkeleton(loading);
    return loading ? <Skeleton className="h-3 w-20" /> : <span className={tint}>Roman Reigns</span>;
  }

  it("tints in", () => {
    const { rerender } = render(<Name loading />);
    rerender(<Name loading={false} />);
    expect(screen.getByText("Roman Reigns")).toHaveClass("tint-in");
  });

  it("is simply there when it never was a skeleton", () => {
    render(<Name loading={false} />);
    expect(screen.getByText("Roman Reigns")).not.toHaveClass("tint-in");
  });
});
