// @vitest-environment jsdom
import { act, fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it } from "vitest";

import { FormPart, useAddedKeys } from "@/components/ui/FormPart";
import { useLeavingRows } from "@/components/ui/LeavingRows";
import { motionInTests } from "@/test-utils/motion";

/** Adding a form part: it opens to its height. Removing: it fades first, then its gap settles. */

function Form({ ids }: { ids: string[] }) {
  const leaving = useLeavingRows(ids, (id) => id);
  const added = useAddedKeys(ids);
  return (
    <div>
      {leaving.rows.map((id) => (
        <FormPart key={id} appear={added.has(id)} {...leaving.props(id)}>
          <p>{id}</p>
        </FormPart>
      ))}
    </div>
  );
}

const animationEnd = (el: Element) => act(() => void fireEvent(el, new Event("webkitAnimationEnd", { bubbles: true })));

beforeEach(() => {
  motionInTests();
  Object.defineProperty(Element.prototype, "getAnimations", { value: () => [], configurable: true });
  return () => {
    delete (Element.prototype as { getAnimations?: unknown }).getAnimations;
  };
});

describe("a form part", () => {
  it("is simply there when the form opens with it, and opens when it is added", () => {
    const { rerender } = render(<Form ids={["a"]} />);
    expect(screen.getByText("a").closest(".unfold")).toHaveAttribute("data-phase", "open");

    rerender(<Form ids={["a", "b"]} />);
    expect(screen.getByText("b").closest(".unfold")).toHaveAttribute("data-phase", "opening");
    expect(screen.getByText("a").closest(".unfold")).toHaveAttribute("data-phase", "open");
  });

  it("fades where it was when removed, then its gap closes, and the gap never waits for the pointer", () => {
    const { rerender } = render(<Form ids={["a", "b"]} />);
    rerender(<Form ids={["a"]} />);

    const gone = screen.getByText("b").closest(".close-gap")!;
    expect(screen.getByText("b").closest(".tint-out")).not.toBeNull();
    expect(gone).toBeInTheDocument();

    animationEnd(gone);
    expect(screen.queryByText("b")).not.toBeInTheDocument();
  });
});
