// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import ExpiringAccessPage from "@/app/review/expiring-access/page";
import type { ExpiringGrantRow } from "@/lib/queries/useExpiringAccess";

/** A-19 P10.2 and P10.3 on Expiring: Select ends with the result, and a row's person is a link. */

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
vi.mock("@/components/people/BulkDialog", () => ({
  BulkDialog: ({ onClose }: { onClose: (applied?: boolean) => void }) => (
    <div role="dialog" aria-label="Bulk">
      <button onClick={() => onClose(true)}>Close on the result</button>
      <button onClick={() => onClose(false)}>Close without applying</button>
    </div>
  ),
}));

function renderPage() {
  return render(
    <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}>
      <ExpiringAccessPage />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  const grant: ExpiringGrantRow = {
    id: "g1",
    user_id: "u1",
    project_id: "pLaser",
    role_key: "trained",
    granted_by: "priya",
    reason: "",
    expires_at: new Date(Date.now() + 5 * 86_400_000).toISOString(),
    created_at: "2026-06-01T12:00:00Z",
  };
  state.rows = [grant];
});

function startExtending() {
  renderPage();
  fireEvent.click(screen.getByRole("button", { name: "Select" }));
  fireEvent.click(screen.getByRole("checkbox", { name: /^Select trained/ }));
  fireEvent.click(screen.getByRole("button", { name: "Preview an extension" }));
  expect(screen.getByRole("dialog")).toBeInTheDocument();
}

describe("Expiring: Select is a mode (Q9)", () => {
  it("ends, and clears the selection, when the dialog closes on its result", () => {
    startExtending();
    fireEvent.click(screen.getByRole("button", { name: "Close on the result" }));
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(screen.queryByRole("checkbox")).toBeNull();
    expect(screen.getByRole("button", { name: "Select" })).toBeInTheDocument();
  });

  it("stays in Select with the selection kept when it closes without applying", () => {
    startExtending();
    fireEvent.click(screen.getByRole("button", { name: "Close without applying" }));
    expect(screen.queryByRole("dialog")).toBeNull();
    expect(screen.getByRole("checkbox", { name: /^Select trained/ })).toBeChecked();
  });
});

describe("Expiring: the person is a link (P10.3)", () => {
  it("links the row's person, as Holds and Requests do", () => {
    renderPage();
    const link = document.querySelector('a[href="/users/u1"]');
    expect(link).not.toBeNull();
  });
});
