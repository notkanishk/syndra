// @vitest-environment jsdom
import { act, fireEvent, render, screen } from "@testing-library/react";
import { StrictMode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { Modal } from "@/components/ui/Modal";
import { setMediaQuery } from "@/test-utils/media";
import { motionInTests } from "@/test-utils/motion";

/**
 * Motion v2 S1 and S3. Tests run still by default (so "gone after Close" is one
 * line later everywhere else); these opt in and drive the clock.
 */

const ghost = () => document.body.querySelector<HTMLElement>(".leave-scrim");

function Sheet({ open = true, busy = false, onClose = () => {} }: { open?: boolean; busy?: boolean; onClose?: () => void }) {
  return (
    <Modal open={open} onClose={onClose} busy={busy} size="sm" labelledBy="t">
      <h2 id="t">Rename this bundle</h2>
      <button type="button">Save</button>
    </Modal>
  );
}

beforeEach(() => {
  // The previous test's own unmount, at cleanup, left a copy of its dialog.
  document.querySelectorAll(".leave-scrim").forEach((copy) => copy.remove());
  vi.useFakeTimers();
  motionInTests();
});
afterEach(() => vi.useRealTimers());

describe("Modal closing (S1)", () => {
  it("leaves a copy that fades and sinks, then the scrim 40ms behind, then nothing", () => {
    const { rerender } = render(<Sheet />);
    expect(ghost()).toBeNull();

    rerender(<Sheet open={false} />);

    // The real dialog is gone at once: nothing can focus it or click it.
    expect(screen.queryByRole("dialog")).toBeNull();
    const copy = ghost()!;
    expect(copy).not.toBeNull();
    expect(copy).toHaveAttribute("aria-hidden", "true");
    expect(copy).toHaveTextContent("Rename this bundle");
    expect(copy.querySelector(".leave-panel")).not.toBeNull();
    // Opening's classes are off it, or its own animation would run again.
    expect(copy.classList.contains("settle-scrim")).toBe(false);
    expect(copy.querySelector(".sheet-in")).toBeNull();
    // Ids would answer a second dialog's aria-labelledby.
    expect(copy.querySelector("[id]")).toBeNull();

    act(() => void vi.advanceTimersByTime(150));
    expect(ghost()).not.toBeNull();
    act(() => void vi.advanceTimersByTime(40));
    expect(ghost()).toBeNull();
  });

  it("does the same when the parent unmounts it while open", () => {
    const { unmount } = render(<Sheet />);
    unmount();
    expect(ghost()).not.toBeNull();
    act(() => void vi.advanceTimersByTime(200));
    expect(ghost()).toBeNull();
  });

  it("leaves nothing behind in reduced motion: both leave together, instantly", () => {
    setMediaQuery("(prefers-reduced-motion: reduce)", true);
    const { rerender } = render(<Sheet />);
    rerender(<Sheet open={false} />);
    expect(ghost()).toBeNull();
  });

  it("does not leave a copy when React only simulates an unmount (strict mode)", () => {
    render(
      <StrictMode>
        <Sheet />
      </StrictMode>,
    );
    expect(ghost()).toBeNull();
    expect(screen.getByRole("dialog")).toBeInTheDocument();
  });

  it("returns focus to the opener at the start of the fade", () => {
    function Page({ open }: { open: boolean }) {
      return (
        <>
          <button type="button">Rename</button>
          <Sheet open={open} />
        </>
      );
    }
    const { rerender } = render(<Page open={false} />);
    const opener = screen.getByRole("button", { name: "Rename" });
    opener.focus();
    rerender(<Page open />);
    expect(opener).not.toHaveFocus();
    rerender(<Page open={false} />);
    expect(opener).toHaveFocus();
  });
});

/** jsdom has PointerEvent, but gives the panel no height; the sheet is 600 tall. */
function sheetOf(onClose: () => void, busy = false) {
  const view = render(<Sheet onClose={onClose} busy={busy} />);
  const dialog = screen.getByRole("dialog");
  const panel = dialog.firstElementChild as HTMLElement;
  vi.spyOn(panel, "getBoundingClientRect").mockReturnValue({ height: 600 } as DOMRect);
  const grabber = busy
    ? screen.getByText(/this sheet can't be closed/).parentElement!
    : screen.getByRole("button", { name: "Close this sheet" });
  return { ...view, dialog, panel, grabber };
}

function drag(grabber: HTMLElement, to: number, over = 300) {
  fireEvent.pointerDown(grabber, { clientY: 100, pointerId: 1 });
  act(() => void vi.advanceTimersByTime(over));
  fireEvent.pointerMove(grabber, { clientY: 100 + to, pointerId: 1 });
  return () => fireEvent.pointerUp(grabber, { clientY: 100 + to, pointerId: 1 });
}

describe("Modal as a phone sheet (S3)", () => {
  it("follows the finger 1:1 and the scrim thins with it", () => {
    const { dialog, panel, grabber } = sheetOf(vi.fn());
    drag(grabber, 150);
    expect(panel.style.transform).toBe("translateY(150px)");
    // 150 of 600 is a quarter: the 55% wash is thinned by a quarter.
    expect(dialog.style.backgroundColor).toBe("rgba(0, 0, 0, 0.413)");
  });

  it("settles back when let go short of a third, and stays open", () => {
    const onClose = vi.fn();
    const { dialog, panel, grabber } = sheetOf(onClose);
    drag(grabber, 150)();
    expect(panel.style.transform).toBe("");
    expect(dialog.style.backgroundColor).toBe("");
    act(() => void vi.advanceTimersByTime(600));
    expect(onClose).not.toHaveBeenCalled();
  });

  it("leaves past a third, on settle's duration, then closes", () => {
    const onClose = vi.fn();
    const { panel, grabber } = sheetOf(onClose);
    drag(grabber, 250)();
    expect(panel.style.transform).toBe("translateY(100%)");
    expect(onClose).not.toHaveBeenCalled();
    act(() => void vi.advanceTimersByTime(260));
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("leaves on a quick flick even short of a third", () => {
    const onClose = vi.fn();
    const { grabber } = sheetOf(onClose);
    drag(grabber, 60, 40)();
    act(() => void vi.advanceTimersByTime(260));
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("leaves no copy of itself behind when the drag already took it away", () => {
    const { rerender, grabber } = sheetOf(() => {});
    drag(grabber, 250)();
    act(() => void vi.advanceTimersByTime(260));
    rerender(<Sheet open={false} />);
    expect(ghost()).toBeNull();
  });

  it("a drag is not a tap: the grabber's click after it does not close", () => {
    const onClose = vi.fn();
    const { grabber } = sheetOf(onClose);
    drag(grabber, 20)();
    fireEvent.click(grabber);
    expect(onClose).not.toHaveBeenCalled();
    fireEvent.click(grabber);
    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("while busy, follows at 15% capped at 10px, settles back, and never closes (7a)", () => {
    const onClose = vi.fn();
    const { panel, grabber } = sheetOf(onClose, true);
    const release = drag(grabber, 40);
    expect(panel.style.transform).toBe("translateY(6px)");
    release();
    expect(panel.style.transform).toBe("");

    drag(grabber, 500)();
    act(() => void vi.advanceTimersByTime(600));
    expect(panel.style.transform).toBe("");
    expect(onClose).not.toHaveBeenCalled();
    drag(grabber, 300, 20);
    expect(panel.style.transform).toBe("translateY(10px)");
  });

  it("in reduced motion a drag does not move it, and the busy sentence tints instead", () => {
    setMediaQuery("(prefers-reduced-motion: reduce)", true);
    const onClose = vi.fn();
    const { panel, grabber } = sheetOf(onClose, true);
    const sentence = screen.getByText(/this sheet can't be closed/);
    expect(sentence).toHaveClass("text-faint");
    drag(grabber, 200);
    expect(panel.style.transform).toBe("");
    expect(sentence).toHaveClass("text-ink");
    act(() => void vi.advanceTimersByTime(700));
    expect(sentence).toHaveClass("text-faint");
    expect(onClose).not.toHaveBeenCalled();
  });
});
