"use client";

import { useEffect, useState, useCallback } from "react";
import { Card, CardHeader, CardTitle } from "@/components/ui/Card";
import { Badge } from "@/components/ui/Badge";

interface AuditEntry {
  id: string;
  actor_id: string;
  target_id: string;
  action: string;
  resource_id: string;
  created_at: string;
}

export default function AuditView() {
  const [logs, setLogs] = useState<AuditEntry[]>([]);
  const [loading, setLoading] = useState(true);

  const loadLogs = useCallback(async () => {
    setLoading(true);
    try {
      const res = await fetch("/api/proxy/audit?limit=50");
      const data = await res.json();
      setLogs(Array.isArray(data) ? data : []);
    } catch {
      setLogs([]);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    loadLogs();
  }, [loadLogs]);

  const actionColor = (action: string) => {
    if (action.includes("created")) return "text-emerald-500";
    if (action.includes("deleted")) return "text-red-500";
    if (action.includes("updated")) return "text-amber-500";
    return "text-primary";
  };

  return (
    <div className="space-y-6 animate-fade-in-up">
      <header>
        <h1 className="text-3xl font-bold text-foreground">Audit Log</h1>
        <p className="text-muted mt-2">
          Timeline of all governance actions — who granted what, and when.
        </p>
      </header>

      <Card>
        <div className="flex items-center justify-between mb-6">
          <CardTitle>Recent Activity</CardTitle>
          <button
            onClick={loadLogs}
            className="text-sm text-muted hover:text-foreground transition-colors"
          >
            ↻ Refresh
          </button>
        </div>

        {loading ? (
          <div className="text-center py-10">
            <div className="inline-block w-6 h-6 border-2 border-primary border-t-transparent rounded-full animate-spin"></div>
            <p className="text-muted mt-3 text-sm">Loading audit trail...</p>
          </div>
        ) : logs.length === 0 ? (
          <div className="text-center py-10 border border-dashed border-border rounded-lg">
            <p className="text-muted">No audit events recorded yet.</p>
            <p className="text-xs text-muted mt-1">Actions will appear here as rules and bundles are created.</p>
          </div>
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
                  <span className="text-muted text-xs font-mono">
                    {new Date(log.created_at).toLocaleString()}
                  </span>
                  <span className={`font-medium ${actionColor(log.action)}`}>
                    {log.action}
                  </span>
                  <span className="text-foreground font-mono text-xs">{log.actor_id}</span>
                  <span className="text-muted font-mono text-xs">{log.target_id}</span>
                  <span className="text-muted font-mono text-xs truncate">{log.resource_id}</span>
                </div>
              ))}
            </div>
          </div>
        )}
      </Card>
    </div>
  );
}
