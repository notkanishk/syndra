// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { UnexplainedAccess } from "@/components/review/UnexplainedAccess";
import type { DriftTriageItem } from "@/lib/queries/useDrift";

const drift = vi.hoisted(() => ({ data: [] as DriftTriageItem[] }));

// What the bulk endpoints report back. Defaults to "everything worked"; the
// partial-failure tests override it.
const bulk = vi.hoisted(() => ({
  result: { attributed: 0, marked: 0, failed: 0, failed_ids: [] as string[] },
}));

const toasts = vi.hoisted(() => ({
  success: [] as string[],
  warning: [] as string[],
  error: [] as string[],
}));

vi.mock("sonner", () => ({
  toast: {
    success: (message: string) => toasts.success.push(message),
    warning: (message: string) => toasts.warning.push(message),
    error: (message: string) => toasts.error.push(message),
  },
}));

vi.mock("next/navigation", () => ({
  useSearchParams: () => new URLSearchParams(""),
  useRouter: () => ({ replace: () => {} }),
}));

vi.mock("@/lib/queries/useDrift", () => ({
  useDriftItems: () => ({ ...drift, isLoading: false, error: null, refetch: () => {} }),
  useAttributeDrift: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useRevokeDrift: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useMarkExternalDrift: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useBulkAttributeDrift: () => ({
    mutateAsync: async () => bulk.result,
    isPending: false,
  }),
  useBulkMarkExternalDrift: () => ({
    mutateAsync: async () => bulk.result,
    isPending: false,
  }),
  useReconcileNow: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));

vi.mock("@/lib/queries/useGrants", () => ({
  useReconciliationDiff: () => ({ data: undefined, isLoading: false, error: null, refetch: () => {} }),
}));

function item(overrides: Partial<DriftTriageItem> = {}): DriftTriageItem {
  return {
    id: "d1",
    user_id: "u1",
    project_id: "p1",
    role_keys: ["operator"],
    drift_type: "zitadel_only",
    detection_source: "reconciliation_sweep",
    detected_at: "2026-07-22T06:00:00Z",
    role_in_catalogue: true,
    user_is_service_account: false,
    other_items_for_user: 0,
    ...overrides,
  } as DriftTriageItem;
}

function renderTriage() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <UnexplainedAccess />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  drift.data = [item()];
  bulk.result = { attributed: 0, marked: 0, failed: 0, failed_ids: [] };
  toasts.success = [];
  toasts.warning = [];
  toasts.error = [];
});

