// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it } from "vitest";

import { FormPart, useAddedKeys } from "@/components/ui/FormPart";
import { useLeavingRows } from "@/components/ui/LeavingRows";
import { gapSettled } from "@/test-utils/leaving";
import { motionInTests } from "@/test-utils/motion";

/** Adding a form part: it opens to its height. Removing: it fades first, then its gap settles. */

function Form({ ids, narrowedBy }: { ids: string[]; narrowedBy?: string }) {
  const leaving = useLeavingRows(ids, (id) => id, true, { narrowedBy });
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

    const gone = screen.getByText("b").closest("[data-closing]")!;
    expect(screen.getByText("b").closest("[data-leaving]")).not.toBeNull();
    expect(gone).toBeInTheDocument();

    gapSettled(gone);
    expect(screen.queryByText("b")).not.toBeInTheDocument();
  });

  it("a part the reader added opens by its own Unfold only; one brought back opens as a returning row", () => {
    const { rerender } = render(<Form ids={["a", "b"]} narrowedBy="0" />);
    rerender(<Form ids={["a"]} narrowedBy="1" />);
    gapSettled(screen.getByText("b").closest("[data-closing]")!);
    // Added with a Discard just before it (the reader's window is open): the Unfold is its opening.
    rerender(<Form ids={["a", "c"]} narrowedBy="2" />);
    expect(screen.getByText("c").closest(".leave-row")).not.toHaveAttribute("data-opens");
    expect(screen.getByText("c").closest(".unfold")).toHaveAttribute("data-phase", "opening");
    // Brought back: it is not `appear`ing, so the row's own gap opens and the Unfold has nothing to do.
    rerender(<Form ids={["a", "c", "b"]} narrowedBy="3" />);
    expect(screen.getByText("b").closest(".leave-row")).toHaveAttribute("data-opens");
  });
});
