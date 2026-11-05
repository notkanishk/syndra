// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import UpstreamProjectsPage from "@/app/zitadel/projects/page";
import { ApiError } from "@/lib/api-client";

/**
 * The delete-role confirm retires on success and survives a failure, like its
 * sibling (the RoleDialog submit, same file). Who loses the role, and the
 * count that confirms it, are pinned in projects/__tests__/deleteRole.test.tsx.
 */

const deleteRole = vi.hoisted(() => vi.fn());

vi.mock("@/lib/queries/useUpstream", () => ({
  useUpstreamProjects: () => ({
    data: { items: [{ id: "p1", name: "Laser Lab", state: "active" }] },
    isLoading: false,
    error: null,
    refetch: () => {},
  }),
  useUpstreamProjectRoles: () => ({
    data: { items: [{ key: "trained", displayName: "Trained", group: "" }] },
    isLoading: false,
    error: null,
    refetch: () => {},
  }),
  useUpstreamCreateRole: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useUpstreamUpdateRole: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useUpstreamDeleteRole: () => ({ mutateAsync: deleteRole, isPending: false }),
}));

function renderPage() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <UpstreamProjectsPage />
    </QueryClientProvider>,
  );
}

function openConfirm() {
  fireEvent.click(screen.getByRole("button", { name: /Delete Trained/i }));
}

describe("upstream role delete", () => {
  it("retires the delete button on success, and flips Cancel to Close", async () => {
    deleteRole.mockResolvedValueOnce({});
    renderPage();
    openConfirm();

    fireEvent.click(screen.getByRole("button", { name: "Delete role in Zitadel" }));
    await screen.findByText(/deleted in Zitadel/i);

    expect(screen.queryByRole("button", { name: "Delete role in Zitadel" })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Close" })).toBeInTheDocument();
    expect(deleteRole).toHaveBeenCalledTimes(1);
  });

  it("keeps the delete button after a failure, so there is a retry path", async () => {
    deleteRole.mockRejectedValueOnce(new Error("boom"));
    renderPage();
    openConfirm();

    fireEvent.click(screen.getByRole("button", { name: "Delete role in Zitadel" }));
    await screen.findByText(/boom/i);

    expect(screen.getByRole("button", { name: "Delete role in Zitadel" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Cancel" })).toBeInTheDocument();
  });

  it("stops describing the check once the delete has happened", async () => {
    deleteRole.mockResolvedValueOnce({});
    renderPage();
    openConfirm();
    // C6: the lede names both refusals before the operator commits.
    expect(screen.getByText(/It refuses while Syndra itself gives this role, or while it can’t tell who holds it/i)).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Delete role in Zitadel" }));
    await screen.findByText(/deleted in Zitadel/i);

    expect(screen.queryByText(/Syndra checks first/i)).not.toBeInTheDocument();
  });

  // C6: a lost answer whose read-back failed is not known, never "Nothing was changed",
  // and there is no armed button to press it again.
  it("reports an unknown outcome as not known, never as nothing changed", async () => {
    deleteRole.mockRejectedValueOnce(
      new ApiError(502, { error: "OUTCOME_UNKNOWN", message: "Zitadel didn't answer, and Syndra couldn't read the role back." }),
    );
    renderPage();
    openConfirm();
    fireEvent.click(screen.getByRole("button", { name: "Delete role in Zitadel" }));

    expect(await screen.findByText("Not known")).toBeInTheDocument();
    expect(screen.getByText(/can't tell whether Zitadel deleted Trained/)).toBeInTheDocument();
    expect(screen.getByText(/Open this project again in a minute/)).toBeInTheDocument();
    expect(screen.queryByText(/Nothing was changed/)).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Delete role in Zitadel" })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Close" })).toBeInTheDocument();
  });
});
