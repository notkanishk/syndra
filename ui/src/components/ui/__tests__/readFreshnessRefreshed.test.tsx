// @vitest-environment jsdom
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";

import { ReadFreshness, useRefreshPress } from "@/components/ui/ReadFreshness";
import { motionInTests } from "@/test-utils/motion";

// S17 Refreshed: a refresh the reader pressed is the only read that blooms.
const NOW = Date.now();
const ago = (minutes: number) => new Date(NOW - minutes * 60_000).toISOString();
const dot = () => document.querySelector("[data-freshness-dot]") as HTMLElement;

function deferred() {
  let resolve!: () => void;
  const promise = new Promise<void>((r) => (resolve = r));
  return { promise, resolve };
}

describe("S17 Refreshed", () => {
  afterEach(cleanup);

  it("pressing shows its own busy state, and the line keeps the old age until the new read lands", async () => {
    const d = deferred();
    const { rerender } = render(<ReadFreshness state={{ readAt: ago(20) }} onRefresh={() => d.promise} />);
    expect(screen.getByText(/20 min ago/)).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Read again" }));
    const busy = screen.getByRole("button", { name: /Reading/ });
    expect(busy).toHaveAttribute("aria-busy", "true");
    // Never a time it hasn't read.
    expect(screen.getByText(/20 min ago/)).toBeInTheDocument();
    expect(dot().className).not.toContain("bloom");

    await act(async () => {
      d.resolve();
      await d.promise;
    });
    rerender(<ReadFreshness state={{ readAt: new Date().toISOString() }} onRefresh={() => d.promise} />);
    expect(screen.getByText(/was read just now/)).toBeInTheDocument();
    expect(dot().className).toContain("bloom");
  });

  it("a background read never blooms", () => {
    const { rerender } = render(<ReadFreshness state={{ readAt: ago(5) }} onRefresh={() => {}} />);
    rerender(<ReadFreshness state={{ readAt: new Date().toISOString() }} onRefresh={() => {}} />);
    expect(screen.getByText(/was read just now/)).toBeInTheDocument();
    expect(dot().className).not.toContain("bloom");
  });

  it("a refresh that fails to bring a new read does not leave the next background read to bloom", async () => {
    const d = deferred();
    const { rerender } = render(<ReadFreshness state={{ readAt: ago(20) }} onRefresh={() => d.promise} />);
    fireEvent.click(screen.getByRole("button", { name: "Read again" }));
    await act(async () => {
      d.resolve();
      await d.promise;
      // Past the grace the landing is given.
      await new Promise((r) => setTimeout(r, 400));
    });
    rerender(<ReadFreshness state={{ readAt: new Date().toISOString() }} onRefresh={() => d.promise} />);
    expect(dot().className).not.toContain("bloom");
  });

  it("a read that lands still stale does not bloom: nothing went well", async () => {
    const d = deferred();
    const { rerender } = render(<ReadFreshness state={{ readAt: ago(30) }} onRefresh={() => d.promise} />);
    fireEvent.click(screen.getByRole("button", { name: "Read again" }));
    rerender(<ReadFreshness state={{ readAt: ago(25) }} onRefresh={() => d.promise} />);
    expect(dot().className).not.toContain("bloom");
  });
});

// "Check Zitadel again" is a button elsewhere on the page, and it answers the same way: the press
// is read by the freshness line it sits apart from.
describe("S17 Refreshed, pressed from elsewhere", () => {
  afterEach(() => {
    cleanup();
    motionInTests(false);
  });

  function Page({ readAt, run }: { readAt: string; run: () => Promise<void> }) {
    const refreshed = useRefreshPress(readAt);
    return (
      <>
        <button type="button" onClick={() => void refreshed.press(run)} disabled={refreshed.pressing}>
          Check again
        </button>
        <ReadFreshness state={{ readAt }} refresh={refreshed} />
      </>
    );
  }

  it("keeps the old age while it reads, then crossfades just now in and blooms the dot once", async () => {
    motionInTests();
    const d = deferred();
    const { rerender } = render(<Page readAt={ago(5)} run={() => d.promise} />);

    fireEvent.click(screen.getByRole("button", { name: "Check again" }));
    expect(screen.getByRole("button", { name: "Check again" })).toBeDisabled();
    expect(screen.getByText(/5 min ago/)).toBeInTheDocument();

    await act(async () => {
      d.resolve();
      await d.promise;
    });
    rerender(<Page readAt={new Date().toISOString()} run={() => d.promise} />);

    expect(dot().className).toContain("bloom");
    expect(screen.getByText(/was read just now/).closest(".tint-in")).not.toBeNull();
    // The old line is still there, fading out, until the new one has taken its place.
    expect(screen.getByText(/5 min ago/).closest(".tint-out")).not.toBeNull();
  });

  it("a background read of the same page never blooms", () => {
    const { rerender } = render(<Page readAt={ago(5)} run={async () => {}} />);
    rerender(<Page readAt={new Date().toISOString()} run={async () => {}} />);
    expect(dot().className).not.toContain("bloom");
  });
});
