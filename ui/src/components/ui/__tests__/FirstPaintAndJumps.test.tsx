// @vitest-environment jsdom
import { act, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import NotFound from "@/app/not-found";
import { UpstreamShell } from "@/components/upstream/UpstreamShell";
import { NotOpenYet } from "@/components/ui/NotOpenYet";
import { ARRIVE_SETTLED_MS } from "@/components/ui/ArriveList";
import { Skeleton, useTintAfterSkeleton } from "@/components/ui/Skeleton";
import { TINT_MS } from "@/lib/motion";

const glide = vi.hoisted(() => ({ to: [] as HTMLElement[] }));
vi.mock("@/lib/motion", async (original) => ({
  ...(await original<typeof import("@/lib/motion")>()),
  glideTo: (target: HTMLElement) => {
    glide.to.push(target);
    return Promise.resolve(true);
  },
}));

/** S11: the screens that are not a `PageHeader` still tint in, then what follows arrives. */
describe("a screen's first paint without a PageHeader", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    window.history.replaceState({}, "", "/somewhere");
  });
  afterEach(() => vi.useRealTimers());

  it("the upstream pages wear the arrival on their title block, then hold still", () => {
    render(
      <UpstreamShell title="Zitadel users" lede="Who Zitadel has." syndraHref="/users" syndraLabel="People">
        <p>the list</p>
      </UpstreamShell>,
    );
    const head = screen.getByRole("heading", { level: 1 }).parentElement!;
    expect(head).toHaveClass("page-head");
    expect(head).not.toHaveAttribute("data-settled");
    act(() => void vi.advanceTimersByTime(TINT_MS + ARRIVE_SETTLED_MS));
    expect(head).toHaveAttribute("data-settled");
  });

  it("a Back to the page finds it already there", () => {
    act(() => void window.dispatchEvent(new PopStateEvent("popstate")));
    render(
      <UpstreamShell title="Zitadel users" lede="Who Zitadel has." syndraHref="/users" syndraLabel="People">
        <p>the list</p>
      </UpstreamShell>,
    );
    expect(screen.getByRole("heading", { level: 1 }).parentElement).toHaveAttribute("data-settled");
  });

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
    return loading ? <Skeleton className="h-3 w-20" /> : <span className={tint}>Aanya Rao</span>;
  }

  it("tints in", () => {
    const { rerender } = render(<Name loading />);
    rerender(<Name loading={false} />);
    expect(screen.getByText("Aanya Rao")).toHaveClass("tint-in");
  });

  it("is simply there when it never was a skeleton", () => {
    render(<Name loading={false} />);
    expect(screen.getByText("Aanya Rao")).not.toHaveClass("tint-in");
  });
});
