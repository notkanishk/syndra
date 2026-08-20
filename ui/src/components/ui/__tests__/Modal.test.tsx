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

  // The grabber is a redundant pointer affordance, and it is the panel's first
  // child. Left in the tab order it takes the focus the trap gives on open, so
  // every dialog in the product opens with the cursor on "get rid of this"
  // rather than on the thing the operator came to do.
  it("never opens a dialog with focus on its own dismissal", async () => {
    render(
      <Modal open={true} onClose={() => {}}>
        <button type="button">First</button>
      </Modal>,
    );
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(screen.getByRole("button", { name: "First" })).toHaveFocus();
    expect(screen.getByRole("button", { name: "Dismiss" }).tabIndex).toBe(-1);
  });

  // A rehearsal's done step carries a named "Close". Two controls answering to
  // one accessible name make "this dialog is finished" and "this sheet has a
  // handle" indistinguishable to anything querying by name.
  it("does not name its handle after any control a dialog already has", () => {
    render(
      <Modal open={true} onClose={() => {}}>
        <button type="button">Close</button>
      </Modal>,
    );
    expect(screen.getAllByRole("button", { name: "Close" })).toHaveLength(1);
  });

  it("says why it cannot be dismissed while an action is in flight", () => {
    render(
      <Modal open={true} onClose={() => {}} busy>
        <button type="button">Inside</button>
      </Modal>,
    );
    // Silently ignoring a drag or a tap reads as a frozen app, and the
    // operator's next move is to reload the page mid-mutation.
    expect(screen.getByText("Working — this can't be closed yet.")).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Dismiss" })).toBeNull();
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

/**
 * The rule the sheet work turned into a property.
 *
 * Two dialogs at once is a desktop nuisance and a phone failure: two scrims,
 * two focus traps arguing over Tab, and a dismiss whose meaning depends on
 * which panel is on top. Nothing nests one today — this is what keeps that
 * true after the next screen is written.
 */
describe("sheets push, they do not stack", () => {
  it("complains loudly when a dialog opens inside a dialog", () => {
    const complaint = vi.spyOn(console, "error").mockImplementation(() => {});
    render(
      <Modal open onClose={() => {}} labelledBy="outer">
        <Modal open onClose={() => {}} labelledBy="inner">
          <p>second</p>
        </Modal>
      </Modal>,
    );
    expect(complaint).toHaveBeenCalledWith(expect.stringContaining("do not stack"));
    complaint.mockRestore();
  });

  // It complains; it does not withhold. Refusing to render would trade a
  // layout problem for a missing confirmation, which is the worse of the two.
  it("still renders the inner one rather than swallowing a confirmation", () => {
    const complaint = vi.spyOn(console, "error").mockImplementation(() => {});
    render(
      <Modal open onClose={() => {}} labelledBy="outer">
        <Modal open onClose={() => {}} labelledBy="inner">
          <p>second</p>
        </Modal>
      </Modal>,
    );
    expect(screen.getByText("second")).toBeTruthy();
    complaint.mockRestore();
  });

  it("says nothing about a single dialog", () => {
    const complaint = vi.spyOn(console, "error").mockImplementation(() => {});
    render(
      <Modal open onClose={() => {}} labelledBy="only">
        <p>alone</p>
      </Modal>,
    );
    expect(complaint).not.toHaveBeenCalled();
    complaint.mockRestore();
  });
});
