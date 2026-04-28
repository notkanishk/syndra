"use client";

import Link from "next/link";

import { UserName } from "@/components/names/UserName";
import { Badge } from "@/components/ui/Badge";
import { Card, CardHeader, CardTitle } from "@/components/ui/Card";
import { EmptyState } from "@/components/ui/EmptyState";
import { Eyebrow } from "@/components/ui/Eyebrow";
import { Pulse } from "@/components/ui/Pulse";
import { Skeleton } from "@/components/ui/Skeleton";
import { useDashboardSummary } from "@/lib/queries/useDashboard";
import { useIntents } from "@/lib/queries/useIntents";

interface StatCardProps {
  eyebrow: string;
  value: number | string;
  caption: string;
  href?: string;
  loading?: boolean;
  tone?: "default" | "warn" | "error";
}

function StatCard({ eyebrow, value, caption, href, loading, tone = "default" }: StatCardProps) {
  const valueClass =
    tone === "error"
      ? "text-[var(--error)]"
      : tone === "warn"
        ? "text-[var(--warning)]"
        : "text-on-surface";
  const inner = (
    <Card variant="glass" className="h-full">
      <Eyebrow tone="muted">{eyebrow}</Eyebrow>
      <p className={`mt-3 font-display text-5xl font-semibold tracking-tight ${valueClass}`}>
        {loading ? <Skeleton className="inline-block h-12 w-16 align-middle" /> : value}
      </p>
      <p className="mt-2 text-sm text-on-surface-variant">{caption}</p>
    </Card>
  );
  return href ? (
    <Link href={href} className="block focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary-container rounded-card">
      {inner}
    </Link>
  ) : (
    inner
  );
}

function intentTone(status: string): "success" | "warn" | "error" | "info" {
  if (status === "succeeded") return "success";
  if (status === "failed") return "error";
  if (status === "in_flight" || status === "pending") return "warn";
  return "info";
}

interface AdminDashboardProps {
  adminName: string;
}

/**
 * Admin overview client island. Owns its own data via React Query so the
 * surrounding RSC stays small (session check + portal routing only). The
 * member portal lives in the parent server component and never mounts this.
 *
 * Stage 2 contract:
 * - Stat grid is capped at xl:grid-cols-4 with glass cards over the global
 *   bg-blob-hero so the hero never looks empty at ultra-wide.
 * - Recent activity is a 2/3 + 1/3 row; the right rail is the live operations
 *   pulse (top three in-flight intents) admins use to confirm sync activity.
 * - Every UID render goes through <UserName/> so operators see names, never
 *   raw Zitadel UUIDs.
 */
