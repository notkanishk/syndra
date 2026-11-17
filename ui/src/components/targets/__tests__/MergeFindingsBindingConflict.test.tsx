// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { UiViewProvider } from "@/lib/ui-view";
import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { MergeFindings } from "@/components/targets/MergeFindings";
import type { MergeFinding, ResolveFindingInput } from "@/lib/queries/useTargets";

// 7t-4: a binding_conflict finding. The add-on says what happened and what to do
// (shown as written); the default action is the system page's own "Stop tracking" (7t-7 adds the others).

const state = { findings: [] as MergeFinding[], resolved: [] as ResolveFindingInput[] };

vi.mock("@/lib/queries/useTargets", async () => {
  const actual = await vi.importActual<typeof import("@/lib/queries/useTargets")>("@/lib/queries/useTargets");
  return {
    ...actual,
    useMergeFindings: () => ({ data: state.findings, isLoading: false, error: null, refetch: vi.fn() }),
    useResolveMergeFinding: () => ({
      mutate: (input: ResolveFindingInput, opts?: { onSuccess?: () => void }) => {
        state.resolved.push(input);
        // `unbound` settles at once: the refetch that follows no longer lists the finding.
        state.findings = state.findings.filter((f) => f.id !== input.id);
        opts?.onSuccess?.();
      },
      isPending: false,
      error: null,
    }),
  };
});

vi.mock("@/lib/queries/useFieldSchema", async () => (await import("@/lib/__fixtures__/field-schema-mock")).fieldSchemaMock);

const NAMES: Record<string, string> = { "sub-1": "Rey Mysterio", "sub-2": "Edge" };
vi.mock("@/components/names", () => ({
  UserName: ({ id }: { id: string }) => <span>{NAMES[id] ?? id}</span>,
}));

function renderFindings(view: "basic" | "advanced" = "basic") {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <UiViewProvider isOperator initialView={view}>
        <MergeFindings target="truenas" />
      </UiViewProvider>
    </QueryClientProvider>,
  );
}

const DETAIL = "The account rey, recorded for this person, is gone, and its number is now held by a different account, rey2.";
const NEXT = "Stop tracking this person's old account; the next change creates a new one for them. Adopt rey2 instead only if it is theirs.";

function conflict(theirs: Record<string, unknown>): MergeFinding {
  return {
    id: "f1",
    target: "truenas",
    subject_id: "sub-1",
    field: "",
    outcome: "binding_conflict",
    theirs,
    detected_at: "2026-11-15T12:00:00Z",
    last_seen_at: "2026-11-15T12:00:00Z",
  };
}

const numberHeld = { reason: "number_held", username: "rey2", account_id: "4242", adoptable: false, detail: DETAIL, next: NEXT };

beforeEach(() => {
  state.findings = [];
  state.resolved = [];
});

describe("a binding conflict", () => {
  it("shows the add-on's words as written, and nothing blank", () => {
    state.findings = [conflict(numberHeld)];
    renderFindings();
    expect(screen.getByText(`${DETAIL} ${NEXT}`)).toBeTruthy();
  });

  it("offers Stop tracking, never the keep or take buttons that answer 422", () => {
    state.findings = [conflict(numberHeld)];
    renderFindings();
    fireEvent.click(screen.getByText("Decide"));
    expect(screen.getByRole("button", { name: "Stop tracking" })).toBeTruthy();
    for (const gone of ["Keep Syndra's value", "Create it again", "Stop managing it", /^Use TrueNAS/]) {
      expect(screen.queryByRole("button", { name: gone })).toBeNull();
    }
  });

  it("says only what stopping does to the system, and leaves the next step to the add-on's words", () => {
    // Adoptable or not, the page never advises on adoption: that is the add-on's `next`, once.
    for (const adoptable of [true, false]) {
      state.findings = [conflict({ ...numberHeld, adoptable })];
      const view = renderFindings();
      fireEvent.click(screen.getByText("Decide"));
      expect(screen.getByText("Syndra stops tracking this person's account on TrueNAS. Nothing on TrueNAS is changed.")).toBeTruthy();
      expect(screen.queryByText(/adopt it from Not created by Syndra/)).toBeNull();
      // Said once: on the row, by the add-on.
      expect(screen.getAllByText(/the next change creates a new one for them/)).toHaveLength(1);
      view.unmount();
    }
  });

  it("sends `unbound` with the reason, and the row leaves the list", () => {
    state.findings = [conflict(numberHeld)];
    renderFindings();
    fireEvent.click(screen.getByText("Decide"));
    const press = screen.getByRole("button", { name: "Stop tracking" }) as HTMLButtonElement;
    expect(press.disabled).toBe(true);
    fireEvent.change(screen.getByLabelText("Why"), { target: { value: "hand rename on the NAS" } });
    fireEvent.click(screen.getByRole("button", { name: "Stop tracking" }));
    expect(state.resolved).toEqual([{ id: "f1", resolution: "unbound", reason: "hand rename on the NAS" }]);
    expect(screen.queryByText(DETAIL, { exact: false })).toBeNull();
    expect(screen.getByText("Nothing disputed.")).toBeTruthy();
  });

  it("names the other person for claimed_twice, never their id, and offers no adoption", () => {
    state.findings = [
      conflict({
        reason: "claimed_twice",
        username: "rey",
        account_id: "3001",
        bound_to: "sub-2",
        detail: "The account rey is recorded for this person and for someone else, so Syndra changes it for neither.",
        next: "Stop tracking it for whichever of the two it does not belong to.",
      }),
    ];
    renderFindings();
    expect(screen.getByText(/The other person is/).textContent).toBe("The other person is Edge.");
    expect(screen.queryByText("sub-2")).toBeNull();
    fireEvent.click(screen.getByText("Decide"));
    expect(screen.queryByText(/adopt it from Not created by Syndra/)).toBeNull();
  });

  it("lists every holder for ambiguous", () => {
    state.findings = [
      conflict({
        reason: "ambiguous",
        holders: [
          { username: "rey", account_id: "3001" },
          { username: "rey_old", account_id: "3002" },
        ],
        detail: "More than one account on TrueNAS has the name or number this person's account is recorded under.",
        next: "Rename or remove one of them on TrueNAS, then try again.",
      }),
    ];
    renderFindings();
    expect(screen.getByText("Accounts on TrueNAS that hold it: rey, rey_old.")).toBeTruthy();
  });

  it("says a name held the same way, with the add-on's words", () => {
    state.findings = [
      conflict({
        reason: "name_held",
        username: "rey",
        account_id: "3001",
        detail: "The name rey this person's account is recorded under is now held by a different account.",
        next: "Stop tracking this person's old account first, then adopt that account only if it is theirs, or rename it on TrueNAS.",
      }),
    ];
    renderFindings();
    expect(screen.getByText(/now held by a different account\. Stop tracking this person's old account first/)).toBeTruthy();
  });

  it("still ends in a next step when the report carries no words", () => {
    state.findings = [conflict({ reason: "number_held", username: "rey2" })];
    renderFindings();
    expect(screen.getByText(/can't be matched to one account\. Stop tracking it; the next change creates a new account for them\./)).toBeTruthy();
  });

  it("keeps the holder's number for Advanced only", () => {
    state.findings = [conflict(numberHeld)];
    const basic = renderFindings("basic");
    expect(screen.queryByText(/4242/)).toBeNull();
    basic.unmount();
    renderFindings("advanced");
    fireEvent.click(screen.getByRole("button", { name: /Technical detail/ }));
    expect(screen.getByText(/4242/)).toBeTruthy();
  });
});
