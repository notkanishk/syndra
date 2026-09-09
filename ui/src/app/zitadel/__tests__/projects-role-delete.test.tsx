// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import UpstreamProjectsPage from "@/app/zitadel/projects/page";

/**
 * The delete-role confirm is the one destructive control on this page that
 * never got the "retire on success, survive a failure" treatment its sibling
 * (the RoleDialog submit, same file) already has. These tests pin that down
 * directly, rather than trusting a read of the JSX.
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

async function openConfirm() {
  fireEvent.click(screen.getByRole("button", { name: /Delete Trained/i }));
  // Acknowledge the consequence, otherwise the confirm stays disabled.
  fireEvent.click(screen.getByRole("checkbox"));
}

describe("upstream role delete", () => {
  it("names the missing acknowledgement instead of failing silently", async () => {
    renderPage();
    fireEvent.click(screen.getByRole("button", { name: /Delete Trained/i }));
    expect(
      screen.getByText(/Check the box above to confirm you understand the consequence/i),
    ).toBeInTheDocument();
  });

  it("retires the delete button on success, and flips Cancel to Done", async () => {
    deleteRole.mockResolvedValueOnce({});
    renderPage();
    await openConfirm();

    fireEvent.click(screen.getByRole("button", { name: "Delete role in Zitadel" }));
    await screen.findByText(/deleted in Zitadel/i);

    expect(screen.queryByRole("button", { name: "Delete role in Zitadel" })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Done" })).toBeInTheDocument();
    expect(deleteRole).toHaveBeenCalledTimes(1);
  });

  it("keeps the delete button after a failure, so there is a retry path", async () => {
    deleteRole.mockRejectedValueOnce(new Error("boom"));
    renderPage();
    await openConfirm();

    fireEvent.click(screen.getByRole("button", { name: "Delete role in Zitadel" }));
    await screen.findByText(/boom/i);

    expect(screen.getByRole("button", { name: "Delete role in Zitadel" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Cancel" })).toBeInTheDocument();
  });

  it("stops describing the delete as imminent once it has already happened", async () => {
    deleteRole.mockResolvedValueOnce({});
    renderPage();
    await openConfirm();
    expect(screen.getByText(/the moment you press the button/i)).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Delete role in Zitadel" }));
    await screen.findByText(/deleted in Zitadel/i);

    expect(screen.queryByText(/the moment you press the button/i)).not.toBeInTheDocument();
    expect(screen.getByText(/is deleted\. Everyone who held it has already lost it\./i)).toBeInTheDocument();
  });
});
