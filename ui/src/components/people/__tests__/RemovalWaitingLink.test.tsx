// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import type { RoleReason } from "@/components/access/AccessSource";
import { RemovalDialog } from "@/components/people/RemovalDialog";

/**
 * A-19 P3: "Waiting for you to send" in the Removal dialog opens Pending
 * changes narrowed to the person whose bundle was just removed.
 */

const removeBundle = vi.hoisted(() => vi.fn());

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
    removeBundle.mockResolvedValue({ cascade: { enqueued: 2, mode: "manual" } });
    render(
      <QueryClientProvider client={new QueryClient()}>
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
    fireEvent.click(screen.getByRole("button", { name: "Remove bundle" }));
    const link = await screen.findByRole("link", { name: /Waiting for you to send/ });
    expect(link).toHaveAttribute("href", `/governance/pending?person=${encodeURIComponent("u 2f/81")}`);
  });
});
