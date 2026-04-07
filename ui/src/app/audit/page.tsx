"use client";

import { useEffect, useState } from "react";

import { Card, CardHeader, CardTitle } from "@/components/ui/Card";

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

export default function AuditView() {
  const [logs, setLogs] = useState<AuditEntry[]>([]);
  const [summary, setSummary] = useState<GovernanceSummary | null>(null);
  const [loading, setLoading] = useState(true);

  async function loadAll() {
    setLoading(true);
    try {
      const [logsRes, summaryRes] = await Promise.all([
        fetch("/api/proxy/audit?limit=50"),
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
    loadAll();
  }, []);

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
          <div className="flex items-center justify-between mb-6">
            <CardTitle>Recent Activity</CardTitle>
            <button onClick={loadAll} className="text-sm text-muted hover:text-foreground transition-colors">
              Refresh
            </button>
          </div>

          {loading ? (
            <p className="text-sm text-muted">Loading audit trail...</p>
          ) : logs.length === 0 ? (
            <p className="text-sm text-muted">No audit events recorded yet.</p>
          ) : (
            <div className="border border-border rounded-lg overflow-hidden">
              <div className="grid grid-cols-5 gap-4 px-4 py-3 bg-surfaceHover border-b border-border text-xs font-semibold text-muted uppercase tracking-wider">
                <span>Timestamp</span>
                <span>Action</span>
                <span>Actor</span>
                <span>Target</span>
                <span>Resource</span>
              </div>
              <div className="divide-y divide-border">
                {logs.map((log) => (
                  <div key={log.id} className="grid grid-cols-5 gap-4 px-4 py-3 text-sm hover:bg-surfaceHover transition-colors">
                    <span className="text-muted text-xs font-mono">{new Date(log.created_at).toLocaleString()}</span>
                    <span className={`font-medium ${actionColor(log.action)}`}>{log.action}</span>
                    <span className="text-foreground font-mono text-xs">{log.actor_id}</span>
                    <span className="text-muted font-mono text-xs">{log.target_id}</span>
                    <span className="text-muted font-mono text-xs truncate">{log.resource_id}</span>
                  </div>
                ))}
              </div>
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
                {(summary?.expiring_grants || []).map((grant) => (
                  <div key={grant.id} className="rounded-lg border border-border bg-surfaceHover p-3">
                    <p className="font-medium text-foreground">
                      {grant.user_id} {"->"} {grant.project_id}:{grant.role_key}
                    </p>
                    <p className="mt-1 text-xs text-muted">
                      Expires {grant.expires_at ? new Date(grant.expires_at).toLocaleString() : "soon"}
                    </p>
                  </div>
                ))}
              </div>
            </div>

            <div>
              <p className="text-xs uppercase tracking-[0.22em] text-muted">Cleanup Suggestions</p>
              <div className="mt-2 space-y-2">
                {(summary?.cleanup_hints || []).map((hint) => (
                  <div key={hint} className="rounded-lg border border-border bg-surfaceHover p-3 text-sm text-muted">
                    {hint}
                  </div>
                ))}
              </div>
            </div>
          </div>
        </Card>
      </div>
    </div>
  );
}
