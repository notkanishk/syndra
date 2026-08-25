// @vitest-environment jsdom
import { render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { ChimeToggle } from "@/components/settings/ChimeToggle";
import { setChimeEnabled } from "@/lib/driftChime";
import { setMediaQuery } from "@/test-utils/media";

const REDUCED = "(prefers-reduced-motion: reduce)";

/**
 * The chime is silenced for anybody who asked for less motion — a sound
 * arriving on its own is movement in the room. That is the right behaviour,
 * and it leaves this control able to say "Sound on" while guaranteeing
 * silence. A toggle that claims to be on while nothing can be heard is a lie
 * the page is in a position to catch, so it catches it.
 */
describe("the chime toggle", () => {
  it("admits it will not play when the browser asked for less motion", async () => {
    setChimeEnabled(true);
    setMediaQuery(REDUCED, true);
    render(<ChimeToggle />);

    expect(await screen.findByRole("switch", { name: /Sound on|Play a sound/i })).toBeTruthy();
    expect(screen.getByText(/Nothing will play/)).toBeTruthy();
  });

  it("says nothing extra when it will play", async () => {
    setChimeEnabled(true);
    setMediaQuery(REDUCED, false);
    render(<ChimeToggle />);

    await waitFor(() => expect(screen.getByRole("switch")).toBeTruthy());
    expect(screen.queryByText(/Nothing will play/)).toBeNull();
  });

  // Off is off. The reduced-motion note explains why an ON toggle is silent,
  // and on an off one it would be explaining something nobody asked.
  it("does not explain the silence of a toggle that is already off", async () => {
    setChimeEnabled(false);
    setMediaQuery(REDUCED, true);
    render(<ChimeToggle />);

    await waitFor(() => expect(screen.getByRole("switch")).toBeTruthy());
    expect(screen.queryByText(/Nothing will play/)).toBeNull();
  });
});
