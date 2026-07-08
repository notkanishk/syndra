"use client";

import { Card } from "@/components/ui/Card";
import { Eyebrow } from "@/components/ui/Eyebrow";
import { Pulse } from "@/components/ui/Pulse";
import { useZitadelHealth } from "@/lib/queries/useZitadel";

import AllGrants from "@/components/zitadel/AllGrants";
import Health from "@/components/zitadel/Health";
import Projects from "@/components/zitadel/Projects";
import Rotation from "@/components/zitadel/Rotation";
import Users from "@/components/zitadel/Users";

export default function ZitadelDiagnostics() {
  return (
    <div className="p-8 space-y-6 animate-fade-in-up relative z-10">
      <header className="flex flex-col gap-2">
        <Eyebrow>Operations</Eyebrow>
        <h1 className="text-3xl font-semibold text-on-surface mt-1 font-display">
          Zitadel diagnostics
        </h1>
        <p className="text-sm text-on-surface-variant max-w-2xl">
          Exercises the live <code className="text-on-surface">/api/v1/zitadel/*</code>{" "}
          management surface — health probe, projects &amp; roles, users &amp; grants. Use
          this to verify a new Zitadel deployment or service-account rotation before
          trusting the orchestrator.
        </p>
      </header>

      <LiveStatusTile />
      <Health />
      <Rotation />
      <Projects />
      <Users />
      <AllGrants />
    </div>
  );
}

/**
 * Top-of-page glass tile. Polls /zitadel/health every 10s and shows the live
 * connection state through a Pulse — steady-green when ok, amber-pulse when
 * disabled (locally configured but not exercising live calls), red-pulse when
 * the management API is unreachable. The polling cadence matches the spec
 * (operations dashboards ≤10s) and pauses when the tab is hidden.
 */
function LiveStatusTile() {
  const { data, isFetching, error } = useZitadelHealth();

  const status = data?.status;
  const variant: "success" | "warn" | "error" | "info" =
    status === "ok"
      ? "success"
      : status === "disabled"
        ? "warn"
        : status === "error" || error
          ? "error"
          : "info";
  const label =
    status === "ok"
      ? "Connected"
      : status === "disabled"
        ? "Disabled (local-policy-only)"
        : status === "error"
          ? "Error"
          : error
            ? "Unreachable"
            : "Checking…";

  return (
    <Card variant="glass">
      <div className="flex flex-wrap items-center justify-between gap-4">
        <div className="flex items-center gap-3">
          <Pulse variant={variant} static={variant === "success"} ariaLabel={label} />
          <div>
            <Eyebrow>Live status</Eyebrow>
            <p className="mt-1 text-xl font-semibold text-on-surface font-display">
              {label}
            </p>
          </div>
        </div>
        <div className="flex flex-wrap items-center gap-3 text-xs text-on-surface-variant">
          {data?.domain && <span>domain: {data.domain}</span>}
          {data?.latency_ms !== undefined && <span>· {data.latency_ms}ms</span>}
          {data?.projects_total !== undefined && (
            <span>· {data.projects_total} projects</span>
          )}
          {isFetching && <span aria-hidden="true">·</span>}
          {isFetching && <span>refreshing…</span>}
        </div>
      </div>
      {data?.error && <p className="mt-3 text-sm text-[var(--error)]">{data.error}</p>}
    </Card>
  );
}
