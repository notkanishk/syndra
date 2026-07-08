"use client";

import { useCallback, useState } from "react";

import { Badge } from "@/components/ui/Badge";
import { Card, CardHeader, CardTitle } from "@/components/ui/Card";
import { request } from "@/lib/api-client";
import type { Paginated, UserGrant } from "@/components/zitadel/types";

// --- Section 4: All Grants ---
export default function AllGrants() {
  const [grants, setGrants] = useState<UserGrant[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  const run = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      const res = await request<Paginated<UserGrant>>("zitadel/grants?limit=500", { cache: "no-store" });
      setGrants(res.items ?? []);
      setTotal(res.total ?? 0);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(false);
    }
  }, []);

  return (
    <Card>
      <CardHeader>
        <CardTitle>All Grants</CardTitle>
      </CardHeader>
      <div className="flex items-center gap-3 flex-wrap">
        <button
          onClick={run}
          disabled={loading}
          className="rounded-lg bg-primary px-4 py-2 text-sm font-medium text-on-primary disabled:opacity-50"
        >
          {loading ? "Loading..." : "Load all grants"}
        </button>
        {total > 0 && <span className="text-xs text-on-surface-variant">{total} total · showing {grants.length}</span>}
      </div>

      {error && <p className="mt-3 text-sm text-error">{error}</p>}

      {grants.length > 0 && (
        <div className="mt-4 overflow-x-auto">
          <table className="min-w-full text-sm">
            <thead>
              <tr className="text-left text-xs uppercase tracking-[0.22em] text-on-surface-variant">
                <th className="py-2 pr-4">User ID</th>
                <th className="py-2 pr-4">Project ID</th>
                <th className="py-2 pr-4">Roles</th>
                <th className="py-2 pr-4">Grant ID</th>
              </tr>
            </thead>
            <tbody>
              {grants.map((g) => (
                <tr key={g.id} className="border-t border-outline-variant">
                  <td className="py-2 pr-4 font-mono text-xs">{g.userId}</td>
                  <td className="py-2 pr-4 font-mono text-xs">{g.projectId}</td>
                  <td className="py-2 pr-4">
                    <div className="flex flex-wrap gap-1">
                      {g.roleKeys.map((rk) => (
                        <Badge key={rk} variant="outline">{rk}</Badge>
                      ))}
                    </div>
                  </td>
                  <td className="py-2 pr-4 font-mono text-xs text-on-surface-variant">{g.id}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </Card>
  );
}
