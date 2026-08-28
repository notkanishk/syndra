// @vitest-environment jsdom
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { DirectWriteWarning, UpstreamShell } from "@/components/upstream/UpstreamShell";

/**
 * The four upstream consoles write to Zitadel with no rehearsal, no cascade
 * preview and no ledger row. By the ladder's own rule — the rung is set by
 * what cannot be undone, not by how many rows are touched — these are the
 * least undoable actions in the product, and until now they were the only
 * writes in it with no ceremony at all: a paragraph above an armed button.
 */
describe("a write that goes straight to the provider", () => {
  it("names what is missing rather than counting what changes", () => {
    render(
      <DirectWriteWarning what="Anything." acknowledged={false} onAcknowledge={() => {}} />,
    );
    expect(
      screen.getByText(/changes Zitadel now, with no preview and no record in Syndra/),
    ).toBeTruthy();
  });

  it("is a tick, so it can gate the button", () => {
    const onAcknowledge = vi.fn();
    render(<DirectWriteWarning what="Anything." acknowledged={false} onAcknowledge={onAcknowledge} />);
    fireEvent.click(screen.getByRole("checkbox"));
    expect(onAcknowledge).toHaveBeenCalledWith(true);
  });

  // The paragraph on its own is still available for a context that only
  // describes rather than acts — but a missing handler must not silently turn
  // the rung off on a screen that does write.
  it("renders no tick when no handler is given", () => {
    render(<DirectWriteWarning what="Anything." />);
    expect(screen.queryByRole("checkbox")).toBeNull();
  });
});

describe("the standing line on an upstream console", () => {
  // A warning beside each button is met at the moment somebody has already
  // decided, which teaches them to dismiss it. This one is met on arrival.
  it("says where you are before you decide anything", () => {
    render(
      <UpstreamShell title="Users" lede="Read live." syndraHref="/users" syndraLabel="See Syndra's view">
        <p>rows</p>
      </UpstreamShell>,
    );
    expect(screen.getByText(/Changes here go straight to Zitadel \(the service everyone signs in through\), and Syndra keeps no record of them\./)).toBeTruthy();
  });
});
