// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { RequestsScreen } from "@/components/requests/RequestsScreen";

// Ask for access, from where it is asked: the place list comes from /catalog,
// the one list a member may read, and an "Ask for this" link opens the dialog
// already filled in, for a member and for an operator asking for themselves.

const search = vi.hoisted(() => ({ value: "" }));

vi.mock("next/navigation", () => ({
  useSearchParams: () => new URLSearchParams(search.value),
  useRouter: () => ({ replace: () => {} }),
}));
vi.mock("@/lib/queries/useRequests", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/queries/useRequests")>()),
  useRequestsMine: () => ({ data: [], isLoading: false, error: null, refetch: () => {} }),
  useRequestsAdmin: () => ({ data: [], isLoading: false, error: null, refetch: () => {} }),
  useCreateRequest: () => ({ mutateAsync: vi.fn(), isPending: false }),
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
