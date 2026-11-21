// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { UiViewProvider } from "@/lib/ui-view";
import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { MergeFindings } from "@/components/targets/MergeFindings";
import type { MergeFinding, ResolveFindingInput, TargetReconcileResult } from "@/lib/queries/useTargets";

// 7t-7: the three more answers to a binding conflict (owner ruling 17 Nov 2026). Each shows only
// where it can work, each says in the decision form what happens, and Check again waits out the
// add-on's cooldown with the time said.

const state = {
  findings: [] as MergeFinding[],
  resolved: [] as ResolveFindingInput[],
  checks: 0,
  pass: { target: "truenas", bound: 1, queued: 0, read: { state: "now", taken_at: "2026-11-21T10:00:00Z" } } as TargetReconcileResult,
};

vi.mock("@/lib/queries/useTargets", async () => {
  const actual = await vi.importActual<typeof import("@/lib/queries/useTargets")>("@/lib/queries/useTargets");
  return {
    ...actual,
    useMergeFindings: () => ({ data: state.findings, isLoading: false, error: null, refetch: vi.fn() }),
    useResolveMergeFinding: () => ({
      mutate: (input: ResolveFindingInput, opts?: { onSuccess?: () => void }) => {
        state.resolved.push(input);
        opts?.onSuccess?.();
      },
      isPending: false,
      error: null,
    }),
    useReconcileTarget: () => ({
      mutate: (_: undefined, opts?: { onSuccess?: (pass: TargetReconcileResult) => void }) => {
        state.checks++;
        opts?.onSuccess?.(state.pass);
      },
      isPending: false,
      error: null,
    }),
  };
});

vi.mock("@/lib/queries/useFieldSchema", async () => (await import("@/lib/__fixtures__/field-schema-mock")).fieldSchemaMock);

const NAMES: Record<string, string> = { "sub-1": "Rey Mysterio", "sub-2": "Edge", "op-1": "Bayley" };
vi.mock("@/components/names", () => ({
  UserName: ({ id }: { id: string }) => <span>{NAMES[id] ?? id}</span>,
}));
vi.mock("@/components/names/shownNames", () => ({
  useShownNames: () => ({ user: (id: string, fallback = id) => NAMES[id] ?? fallback }),
}));

function view(mode: "basic" | "advanced" = "basic") {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const tree = () => (
    <QueryClientProvider client={client}>
      <UiViewProvider isOperator initialView={mode}>
        <MergeFindings target="truenas" />
      </UiViewProvider>
    </QueryClientProvider>
  );
  const r = render(tree());
  return { ...r, again: () => r.rerender(tree()) };
}

function conflict(theirs: Record<string, unknown>, extra: Partial<MergeFinding> = {}): MergeFinding {
  return {
    id: "f1",
    target: "truenas",
    subject_id: "sub-1",
    field: "",
    outcome: "binding_conflict",
    theirs: { detail: "It is held.", next: "Decide it.", ...theirs },
    detected_at: "2026-11-17T10:00:00Z",
    last_seen_at: "2026-11-17T10:00:00Z",
    ...extra,
  };
}

const radios = () => screen.getAllByRole("radio").map((r) => r.closest("label")?.textContent);
const checkedRadio = () => (screen.getAllByRole("radio") as HTMLInputElement[]).find((r) => r.checked)?.closest("label")?.textContent;
const pick = (label: string) => fireEvent.click(screen.getByRole("radio", { name: label }));
const why = (text: string) => fireEvent.change(screen.getByLabelText("Why"), { target: { value: text } });

const nameHeld = { reason: "name_held", username: "rey", account_id: "3051", adoptable: true };

beforeEach(() => {
  state.findings = [];
  state.resolved = [];
  state.checks = 0;
  state.pass = { target: "truenas", bound: 1, queued: 0, read: { state: "now", taken_at: "2026-11-21T10:00:00Z" } };
});

