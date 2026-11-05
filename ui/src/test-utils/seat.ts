import { screen, waitFor } from "@testing-library/react";

import { FORECAST } from "@/lib/outcome";

/**
 * The seat's TRUE word. For one beat after a press the seat forecasts in the
 * future tense (A-19 P2: "Will be approved"); a test about what was decided
 * waits past that, because the forecast is not an outcome.
 */
export async function trueSeat(): Promise<HTMLElement> {
  const forecasts: string[] = Object.values(FORECAST);
  return waitFor(() => {
    const seat = screen.getByRole("status");
    if (forecasts.some((word) => seat.textContent?.startsWith(word))) throw new Error("still forecasting");
    return seat;
  });
}
