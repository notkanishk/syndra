// @vitest-environment jsdom
import { act, fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ARRIVE_MEMORY_MS, armArrive } from "@/components/ui/ArriveList";
import { LeavingRow, useLeavingRows } from "@/components/ui/LeavingRows";
import { setMediaQuery } from "@/test-utils/media";
import { motionInTests } from "@/test-utils/motion";

/** Motion v2 S5: a row that leaves on a later read fades, then its gap closes. */

function List({
  ids,
  ready = true,
  stay,
  scope,
  narrowedBy,
}: {
  ids: string[];
  ready?: boolean;
  stay?: ReadonlySet<string>;
  scope?: string;
  narrowedBy?: string;
}) {
  const leaving = useLeavingRows(ids, (id) => id, ready, { stay, scope, narrowedBy });
  return (
    <div data-testid="list" {...leaving.listProps}>
      <button type="button">inside</button>
      {leaving.rows.map((id) => (
        <LeavingRow key={id} {...leaving.props(id)}>
          <p>{id}</p>
        </LeavingRow>
      ))}
    </div>
  );
}

const rowOf = (id: string) => screen.getByText(id).parentElement!.parentElement!;
// jsdom has no AnimationEvent, so React listens for the prefixed name here.
const animationEnd = (el: Element) => act(() => void fireEvent(el, new Event("webkitAnimationEnd", { bubbles: true })));

beforeEach(() => motionInTests());

