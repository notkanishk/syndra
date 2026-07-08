import { describe, it, expect, vi, beforeEach } from "vitest";
import type { Mock } from "vitest";
import { redirect } from "next/navigation";

// redirect throws in real Next.js to abort rendering; mock preserves that
// control-flow so the guard's early-return semantics are exercised.
vi.mock("next/navigation", () => ({
  redirect: vi.fn(() => {
    throw new Error("REDIRECT");
  }),
}));
vi.mock("@/lib/session", () => ({ getSession: vi.fn() }));

import { getSession } from "@/lib/session";
import OperationsPage from "@/app/operations/page";
import RecentCascadesPage from "@/app/operations/cascades/page";
import PendingPropagationsPage from "@/app/governance/pending/page";
import DriftPage from "@/app/governance/drift/page";
import GrantsPage from "@/app/grants/page";
import ZitadelPage from "@/app/zitadel/page";

// [route label, page server component]. New page-gated routes get a row here.
const ROUTES: Array<[string, () => Promise<unknown>]> = [
  ["/operations", OperationsPage],
  ["/operations/cascades", RecentCascadesPage],
  ["/governance/pending", PendingPropagationsPage],
  ["/governance/drift", DriftPage],
  ["/grants", GrantsPage],
  ["/zitadel", ZitadelPage],
];

describe("admin page server guards", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  for (const [route, Page] of ROUTES) {
    describe(route, () => {
      it("redirects members to /", async () => {
        (getSession as Mock).mockResolvedValue({ id: "u1", role: "user" });
        await expect(Page()).rejects.toThrow("REDIRECT");
        expect(redirect).toHaveBeenCalledWith("/");
      });

      it("redirects anonymous requests to /login", async () => {
        (getSession as Mock).mockResolvedValue(null);
        await expect(Page()).rejects.toThrow("REDIRECT");
        expect(redirect).toHaveBeenCalledWith("/login");
      });

      it("renders for admins without redirecting", async () => {
        (getSession as Mock).mockResolvedValue({ id: "a1", role: "admin" });
        await expect(Page()).resolves.toBeTruthy();
        expect(redirect).not.toHaveBeenCalled();
      });
    });
  }
});
