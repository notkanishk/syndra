// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { seedAutoRetry } from "@/test-utils/drain";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import type { RoleReason } from "@/components/access/AccessSource";
import { RemovalDialog } from "@/components/people/RemovalDialog";
import { actPlan } from "@/test-utils/plan";

/**
 * A-19 P3: "Waiting for you to send" in the Removal dialog opens Pending
 * changes narrowed to the person whose bundle was just removed.
 */

const removeBundle = vi.hoisted(() => vi.fn());

vi.mock("@/lib/api-client", async (original) => ({
  ...(await original<typeof import("@/lib/api-client")>()),
  request: async (path: string) => {
    if (path.includes("/plan?")) return actPlan();
    throw new Error(`unmocked request ${path}`);
  },
}));

vi.mock("next/link", () => ({
  default: ({ children, href }: { children: React.ReactNode; href: string }) => <a href={href}>{children}</a>,
}));
vi.mock("@/lib/queries/useRoleMembers", () => ({
  useRemoveDirectGrant: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));
vi.mock("@/lib/queries/useBundles", () => ({
  useRemoveBundle: () => ({ mutateAsync: removeBundle, isPending: false }),
}));

const bundle: RoleReason = { kind: "bundle", bundle_id: "b1", bundle_name: "Lab Tech" };

describe("the Removal dialog's way to Pending changes", () => {
  it("carries the person it removed the bundle from", async () => {
    removeBundle.mockResolvedValue({ cascade: { enqueued: 2, waiting: 2, wait_reason: "role_waits" } });
    render(
      <QueryClientProvider client={seedAutoRetry(new QueryClient(), true)}>
        <RemovalDialog
          removal={{
            projectId: "pLaser",
            projectName: "Laser Lab",
            roleKey: "trained",
            userId: "u 2f/81",
            userName: "Tomas Beck",
            sources: [bundle],
          }}
          userId="u 2f/81"
          userName="Tomas Beck"
          onClose={() => {}}
        />
      </QueryClientProvider>,
    );
    await waitFor(() => expect(screen.getByRole("button", { name: "Remove the bundle" })).toBeEnabled());
    fireEvent.click(screen.getByRole("button", { name: "Remove the bundle" }));
    const link = await screen.findByRole("link", { name: /Waiting for you to send/ });
    expect(link).toHaveAttribute("href", `/governance/pending?person=${encodeURIComponent("u 2f/81")}`);
  });
});
