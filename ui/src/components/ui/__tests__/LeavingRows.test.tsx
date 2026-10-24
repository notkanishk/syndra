// @vitest-environment jsdom
import { act, fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ARRIVE_MEMORY_MS, armArrive } from "@/components/ui/ArriveList";
import { LeavingRow, useLeavingRows } from "@/components/ui/LeavingRows";
import { setMediaQuery } from "@/test-utils/media";
import { gapSettled } from "@/test-utils/leaving";
import { motionInTests } from "@/test-utils/motion";

/** Motion v2 S5: a row that leaves on a later read fades, then its gap closes. */

function List({
  ids,
  ready = true,
  stay,
  scope,
  narrowedBy,
  limit,
}: {
  ids: string[];
  ready?: boolean;
  stay?: ReadonlySet<string>;
  scope?: string;
  narrowedBy?: string;
  /** A page's limit: the caller draws only the first rows (People, Audit). */
  limit?: number;
}) {
  const leaving = useLeavingRows(ids, (id) => id, ready, { stay, scope, narrowedBy });
  return (
    <div data-testid="list" {...leaving.listProps}>
      <button type="button">inside</button>
      {(limit === undefined ? leaving.rows : leaving.window(limit)).map((id) => (
        <LeavingRow key={id} {...leaving.props(id)}>
          <p>{id}</p>
        </LeavingRow>
      ))}
    </div>
  );
}

const rowOf = (id: string) => screen.getByText(id).parentElement!.parentElement!;

beforeEach(() => motionInTests());

describe("a row leaving after a read", () => {
  it("does not mistake a new array of the same rows for a read", () => {
    const { rerender } = render(<List ids={["a", "b"]} />);
    rerender(<List ids={["a", "b"]} />);
    expect(rowOf("b")).not.toHaveAttribute("data-leaving");
  });

  it("fades where it was, then closes its gap, then is gone; the rows that stay never move", () => {
    const { rerender } = render(<List ids={["a", "b", "c"]} />);
    const a = screen.getByText("a");
    rerender(<List ids={["a", "c"]} />);

    expect(screen.getAllByText(/^[abc]$/).map((p) => p.textContent)).toEqual(["a", "b", "c"]);
    const gone = rowOf("b");
    expect(gone).toHaveAttribute("aria-hidden", "true");
    expect(rowOf("b")).toHaveAttribute("data-leaving");
    expect(gone).toHaveAttribute("data-closing");
    // The ones that stayed are the same elements: nothing remounted, nothing replayed.
    expect(screen.getByText("a")).toBe(a);
    expect(rowOf("a")).not.toHaveAttribute("data-closing");

    gapSettled(gone);
    expect(screen.queryByText("b")).not.toBeInTheDocument();
  });

  it("is gone after its own duration even if no animation ever ends", () => {
    vi.useFakeTimers();
    try {
      const { rerender } = render(<List ids={["a", "b"]} />);
      rerender(<List ids={["a"]} />);
      expect(screen.getByText("b")).toBeInTheDocument();
      act(() => void vi.advanceTimersByTime(120 + 260 + 100));
      expect(screen.queryByText("b")).not.toBeInTheDocument();
    } finally {
      vi.useRealTimers();
    }
  });

  it("waits for the pointer to leave the list before the gap closes", () => {
    const { rerender } = render(<List ids={["a", "b"]} />);
    fireEvent.pointerEnter(screen.getByTestId("list"));
    rerender(<List ids={["a"]} />);

    expect(rowOf("b")).toHaveAttribute("data-leaving");
    expect(rowOf("b")).not.toHaveAttribute("data-closing");

    fireEvent.pointerLeave(screen.getByTestId("list"));
    expect(rowOf("b")).toHaveAttribute("data-closing");
  });

  it("waits for focus to leave the list too", () => {
    const { rerender } = render(<List ids={["a", "b"]} />);
    const inside = screen.getByRole("button", { name: "inside" });
    fireEvent.focus(inside);
    rerender(<List ids={["a"]} />);
    expect(rowOf("b")).not.toHaveAttribute("data-closing");

    fireEvent.blur(inside, { relatedTarget: document.body });
    expect(rowOf("b")).toHaveAttribute("data-closing");
  });

  it("focus moving from one place in the list to another is not leaving it", () => {
    const { rerender } = render(<List ids={["a", "b"]} />);
    const inside = screen.getByRole("button", { name: "inside" });
    fireEvent.focus(inside);
    rerender(<List ids={["a"]} />);
    fireEvent.blur(inside, { relatedTarget: screen.getByText("a") });
    expect(rowOf("b")).not.toHaveAttribute("data-closing");
  });

  it("is a row again if the next read brings it back", () => {
    const { rerender } = render(<List ids={["a", "b"]} />);
    fireEvent.pointerEnter(screen.getByTestId("list"));
    rerender(<List ids={["a"]} />);
    rerender(<List ids={["a", "b"]} />);
    expect(rowOf("b")).not.toHaveAttribute("aria-hidden");
    expect(rowOf("b")).not.toHaveAttribute("data-leaving");
  });

  it("is gone at once in reduced motion", () => {
    setMediaQuery("(prefers-reduced-motion: reduce)", true);
    const { rerender } = render(<List ids={["a", "b"]} />);
    rerender(<List ids={["a"]} />);
    expect(screen.queryByText("b")).not.toBeInTheDocument();
  });

  it("does not treat a read that has not landed as every row leaving", () => {
    const { rerender } = render(<List ids={["a", "b"]} />);
    rerender(<List ids={[]} ready={false} />);
    expect(screen.queryByText("a")).not.toBeInTheDocument();
  });
});

