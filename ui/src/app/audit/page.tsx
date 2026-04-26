"use client";

import { useEffect, useMemo, useState } from "react";

import { Badge } from "@/components/ui/Badge";
import { Card, CardHeader, CardTitle } from "@/components/ui/Card";
import { EmptyState } from "@/components/ui/EmptyState";
import { Skeleton } from "@/components/ui/Skeleton";
import { describeExpiry } from "@/lib/format";
import { useDebounce } from "@/lib/useDebounce";

interface AuditEntry {
  id: string;
  actor_id: string;
  target_id: string;
  action: string;
  resource_id: string;
  created_at: string;
}

interface GovernanceSummary {
  pending_requests: Array<{ id: string }>;
  expiring_grants: Array<{
    id: string;
    user_id: string;
    project_id: string;
    role_key: string;
    expires_at?: string | null;
  }>;
  cleanup_hints: string[];
}

const DAY_MS = 24 * 60 * 60 * 1000;

function dayBucketLabel(ts: string, now: Date = new Date()): string {
  const d = new Date(ts);
  const startOfDay = (date: Date) => new Date(date.getFullYear(), date.getMonth(), date.getDate()).getTime();
  const diffDays = Math.round((startOfDay(now) - startOfDay(d)) / DAY_MS);
  if (diffDays <= 0) return "Today";
  if (diffDays === 1) return "Yesterday";
  if (diffDays < 7) return d.toLocaleDateString(undefined, { weekday: "long" });
  return d.toLocaleDateString();
}

function actionCategory(action: string): "approved" | "rejected" | "created" | "updated" | "other" {
  if (action.includes("approved")) return "approved";
  if (action.includes("rejected") || action.includes("revoked") || action.includes("deleted")) return "rejected";
  if (action.includes("created")) return "created";
  if (action.includes("updated") || action.includes("bumped")) return "updated";
  return "other";
}

