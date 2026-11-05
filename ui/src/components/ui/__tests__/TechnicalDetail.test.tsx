// @vitest-environment jsdom
import { fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { CopyableValue } from "@/components/ui/CopyableValue";
import { TechnicalDetail, TechnicalPayload } from "@/components/ui/TechnicalDetail";

/** A-20 P11: the one disclosure for the machine's words. */
afterEach(() => sessionStorage.clear());
// A Back is what hands a place back; the store's own gate is exercised in place.test.
vi.mock("@/lib/motion", async (orig) => ({ ...(await orig<typeof import("@/lib/motion")>()), returnedToThisPage: () => true }));

describe("TechnicalDetail", () => {
  it("is closed by default: nothing inside is on the page until it is opened", () => {
    render(
      <TechnicalDetail>
        <CopyableValue value="zitadel.grant_added" label="Event type" />
        <TechnicalPayload>{'{"id":"abc"}'}</TechnicalPayload>
      </TechnicalDetail>,
    );
    const fold = screen.getByRole("button", { name: /Technical detail/ });
    expect(fold).toHaveAttribute("aria-expanded", "false");
    expect(screen.getByText("for whoever runs the Syndra server")).toBeInTheDocument();
    expect(screen.queryByText("zitadel.grant_added")).toBeNull();
    expect(screen.queryByText(/"id"/)).toBeNull();

    fireEvent.click(fold);
    expect(fold).toHaveAttribute("aria-expanded", "true");
    expect(screen.getByText("zitadel.grant_added")).toBeInTheDocument();
    expect(screen.getByText(/"id"/)).toBeInTheDocument();
  });

  it("copies a value from inside it", async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", { value: { writeText }, configurable: true });
    render(
      <TechnicalDetail>
        <CopyableValue value="op-123" label="Operation id" />
      </TechnicalDetail>,
    );
    fireEvent.click(screen.getByRole("button", { name: /Technical detail/ }));
    fireEvent.click(await screen.findByRole("button", { name: /Copy/i }));
    expect(writeText).toHaveBeenCalledWith("op-123");
  });

  it("can be driven from outside, and tells its owner", () => {
    const onToggle = vi.fn();
    const { rerender } = render(
      <TechnicalDetail open={false} onToggle={onToggle}>
        <span>inside</span>
      </TechnicalDetail>,
    );
    fireEvent.click(screen.getByRole("button", { name: /Technical detail/ }));
    expect(onToggle).toHaveBeenCalledWith(true);
    expect(screen.queryByText("inside")).toBeNull();
    rerender(
      <TechnicalDetail open onToggle={onToggle}>
        <span>inside</span>
      </TechnicalDetail>,
    );
    expect(screen.getByText("inside")).toBeInTheDocument();
  });

  it("keeps a placed fold open across a remount (Back)", () => {
    const { unmount } = render(
      <TechnicalDetail placeKey="k">
        <span>inside</span>
      </TechnicalDetail>,
    );
    fireEvent.click(screen.getByRole("button", { name: /Technical detail/ }));
    unmount();
    render(
      <TechnicalDetail placeKey="k">
        <span>inside</span>
      </TechnicalDetail>,
    );
    expect(screen.getByText("inside")).toBeInTheDocument();
  });
});
