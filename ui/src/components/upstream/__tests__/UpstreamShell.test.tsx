// @vitest-environment jsdom
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { DirectWriteWarning, UpstreamShell, sendsWhen } from "@/components/upstream/UpstreamShell";

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
    expect(screen.getByText(/goes through Syndra/)).toBeInTheDocument();
  });

  // C3: the traced path writes a record, so its tick never says there is none,
  // and its caption says when the change is sent, never "when Pending changes is sent".
  it("on the traced path, ticks for what is missing and says when it is sent", () => {
    render(<DirectWriteWarning traced what="Anything." sends={sendsWhen([])} acknowledged={false} onAcknowledge={() => {}} />);
    expect(screen.getByText("I understand this changes Zitadel now, with no preview and no reason.")).toBeInTheDocument();
    expect(document.body.textContent).not.toMatch(/no record in Syndra/);
    expect(document.body.textContent).toMatch(/It is sent to Zitadel straight away\./);
    expect(document.body.textContent).not.toMatch(/when Pending changes is sent/);
  });
});

describe("when a direct change is sent (C3)", () => {
  it("says straight away, and names the roles that wait", () => {
    expect(sendsWhen([])).toBe("It is sent to Zitadel straight away.");
    expect(sendsWhen(["trained"])).toBe(
      "It is sent to Zitadel straight away, except trained: that role always waits for you, so its change waits in Pending changes until you send it.",
    );
    expect(sendsWhen(["trained", "keyholder"])).toMatch(/^It is sent to Zitadel straight away, except keyholder and trained: those roles always wait for you/);
  });

  it("before the list of roles that wait is read, names the case without guessing", () => {
    expect(sendsWhen(undefined)).toMatch(/straight away, unless a role waits for you/);
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
