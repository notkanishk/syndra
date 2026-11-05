"use client";

import { useDegraded } from "@/components/states/DegradedBanner";

/**
 * The line a dialog says BEFORE its act while Zitadel isn't answering (A-07 §9):
 * a person's page, Requests and Expiring stay on, because the change queues
 * safely, and the dialog says so rather than disabling its button. Its seat is
 * the footer's note, ahead of whatever that dialog usually says there. Drift is
 * not one of these: it stays off.
 *
 * Exact per op (owner, 5 Nov 2026). It used to say every change "will wait for
 * you to send it", which was false: a removal goes on its own (the revocation
 * drain), and a give that finds Zitadel down is now retried on its own too
 * (services.Deliver → db.ScheduleRetry). Only a role marked to wait still waits
 * for Send, whatever Zitadel is doing, so the line says that and nothing more.
 */
export type WaitsOp = "give" | "remove" | "change";

const WHAT: Record<WaitsOp, string> = {
  give: "the give",
  remove: "the removal",
  change: "the change",
};

export function waitsBeforeLine(op: WaitsOp): string {
  return (
    `Zitadel isn't answering. Syndra records ${WHAT[op]} now and sends it on its own once Zitadel answers, ` +
    "so there is nothing for you to send. A role marked to wait still waits for you to send it."
  );
}

export function useWaitsBeforeLine(op: WaitsOp): string | undefined {
  return useDegraded() ? waitsBeforeLine(op) : undefined;
}
