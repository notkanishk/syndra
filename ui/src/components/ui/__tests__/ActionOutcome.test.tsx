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

  it("seat: a refusal shows its message, as an alert", () => {
    render(
      <ActionOutcome
        placement="seat"
        outcome={{ kind: "refused", message: "That plan is no longer current" }}
      />,
    );
    expect(screen.getByRole("alert")).toHaveTextContent("That plan is no longer current.");
  });

  it("seat: queued shows the link to Pending changes", () => {
    render(<ActionOutcome placement="seat" word="Approved" outcome={queued} />);
    expect(screen.getByText("Approved")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Waiting for you to send/ })).toHaveAttribute(
      "href",
      "/governance/pending",
    );
  });
});