export default function AuditView() {
  const [logs, setLogs] = useState<AuditEntry[]>([]);
  const [summary, setSummary] = useState<GovernanceSummary | null>(null);
  const [loading, setLoading] = useState(true);
  const [limit, setLimit] = useState(50);
  const [actionFilter, setActionFilter] = useState<"all" | "approved" | "rejected" | "created" | "updated" | "other">("all");
  const [actorFilter, setActorFilter] = useState<string>("");
  const [searchQuery, setSearchQuery] = useState("");
  const debouncedSearch = useDebounce(searchQuery, 200);

  async function loadAll(currentLimit = limit) {
    setLoading(true);
    try {
      const [logsRes, summaryRes] = await Promise.all([
        fetch(`/api/proxy/audit?limit=${currentLimit}`),
        fetch("/api/proxy/governance/summary"),
      ]);
      const logData = await logsRes.json();
      const summaryData = await summaryRes.json();
      setLogs(Array.isArray(logData) ? logData : []);
      setSummary(summaryData);
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => {
    loadAll(50);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Distinct actors observed in the loaded logs feed the actor select.
  const actors = useMemo(() => {
    const set = new Set<string>();
    for (const log of logs) set.add(log.actor_id);
    return Array.from(set).sort();
  }, [logs]);

  // Apply filters and group by day.
  const filteredGroups = useMemo(() => {
    const q = debouncedSearch.trim().toLowerCase();
    const matched = logs.filter((log) => {
      if (actionFilter !== "all" && actionCategory(log.action) !== actionFilter) return false;
      if (actorFilter && log.actor_id !== actorFilter) return false;
      if (q) {
        const haystack = `${log.action} ${log.actor_id} ${log.target_id} ${log.resource_id}`.toLowerCase();
        if (!haystack.includes(q)) return false;
      }
      return true;
    });

    const groups = new Map<string, AuditEntry[]>();
    for (const log of matched) {
      const key = dayBucketLabel(log.created_at);
      const arr = groups.get(key) ?? [];
      arr.push(log);
      groups.set(key, arr);
    }
    return Array.from(groups.entries());
  }, [logs, actionFilter, actorFilter, debouncedSearch]);

  const actionColor = (action: string) => {
    if (action.includes("approved") || action.includes("created")) return "text-emerald-500";
    if (action.includes("rejected")) return "text-red-500";
    if (action.includes("updated")) return "text-amber-500";
    return "text-primary";
  };

  return (
    <div className="space-y-6 animate-fade-in-up">
      <header>
        <h1 className="text-3xl font-bold text-foreground">Audit & Governance</h1>
        <p className="text-muted mt-2">Track approvals, expiring access, and cleanup signals in one operational timeline.</p>
      </header>

      <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
        <Card>
          <CardHeader>
            <CardTitle>Pending Requests</CardTitle>
          </CardHeader>
          <p className="text-4xl font-bold text-primary">{summary?.pending_requests.length || 0}</p>
          <p className="text-sm text-muted mt-1">Awaiting admin review</p>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle>Expiring Grants</CardTitle>
          </CardHeader>
          <p className="text-4xl font-bold text-primary">{summary?.expiring_grants.length || 0}</p>
          <p className="text-sm text-muted mt-1">Due within 14 days</p>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle>Cleanup Hints</CardTitle>
          </CardHeader>
          <p className="text-4xl font-bold text-primary">{summary?.cleanup_hints.length || 0}</p>
          <p className="text-sm text-muted mt-1">Governance nudges</p>
        </Card>
      </div>

      <div className="grid grid-cols-1 xl:grid-cols-[1.15fr,0.85fr] gap-6">
        <Card>
          <div className="flex items-center justify-between mb-4">
            <CardTitle>Recent Activity</CardTitle>
            <button
              onClick={() => loadAll(limit)}
              className="text-sm text-muted hover:text-foreground transition-colors focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary rounded"
            >
              Refresh
            </button>
          </div>

          <div className="mb-4 grid grid-cols-1 md:grid-cols-3 gap-2">
            <input
              type="search"
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              placeholder="Search by action, actor, or resource"
              aria-label="Search audit log"
              className="rounded-lg border border-border bg-surface px-3 py-2 text-sm focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
            />
            <select
              value={actionFilter}
              onChange={(e) => setActionFilter(e.target.value as typeof actionFilter)}
              aria-label="Filter by action category"
              className="rounded-lg border border-border bg-surface px-3 py-2 text-sm"
            >
              <option value="all">All actions</option>
              <option value="approved">Approved</option>
              <option value="rejected">Rejected / Revoked</option>
              <option value="created">Created</option>
              <option value="updated">Updated</option>
              <option value="other">Other</option>
            </select>
            <select
              value={actorFilter}
              onChange={(e) => setActorFilter(e.target.value)}
              aria-label="Filter by actor"
              className="rounded-lg border border-border bg-surface px-3 py-2 text-sm"
            >
              <option value="">All actors</option>
              {actors.map((actor) => (
                <option key={actor} value={actor}>{actor}</option>
              ))}
            </select>
          </div>

          {loading ? (
            <div className="space-y-2" aria-busy="true" aria-label="Loading audit trail">
              {Array.from({ length: 6 }).map((_, i) => (
                <Skeleton key={i} className="h-10 w-full" />
              ))}
            </div>
          ) : logs.length === 0 ? (
            <EmptyState
              title="No activity yet"
              description="Bundle assignments, role grants, and access decisions will appear here as admins act."
            />
          ) : filteredGroups.length === 0 ? (
            <EmptyState
              title="No entries match the current filters"
              description="Clear a filter or change the search to see more activity."
              action={{ label: "Clear filters", onClick: () => { setActionFilter("all"); setActorFilter(""); setSearchQuery(""); } }}
            />
          ) : (
            <div className="space-y-6">
              {filteredGroups.map(([day, entries]) => (
                <section key={day}>
                  <h3 className="sticky top-0 z-10 bg-surface/90 backdrop-blur px-1 py-1 text-[10px] font-semibold uppercase tracking-[0.18em] text-muted">
                    {day} · <span className="text-foreground">{entries.length}</span>
                  </h3>
                  <div className="border border-border rounded-lg overflow-hidden">
                    <table className="w-full text-left">
                      <caption className="sr-only">{`Audit entries for ${day}`}</caption>
                      <thead className="bg-surfaceHover border-b border-border">
                        <tr className="text-xs font-semibold text-muted uppercase tracking-wider">
                          <th scope="col" className="px-4 py-3 font-semibold">Time</th>
                          <th scope="col" className="px-4 py-3 font-semibold">Action</th>
                          <th scope="col" className="px-4 py-3 font-semibold">Actor</th>
                          <th scope="col" className="px-4 py-3 font-semibold">Target</th>
                          <th scope="col" className="px-4 py-3 font-semibold">Resource</th>
                        </tr>
                      </thead>
                      <tbody className="divide-y divide-border">
                        {entries.map((log) => (
                          <tr key={log.id} className="text-sm hover:bg-surfaceHover transition-colors">
                            <td className="px-4 py-3 text-muted text-xs font-mono whitespace-nowrap">
                              {new Date(log.created_at).toLocaleTimeString()}
                            </td>
                            <td className={`px-4 py-3 font-medium ${actionColor(log.action)}`}>{log.action}</td>
                            <td className="px-4 py-3 text-foreground font-mono text-xs">{log.actor_id}</td>
                            <td className="px-4 py-3 text-muted font-mono text-xs">{log.target_id}</td>
                            <td className="px-4 py-3 text-muted font-mono text-xs truncate max-w-[16rem]">{log.resource_id}</td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                </section>
              ))}

              {logs.length >= limit && limit < 200 && (
                <div className="flex justify-center pt-2">
                  <button
                    onClick={() => {
                      const next = Math.min(limit + 50, 200);
                      setLimit(next);
                      loadAll(next);
                    }}
                    className="rounded-lg border border-border px-4 py-2 text-xs font-semibold uppercase tracking-[0.16em] text-muted hover:text-foreground hover:border-primary/40 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
                  >
                    Load more
                  </button>
                </div>
              )}
            </div>
          )}
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Governance Watchlist</CardTitle>
          </CardHeader>
          <div className="space-y-4">
            <div>
              <p className="text-xs uppercase tracking-[0.22em] text-muted">Expiring Access</p>
              <div className="mt-2 space-y-2">
                {(summary?.expiring_grants || []).length === 0 ? (
                  <p className="text-xs text-muted">No grants expiring in the next 14 days.</p>
                ) : (
                  [...(summary?.expiring_grants || [])]
                    .sort((a, b) => {
                      const at = a.expires_at ? new Date(a.expires_at).getTime() : Number.MAX_SAFE_INTEGER;
                      const bt = b.expires_at ? new Date(b.expires_at).getTime() : Number.MAX_SAFE_INTEGER;
                      return at - bt;
                    })
                    .map((grant) => {
                      const exp = describeExpiry(grant.expires_at);
                      const tone = exp?.tone ?? "neutral";
                      const toneClasses =
                        tone === "expired"
                          ? "border-red-500/40 bg-red-500/10"
                          : tone === "critical"
                            ? "border-red-500/30 bg-red-500/5"
                            : tone === "warning"
                              ? "border-amber-500/30 bg-amber-500/5"
                              : "border-border bg-surfaceHover";
                      const badgeVariant: "destructive" | "outline" | "secondary" =
                        tone === "expired" || tone === "critical"
                          ? "destructive"
                          : tone === "warning"
                            ? "outline"
                            : "secondary";
                      return (
                        <div key={grant.id} className={`rounded-lg border p-3 ${toneClasses}`}>
                          <div className="flex items-start justify-between gap-3">
                            <div>
                              <p className="font-medium text-foreground">
                                {grant.user_id} → {grant.project_id}:{grant.role_key}
                              </p>
                              <p className="mt-1 text-xs text-muted">
                                {grant.expires_at ? new Date(grant.expires_at).toLocaleString() : "no fixed expiry"}
                              </p>
                            </div>
                            {exp && (
                              <Badge variant={badgeVariant} className="text-[10px] uppercase tracking-[0.16em]">
                                {exp.countdown}
                              </Badge>
                            )}
                          </div>
                        </div>
                      );
                    })
                )}
              </div>
            </div>

            <div>
              <p className="text-xs uppercase tracking-[0.22em] text-muted">Cleanup Suggestions</p>
              <div className="mt-2 space-y-2">
                {(summary?.cleanup_hints || []).length === 0 ? (
                  <p className="text-xs text-muted">No cleanup hints right now — everything looks tidy.</p>
                ) : (
                  (summary?.cleanup_hints || []).map((hint) => (
                    <div key={hint} className="rounded-lg border border-border bg-surfaceHover p-3 text-sm text-muted">
                      {hint}
                    </div>
                  ))
                )}
              </div>
            </div>
          </div>
        </Card>
      </div>
    </div>
  );
}
