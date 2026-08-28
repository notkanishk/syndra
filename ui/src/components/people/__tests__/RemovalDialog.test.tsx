// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { RemovalDialog, type Removal } from "@/components/people/RemovalDialog";
import type { RoleReason } from "@/components/access/AccessSource";

const removeDirect = vi.hoisted(() => vi.fn());
const removeBundle = vi.hoisted(() => vi.fn());

vi.mock("next/link", () => ({
  default: ({ children }: { children: React.ReactNode }) => <span>{children}</span>,
}));
vi.mock("@/lib/queries/useRoleMembers", () => ({
  useRemoveDirectGrant: () => ({ mutateAsync: removeDirect, isPending: false }),
}));
vi.mock("@/lib/queries/useBundles", () => ({
  useRemoveBundle: () => ({ mutateAsync: removeBundle, isPending: false }),
}));

const direct: RoleReason = { kind: "direct" };
const bundle: RoleReason = { kind: "bundle", bundle_id: "b1", bundle_name: "Lab Tech" };
const mapping: RoleReason = { kind: "mapping", trigger_project: "3D Lab", trigger_role: "operator" };

function open(removal: Partial<Removal> & { sources: RoleReason[] }) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <RemovalDialog
        removal={{
          projectId: "pLaser",
          projectName: "Laser Lab",
          roleKey: "trained",
          userId: "u_2f81",
          userName: "Tomas Beck",
          ...removal,
        }}
        onClose={() => {}}
      />
    </QueryClientProvider>,
  );
}

describe("source-specific removal", () => {
  // The residual-outcome sentence is the difference between a safe click and
  // an outage on the laser cutter.
  it("says they will LOSE the role when nothing else gives it to them", () => {
    open({ sources: [direct], grantId: "g_88" });
    expect(screen.getByText(/will lose this role/i)).toBeInTheDocument();
    expect(screen.queryByText(/will still hold/i)).toBeNull();
  });

  it("says they will STILL HOLD it when another source survives", () => {
    open({ sources: [direct, bundle], grantId: "g_88" });
    // Two sources → the menu names one removal per source, never a guess.
    fireEvent.click(screen.getByText("Revoke direct access"));

    expect(screen.getByText(/will still hold this role/i)).toBeInTheDocument();
    expect(screen.getByText(/via Lab Tech/i)).toBeInTheDocument();
  });

  it("blocks the confirm with a visible reason when there is no grant to remove", () => {
    // Reachable by design so the copy can be reviewed; the button carries its
    // reason in text rather than only in a title attribute.
    open({ sources: [direct], grantId: undefined });
    const confirm = screen.getByRole("button", { name: "Revoke access" });
    expect(confirm).toBeDisabled();
    expect(screen.getByText(/no direct access to revoke/i)).toBeInTheDocument();
  });

  it("offers no removal at all for an automatic source", () => {
    open({ sources: [mapping] });

    expect(screen.getByText("This one isn't yours to remove.")).toBeInTheDocument();
    // No destructive colour anywhere, because nothing is being destroyed.
    expect(screen.queryByRole("button", { name: /remove/i })).toBeNull();
    // It names the input role — the rule's input is the only per-person route,
    // stated both in the rule sentence and in the instruction beneath it.
    expect(screen.getAllByText(/3D Lab \/ operator/).length).toBeGreaterThan(0);
  });

  it("names the bundle and what is lost with it", () => {
    open({ sources: [bundle] });

    expect(screen.getByText(/Remove the Lab Tech bundle from Tomas Beck\?/)).toBeInTheDocument();
    expect(screen.getByText("They will lose")).toBeInTheDocument();
    expect(
      screen.getByText(/Every other role this bundle carries is removed too/i),
    ).toBeInTheDocument();
  });

  it("lists one entry per source when a role is held more than one way", () => {
    open({ sources: [direct, bundle, mapping], grantId: "g_88" });

    expect(screen.getByText(/holds Laser Lab \/ trained 3 ways/i)).toBeInTheDocument();
    expect(screen.getByText("Revoke direct access")).toBeInTheDocument();
    expect(screen.getByText("Remove bundle assignment")).toBeInTheDocument();
    expect(screen.getByText("Open the rule")).toBeInTheDocument();
  });

  it("removes the grant the row is about, not some other one", async () => {
    removeDirect.mockClear();
    open({ sources: [direct], grantId: "g_88" });

    fireEvent.click(screen.getByRole("button", { name: "Revoke access" }));
    expect(removeDirect).toHaveBeenCalledWith({ userId: "u_2f81", grantId: "g_88" });
  });
});
