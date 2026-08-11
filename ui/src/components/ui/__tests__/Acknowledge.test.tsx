// @vitest-environment jsdom
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { AcknowledgeCount, ConfirmByTyping, typedMatches } from "@/components/ui/Acknowledge";

/**
 * §31 B — the two gestures that are not "press the button", held to being the
 * same gesture wherever they appear.
 */

describe("rung 2 — ticking the number", () => {
  // The whole mechanism is that the quantity is INSIDE the sentence being
  // ticked. A checkbox labelled "I understand" beside a figure in another
  // paragraph is one people tick without moving their eyes.
  it("puts the count inside the sentence", () => {
    render(
      <AcknowledgeCount checked={false} onChange={() => {}} count={34} noun="people" />,
    );
    expect(screen.getByText(/I understand this changes/i)).toBeInTheDocument();
    expect(screen.getByText("34 people")).toBeInTheDocument();
  });

  it("agrees with itself about one", () => {
    render(<AcknowledgeCount checked={false} onChange={() => {}} count={1} noun="people" />);
    expect(screen.getByText("1 person")).toBeInTheDocument();
  });

  it("carries the irreversible part when the count is not it", () => {
    render(
      <AcknowledgeCount
        checked={false}
        onChange={() => {}}
        count={41}
        noun="accounts"
        verb="removes"
        consequence="41.2 GB of their files goes with the accounts."
      />,
    );
    expect(screen.getByText(/41.2 GB of their files/)).toBeInTheDocument();
  });
});

describe("rung 3 — typing the name", () => {
  it("matches on trimmed, case-insensitive input", () => {
    expect(typedMatches("ada", "  ADA ")).toBe(true);
    expect(typedMatches("ada", "adam")).toBe(false);
  });

  // An empty expectation must never match, or a dialog rendered before its
  // subject loads arms its own confirm button.
  it("never matches an empty expectation", () => {
    expect(typedMatches("", "")).toBe(false);
    expect(typedMatches("   ", "")).toBe(false);
  });

  it("names what is being typed, in the value's own type", () => {
    const onChange = vi.fn();
    render(
      <ConfirmByTyping expected="legacy-hand-made" noun="account name" value="" onChange={onChange} />,
    );
    expect(screen.getByText(/Type the account name/)).toBeInTheDocument();
    expect(screen.getByText("legacy-hand-made")).toBeInTheDocument();

    fireEvent.change(screen.getByRole("textbox"), { target: { value: "l" } });
    expect(onChange).toHaveBeenCalledWith("l");
  });
});