describe("Unexplained access — triage", () => {
  it("offers the three resolutions in one fixed order on every row", () => {
    drift.data = [item({ id: "a" }), item({ id: "b", user_id: "u2" })];
    renderTriage();

    const rows = screen.getAllByRole("button", { name: "Adopt" });
    expect(rows).toHaveLength(2);

    // A scanning eye should learn one sequence: Adopt · Revoke · Owned elsewhere.
    const labels = screen
      .getAllByRole("button")
      .map((button) => button.textContent?.trim())
      .filter((label) => label === "Adopt" || label === "Revoke" || label === "Owned elsewhere");
    expect(labels.slice(0, 3)).toEqual(["Adopt", "Revoke", "Owned elsewhere"]);
  });

  it("states that bulk revoke is deliberately absent rather than leaving a hole", () => {
    renderTriage();
    expect(
      screen.getByText(/Bulk adopt and bulk mark-as-external exist; bulk revoke does not\./),
    ).toBeInTheDocument();
  });

  it("offers bulk adopt and bulk mark-external once rows are selected — and never bulk revoke", () => {
    renderTriage();
    fireEvent.click(screen.getAllByRole("checkbox")[0]);

    expect(screen.getByRole("button", { name: /Adopt 1 in MkAuth/ })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Mark 1 as owned elsewhere/ })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Revoke 1/ })).not.toBeInTheDocument();
  });

  it("names the upstream actor and date when the detector knew them", () => {
    drift.data = [
      item({ upstream_actor: "svc-badge-sync", upstream_created_at: "2026-07-21T14:12:04Z" }),
    ];
    renderTriage();
    expect(screen.getByText(/by svc-badge-sync/)).toBeInTheDocument();
  });

  it("admits it does not know the actor rather than naming a plausible one", () => {
    renderTriage();
    expect(
      screen.getByText(/compares grant lists and can't see who made the change/),
    ).toBeInTheDocument();
  });

  it("neutralises Adopt for a service account instead of hiding it", () => {
    drift.data = [item({ user_is_service_account: true })];
    renderTriage();
    expect(screen.getByRole("button", { name: "Adopt" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Owned elsewhere" })).toBeEnabled();
  });

  it("says a role is missing from the catalogue, in words as well as colour", () => {
    drift.data = [item({ role_in_catalogue: false })];
    renderTriage();
    expect(screen.getByText("Role not in catalogue")).toBeInTheDocument();
  });

  it("puts the revoke consequence and the person's other items inside the dialog", () => {
    drift.data = [item({ other_items_for_user: 2 })];
    renderTriage();
    fireEvent.click(screen.getByRole("button", { name: "Revoke" }));

    const dialog = screen.getByRole("dialog");
    expect(within(dialog).getByText(/They will lose this role\./)).toBeInTheDocument();
    expect(within(dialog).getByText(/2 more/)).toBeInTheDocument();
  });

  it("shows the evidence grid only when a row is expanded", () => {
    drift.data = [item({ zitadel_grant_id: "zg_4f19c8" })];
    renderTriage();

    expect(screen.queryByText("zg_4f19c8")).not.toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { expanded: false }));
    expect(screen.getByText("zg_4f19c8")).toBeInTheDocument();
  });
});

describe("Unexplained access — bulk resolution feedback", () => {
  async function selectBothAndAdopt() {
    drift.data = [item({ id: "d1" }), item({ id: "d2", user_id: "u2" })];
    renderTriage();
    fireEvent.click(screen.getAllByRole("checkbox")[0]);
    fireEvent.click(screen.getAllByRole("checkbox")[1]);
    fireEvent.click(screen.getByRole("button", { name: /Adopt 2 in MkAuth/ }));
    // Wait for the mutation to settle: exactly one toast is raised per batch,
    // whatever the outcome.
    await waitFor(() => {
      expect(toasts.success.length + toasts.warning.length + toasts.error.length).toBe(1);
    });
  }

  it("reports the count the server resolved, not the count that was selected", async () => {
    bulk.result = { attributed: 1, marked: 0, failed: 1, failed_ids: ["d2"] };
    await selectBothAndAdopt();

    // The bug this pins: two selected, one resolved, and the operator told
    // "2 adopted" — leaving one piece of unexplained access reported as handled.
    expect(toasts.success).toHaveLength(0);
    expect(toasts.warning.join(" ")).toMatch(/1 adopted in MkAuth/);
    expect(toasts.warning.join(" ")).toMatch(/1 failed/);
  });

  it("keeps the rows that failed selected so the retry is exactly those", async () => {
    bulk.result = { attributed: 1, marked: 0, failed: 1, failed_ids: ["d2"] };
    await selectBothAndAdopt();

    expect(await screen.findByRole("button", { name: /Adopt 1 in MkAuth/ })).toBeInTheDocument();
  });

  it("says nothing succeeded when nothing did", async () => {
    bulk.result = { attributed: 0, marked: 0, failed: 2, failed_ids: ["d1", "d2"] };
    await selectBothAndAdopt();

    expect(toasts.success).toHaveLength(0);
    expect(toasts.error.join(" ")).toMatch(/Nothing was adopted in MkAuth/);
  });

  it("clears the selection and celebrates only on a clean batch", async () => {
    bulk.result = { attributed: 2, marked: 0, failed: 0, failed_ids: [] };
    await selectBothAndAdopt();

    expect(toasts.success.join(" ")).toMatch(/2 adopted in MkAuth\./);
    expect(toasts.warning).toHaveLength(0);
    await waitFor(() => {
      expect(screen.queryByRole("button", { name: /Adopt \d+ in MkAuth/ })).not.toBeInTheDocument();
    });
  });
});
