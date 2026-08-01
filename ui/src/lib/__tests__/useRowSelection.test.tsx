// @vitest-environment jsdom
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { useRowSelection } from "@/lib/useRowSelection";

/**
 * A list harness that exercises the hook exactly as a real surface does —
 * through the props it hands out, not by poking at its internals.
 */
function Harness({ ids }: { ids: string[] }) {
  const selection = useRowSelection(ids);
  return (
    <div data-selection-scope {...selection.containerProps}>
      <input aria-label="select all" type="checkbox" {...selection.headerCheckboxProps} />
      <span data-testid="count">{selection.count}</span>
      <span data-testid="selected">{Array.from(selection.selected).sort().join(",")}</span>
      {ids.map((id) => (
        <div key={id} {...selection.rowProps(id)} data-testid={`row-${id}`}>
          <input type="checkbox" aria-label={`row ${id}`} {...selection.checkboxProps(id)} />
        </div>
      ))}
    </div>
  );
}

const IDS = ["a", "b", "c", "d", "e"];

function renderList(ids: string[] = IDS) {
  return render(<Harness ids={ids} />);
}

const box = (id: string) => screen.getByLabelText(`row ${id}`);
const selected = () => screen.getByTestId("selected").textContent;

describe("useRowSelection — clicking", () => {
  it("toggles one row and toggles it back", () => {
    renderList();
    fireEvent.click(box("b"));
    expect(selected()).toBe("b");
    fireEvent.click(box("b"));
    expect(selected()).toBe("");
  });

  it("selects and clears everything from the header", () => {
    renderList();
    fireEvent.click(screen.getByLabelText("select all"));
    expect(selected()).toBe("a,b,c,d,e");
    fireEvent.click(screen.getByLabelText("select all"));
    expect(selected()).toBe("");
  });
});

describe("useRowSelection — shift-click ranges", () => {
  it("extends downward from the last clicked row", () => {
    renderList();
    fireEvent.click(box("b"));
    fireEvent.click(box("d"), { shiftKey: true });
    expect(selected()).toBe("b,c,d");
  });

  it("extends upward just the same", () => {
    renderList();
    fireEvent.click(box("d"));
    fireEvent.click(box("b"), { shiftKey: true });
    expect(selected()).toBe("b,c,d");
  });

  it("is idempotent — shift-clicking the same range twice does not undo it", () => {
    renderList();
    fireEvent.click(box("a"));
    fireEvent.click(box("c"), { shiftKey: true });
    fireEvent.click(box("c"), { shiftKey: true });
    expect(selected()).toBe("a,b,c");
  });

  it("deselects a range when the anchor row is itself deselected", () => {
    renderList();
    fireEvent.click(screen.getByLabelText("select all"));
    fireEvent.click(box("b")); // anchor is now b, and b is off
    fireEvent.click(box("d"), { shiftKey: true });
    expect(selected()).toBe("a,e");
  });

  it("drops the anchor when the list changes underneath it", () => {
    const { rerender } = renderList();
    fireEvent.click(box("b"));
    // A filter or a refetch reorders the rows; a range spanning "where that row
    // used to be" would select rows nobody pointed at.
    rerender(<Harness ids={["b", "a", "c"]} />);
    fireEvent.click(box("c"), { shiftKey: true });
    expect(selected()).toBe("b,c");
  });

  it("behaves like a plain click when there is no anchor yet", () => {
    renderList();
    fireEvent.click(box("c"), { shiftKey: true });
    expect(selected()).toBe("c");
  });
});

