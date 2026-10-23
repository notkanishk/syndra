// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { ActionOutcome } from "@/components/ui/ActionOutcome";

const queued = { kind: "queued" as const, message: "Given in Syndra", detail: "It waits." };

describe("ActionOutcome", () => {
  it("says a queued block outcome as the link to send it, never as 'Waiting to be sent'", () => {
    render(<ActionOutcome outcome={queued} />);
    expect(screen.getByRole("link", { name: /Waiting for you to send/ })).toHaveAttribute(
      "href",
      "/governance/pending",
    );
    expect(screen.queryByText(/Waiting to be sent/)).not.toBeInTheDocument();
  });

  it("seat: wears the row's own word and the detail when applied", () => {
    render(
      <ActionOutcome
        placement="seat"
        word="Approved"
        outcome={{ kind: "applied", message: "Done", detail: "Ada now holds Operator." }}
      />,
    );
    expect(screen.getByText("Approved")).toBeInTheDocument();
    expect(screen.getByText("Ada now holds Operator.")).toBeInTheDocument();
    expect(screen.getByRole("status")).toBeInTheDocument();
  });

  it("seat: blooms only for what went well; queued and failed stay steady (S4)", () => {
    const { container, rerender } = render(
      <ActionOutcome placement="seat" word="Approved" outcome={{ kind: "applied", message: "Done" }} />,
    );
    expect(container.querySelector(".bloom")).not.toBeNull();
    rerender(<ActionOutcome placement="seat" word="Approved" outcome={queued} />);
    expect(container.querySelector(".bloom")).toBeNull();
    rerender(<ActionOutcome placement="seat" outcome={{ kind: "failed", message: "Nope" }} />);
    expect(container.querySelector(".bloom")).toBeNull();
  });

  it("seat: a refusal shows its message, as an alert", () => {
    render(
      <ActionOutcome
        placement="seat"
        outcome={{ kind: "refused", message: "That plan is no longer current" }}
      />,
    );
    expect(screen.getByRole("alert")).toHaveTextContent("That plan is no longer current.");
  });

  it("seat: queued says why it waits, then the link to Pending changes", () => {
    render(<ActionOutcome placement="seat" word="Approved" outcome={queued} />);
    expect(screen.getByText("Approved")).toBeInTheDocument();
    // The faint why line the board draws between the word and the link.
    expect(screen.getByText("It waits.")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Waiting for you to send/ })).toHaveAttribute(
      "href",
      "/governance/pending",
    );
  });
});

describe("block form with a word", () => {
  const done = { kind: "applied" as const, message: "Rohan can use the laser cutter on his own now" };

  it("is a dot, the word in the display face, one sentence, one faint line", () => {
    render(<ActionOutcome word="Given" outcome={{ ...done, detail: "He was in no other room." }} />);
    const status = screen.getByRole("status");
    const word = screen.getByText("Given");
    expect(word.className).toContain("font-display");
    expect(word.className).toContain("text-[22px]");
    expect(screen.getByText("Rohan can use the laser cutter on his own now.")).toBeInTheDocument();
    expect(screen.getByText("He was in no other room.").className).toContain("text-faint");
    // The old pill is gone: the word is the status.
    expect(status).not.toHaveTextContent("Applied");
  });

  it("done now: healthy dot that blooms once, and the result lifts into place", () => {
    const { container } = render(<ActionOutcome word="Given" outcome={done} />);
    const dot = container.querySelector(".bg-healthy");
    expect(dot).not.toBeNull();
    expect(dot!.className).toContain("bloom");
    expect(screen.getByRole("status").className).toContain("lift-in");
  });

  it("queued: amber dot, no bloom, and the link to Pending changes", () => {
    const { container } = render(<ActionOutcome word="Given" outcome={queued} />);
    const dot = container.querySelector(".bg-warn");
    expect(dot).not.toBeNull();
    expect(dot!.className).not.toContain("bloom");
    expect(screen.getByRole("link", { name: /Waiting for you to send/ })).toHaveAttribute(
      "href",
      "/governance/pending",
    );
    expect(screen.getByText("It waits.")).toBeInTheDocument();
  });

  it("asked or not sent: a neutral dot, and no claim that anything was applied", () => {
    const { container } = render(
      <ActionOutcome word="Asked" outcome={{ kind: "no_change", message: "Makerspace staff decide" }} />,
    );
    expect(container.querySelector(".bg-faint")).not.toBeNull();
    expect(container.querySelector(".bloom")).toBeNull();
    expect(screen.getByText("Asked")).toBeInTheDocument();
    expect(screen.queryByText("Applied")).not.toBeInTheDocument();
  });

  it("failed: a steady danger dot, an alert, the request id, and nothing lifts or blooms", () => {
    const { container } = render(
      <ActionOutcome
        word="Not given"
        outcome={{ kind: "failed", message: "Zitadel did not answer", requestId: "req-1" }}
      />,
    );
    const alert = screen.getByRole("alert");
    expect(alert.className).not.toContain("lift-in");
    expect(container.querySelector(".bg-danger")).not.toBeNull();
    expect(container.querySelector(".bloom")).toBeNull();
    expect(alert).toHaveTextContent("Zitadel did not answer. Nothing was changed.");
    expect(screen.getByText("req-1")).toBeInTheDocument();
  });

  it("without a word it is the block it was", () => {
    render(<ActionOutcome outcome={done} />);
    expect(screen.getByText("Applied")).toBeInTheDocument();
  });
});

// N21: a session that ended mid-action is recovered in a new tab, so this one keeps its work.
describe("a session that ended before the action ran", () => {
  it("offers to sign in again in a new tab, and says this tab stays", () => {
    render(
      <ActionOutcome
        outcome={{ kind: "refused", message: "Your session ended before this ran", sessionEnded: true }}
      />,
    );
    const link = screen.getByRole("link", { name: /Sign in again in a new tab/ });
    expect(link).toHaveAttribute("href", "/auth/zitadel?then=close");
    expect(link).toHaveAttribute("target", "_blank");
    expect(screen.getByText(/This tab stays as it is/)).toBeInTheDocument();
  });
});
