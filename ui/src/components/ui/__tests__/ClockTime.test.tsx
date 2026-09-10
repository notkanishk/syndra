// @vitest-environment jsdom
import { render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { ClockTime } from "@/components/ui/Time";

/**
 * "last checked" has to be when something was checked.
 *
 * It read `new Date()` on mount, so it named the moment the component
 * rendered — a freshness claim sourced from the render loop — and never
 * ticked, so an hour later it still named the minute you arrived. It also
 * formatted with the browser's locale while every other time in the product
 * uses en-GB on a 24-hour clock, so one page said "12:08 AM" where the rest
 * would say "00:08".
 */
describe("the clock under 'last checked'", () => {
  it("shows when the data came back, not when the component rendered", async () => {
    // 09:15 UTC on a fixed day, asserted in UTC so the test does not depend on
    // the machine running it.
    const at = Date.UTC(2026, 8, 10, 9, 15);
    render(<ClockTime at={at} />);

    const expected = new Date(at).toLocaleTimeString("en-GB", {
      hour: "2-digit",
      minute: "2-digit",
      hour12: false,
    });
    await waitFor(() => expect(screen.getByText(expected)).toBeInTheDocument());
  });

  it("uses the product's 24-hour clock, never a 12-hour one", async () => {
    render(<ClockTime at={Date.UTC(2026, 8, 10, 0, 8)} />);
    await waitFor(() => expect(document.body.textContent).not.toMatch(/AM|PM/i));
  });
});