describe("useRowSelection — drag painting", () => {
  function drag(from: string, through: string[]) {
    fireEvent.pointerDown(box(from), { button: 0, clientX: 0, clientY: 0 });
    for (const id of through) {
      fireEvent.pointerEnter(screen.getByTestId(`row-${id}`));
    }
    fireEvent.pointerUp(window);
  }

  it("paints the whole path, including the row it started on", () => {
    renderList();
    drag("a", ["b", "c"]);
    expect(selected()).toBe("a,b,c");
  });

  it("takes its direction from the row it started on, not from each row it crosses", () => {
    renderList();
    fireEvent.click(screen.getByLabelText("select all"));
    // Starting on a selected row means the gesture is UNticking. If it flipped
    // each row instead, the result would depend on the route the mouse took.
    drag("a", ["b", "c"]);
    expect(selected()).toBe("d,e");
  });

  it("does not undo its own path when the drag doubles back", () => {
    renderList();
    drag("a", ["b", "c", "b", "a"]);
    expect(selected()).toBe("a,b,c");
  });

  it("leaves a press that never moved as an ordinary click", () => {
    renderList();
    fireEvent.pointerDown(box("b"), { button: 0, clientX: 0, clientY: 0 });
    fireEvent.pointerUp(window);
    fireEvent.click(box("b"));
    expect(selected()).toBe("b");
  });

  it("does not toggle the start row a second time when the drag ends", () => {
    // The browser fires a click on the element the press began on. Without
    // suppression that click would undo the first row of every drag.
    renderList();
    drag("a", ["b"]);
    fireEvent.click(box("a"));
    expect(selected()).toBe("a,b");
  });

  it("ignores a non-primary button", () => {
    renderList();
    fireEvent.pointerDown(box("a"), { button: 2, clientX: 0, clientY: 0 });
    fireEvent.pointerEnter(screen.getByTestId("row-b"));
    fireEvent.pointerUp(window);
    expect(selected()).toBe("");
  });

  it("ends the gesture when the pointer is released outside the list", () => {
    renderList();
    fireEvent.pointerDown(box("a"), { button: 0, clientX: 0, clientY: 0 });
    fireEvent.pointerUp(window);
    // Not armed any more: entering another row must not paint it.
    fireEvent.pointerEnter(screen.getByTestId("row-c"));
    expect(selected()).toBe("");
  });
});

describe("useRowSelection — keyboard", () => {
  it("toggles on Space, so shift-click is not a mouse-only capability", () => {
    renderList();
    fireEvent.keyDown(box("b"), { key: " " });
    expect(selected()).toBe("b");
  });

  it("extends a range with Shift+Arrow", () => {
    renderList();
    fireEvent.keyDown(box("b"), { key: " " });
    fireEvent.keyDown(box("b"), { key: "ArrowDown", shiftKey: true });
    fireEvent.keyDown(box("c"), { key: "ArrowDown", shiftKey: true });
    expect(selected()).toBe("b,c,d");
  });

  it("moves focus without selecting when Shift is not held", () => {
    renderList();
    fireEvent.keyDown(box("b"), { key: "ArrowDown" });
    expect(selected()).toBe("");
    expect(document.activeElement).toBe(box("c"));
  });

  it("stops at the ends of the list", () => {
    renderList();
    fireEvent.keyDown(box("a"), { key: "ArrowUp", shiftKey: true });
    expect(selected()).toBe("");
  });

  it("clears on Escape and selects everything on `a`", () => {
    renderList();
    const scope = screen.getByTestId("row-a").parentElement!;
    fireEvent.keyDown(scope, { key: "a" });
    expect(selected()).toBe("a,b,c,d,e");
    fireEvent.keyDown(scope, { key: "Escape" });
    expect(selected()).toBe("");
  });

  it("leaves `a` alone while somebody is typing in the list", () => {
    render(
      <div>
        <Harness ids={IDS} />
      </div>,
    );
    const scope = screen.getByTestId("row-a").parentElement!;
    const field = document.createElement("input");
    scope.appendChild(field);
    fireEvent.keyDown(field, { key: "a" });
    expect(selected()).toBe("");
  });
});

describe("useRowSelection — scope changes", () => {
  it("keeps a selection alive when the list only grows", () => {
    const { rerender } = renderList(["a", "b"]);
    fireEvent.click(box("a"));
    // "Load more" adds rows; the rows already chosen are still chosen.
    rerender(<Harness ids={["a", "b", "c"]} />);
    expect(selected()).toBe("a");
  });

  it("drops rows that have left the list entirely", () => {
    const { rerender } = renderList(["a", "b"]);
    fireEvent.click(screen.getByLabelText("select all"));
    // A bulk action aimed at a row no longer on screen is aimed at nothing.
    rerender(<Harness ids={["a"]} />);
    expect(selected()).toBe("a");
  });
});
