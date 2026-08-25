// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { HoldDialog } from "@/components/review/HoldDialog";
import { TakeAwayDialog } from "@/components/targets/TakeAwayDialog";
import type { RevocationResult } from "@/lib/queries/useHolds";

/**
 * §27 and §25 — pause it, or end it. The two answers to one question, and the
 * ceremony sized to each.
 */

const state: {
  revoked: Array<{ reason: string; reviewDate?: string }>;
  held: Array<Record<string, unknown>>;
  result: RevocationResult | null;
} = { revoked: [], held: [], result: null };

vi.mock("@/lib/queries/useHolds", async () => {
  const actual = await vi.importActual<typeof import("@/lib/queries/useHolds")>(
    "@/lib/queries/useHolds",
  );
  return {
    ...actual,
    useRevokeTargetAccess: () => ({
      mutate: (input: { reason: string; reviewDate?: string }, opts?: { onSuccess?: (r: RevocationResult) => void }) => {
        state.revoked.push(input);
        if (state.result) opts?.onSuccess?.(state.result);
      },
      isPending: false,
      error: null,
    }),
    useCreateHold: () => ({
      mutate: (input: Record<string, unknown>, opts?: { onSuccess?: () => void }) => {
        state.held.push(input);
        opts?.onSuccess?.();
      },
      isPending: false,
      error: null,
    }),
  };
});

function renderTakeAway() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <TakeAwayDialog target="truenas" subjectId="u1" subjectName="Ada Rivera" onClose={() => {}} />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  state.revoked = [];
  state.held = [];
  state.result = null;
});

describe("taking access away on a target", () => {
  // The sentence is fixed by the backend and shown verbatim. A UI implying the
  // access is gone the moment the button is pressed is the exact failure that
  // endpoint's whole design exists to prevent.
  it("says what it cannot do, before the form", () => {
    renderTakeAway();
    expect(
      screen.getByText(/Sessions already established end when they next reconnect/),
    ).toBeInTheDocument();
    expect(screen.getByText(/this target has no way to close one/)).toBeInTheDocument();
  });

  // What it does, not what it means. Everything on these screens queues, and
  // this button says so rather than claiming the access is gone.
  it("names the queue in the button", () => {
    renderTakeAway();
    expect(screen.getByRole("button", { name: /queue the revocation/i })).toBeInTheDocument();
  });

  // Rung 3, the same gesture as every other revocation: muscle memory must not
  // depend on which one an operator is doing.
  it("will not fire until the name is typed and a reason given", () => {
    renderTakeAway();
    const confirm = screen.getByRole("button", { name: /queue the revocation/i });
    expect(confirm).toBeDisabled();

    fireEvent.change(screen.getByLabelText(/Shown to them on their own page/), { target: { value: "offboarding" } });
    expect(confirm).toBeDisabled();

    fireEvent.change(screen.getByRole("textbox", { name: /type the person's name/i }), {
      target: { value: "ada rivera" },
    });
    expect(confirm).toBeEnabled();

    fireEvent.click(confirm);
    expect(state.revoked).toHaveLength(1);
    expect(state.revoked[0].reason).toBe("offboarding");
  });

  // Half of it going through is its own outcome, and the surface names which
  // half is outstanding rather than reporting a failure.
  it("reports a partial revocation as a partial one", () => {
    state.result = {
      status: "partially_revoked",
      allowance_id: "a1",
      rotated: false,
      detail: "New connections are refused now — that half is recorded and queued.",
      outstanding: "password.rotate",
    };
    renderTakeAway();

    fireEvent.change(screen.getByLabelText(/Shown to them on their own page/), { target: { value: "offboarding" } });
    fireEvent.change(screen.getByRole("textbox", { name: /type the person's name/i }), {
      target: { value: "Ada Rivera" },
    });
    fireEvent.click(screen.getByRole("button", { name: /queue the revocation/i }));

    expect(screen.getByText(/Half of it went through/i)).toBeInTheDocument();
    expect(screen.getByText(/Still outstanding: password.rotate/)).toBeInTheDocument();
  });
});

describe("putting a hold on", () => {
  function renderHold() {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    return render(
      <QueryClientProvider client={client}>
        <HoldDialog
          subjectId="u1"
          subjectName="Ada Rivera"
          target="truenas"
          field="enabled"
          value="true"
          label="access to this target"
          onClose={() => {}}
        />
      </QueryClientProvider>,
    );
  }

  // Framed as what happens if nobody comes back, which is the actual decision.
  it("asks how it ends rather than for an expiry", () => {
    renderHold();
    expect(screen.getByText(/How it ends/)).toBeInTheDocument();
    expect(screen.getByText(/It stays until somebody decides/)).toBeInTheDocument();
    expect(screen.getByText(/It lifts itself on a date/)).toBeInTheDocument();
  });

  // Two bounded forms and no third: a hold with neither is refused by the
  // backend, so the UI does not offer one.
  it("requires a date and a reason either way", () => {
    renderHold();
    const confirm = screen.getByRole("button", { name: /hold access to this target/i });
    expect(confirm).toBeDisabled();

    fireEvent.change(screen.getByLabelText(/Ask about it on/), { target: { value: "2026-12-01" } });
    expect(confirm).toBeDisabled();

    fireEvent.change(screen.getByLabelText(/shown to Ada Rivera/i), {
      target: { value: "safety review" },
    });
    expect(confirm).toBeEnabled();
  });

  // "true" is the right thing to send and the wrong thing to say. A dialog
  // using one string for both would either lie to the operator or be refused
  // by the resolver.
  it("sends the resolver's value and shows the operator's words", () => {
    renderHold();
    expect(screen.getByText(/Hold access to this target for Ada Rivera/)).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText(/Ask about it on/), { target: { value: "2026-12-01" } });
    fireEvent.change(screen.getByLabelText(/shown to Ada Rivera/i), {
      target: { value: "safety review" },
    });
    fireEvent.click(screen.getByRole("button", { name: /hold access to this target/i }));

    expect(state.held[0]).toMatchObject({ field: "enabled", value: "true", reason: "safety review" });
    // "It stays" is the default, so the date is a review date and not an expiry:
    // doing nothing must keep the access blocked.
    expect(state.held[0].reviewDate).toBeTruthy();
    expect(state.held[0].expiresAt).toBeUndefined();
  });
});
