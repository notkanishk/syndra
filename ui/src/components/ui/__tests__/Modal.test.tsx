// @vitest-environment jsdom
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { Modal } from "@/components/ui/Modal";

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
});