export function AdminDashboard({ adminName }: AdminDashboardProps) {
  const summary = useDashboardSummary();
  const intents = useIntents({ status: "in_flight", limit: 3 });

  const pendingCount = summary.governance.data?.pending_requests.length ?? 0;
  const expiringCount = summary.governance.data?.expiring_grants.length ?? 0;
  const projectCount = summary.projects.data?.length ?? 0;
  const bundleCount = summary.bundles.data?.length ?? 0;
  const recent = (summary.audit.data ?? []).slice(0, 8);

  return (
    <div className="space-y-8 animate-fade-in-up">
      <header>
        <Eyebrow tone="primary">Operator Console</Eyebrow>
        <h1 className="mt-3 font-display text-4xl font-semibold tracking-tight text-on-surface">
          Welcome back, {adminName}
        </h1>
        <p className="mt-2 text-on-surface-variant">
          Identity orchestrator across control-plane workflows, access lineage, and token simulation.
        </p>
      </header>

      <div className="grid grid-cols-1 gap-6 md:grid-cols-2 xl:grid-cols-4">
        <StatCard
          eyebrow="Pending Requests"
          value={pendingCount}
          caption="Awaiting admin review on /requests"
          href="/requests"
          loading={summary.governance.isLoading}
          tone={pendingCount > 0 ? "warn" : "default"}
        />
        <StatCard
          eyebrow="Expiring Grants"
          value={expiringCount}
          caption="Direct grants ending in the next 14 days"
          href="/audit"
          loading={summary.governance.isLoading}
          tone={expiringCount > 0 ? "warn" : "default"}
        />
        <StatCard
          eyebrow="Projects"
          value={projectCount}
          caption="Policy domains mapped from Zitadel"
          href="/projects"
          loading={summary.projects.isLoading}
        />
        <StatCard
          eyebrow="Bundles"
          value={bundleCount}
          caption="Reusable role groupings"
          href="/bundles"
          loading={summary.bundles.isLoading}
        />
      </div>

      <div className="grid grid-cols-1 gap-6 xl:grid-cols-3">
        <Card variant="glass" className="xl:col-span-2">
          <CardHeader>
            <CardTitle>Recent activity</CardTitle>
          </CardHeader>
          {summary.audit.isLoading ? (
            <div className="space-y-3" aria-busy="true">
              {Array.from({ length: 4 }).map((_, i) => (
                <Skeleton key={i} className="h-14 w-full" />
              ))}
            </div>
          ) : recent.length === 0 ? (
            <EmptyState
              title="No activity yet"
              description="Bundle assignments, role grants, and access decisions will appear here as admins act."
              action={{ label: "Open audit log", href: "/audit" }}
            />
          ) : (
            <ul className="space-y-3">
              {recent.map((entry) => (
                <li
                  key={entry.id}
                  className="rounded-card border border-outline-variant bg-surface-container-low/40 p-4"
                >
                  <div className="flex items-start justify-between gap-3">
                    <div className="min-w-0">
                      <div className="flex flex-wrap items-baseline gap-2">
                        <Badge variant="secondary" className="text-[10px]">
                          {entry.action}
                        </Badge>
                        <span className="text-sm text-on-surface">
                          <UserName id={entry.actor_id} />
                        </span>
                        {entry.target_id && entry.target_id !== "-" && (
                          <span className="text-sm text-on-surface-variant">
                            →{" "}
                            <span className="text-on-surface">
                              <UserName id={entry.target_id} />
                            </span>
                          </span>
                        )}
                      </div>
                    </div>
                    <span className="shrink-0 text-xs text-on-surface-variant">
                      {new Date(entry.created_at).toLocaleString()}
                    </span>
                  </div>
                </li>
              ))}
            </ul>
          )}
        </Card>

        <Card variant="glass">
          <CardHeader>
            <CardTitle>Live operations pulse</CardTitle>
          </CardHeader>
          <p className="text-xs text-on-surface-variant">
            Top three provisioning intents currently in flight. Polled every 5 seconds.
          </p>
          <div className="mt-4 space-y-3">
            {intents.isLoading ? (
              <Skeleton className="h-20 w-full" />
            ) : (intents.data ?? []).length === 0 ? (
              <p className="rounded-card border border-outline-variant bg-surface-container-low/40 p-4 text-sm text-on-surface-variant">
                Sync queues are idle — nothing in flight right now.
              </p>
            ) : (
              (intents.data ?? []).map((intent) => (
                <div
                  key={intent.id}
                  className="rounded-card border border-outline-variant bg-surface-container-low/40 p-3 text-sm"
                >
                  <div className="flex items-center gap-2">
                    <Pulse variant={intentTone(intent.status)} />
                    <span className="font-medium text-on-surface">{intent.action}</span>
                    <Badge variant="outline" className="ml-auto text-[10px]">
                      {intent.status}
                    </Badge>
                  </div>
                  <p className="mt-2 text-xs text-on-surface-variant">
                    Target <UserName id={intent.target_uid} /> · group {intent.lldap_group || "—"}
                  </p>
                </div>
              ))
            )}
          </div>
        </Card>
      </div>
    </div>
  );
}