describe("a row leaving after a read", () => {
  it("does not mistake a new array of the same rows for a read", () => {
    const { rerender } = render(<List ids={["a", "b"]} />);
    rerender(<List ids={["a", "b"]} />);
    expect(screen.getByText("b").parentElement).not.toHaveClass("tint-out");
  });

  it("fades where it was, then closes its gap, then is gone; the rows that stay never move", () => {
    const { rerender } = render(<List ids={["a", "b", "c"]} />);
    const a = screen.getByText("a");
    rerender(<List ids={["a", "c"]} />);

    expect(screen.getAllByText(/^[abc]$/).map((p) => p.textContent)).toEqual(["a", "b", "c"]);
    const gone = rowOf("b");
    expect(gone).toHaveAttribute("aria-hidden", "true");
    expect(screen.getByText("b").parentElement).toHaveClass("tint-out");
    expect(gone).toHaveClass("close-gap");
    // The ones that stayed are the same elements: nothing remounted, nothing replayed.
    expect(screen.getByText("a")).toBe(a);
    expect(rowOf("a")).not.toHaveClass("close-gap");

    animationEnd(gone);
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

    expect(screen.getByText("b").parentElement).toHaveClass("tint-out");
    expect(rowOf("b")).not.toHaveClass("close-gap");

    fireEvent.pointerLeave(screen.getByTestId("list"));
    expect(rowOf("b")).toHaveClass("close-gap");
  });

  it("waits for focus to leave the list too", () => {
    const { rerender } = render(<List ids={["a", "b"]} />);
    const inside = screen.getByRole("button", { name: "inside" });
    fireEvent.focus(inside);
    rerender(<List ids={["a"]} />);
    expect(rowOf("b")).not.toHaveClass("close-gap");

    fireEvent.blur(inside, { relatedTarget: document.body });
    expect(rowOf("b")).toHaveClass("close-gap");
  });

  it("focus moving from one place in the list to another is not leaving it", () => {
    const { rerender } = render(<List ids={["a", "b"]} />);
    const inside = screen.getByRole("button", { name: "inside" });
    fireEvent.focus(inside);
    rerender(<List ids={["a"]} />);
    fireEvent.blur(inside, { relatedTarget: screen.getByText("a") });
    expect(rowOf("b")).not.toHaveClass("close-gap");
  });

  it("is a row again if the next read brings it back", () => {
    const { rerender } = render(<List ids={["a", "b"]} />);
    fireEvent.pointerEnter(screen.getByTestId("list"));
    rerender(<List ids={["a"]} />);
    rerender(<List ids={["a", "b"]} />);
    expect(rowOf("b")).not.toHaveAttribute("aria-hidden");
    expect(screen.getByText("b").parentElement).not.toHaveClass("tint-out");
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
    expect(screen.getByText("b").parentElement).not.toHaveClass("tint-out");

    rerender(<List ids={["a"]} stay={new Set()} />);
    expect(screen.getByText("b").parentElement).toHaveClass("tint-out");
    expect(rowOf("b")).toHaveClass("close-gap");
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
  it("what it drops leaves, its gap does not wait on the pointer, and what it brings tints in", () => {
    const { rerender } = render(<List ids={["a", "b"]} narrowedBy="open" />);
    // A search is typed with the pointer parked over the list it narrows.
    fireEvent.pointerEnter(screen.getByTestId("list"));
    rerender(<List ids={["c"]} narrowedBy="done" />);
    expect(screen.getByText("a").parentElement).toHaveClass("tint-out");
    expect(rowOf("a")).toHaveClass("close-gap");
    expect(screen.getByText("c").parentElement).toHaveClass("tint-in");
  });

  it("holds for a read that lands after what is typed has changed: its gaps do not wait either", () => {
    const { rerender } = render(<List ids={["a", "b"]} narrowedBy="" />);
    fireEvent.pointerEnter(screen.getByTestId("list"));
    // The query is typed: nothing has changed yet but what is being looked through.
    rerender(<List ids={["a", "b"]} narrowedBy="a" />);
    rerender(<List ids={["a"]} narrowedBy="a" />);
    expect(rowOf("b")).toHaveClass("close-gap");
  });

  it("is the reader's only for a while: a later read nobody asked for still waits on the pointer", () => {
    vi.useFakeTimers();
    try {
      const { rerender } = render(<List ids={["a", "b"]} narrowedBy="" />);
      rerender(<List ids={["a", "b"]} narrowedBy="a" />);
      act(() => void vi.advanceTimersByTime(ARRIVE_MEMORY_MS + 1000));
      fireEvent.pointerEnter(screen.getByTestId("list"));
      rerender(<List ids={["a"]} narrowedBy="a" />);
      expect(rowOf("b")).not.toHaveClass("close-gap");
    } finally {
      vi.useRealTimers();
    }
  });

  it("keeps the rows that stay: the same elements, nothing remounted", () => {
    const { rerender } = render(<List ids={["a", "b", "c"]} narrowedBy="" />);
    const b = screen.getByText("b");
    rerender(<List ids={["b"]} narrowedBy="b" />);
    expect(screen.getByText("b")).toBe(b);
    expect(b.parentElement).not.toHaveClass("tint-in");
  });
});

describe("a row that comes in on a later read", () => {
  it("tints in where it lands; the rows that stayed, and the first read's rows, do not", () => {
    const { rerender } = render(<List ids={["a", "b"]} />);
    expect(screen.getByText("a").parentElement).not.toHaveClass("tint-in");
    rerender(<List ids={["a", "b", "c"]} />);
    expect(screen.getByText("c").parentElement).toHaveClass("tint-in");
    expect(screen.getByText("a").parentElement).not.toHaveClass("tint-in");
  });

  it("keeps its tint through the next read, so it is never cut off", () => {
    const { rerender } = render(<List ids={["a"]} />);
    rerender(<List ids={["a", "c"]} />);
    rerender(<List ids={["a", "c", "d"]} />);
    expect(screen.getByText("c").parentElement).toHaveClass("tint-in");
    expect(screen.getByText("d").parentElement).toHaveClass("tint-in");
  });

  it("fades out, not in, when it goes", () => {
    const { rerender } = render(<List ids={["a"]} />);
    rerender(<List ids={["a", "c"]} />);
    rerender(<List ids={["a"]} />);
    expect(screen.getByText("c").parentElement).toHaveClass("tint-out");
    expect(screen.getByText("c").parentElement).not.toHaveClass("tint-in");
  });

  it("is not new when it is what the first read brought, even after an empty read", () => {
    const { rerender } = render(<List ids={[]} ready={false} />);
    rerender(<List ids={["a", "b"]} />);
    expect(screen.getByText("a").parentElement).not.toHaveClass("tint-in");
  });

  it("is new when it follows a first read that found nothing", () => {
    const { rerender } = render(<List ids={[]} ready={false} />);
    rerender(<List ids={[]} />);
    rerender(<List ids={["a"]} />);
    expect(screen.getByText("a").parentElement).toHaveClass("tint-in");
  });

  it("arrives instead when the reader asked for it (Show more)", () => {
    const { rerender } = render(<List ids={["a"]} />);
    armArrive();
    rerender(<List ids={["a", "b"]} />);
    expect(screen.getByText("b").parentElement).not.toHaveClass("tint-in");
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
    expect(rowOf("b")).toHaveClass("close-gap");
  });
});
