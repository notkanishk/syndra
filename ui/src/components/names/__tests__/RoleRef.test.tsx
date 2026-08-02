// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { RoleRef } from "@/components/names/RoleRef";

const miss = { value: undefined, resolved: true } as const;

vi.mock("@/lib/queries/useNameResolver", () => ({
  useNameResolver: () => ({
    resolveUser: () => miss,
    resolveProject: (id: string) => {
      const names: Record<string, string> = { "p-print": "Printing Lab", "p-metal": "Metal Shop" };
      return names[id] ? { value: { name: names[id] }, resolved: true } : miss;
    },
    resolveRole: () => miss,
    resolveBundle: () => miss,
  }),
}));

describe("RoleRef", () => {
  it("names the project alongside the key", () => {
    render(<RoleRef projectId="p-print" roleKey="admin" />);
    expect(screen.getByText("Printing Lab")).toBeInTheDocument();
    expect(screen.getByText("admin")).toBeInTheDocument();
  });

  // The whole reason this component exists: `admin` alone is not an identity.
  it("renders the same key differently in two projects", () => {
    const { container: printing } = render(<RoleRef projectId="p-print" roleKey="admin" />);
    const { container: metal } = render(<RoleRef projectId="p-metal" roleKey="admin" />);
    expect(printing.textContent).toContain("Printing Lab");
    expect(metal.textContent).toContain("Metal Shop");
    expect(printing.textContent).not.toBe(metal.textContent);
  });

  it("renders an em dash rather than half an identity when either side is missing", () => {
    const { container: noProject } = render(<RoleRef projectId={null} roleKey="admin" />);
    expect(noProject.textContent).toBe("—");
    const { container: noRole } = render(<RoleRef projectId="p-print" roleKey={undefined} />);
    expect(noRole.textContent).toBe("—");
  });
});