describe("a row held for what it is still saying", () => {
  const HELD = new Set(["b"]);

  it("stays as it was after the read drops it, and leaves like any other once released", () => {
    const { rerender } = render(<List ids={["a", "b"]} stay={HELD} />);
    rerender(<List ids={["a"]} stay={HELD} />);
    expect(rowOf("b")).not.toHaveAttribute("aria-hidden");
    expect(rowOf("b")).not.toHaveAttribute("data-leaving");

    rerender(<List ids={["a"]} stay={new Set()} />);
    expect(rowOf("b")).toHaveAttribute("data-leaving");
    expect(rowOf("b")).toHaveAttribute("data-closing");
  });

  it("in reduced motion stays while held, then is simply gone", () => {
    motionInTests(false);
    const { rerender } = render(<List ids={["a", "b"]} stay={HELD} />);
    rerender(<List ids={["a"]} stay={HELD} />);
    expect(screen.getByText("b")).toBeInTheDocument();
    rerender(<List ids={["a"]} stay={new Set()} />);
    expect(screen.queryByText("b")).not.toBeInTheDocument();
  });

  it("a different scope is a different list: nothing fades out of it", () => {
    const { rerender } = render(<List ids={["a", "b"]} scope="open" />);
    rerender(<List ids={["c"]} scope="done" />);
    expect(screen.queryByText("a")).not.toBeInTheDocument();
    expect(screen.getByText("c")).toBeInTheDocument();
  });
});