describe("a binding conflict's answers", () => {
  it("offers adoption only where the add-on marked the holder adoptable, after Stop tracking", () => {
    for (const [theirs, offered] of [
      [nameHeld, true],
      [{ ...nameHeld, reason: "number_held" }, true],
      [{ ...nameHeld, adoptable: false }, false],
      [{ ...nameHeld, username: undefined }, false],
      [{ ...nameHeld, reason: "claimed_twice", bound_to: "sub-2" }, false],
    ] as const) {
      state.findings = [conflict(theirs)];
      const v = view();
      fireEvent.click(screen.getByText("Decide"));
      expect(radios().includes("It's theirs: adopt it"), JSON.stringify(theirs)).toBe(offered);
      if (offered) expect(radios()).toEqual(["Stop tracking", "It's theirs: adopt it", "Check again"]);
      v.unmount();
    }
  });

  it("says what adopting does, then sends `adopted` with the reason; the id stays out of Basic", () => {
    state.findings = [conflict(nameHeld)];
    view();
    fireEvent.click(screen.getByText("Decide"));
    expect(checkedRadio()).toBe("Stop tracking");
    pick("It's theirs: adopt it");
    expect(
      screen.getByText(
        "Syndra makes rey this person's account on TrueNAS. They get it as it is now: everything already on it, and its password. From now on Syndra manages it, and Syndra's next change for them sets its groups and sharing. Their old account, if it is still on TrueNAS under another name, stays there, and Syndra no longer manages it. If rey has changed on TrueNAS since it was checked, nothing is adopted.",
      ),
    ).toBeTruthy();
    expect(screen.queryByText(/3051/)).toBeNull();
    const press = screen.getByRole("button", { name: "It's theirs: adopt it" }) as HTMLButtonElement;
    expect(press.disabled).toBe(true);
    why("same person, renamed");
    fireEvent.click(press);
    expect(state.resolved).toEqual([{ id: "f1", resolution: "adopted", reason: "same person, renamed" }]);
  });

  it("names both people for a twice-claimed account, keeping it for the other as the default", () => {
    state.findings = [conflict({ reason: "claimed_twice", username: "rey", account_id: "3051", bound_to: "sub-2" })];
    view();
    fireEvent.click(screen.getByText("Decide"));
    expect(radios()).toEqual(["Keep it for Edge", "Keep it for Rey Mysterio", "Check again"]);
    expect(checkedRadio()).toBe("Keep it for Edge");
    expect(screen.queryByRole("radio", { name: "Stop tracking" })).toBeNull();
    expect(screen.queryByText(/sub-2/)).toBeNull();

    pick("Keep it for Rey Mysterio");
    expect(
      screen.getByText(
        "rey stays Rey Mysterio's account on TrueNAS, and Syndra stops tracking it for Edge. Nothing on TrueNAS is changed. Edge's next change sets up an account for them on TrueNAS, or, if their name there is taken, stops and asks you in Pending changes.",
      ),
    ).toBeTruthy();
    why("it is Rey's");
    fireEvent.click(screen.getByRole("button", { name: "Keep it for Rey Mysterio" }));
    expect(state.resolved.at(-1)).toEqual({ id: "f1", resolution: "kept", reason: "it is Rey's" });
  });

  it("sends keeping it for the other person as this person's Stop tracking", () => {
    state.findings = [conflict({ reason: "claimed_twice", username: "rey", account_id: "3051", bound_to: "sub-2" })];
    view();
    fireEvent.click(screen.getByText("Decide"));
    expect(
      screen.getByText(
        "rey stays Edge's account on TrueNAS, and Syndra stops tracking it for Rey Mysterio. Nothing on TrueNAS is changed. Rey Mysterio's next change sets up an account for them on TrueNAS, or, if their name there is taken, stops and asks you in Pending changes.",
      ),
    ).toBeTruthy();
    why("it is Edge's");
    fireEvent.click(screen.getByRole("button", { name: "Keep it for Edge" }));
    expect(state.resolved).toEqual([{ id: "f1", resolution: "unbound", reason: "it is Edge's" }]);
  });

  it("offers no keeping when the other person is not named", () => {
    state.findings = [conflict({ reason: "claimed_twice", username: "rey", account_id: "3051" })];
    view();
    fireEvent.click(screen.getByText("Decide"));
    expect(radios()).toEqual(["Stop tracking", "Check again"]);
  });

  it("puts Check again first for ambiguous, whose fix is on the system itself", () => {
    state.findings = [conflict({ reason: "ambiguous", holders: [{ username: "rey", account_id: "1" }, { username: "rey", account_id: "2" }] })];
    view();
    fireEvent.click(screen.getByText("Decide"));
    expect(radios()).toEqual(["Check again", "Stop tracking"]);
    expect(checkedRadio()).toBe("Check again");
  });
});

