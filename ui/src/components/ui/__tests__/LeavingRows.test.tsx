// @vitest-environment jsdom
import { act, fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it } from "vitest";

import { LeavingRow, useLeavingRows } from "@/components/ui/LeavingRows";
import { setMediaQuery } from "@/test-utils/media";
import { motionInTests } from "@/test-utils/motion";

/** Motion v2 S5: a row that leaves on a later read fades, then its gap closes. */

function List({ ids, ready = true }: { ids: string[]; ready?: boolean }) {
  const leaving = useLeavingRows(ids, (id) => id, ready);
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
