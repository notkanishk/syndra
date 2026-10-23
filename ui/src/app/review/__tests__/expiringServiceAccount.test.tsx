// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import ExpiringAccessPage from "@/app/review/expiring-access/page";
import type { ExpiringGrantRow } from "@/lib/queries/useExpiringAccess";

const state = vi.hoisted(() => ({ rows: [] as unknown[] }));

vi.mock("next/navigation", () => ({
  useSearchParams: () => new URLSearchParams(""),
  useRouter: () => ({ replace: vi.fn() }),
}));
vi.mock("@/lib/queries/useExpiringAccess", () => ({
  useExpiringGrants: () => ({ data: state.rows, isLoading: false, error: null, refetch: vi.fn() }),
  useAcknowledgeExpiry: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useClearExpiryAcknowledgement: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));
vi.mock("@/lib/queries/useUsers", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@/lib/queries/useUsers")>()),
  useCreateGrant: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));
vi.mock("@/lib/queries/useServiceAccounts", async (original) => ({
  ...(await original<typeof import("@/lib/queries/useServiceAccounts")>()),
  useServiceAccountIds: () => new Set(["svc"]),
}));

const grant = (user_id: string): ExpiringGrantRow => ({
  id: `g-${user_id}`,
  user_id,
  project_id: "pLaser",
  role_key: "trained",
  granted_by: "priya",
  reason: "",
  expires_at: new Date(Date.now() + 5 * 86_400_000).toISOString(),
  created_at: "2026-06-01T12:00:00Z",
});

// A-07 §29: a service account stays in Expiring access, described as one, and its
// name opens its own page.
describe("Expiring access, a service account's row", () => {
  it("says Service account where a person has none, and links to the account's page", () => {
    state.rows = [grant("svc"), grant("u1")];
    render(
      <QueryClientProvider client={new QueryClient()}>
        <ExpiringAccessPage />
      </QueryClientProvider>,
    );

    expect(screen.getAllByText("Service account")).toHaveLength(1);
    const hrefs = screen.getAllByRole("link").map((a) => a.getAttribute("href"));
    expect(hrefs).toContain("/system/service-accounts/svc");
    expect(hrefs).toContain("/users/u1");
    expect(hrefs).not.toContain("/users/svc");
  });
});