describe("Check again", () => {
  it("needs no reason, reads the system through Check now's pass, and says what it does", () => {
    state.findings = [conflict({ reason: "ambiguous" })];
    view();
    fireEvent.click(screen.getByText("Decide"));
    expect(
      screen.getByText(
        "Syndra reads TrueNAS again now, the same check as Check now. Nothing on TrueNAS is changed, and any fix it finds waits in Pending changes. Once the accounts are fixed on TrueNAS, this closes by itself.",
      ),
    ).toBeTruthy();
    const press = screen.getByRole("button", { name: "Check again" }) as HTMLButtonElement;
    expect(press.disabled).toBe(false);
    fireEvent.click(press);
    expect(state.checks).toBe(1);
    expect(state.resolved).toEqual([]);
  });

  it("says when it can check again inside the add-on's cooldown, and waits", () => {
    state.findings = [conflict({ reason: "ambiguous" })];
    state.pass = { target: "truenas", bound: 0, queued: 0, halted: true, read: { state: "kept", reason: "cooldown", retry_after: 30, taken_at: "2026-11-21T10:00:00Z" } };
    view();
    fireEvent.click(screen.getByText("Decide"));
    fireEvent.click(screen.getByRole("button", { name: "Check again" }));
    expect(screen.getByText("TrueNAS was read moments ago; you can read again in 30 s.")).toBeTruthy();
    expect((screen.getByRole("button", { name: "Check again" }) as HTMLButtonElement).disabled).toBe(true);
  });

  it("says the system still reports it only once a fresh pass has seen it again", () => {
    state.findings = [conflict({ reason: "ambiguous" })];
    const v = view();
    fireEvent.click(screen.getByText("Decide"));
    fireEvent.click(screen.getByRole("button", { name: "Check again" }));
    const still = "Checked just now: TrueNAS still reports this. Fix it on TrueNAS, then check again, or decide it another way.";
    expect(screen.queryByText(still)).toBeNull();
    expect(screen.getByText("Checked. Updating\u2026")).toBeTruthy();
    state.findings = [conflict({ reason: "ambiguous" }, { last_seen_at: "2026-11-17T10:05:00Z" })];
    v.again();
    expect(screen.getByText(still)).toBeTruthy();
  });

  it("says a stopped pass in the add-on's words with a next step", () => {
    state.findings = [conflict({ reason: "ambiguous" })];
    state.pass = { target: "truenas", bound: 0, queued: 0, halted: true, read: { state: "kept", reason: "cached", taken_at: "2026-11-21T10:00:00Z" }, detail: "Its records can't be read." };
    view();
    fireEvent.click(screen.getByText("Decide"));
    fireEvent.click(screen.getByRole("button", { name: "Check again" }));
    expect(screen.getByText("The check stopped before it concluded anything. Its records can't be read. Check again once that is fixed.")).toBeTruthy();
  });
});

describe("what adopting says it hands over", () => {
  it("says the old account stays only where the name was taken, never for a held number", () => {
    for (const [reason, says] of [["name_held", true], ["number_held", false]] as const) {
      state.findings = [conflict({ ...nameHeld, reason })];
      const v = view();
      fireEvent.click(screen.getByText("Decide"));
      pick("It's theirs: adopt it");
      expect(screen.getByText(/everything already on it, and its password/)).toBeTruthy();
      expect(screen.queryByText(/Their old account, if it is still on TrueNAS/) !== null, reason).toBe(says);
      v.unmount();
    }
  });
});

describe("a decided conflict", () => {
  it("offers Finish for an adoption or keep, sending the same choice again on the repair route", () => {
    for (const [decision, label] of [
      ["adopted", "Finish: adopt it"],
      ["kept", "Finish: keep it for them"],
    ] as const) {
      state.findings = [conflict(nameHeld, { decision, decided_by: "op-1" })];
      state.resolved = [];
      const v = view();
      expect(screen.getByText("If Syndra said this didn't finish, finish it here. Nothing already done is done twice.")).toBeTruthy();
      fireEvent.click(screen.getByRole("button", { name: label }));
      expect(state.resolved).toEqual([{ id: "f1", resolution: decision, reason: "Finishing a choice that did not complete" }]);
      v.unmount();
    }
  });

  it("offers no such Finish on a value finding decided the same way", () => {
    state.findings = [{ ...conflict(nameHeld, { decision: "keep_ours", decided_by: "op-1" }), outcome: "theirs_only", field: "enabled" }];
    view();
    expect(screen.queryByRole("button", { name: /^Finish/ })).toBeNull();
  });

  it("says who adopted or kept it and that the next check closes it", () => {
    for (const [decision, label] of [
      ["adopted", "Adopted the account for them"],
      ["kept", "Kept it for them"],
    ] as const) {
      state.findings = [conflict(nameHeld, { decision, decided_by: "op-1" })];
      const v = view();
      const line = `${label} by Bayley · closes when Syndra next checks TrueNAS`;
      expect(screen.getAllByText((_, el) => el?.tagName === "SPAN" && el.textContent === line)).toHaveLength(1);
      expect(screen.queryByText("Decide")).toBeNull();
      v.unmount();
    }
  });
});
