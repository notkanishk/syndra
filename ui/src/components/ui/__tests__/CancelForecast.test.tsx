// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { PushedStep, useCancelStep } from "@/components/ui/CancelQueuedStep";
import { Modal } from "@/components/ui/Modal";
import { WouldCancelQueued } from "@/lib/api-client";
import type { QueuedChange } from "@/lib/cancel-confirm";
import { FORECAST } from "@/lib/outcome";
import type { Indicators } from "@/lib/queries/useIndicators";

/**
 * A-20 P8: cancelling a queued change forecasts "Will be cancelled" from the
 * moment the operator goes ahead, and the rail's Pending count drops by what
 * was cancelled only once the write has answered.
 */
vi.mock("@/lib/queries/useNameResolver", () => ({
  useNameResolver: () => ({
    resolveUser: () => ({ value: { display_name: "Rohan Mehta", email: "" }, resolved: true }),
    resolveProject: () => ({ value: { name: "Network storage" }, resolved: true }),
    resolveRole: () => ({ value: { display_name: "Member" }, resolved: true }),
    resolveBundle: () => ({ value: undefined, resolved: true }),
  }),
}));

const change = (n: number): QueuedChange => ({
  id: `00000000-0000-0000-0000-${String(n).padStart(12, "0")}`,
  op_type: "revoke",
  user_id: "u1",
  project_id: "pNas",
  role_keys: ["member"],
  source: "direct",
  initiated_by: "op-2",
  said: "left the course",
  queued_at: new Date(Date.now() - 3_600_000).toISOString(),
});

const wire = vi.hoisted(() => ({ settle: null as null | { resolve: () => void; reject: (e: unknown) => void } }));

function Host({ cancels }: { cancels: QueuedChange[] }) {
  const step = useCancelStep({ host: "Give Rohan Mehta a role", goAhead: "give Member" });
  let first = true;
  return (
    <Modal open onClose={() => undefined} size="md" labelledBy="host-title">
      <PushedStep step={step}>
        <h2 id="host-title">Give a role</h2>
        <button
          onClick={() =>
            step
              .run(async () => {
                if (first) {
                  first = false;
                  throw new WouldCancelQueued(cancels, "POST", "/users/u1/grants");
                }
                await new Promise<void>((resolve, reject) => (wire.settle = { resolve, reject }));
              })
              .catch(() => {})
          }
        >
          Send
        </button>
      </PushedStep>
    </Modal>
  );
}

const rail: Indicators = {
  pending_requests: 0,
  expiring_grants: 0,
  pending_propagation: 5,
  drift: 0,
  unconfirmed_revocations: 0,
  holds_due: 0,
  revocations_escalated: false,
  zitadel_reachable: true,
};
let client: QueryClient;
const pendingNow = () => client.getQueryData<Indicators>(["governance", "indicators"])!.pending_propagation;

beforeEach(() => {
  wire.settle = null;
  client = new QueryClient();
  client.setQueryData(["governance", "indicators"], rail);
});
afterEach(() => {
  cleanup();
});

async function goAhead(cancels: QueuedChange[]) {
  render(
    <QueryClientProvider client={client}>
      <Host cancels={cancels} />
    </QueryClientProvider>,
  );
  fireEvent.click(screen.getByRole("button", { name: "Send" }));
  fireEvent.click(await screen.findByRole("button", { name: /^Cancel (giving it|removing it|all \d+) and give Member$/ }));
}

describe("Cancel a queued change: the first 100ms", () => {
  it("says 'Will be cancelled' while out, and the rail has not moved ahead of the answer", async () => {
    await goAhead([change(1), change(2)]);
    expect(await screen.findByText(FORECAST.cancel)).toBeInTheDocument();
    expect(pendingNow()).toBe(5);
    wire.settle!.resolve();
    // Answered: the rail drops by the two cancelled, and the sheet no longer forecasts.
    await waitFor(() => expect(pendingNow()).toBe(3));
  });

  it("an error leaves the rail alone and the forecast gone", async () => {
    await goAhead([change(1)]);
    await screen.findByText(FORECAST.cancel);
    wire.settle!.reject(new Error("boom"));
    await waitFor(() => expect(screen.queryByText(FORECAST.cancel)).toBeNull());
    expect(pendingNow()).toBe(5);
  });
});
