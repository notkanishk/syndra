// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { ConfirmByTyping, typedMatches } from "@/components/ui/Acknowledge";
import { setMediaQuery } from "@/test-utils/media";

const SHORT = "(max-height: 26rem)";

/**
 * A phone in landscape is 844 × 390. Almost all of that needs no special
 * case: 844px of width is already past the tablet breakpoint, so the rail
 * returns and sheets become centred dialogs on their own.
 *
 * The type-the-name gate is the exception, and the rule is a refusal rather
 * than a squeeze.
 */
describe("the rung-3 gate in landscape", () => {
  it("asks for the phone upright rather than squeezing the consequence away", () => {
    setMediaQuery(SHORT, true);
    render(
      <ConfirmByTyping expected="Aditi Rao" value="" onChange={() => {}} noun="person's name" />,
    );

    expect(screen.getByText(/Turn your phone upright to confirm this/)).toBeTruthy();
    // The field is not merely disabled — it is absent, so there is nothing to
    // type into and nothing to arm.
    expect(screen.queryByRole("textbox")).toBeNull();
  });

  // The armed red fill is gated on `typedMatches`, and with no field there is
  // nothing to type — so the control cannot arm from a screen that refuses to
  // offer the gate. This is the property that makes the refusal safe rather
  // than merely polite.
  it("cannot arm, because nothing was typed", () => {
    expect(typedMatches("Aditi Rao", "")).toBe(false);
  });

  it("offers the gate normally when there is room", () => {
    setMediaQuery(SHORT, false);
    render(
      <ConfirmByTyping expected="Aditi Rao" value="" onChange={() => {}} noun="person's name" />,
    );

    expect(screen.getByRole("textbox")).toBeTruthy();
    expect(screen.queryByText(/Turn your phone upright/)).toBeNull();
  });
});
