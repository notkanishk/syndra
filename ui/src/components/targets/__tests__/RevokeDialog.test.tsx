// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { seedAutoRetry } from "@/test-utils/drain";
import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { HoldDialog } from "@/components/review/HoldDialog";
import { RevokeDialog } from "@/components/targets/RevokeDialog";
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

vi.mock("@/lib/queries/useFieldSchema", async (orig) => (await import("@/lib/__fixtures__/field-schema-mock")).withRosterNames(orig));
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
      mutateAsync: async (input: Record<string, unknown>) => {
        state.held.push(input);
      },
      isPending: false,
      error: null,
    }),
  };
});

function renderRevoke() {
  const client = seedAutoRetry(new QueryClient({ defaultOptions: { queries: { retry: false } } }), true);
  return render(
    <QueryClientProvider client={client}>
      <RevokeDialog target="truenas" subjectId="u1" subjectName="Charlotte Flair" onClose={() => {}} />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  state.revoked = [];
  state.held = [];
  state.result = null;
});

describe("revoking access on a target", () => {
  // The sentence is fixed by the backend and shown verbatim. A UI implying the
  // access is gone the moment the button is pressed is the exact failure that
  // endpoint's whole design exists to prevent.
  it("says what it cannot do, before the form", () => {
    renderRevoke();
    expect(
      screen.getByText(/they stay connected until they disconnect/),
    ).toBeInTheDocument();
    expect(screen.getByText(/TrueNAS cannot end a session/)).toBeInTheDocument();
  });

  // What it does, not what it means. Everything on these screens queues, and
  // this button says so rather than claiming the access is gone.
  it("pairs Keep it with the act, and the session sentence is in the What happens list", () => {
    renderRevoke();
    expect(screen.getByRole("button", { name: "Keep it" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Cancel" })).toBeNull();
    expect(screen.queryByText("Anyone connected stays connected for now.")).toBeNull();
    expect(screen.getByText(/they stay connected until they disconnect/).closest("li")).not.toBeNull();
  });

  it("names the queue in the button", () => {
    renderRevoke();
    expect(screen.getByRole("button", { name: /revoke access for Charlotte Flair/i })).toBeInTheDocument();
  });

  // Rung 3, the same gesture as every other revocation: muscle memory must not
  // depend on which one an operator is doing.
  it("will not fire until the name is typed and a reason given", () => {
    renderRevoke();
    // Re-queried at every step rather than held. `Button` wraps itself in a
    // span to carry a `reason`, so the button is a DIFFERENT DOM node once the
    // reason clears — a held reference goes stale and reads as never enabling.
    const confirm = () => screen.getByRole("button", { name: /revoke access for Charlotte Flair/i });
    expect(confirm()).toBeDisabled();

    // Queried by the label, which is now all the label is. The hint used to
    // live inside it, so this field's accessible name was its title plus a
    // paragraph — and this test could only find it by that paragraph.
    fireEvent.change(screen.getByLabelText("Why"), { target: { value: "offboarding" } });
    expect(confirm()).toBeDisabled();

    fireEvent.change(screen.getByRole("textbox", { name: /type the person's name/i }), {
      target: { value: "charlotte flair" },
    });
    expect(confirm()).toBeEnabled();

    fireEvent.click(confirm());
    expect(state.revoked).toHaveLength(1);
    expect(state.revoked[0].reason).toBe("offboarding");
  });

  // The footer used to say the hold "reaches TrueNAS when someone sends it
  // from Pending changes" — direct contradiction of the backend's own
  // disclosure, below, that a revocation drains on its own. One dialog must
  // not tell the operator both.
  it("does not claim the hold needs a manual send from Pending changes", () => {
    renderRevoke();
    expect(screen.getByText(/revocations send on their own, within a minute/i)).toBeInTheDocument();
    expect(screen.queryByText(/sends it from Pending changes/i)).toBeNull();
  });

  const confirm = () => {
    fireEvent.change(screen.getByLabelText("Why"), { target: { value: "offboarding" } });
    fireEvent.change(screen.getByRole("textbox", { name: /type the person's name/i }), {
      target: { value: "Charlotte Flair" },
    });
    fireEvent.click(screen.getByRole("button", { name: /revoke access for Charlotte Flair/i }));
  };
  const recorded = (over: Partial<RevocationResult>): RevocationResult => ({
    status: "recorded",
    allowance_id: "a1",
    recorded_at: "2026-11-21T10:00:00Z",
    landed_at: null,
    rotated: true,
    operation: "op-1",
    detail:
      "Recorded. Not yet applied on TrueNAS. Until it is, the account there still works. Syndra applies it by itself when TrueNAS answers. The credential is already replaced. Sessions already established end when they next reconnect — this system has no way to close one.",
    ...over,
  });

  // Design §13.4: recorded and landed are two facts. The reply is never drawn as done, and it says
  // the account there still works until the add-on answers that it applied it.
  it("says Recorded and that the account still works until it lands, never Revoked or Partly done", () => {
    state.result = recorded({});
    renderRevoke();
    confirm();
    expect(screen.getByText("Recorded")).toBeInTheDocument();
    expect(screen.getByText(/Until it is, the account there still works/)).toBeInTheDocument();
    expect(screen.queryByText("Revoked")).toBeNull();
    expect(screen.queryByText(/Partly done/)).toBeNull();
    // Syndra applies it by itself: no Send step is offered.
    expect(screen.queryByText(/Waiting for you to send/)).toBeNull();
  });

  // The credential half that did not land is named with its own next step, never as a failed revocation.
  it.each([
    [
      "refused",
      { status: "refused" as const, detail: "The credential was not replaced: TrueNAS refused: account locked. Replace it once that is resolved." },
      /Replace it once that is resolved/,
    ],
    ["unconfirmed", { status: "unconfirmed" as const, detail: "This may have been applied." }, /may have been replaced\. Syndra is checking; don't send it again\./],
    [
      "nothing sent",
      { error: "NOTHING_SENT", message: "Nothing went out. Syndra can't reach TrueNAS's add-on. Check that its container is running. Syndra keeps trying. Try again when TrueNAS answers." },
      /Try again when TrueNAS answers/,
    ],
  ])("names a credential half that %s, with its own step", (_name, rotation, step) => {
    state.result = recorded({ rotated: false, operation: undefined, outstanding: "password.rotate", rotation, detail: "Recorded. Not yet applied on TrueNAS. Until it is, the account there still works. Syndra applies it by itself when TrueNAS answers." });
    renderRevoke();
    confirm();
    expect(screen.getByText("Recorded")).toBeInTheDocument();
    expect(screen.getByText(step)).toBeInTheDocument();
    // The sheet is its result: form, list and Keep it are gone, one Close remains.
    expect(screen.queryByLabelText("Why")).toBeNull();
    expect(screen.queryByText("What happens")).toBeNull();
    expect(screen.queryByRole("button", { name: "Keep it" })).toBeNull();
    expect(screen.getByRole("button", { name: "Close" })).toBeInTheDocument();
  });
});

describe("putting a hold on", () => {
  function renderHold() {
    const client = seedAutoRetry(new QueryClient({ defaultOptions: { queries: { retry: false } } }), true);
    return render(
      <QueryClientProvider client={client}>
        <HoldDialog
          subjectId="u1"
          subjectName="Charlotte Flair"
          target="truenas"
          field="enabled"
          value="true"
          label="access to TrueNAS"
          onClose={() => {}}
        />
      </QueryClientProvider>,
    );
  }

  // Framed as what happens if nobody comes back, which is the actual decision.
  it("asks how it ends rather than for an expiry", () => {
    renderHold();
    expect(screen.getByText(/What happens if nobody comes back to this/)).toBeInTheDocument();
    expect(screen.getByText(/It stays until somebody decides/)).toBeInTheDocument();
    expect(screen.getByText(/It lifts itself on a date/)).toBeInTheDocument();
  });

  // Two bounded forms and no third: a hold with neither is refused by the
  // backend, so the UI does not offer one.
  it("requires a date and a reason either way", () => {
    renderHold();
    // Re-queried, not held: carrying a `reason` wraps the button in a span, so
    // it is a different DOM node once the reason clears.
    const confirm = () => screen.getByRole("button", { name: /put access to TrueNAS on hold/i });
    expect(confirm()).toBeDisabled();

    fireEvent.change(screen.getByLabelText(/Remind us on/), { target: { value: "2026-12-01" } });
    expect(confirm()).toBeDisabled();

    fireEvent.change(screen.getByLabelText(/shown to Charlotte Flair/i), {
      target: { value: "safety review" },
    });
    expect(confirm()).toBeEnabled();
  });

  // "true" is the right thing to send and the wrong thing to say. A dialog
  // using one string for both would either lie to the operator or be refused
  // by the resolver.
  it("sends the resolver's value and shows the operator's words", () => {
    renderHold();
    expect(screen.getByText(/Put access to TrueNAS on hold for Charlotte Flair/)).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText(/Remind us on/), { target: { value: "2026-12-01" } });
    fireEvent.change(screen.getByLabelText(/shown to Charlotte Flair/i), {
      target: { value: "safety review" },
    });
    fireEvent.click(screen.getByRole("button", { name: /put access to TrueNAS on hold/i }));

    expect(state.held[0]).toMatchObject({ field: "enabled", value: "true", reason: "safety review" });
    // "It stays" is the default, so the date is a review date and not an expiry:
    // doing nothing must keep the access blocked.
    expect(state.held[0].reviewDate).toBeTruthy();
    expect(state.held[0].expiresAt).toBeUndefined();
  });
});