describe("rows narrowed by the reader (a search or a filter)", () => {
  it("what it drops leaves, its gap does not wait on the pointer, and what it brings opens its gap, then tints in", () => {
    const { rerender } = render(<List ids={["a", "b"]} narrowedBy="open" />);
    // A search is typed with the pointer parked over the list it narrows.
    fireEvent.pointerEnter(screen.getByTestId("list"));
    rerender(<List ids={["c"]} narrowedBy="done" />);
    expect(rowOf("a")).toHaveAttribute("data-leaving");
    expect(rowOf("a")).toHaveAttribute("data-closing");
    expect(rowOf("c")).toHaveAttribute("data-opens");
    // One element carries both moves: `leave-row` opens the gap, then the contents tint in behind.
    expect(rowOf("c")).not.toHaveAttribute("data-fresh");
  });

  it("a row the reader brings back after it has gone opens, and is an ordinary row once its gap has settled", () => {
    const { rerender } = render(<List ids={["a", "b"]} narrowedBy="" />);
    rerender(<List ids={["a"]} narrowedBy="a" />);
    gapSettled(rowOf("b"));
    expect(screen.queryByText("b")).toBeNull();

    rerender(<List ids={["a", "b"]} narrowedBy="" />);
    expect(rowOf("b")).toHaveAttribute("data-opens");
    expect(rowOf("a")).not.toHaveAttribute("data-opens");
    gapSettled(rowOf("b"));
    // Nothing is left on it to clip a focus ring once it has opened.
    expect(rowOf("b")).not.toHaveAttribute("data-opens");
  });

  it("a row new from a refresh nobody asked for only tints in: no gap opens", () => {
    vi.useFakeTimers();
    try {
      const { rerender } = render(<List ids={["a"]} narrowedBy="" />);
      rerender(<List ids={["a"]} narrowedBy="a" />);
      act(() => void vi.advanceTimersByTime(ARRIVE_MEMORY_MS + 1000));
      rerender(<List ids={["a", "c"]} narrowedBy="a" />);
      expect(rowOf("c")).toHaveAttribute("data-fresh");
      expect(rowOf("c")).not.toHaveAttribute("data-opens");
    } finally {
      vi.useRealTimers();
    }
  });

  it("a row back while it is still fading turns round: the same element in the same seat, nothing remounted or replayed", () => {
    const { rerender } = render(<List ids={["a", "b", "c"]} narrowedBy="" />);
    const b = screen.getByText("b");
    const row = rowOf("b");
    // Typing "ac" starts b's fade...
    rerender(<List ids={["a", "c"]} narrowedBy="ac" />);
    expect(row).toHaveAttribute("data-leaving");
    expect(row).toHaveAttribute("data-closing");
    // ...and a backspace brings it back before it is gone. It keeps its node and its seat, and loses
    // its leaving state: `leave-row` is a transition, so dropping the state is what turns it round.
    rerender(<List ids={["a", "b", "c"]} narrowedBy="a" />);
    expect(screen.getByText("b")).toBe(b);
    expect(rowOf("b")).toBe(row);
    expect(screen.getAllByText(/^[abc]$/).map((p) => p.textContent)).toEqual(["a", "b", "c"]);
    expect(row).not.toHaveAttribute("data-leaving");
    expect(row).not.toHaveAttribute("data-closing");
    expect(row).not.toHaveAttribute("aria-hidden");
    // Nothing that would replay: no entry move for a row that never left the page.
    expect(row).not.toHaveAttribute("data-opens");
    expect(row).not.toHaveAttribute("data-fresh");
    // The fold it had started does not take it away later.
    gapSettled(row);
    expect(screen.getByText("b")).toBe(b);
  });

  it("a row back while its gap was held open by the pointer turns round too", () => {
    const { rerender } = render(<List ids={["a", "b"]} />);
    fireEvent.pointerEnter(screen.getByTestId("list"));
    rerender(<List ids={["a"]} />);
    const row = rowOf("b");
    expect(row).toHaveAttribute("data-leaving");
    expect(row).not.toHaveAttribute("data-closing");
    rerender(<List ids={["a", "b"]} />);
    expect(rowOf("b")).toBe(row);
    expect(row).not.toHaveAttribute("data-leaving");
    expect(row).not.toHaveAttribute("data-fresh");
  });

  it("a row is gone in reduced motion with nothing to open: a returning row is simply there", () => {
    motionInTests(false);
    const { rerender } = render(<List ids={["a", "b"]} narrowedBy="" />);
    rerender(<List ids={["a"]} narrowedBy="a" />);
    expect(screen.queryByText("b")).toBeNull();
    rerender(<List ids={["a", "b"]} narrowedBy="" />);
    // Its attributes stay; the stylesheet collapses their durations. The row is in at once.
    expect(screen.getByText("b")).toBeInTheDocument();
  });

  it("holds for a read that lands after what is typed has changed: its gaps do not wait either", () => {
    const { rerender } = render(<List ids={["a", "b"]} narrowedBy="" />);
    fireEvent.pointerEnter(screen.getByTestId("list"));
    // The query is typed: nothing has changed yet but what is being looked through.
    rerender(<List ids={["a", "b"]} narrowedBy="a" />);
    rerender(<List ids={["a"]} narrowedBy="a" />);
    expect(rowOf("b")).toHaveAttribute("data-closing");
  });

  it("is the reader's only for a while: a later read nobody asked for still waits on the pointer", () => {
    vi.useFakeTimers();
    try {
      const { rerender } = render(<List ids={["a", "b"]} narrowedBy="" />);
      rerender(<List ids={["a", "b"]} narrowedBy="a" />);
      act(() => void vi.advanceTimersByTime(ARRIVE_MEMORY_MS + 1000));
      fireEvent.pointerEnter(screen.getByTestId("list"));
      rerender(<List ids={["a"]} narrowedBy="a" />);
      expect(rowOf("b")).not.toHaveAttribute("data-closing");
    } finally {
      vi.useRealTimers();
    }
  });

  it("keeps the rows that stay: the same elements, nothing remounted", () => {
    const { rerender } = render(<List ids={["a", "b", "c"]} narrowedBy="" />);
    const b = screen.getByText("b");
    rerender(<List ids={["b"]} narrowedBy="b" />);
    expect(screen.getByText("b")).toBe(b);
    expect(rowOf("b")).not.toHaveAttribute("data-fresh");
  });
});

