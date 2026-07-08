"use client";

import { useCallback, useState } from "react";

import { Badge } from "@/components/ui/Badge";
import { Card, CardHeader, CardTitle } from "@/components/ui/Card";
import { request } from "@/lib/api-client";
import type { HealthResponse } from "@/components/zitadel/types";

// --- Section 1: M2M Health ---
export default function Health() {
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState<HealthResponse | null>(null);
  const [networkError, setNetworkError] = useState<string>("");

  const run = useCallback(async () => {
    setLoading(true);
    setNetworkError("");
    try {
      // preserveErrorBody: structured non-2xx payloads ({status: "disabled"} or
      // {status: "error", error: ...}) render their full diagnostic detail
      // instead of collapsing into a generic error.
      setResult(
        await request<HealthResponse>("zitadel/health", {
          preserveErrorBody: true,
          cache: "no-store",
        }),
      );
    } catch (err) {
      // Only reached on transport-level failures (proxy unreachable, JSON parse).
      setNetworkError(err instanceof Error ? err.message : String(err));
      setResult(null);
    } finally {
      setLoading(false);
    }
  }, []);

  const badge = result?.status === "ok"
    ? <Badge>ok</Badge>
    : result?.status === "disabled"
      ? <Badge variant="secondary">disabled</Badge>
      : result?.status === "error"
        ? <Badge variant="destructive">error</Badge>
        : networkError
          ? <Badge variant="destructive">unreachable</Badge>
          : null;

  return (
    <Card>
      <CardHeader>
        <CardTitle>M2M Health</CardTitle>
      </CardHeader>
      <div className="flex items-center gap-3 flex-wrap">
        <button
          onClick={run}
          disabled={loading}
          className="rounded-lg bg-primary px-4 py-2 text-sm font-medium text-on-primary disabled:opacity-50"
        >
          {loading ? "Checking..." : "Check connection"}
        </button>
        {badge}
        {result?.domain && <span className="text-xs text-on-surface-variant">domain: {result.domain}</span>}
        {result?.latency_ms !== undefined && <span className="text-xs text-on-surface-variant">· {result.latency_ms}ms</span>}
        {result?.projects_total !== undefined && (
          <span className="text-xs text-on-surface-variant">· {result.projects_total} projects</span>
        )}
      </div>
      {result?.error && <p className="mt-3 text-sm text-error">{result.error}</p>}
      {networkError && <p className="mt-3 text-sm text-error">{networkError}</p>}
      {result && (
        <details className="mt-3">
          <summary className="cursor-pointer text-xs text-on-surface-variant">raw response</summary>
          <pre className="mt-2 overflow-x-auto rounded-lg bg-surface-container-high p-3 text-xs text-on-surface">
            {JSON.stringify(result, null, 2)}
          </pre>
        </details>
      )}
    </Card>
  );
}
