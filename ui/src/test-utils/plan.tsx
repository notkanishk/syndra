import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactElement } from "react";

import type { QueuedChange } from "@/lib/cancel-confirm";
import type { ActPlan } from "@/lib/usePlanConsent";

/** An ActPlan that applies and cancels nothing; a test overrides what it is about. */
export const actPlan = (over: Partial<ActPlan> = {}): ActPlan => ({
  effect: "apply",
  detail: "",
  said: [],
  rules: [],
  delivery: "straight_away",
  waits: [],
  goes: [],
  cancels: [],
  fingerprint: "fp-1",
  cancel_ids: [],
  ...over,
});

/** A change waiting for someone to send, as a plan's `cancels` carries it. */
export const queuedChange = (over: Partial<QueuedChange> = {}): QueuedChange => ({
  id: "00000000-0000-0000-0000-0000000000a1",
  op_type: "revoke",
  user_id: "u1",
  project_id: "p1",
  role_keys: ["operator"],
  source: "direct",
  initiated_by: "op-2",
  initiated_by_name: "Sam Lee",
  said: "left the course",
  queued_at: new Date(Date.now() - 2 * 3_600_000).toISOString(),
  ...over,
});

/** A plan whose cancels are `changes`, with the ids the write carries back. */
export const cancelling = (changes: QueuedChange[], over: Partial<ActPlan> = {}): ActPlan =>
  actPlan({ cancels: changes, cancel_ids: changes.map((c) => c.id), ...over });

/** A fresh cache that never retries: a failed plan read is failed at once. */
export const planClient = () => new QueryClient({ defaultOptions: { queries: { retry: false } } });

export const withPlanClient = (ui: ReactElement, client = planClient()) => (
  <QueryClientProvider client={client}>{ui}</QueryClientProvider>
);