describe("a list that draws a window of its rows (a page's limit)", () => {
  const ids = Array.from({ length: 30 }, (_, index) => `r${index}`);

  it("fades only what was on screen: a search's matches past the old page show at once", () => {
    const { rerender } = render(<List ids={ids} narrowedBy="" limit={10} />);
    // A search keeps r2 and r25..r27: r25 was never drawn under the old page.
    rerender(<List ids={["r2", "r25", "r26", "r27"]} narrowedBy="x" limit={10} />);
    const drawn = screen.getAllByText(/^r\d+$/).map((p) => p.textContent);
    // The nine rows that were on screen fade where they were; the matches are all drawn now.
    expect(drawn.filter((id) => ["r25", "r26", "r27"].includes(id!))).toEqual(["r25", "r26", "r27"]);
    expect(drawn).not.toContain("r12");
    expect(rowOf("r0")).toHaveAttribute("data-leaving");
  });

  it("lets a fading row pushed out of the window go, rather than hold a seat it never shows in", () => {
    const { rerender } = render(<List ids={["a", "b", "c"]} limit={3} />);
    // b leaves, and two new rows land above it: the window of three no longer reaches it.
    rerender(<List ids={["n1", "n2", "a", "c"]} limit={3} />);
    expect(screen.queryByText("b")).toBeNull();
    // ...and it is gone from the list, not waiting past the limit for a fade that never runs.
    rerender(<List ids={["n1", "n2", "a", "c"]} limit={10} />);
    expect(screen.queryByText("b")).toBeNull();
  });
});

describe("a row that comes in on a later read", () => {
  it("tints in where it lands; the rows that stayed, and the first read's rows, do not", () => {
    const { rerender } = render(<List ids={["a", "b"]} />);
    expect(rowOf("a")).not.toHaveAttribute("data-fresh");
    rerender(<List ids={["a", "b", "c"]} />);
    expect(rowOf("c")).toHaveAttribute("data-fresh");
    expect(rowOf("a")).not.toHaveAttribute("data-fresh");
  });

  it("keeps its tint through the next read, so it is never cut off", () => {
    const { rerender } = render(<List ids={["a"]} />);
    rerender(<List ids={["a", "c"]} />);
    rerender(<List ids={["a", "c", "d"]} />);
    expect(rowOf("c")).toHaveAttribute("data-fresh");
    expect(rowOf("d")).toHaveAttribute("data-fresh");
  });

  it("fades out, not in, when it goes", () => {
    const { rerender } = render(<List ids={["a"]} />);
    rerender(<List ids={["a", "c"]} />);
    rerender(<List ids={["a"]} />);
    expect(rowOf("c")).toHaveAttribute("data-leaving");
    expect(rowOf("c")).not.toHaveAttribute("data-fresh");
  });

  it("is not new when it is what the first read brought, even after an empty read", () => {
    const { rerender } = render(<List ids={[]} ready={false} />);
    rerender(<List ids={["a", "b"]} />);
    expect(rowOf("a")).not.toHaveAttribute("data-fresh");
  });

  it("is new when it follows a first read that found nothing", () => {
    const { rerender } = render(<List ids={[]} ready={false} />);
    rerender(<List ids={[]} />);
    rerender(<List ids={["a"]} />);
    expect(rowOf("a")).toHaveAttribute("data-fresh");
  });

  it("arrives instead when the reader asked for it (Show more)", () => {
    const { rerender } = render(<List ids={["a"]} />);
    armArrive();
    rerender(<List ids={["a", "b"]} />);
    expect(rowOf("b")).not.toHaveAttribute("data-fresh");
  });
});

describe("focus that went with a removed button", () => {
  it("does not keep the gap open once the pointer has left", () => {
    const { rerender } = render(<List ids={["a", "b"]} />);
    fireEvent.pointerEnter(screen.getByTestId("list"));
    // Focus entered the list on a button that is then removed: no blur is ever sent.
    fireEvent.focus(screen.getByRole("button", { name: "inside" }));
    rerender(<List ids={["a"]} />);
    (document.activeElement as HTMLElement | null)?.blur();

    fireEvent.pointerLeave(screen.getByTestId("list"));
    expect(rowOf("b")).toHaveAttribute("data-closing");
  });
});
