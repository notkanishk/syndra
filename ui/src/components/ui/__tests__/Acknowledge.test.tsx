// @vitest-environment jsdom
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { AcknowledgeCount, ConfirmByTyping, typedMatches, typingWarmth } from "@/components/ui/Acknowledge";

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

// S16: each matching letter warms the field's border one step toward danger,
// 70% at the full name; a wrong letter returns it to neutral, with no shake.
describe("S16 typing the name to confirm", () => {
  const field = (value: string) => {
    render(<ConfirmByTyping expected="legacy-hand-made" value={value} onChange={() => {}} />);
    return screen.getByRole("textbox");
  };

  it("warms one step per matching letter, reaching 70 at the full name", () => {
    expect(typingWarmth("legacy-hand-made", "")).toBe(0);
    expect(typingWarmth("legacy-hand-made", "l")).toBeGreaterThan(0);
    expect(typingWarmth("legacy-hand-made", "leg")).toBeGreaterThan(typingWarmth("legacy-hand-made", "le"));
    expect(typingWarmth("legacy-hand-made", "legacy-hand-made")).toBe(70);
    // Case and surrounding space are forgiven, as the match itself is.
    expect(typingWarmth("legacy-hand-made", "  LEGACY-hand-made ")).toBe(70);
  });

  it("a wrong letter returns it to neutral", () => {
    expect(typingWarmth("legacy-hand-made", "lex")).toBe(0);
    expect(typingWarmth("legacy-hand-made", "legacy-hand-madee")).toBe(0);
  });

  it("an empty name never warms", () => {
    expect(typingWarmth("", "")).toBe(0);
  });

  it("the field carries the state: a prefix warms it, a mismatch clears it, the full name arms it", () => {
    const prefix = field("lega");
    expect(prefix).toHaveAttribute("data-warmth", String(typingWarmth("legacy-hand-made", "lega")));
    expect(prefix.style.borderColor).toContain("var(--danger)");
    expect(prefix).not.toHaveAttribute("data-armed");
    cleanup();

    const wrong = field("lex");
    expect(wrong).toHaveAttribute("data-warmth", "0");
    expect(wrong.style.borderColor).toBe("");
    cleanup();

    const full = field("legacy-hand-made");
    expect(full).toHaveAttribute("data-warmth", "70");
    expect(full).toHaveAttribute("data-armed", "true");
    expect(full.style.borderColor).toContain("70%");
  });
});
