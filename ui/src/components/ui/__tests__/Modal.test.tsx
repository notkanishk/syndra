// @vitest-environment jsdom
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { Modal, ModalFooter } from "@/components/ui/Modal";

describe("Modal", () => {
  it("does not render when open=false", () => {
    const onClose = vi.fn();
    render(
      <Modal open={false} onClose={onClose}>
        <button type="button">Inside</button>
      </Modal>,
    );
    expect(screen.queryByRole("dialog")).toBeNull();
  });

  it("renders with role=dialog and aria-modal=true when open", () => {
    render(
      <Modal open={true} onClose={() => {}} labelledBy="t" describedBy="d">
        <h2 id="t">Title</h2>
        <p id="d">Description</p>
        <button type="button">Action</button>
      </Modal>,
    );
    const dialog = screen.getByRole("dialog");
    expect(dialog).toHaveAttribute("aria-modal", "true");
    expect(dialog).toHaveAttribute("aria-labelledby", "t");
    expect(dialog).toHaveAttribute("aria-describedby", "d");
  });

  it("focuses the first focusable element on open (focus trap entry)", async () => {
    render(
      <Modal open={true} onClose={() => {}}>
        <button type="button">First</button>
        <button type="button">Second</button>
      </Modal>,
    );
    // Focus shifts asynchronously inside useEffect after mount.
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(screen.getByRole("button", { name: "First" })).toHaveFocus();
  });

  it("dismisses on Escape", () => {
    const onClose = vi.fn();
    render(
      <Modal open={true} onClose={onClose}>
        <button type="button">Inside</button>
      </Modal>,
    );
    fireEvent.keyDown(document, { key: "Escape" });
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("does not dismiss on Escape when busy", () => {
    const onClose = vi.fn();
    render(
      <Modal open={true} onClose={onClose} busy>
        <button type="button">Inside</button>
      </Modal>,
    );
    fireEvent.keyDown(document, { key: "Escape" });
    expect(onClose).not.toHaveBeenCalled();
  });

  it("dismisses on click-outside (scrim)", () => {
    const onClose = vi.fn();
    render(
      <Modal open={true} onClose={onClose}>
        <button type="button">Inside</button>
      </Modal>,
    );
    const dialog = screen.getByRole("dialog");
    // Clicking the scrim (target === currentTarget) should dismiss.
    fireEvent.click(dialog, { target: dialog });
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("does not dismiss on click-outside when busy", () => {
    const onClose = vi.fn();
    render(
      <Modal open={true} onClose={onClose} busy>
        <button type="button">Inside</button>
      </Modal>,
    );
    const dialog = screen.getByRole("dialog");
    fireEvent.click(dialog, { target: dialog });
    expect(onClose).not.toHaveBeenCalled();
  });

  it("does not dismiss when clicking inside the panel", () => {
    const onClose = vi.fn();
    render(
      <Modal open={true} onClose={onClose}>
        <button type="button">Inside</button>
      </Modal>,
    );
    const inside = screen.getByRole("button", { name: "Inside" });
    fireEvent.click(inside);
    expect(onClose).not.toHaveBeenCalled();
  });

  // jsdom loads no stylesheets and computes no layout, so a tall dialog cannot
  // be measured here — `getBoundingClientRect` is zero for everything. What can
  // be asserted is the contract that makes the overflow reachable: the panel is
  // bounded and scrolls, rather than clipping. The bug this replaces clipped a
  // long plan and took the confirm button with it, silently, with no scrollbar
  // to admit it.
  it("bounds the panel to the viewport and scrolls rather than clipping", () => {
    render(
      <Modal open={true} onClose={() => {}}>
        <button type="button">Inside</button>
      </Modal>,
    );
    const panel = screen.getByRole("button", { name: "Inside" }).parentElement!;
    expect(panel.className).toContain("overflow-y-auto");
    expect(panel.className).toContain("max-h-[calc(100dvh-32px)]");
    expect(panel.className, "clipping is what hid the footer").not.toContain("overflow-hidden");
  });

  it("keeps the footer in reach when the panel scrolls", () => {
    render(
      <Modal open={true} onClose={() => {}}>
        <ModalFooter>
          <button type="button">Confirm</button>
        </ModalFooter>
      </Modal>,
    );
    const footer = screen.getByRole("button", { name: "Confirm" }).parentElement!.parentElement!;
    expect(footer.className).toContain("sticky");
    expect(footer.className).toContain("bottom-0");
    // Opaque, or scrolled content shows through the actions sitting over it.
    expect(footer.className).toContain("bg-surface-2");
  });
});
