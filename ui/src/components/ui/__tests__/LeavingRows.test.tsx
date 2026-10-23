// @vitest-environment jsdom
import { act, fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it } from "vitest";

import { LeavingRow, useLeavingRows } from "@/components/ui/LeavingRows";
import { setMediaQuery } from "@/test-utils/media";
import { motionInTests } from "@/test-utils/motion";

/** Motion v2 S5: a row that leaves on a later read fades, then its gap closes. */

function List({
  ids,
  ready = true,
  stay,
  scope,
}: {
  ids: string[];
  ready?: boolean;
  stay?: ReadonlySet<string>;
  scope?: string;
}) {
  const leaving = useLeavingRows(ids, (id) => id, ready, { stay, scope });
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
