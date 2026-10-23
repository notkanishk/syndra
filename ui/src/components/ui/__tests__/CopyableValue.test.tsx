// @vitest-environment jsdom
import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { ClipboardUnavailableNote, CopyableValue } from "@/components/ui/CopyableValue";

/**
 * The deployment this ships to is reached over http on a LAN, where
 * `navigator.clipboard` does not exist. A Copy button there is an affordance
 * that does nothing, and the member it fails for is the one standing in the
 * space trying to type a share path into a file manager.
 */

function withClipboard(writeText: () => Promise<void>) {
  Object.defineProperty(navigator, "clipboard", {
    value: { writeText },
    configurable: true,
  });
}

function withoutClipboard() {
  Object.defineProperty(navigator, "clipboard", { value: undefined, configurable: true });
}

afterEach(() => {
  withoutClipboard();
});

describe("a value somebody has to transport", () => {
  it("copies, and confirms in place rather than in a corner", async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    withClipboard(writeText);
    render(<CopyableValue value="smb://fileserver-01/studio" label="Share path" />);

    fireEvent.click(await screen.findByRole("button", { name: "Copy share path" }));
    await waitFor(() => expect(screen.getByRole("button", { name: "Copied share path" })).toBeTruthy());
    expect(writeText).toHaveBeenCalledWith("smb://fileserver-01/studio");
  });

  // S18: the seat answers with a lime dot that blooms once. A selection (the
  // insecure-origin form) did not go anywhere, so it doesn't bloom.
  it("blooms a healthy dot in its own seat once it has copied", async () => {
    withClipboard(vi.fn().mockResolvedValue(undefined));
    render(<CopyableValue value="smb://fileserver-01/studio" label="Share path" />);

    const button = await screen.findByRole("button", { name: "Copy share path" });
    expect(button.querySelector(".bloom")).toBeNull();
    fireEvent.click(button);
    await waitFor(() => expect(screen.getByRole("button", { name: "Copied share path" })).toBeTruthy());
    const done = screen.getByRole("button", { name: "Copied share path" });
    expect(done.querySelector(".bloom.bg-healthy")).not.toBeNull();
    expect(done).toHaveTextContent("Copied");
  });

  // S18: Copy and Copied share one seat, sized to the wider label, so the button never changes
  // width; the words crossfade on tint and it is back to Copy after 900ms.
  it("keeps both labels in one seat and crossfades them, then goes back after 900ms", async () => {
    withClipboard(vi.fn().mockResolvedValue(undefined));
    render(<CopyableValue value="smb://fileserver-01/studio" label="Share path" />);

    const button = await screen.findByRole("button", { name: "Copy share path" });
    const copy = screen.getByText("Copy");
    const copied = screen.getByText("Copied");
    expect(copy).toHaveClass("opacity-100");
    expect(copied).toHaveClass("opacity-0");
    expect(copy.parentElement).toBe(copied.parentElement);

    vi.useFakeTimers();
    try {
      await act(async () => {
        fireEvent.click(button);
      });
      expect(copy).toHaveClass("opacity-0");
      expect(copied).toHaveClass("opacity-100");
      act(() => void vi.advanceTimersByTime(900));
      expect(copy).toHaveClass("opacity-100");
      expect(copied).toHaveClass("opacity-0");
      expect(button.querySelector(".bloom")).toBeNull();
    } finally {
      vi.useRealTimers();
    }
  });

  // The row knows before it is tapped, because the answer is knowable before
  // it is tapped. Offering Copy and failing on the tap teaches a member that
  // the page is broken.
  it("offers Select instead of Copy where the browser cannot copy", async () => {
    withoutClipboard();
    render(<CopyableValue value="smb://fileserver-01/studio" label="Share path" />);

    await waitFor(() => expect(screen.getByRole("button", { name: "Select share path" })).toBeTruthy());
    expect(screen.queryByRole("button", { name: /^Copy/ })).toBeNull();
  });

  it("puts the value under the platform's own copy gesture when tapped", async () => {
    withoutClipboard();
    const selectNodeContents = vi.fn();
    const removeAllRanges = vi.fn();
    const addRange = vi.fn();
    vi.spyOn(document, "createRange").mockReturnValue({
      selectNodeContents,
    } as unknown as Range);
    vi.spyOn(window, "getSelection").mockReturnValue({
      removeAllRanges,
      addRange,
    } as unknown as Selection);

    render(<CopyableValue value="req_9c14e" label="Request id" />);
    fireEvent.click(await screen.findByRole("button", { name: "Select request id" }));

    expect(selectNodeContents).toHaveBeenCalled();
    expect(addRange).toHaveBeenCalled();
    expect(screen.getByRole("button", { name: "Selected request id" })).toBeTruthy();
    vi.restoreAllMocks();
  });

  // A clipboard that exists and then refuses at the moment of the tap — a
  // permission prompt declined, say — must not leave the operator holding a
  // control that appeared to do nothing.
  it("falls back to selection when a clipboard refuses mid-tap", async () => {
    withClipboard(vi.fn().mockRejectedValue(new Error("denied")));
    render(<CopyableValue value="tok_4c9e2f10" label="Token" />);

    fireEvent.click(await screen.findByRole("button", { name: "Copy token" }));
    await waitFor(() => expect(screen.getByRole("button", { name: /Select/ })).toBeTruthy());
  });

  // Half a share path is worse than none, because it looks complete.
  it("wraps a long value rather than truncating it", () => {
    render(<CopyableValue value={"smb://fileserver-01/very/long/".repeat(4)} label="Path" />);
    const code = screen.getByText(/smb:/);
    expect(code.className).toContain("break-all");
    expect(code.className).not.toContain("truncate");
  });

  it("gives the affordance a real target", () => {
    render(<CopyableValue value="x" label="X" />);
    expect(screen.getByRole("button").className).toContain("min-h-[44px]");
  });
});

describe("the note under a page of copy rows", () => {
  it("says nothing at all where copying works", async () => {
    withClipboard(vi.fn());
    const { container } = render(<ClipboardUnavailableNote />);
    await waitFor(() => expect(container.textContent).toBe(""));
  });

  it("explains once, without reading as an error", async () => {
    withoutClipboard();
    render(<ClipboardUnavailableNote />);
    expect(await screen.findByText(/Tap a value to select it, then hold to copy/)).toBeTruthy();
  });
});
