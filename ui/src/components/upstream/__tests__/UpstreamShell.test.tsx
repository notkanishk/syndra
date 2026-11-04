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

  // A box means something is wrong (A-04 §7.1a). The path that skips Syndra's
  // record is a fault in the making: a Notice. The traced path is explained,
  // not warned about: a caption.
  it("is a Notice when Syndra keeps no record, and a caption when it does", () => {
    const { unmount } = render(<DirectWriteWarning what="Anything." />);
    expect(screen.getByRole("status")).toHaveTextContent(/Syndra keeps no record of them/);
    unmount();
    render(<DirectWriteWarning traced what="Anything." />);
    expect(screen.queryByRole("status")).toBeNull();
    expect(screen.getByText(/writes to Zitadel through Syndra/)).toBeInTheDocument();
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

describe("the frame of an upstream console", () => {
  // A permanent "Syndra keeps no record" under the title was false for grants (they are traced),
  // so the shell says nothing of it: each write says its own truth beside the write.
  it("names its parent in the eyebrow and links to Syndra's view, with no standing warning", () => {
    render(
      <UpstreamShell title="Users" lede="Read live." syndraHref="/users" syndraLabel="See Syndra's view">
        <p>rows</p>
      </UpstreamShell>,
    );
    expect(screen.getByRole("heading", { level: 1, name: "Users" })).toBeTruthy();
    expect(screen.getByRole("link", { name: "Zitadel" })).toHaveAttribute("href", "/zitadel");
    expect(screen.getByRole("link", { name: "See Syndra's view" })).toHaveAttribute("href", "/users");
    expect(document.body.textContent).not.toMatch(/keeps no record/);
  });
});
