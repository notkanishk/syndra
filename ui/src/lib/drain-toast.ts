import { toast } from "sonner";

import { describeDrain } from "@/lib/drain-outcome";
import type { DrainResult } from "@/lib/queries/usePropagation";

/**
 * Both "Resume now" buttons report the same drain the same way. Two call sites
 * phrasing one result differently is how "0 applied, 0 failed" and "Queued
 * writes resumed." ended up describing the same pass.
 */
export function toastDrain(result: DrainResult | undefined): void {
  const { tone, message, detail } = describeDrain(result);
  toast[tone](message, detail ? { description: detail } : undefined);
}
