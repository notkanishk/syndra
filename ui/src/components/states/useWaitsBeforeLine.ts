"use client";

import { useDegraded } from "@/components/states/DegradedBanner";

/**
 * The line a dialog says BEFORE its act while Zitadel isn't answering (A-07 §9):
 * a person's page, Requests and Expiring stay on, because the change queues
 * safely, and the dialog says so rather than disabling its button. Its seat is
 * the footer's note, ahead of whatever that dialog usually says there. The result
 * keeps "Waiting for you to send →". Drift is not one of these: it stays off.
 */
export const WAITS_BEFORE_LINE = "Zitadel isn't answering, so this will wait for you to send it once it does.";

export function useWaitsBeforeLine(): string | undefined {
  return useDegraded() ? WAITS_BEFORE_LINE : undefined;
}
