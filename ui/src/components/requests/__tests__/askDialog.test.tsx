// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { RequestsScreen } from "@/components/requests/RequestsScreen";

// Ask for access, from where it is asked: the place list comes from /catalog,
// the one list a member may read, and an "Ask for this" link opens the dialog
// already filled in, for a member and for an operator asking for themselves.

const search = vi.hoisted(() => ({ value: "" }));
const mine = vi.hoisted(() => ({ data: [] as Array<Record<string, unknown>> }));
const held = vi.hoisted(() => ({
  grants: [] as Array<{ id: string; project_id: string; role_key: string; expires_at?: string | null }>,
  observed: [] as string[],
}));
const create = vi.hoisted(() => ({ calls: 0 }));

vi.mock("next/navigation", () => ({
  useSearchParams: () => new URLSearchParams(search.value),
  useRouter: () => ({ replace: () => {} }),
}));
vi.mock("@/lib/queries/useRequests", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/queries/useRequests")>()),
  useRequestsMine: () => ({ data: mine.data, isLoading: false, error: null, refetch: () => {} }),
  useRequestsAdmin: () => ({ data: mine.data, isLoading: false, error: null, refetch: () => {} }),
  useCreateRequest: () => ({
    mutateAsync: async () => {
      create.calls++;
      return { id: "r-new" };
    },
    isPending: false,
  }),
  useWithdrawRequest: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useDecideRequest: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useRehearseBulkDecision: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useApplyBulkDecision: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));
vi.mock("@/lib/queries/useCatalogUsers", () => ({
  useCatalogProjects: () => ({
    data: [
      {
        id: "pLaser",
        name: "Laser Lab",
        kind: "",
        description: "",
        roles: [{ key: "trained", label: "Trained operator", description: "" }],
      },
    ],
    isLoading: false,
    error: null,
    refetch: () => {},
  }),
}));
vi.mock("@/lib/queries/useUsers", () => ({
  useUserGrants: () => ({ data: held.grants }),
  useUserAccess: () => ({
    data: { projects: [{ project_id: "pLaser", observed_role_keys: held.observed }] },
  }),
}));
vi.mock("@/components/names", () => ({
  ProjectName: () => null,
  RoleRef: () => null,
  UserName: ({ id }: { id: string }) => <span>{id}</span>,
}));

function renderAs(isOperator: boolean) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      <RequestsScreen isOperator={isOperator} userId="u1" />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  search.value = "";
  mine.data = [];
  held.grants = [];
  held.observed = [];
  create.calls = 0;
});

describe("Ask for access", () => {
  it("offers a member every place in the catalogue, and its roles", () => {
    search.value = "project=pLaser&role=trained";
    renderAs(false);
    expect(screen.getByRole("option", { name: "Laser Lab" })).toBeInTheDocument();
    expect(screen.getByRole("option", { name: "Trained operator" })).toBeInTheDocument();
    expect(screen.getByLabelText("What do you need to use?")).toHaveValue("pLaser");
  });

  it("opens over the queue for an operator asking for themselves", () => {
    search.value = "project=pLaser&role=trained";
    renderAs(true);
    expect(screen.getByRole("heading", { name: "Ask for access" })).toBeInTheDocument();
    expect(screen.getByLabelText("What do you need to do there?")).toHaveValue("trained");
  });

  it("opens empty from a plain Ask for access link", () => {
    search.value = "ask";
    renderAs(false);
    expect(screen.getByLabelText("What do you need to use?")).toHaveValue("");
  });

  it("stays shut on a plain visit", () => {
    renderAs(true);
    expect(screen.queryByRole("heading", { name: "Ask for access" })).not.toBeInTheDocument();
  });
});


// Found on production, 15 Oct: test_user asked five times, from one dialog,
// for a role they already held. A request is for a role someone lacks, one per
// role (owner, 15 Oct).
describe("Ask for access · only what they lack, once", () => {
  it("won't offer a role they already have", () => {
    held.observed = ["trained"];
    search.value = "project=pLaser&role=trained";
    renderAs(false);
    expect(screen.getByRole("option", { name: /Trained operator · you have this/ })).toBeDisabled();
    expect(screen.getByText(/You already have this, with no end date/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Send request/ })).toBeDisabled();
  });

  it("won't offer one that ends either: staff extend it", () => {
    held.grants = [{ id: "g1", project_id: "pLaser", role_key: "trained", expires_at: "2026-12-18T12:00:00Z" }];
    search.value = "project=pLaser&role=trained";
    renderAs(false);
    expect(screen.getByText(/To keep it longer, ask makerspace/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Send request/ })).toBeDisabled();
  });

  it("won't offer one they've already asked for", () => {
    mine.data = [{ id: "r1", requester_id: "u1", project_id: "pLaser", role_key: "trained", status: "pending", justification: "", created_at: "2026-10-15T12:00:00Z" }];
    search.value = "project=pLaser&role=trained";
    renderAs(false);
    expect(screen.getByRole("option", { name: /already asked/ })).toBeDisabled();
    expect(screen.getByRole("button", { name: /Send request/ })).toBeDisabled();
  });

  it("becomes its result once sent, with nothing left to send again", async () => {
    search.value = "project=pLaser&role=trained";
    renderAs(false);
    fireEvent.click(screen.getByRole("button", { name: /Send request/ }));
    expect(await screen.findByText(/Asked for Trained operator on Laser Lab/)).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Send request/ })).not.toBeInTheDocument();
    expect(screen.queryByLabelText("What do you need to use?")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Done" })).toBeInTheDocument();
    expect(create.calls).toBe(1);
  });
});

describe("The queue · a request for a role they have since come to hold", () => {
  it("says so, and can't be approved", () => {
    mine.data = [
      {
        id: "r1", requester_id: "u9", project_id: "pLaser", role_key: "trained", status: "pending",
        justification: "again", created_at: "2026-10-15T12:00:00Z", holding: { kind: "for_good" },
      },
    ];
    renderAs(true);
    expect(screen.getByText(/Already has this, with no end/)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /^Approve/ })).toBeDisabled();
  });
});
