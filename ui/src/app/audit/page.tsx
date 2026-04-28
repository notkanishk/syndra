"use client";

import { useEffect, useMemo, useState } from "react";

import { ProjectName } from "@/components/names/ProjectName";
import { RoleName } from "@/components/names/RoleName";
import { UserName } from "@/components/names/UserName";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Card, CardHeader, CardTitle } from "@/components/ui/Card";
import { Drawer } from "@/components/ui/Drawer";
import { EmptyState } from "@/components/ui/EmptyState";
import { Eyebrow } from "@/components/ui/Eyebrow";
import { Input } from "@/components/ui/Input";
import { JsonView } from "@/components/ui/JsonView";
import { Pulse } from "@/components/ui/Pulse";
import { Select } from "@/components/ui/Select";
import { Skeleton } from "@/components/ui/Skeleton";
import { describeExpiry } from "@/lib/format";
import { type AuditEntry, useAuditEntries } from "@/lib/queries/useAudit";
import {
  type ExpiringGrant,
  useGovernanceSummary,
} from "@/lib/queries/useGovernance";
import { useNameResolver } from "@/lib/queries/useNameResolver";
import { useDebounce } from "@/lib/useDebounce";

const DAY_MS = 24 * 60 * 60 * 1000;

function dayBucketLabel(ts: string, now: Date = new Date()): string {
  const d = new Date(ts);
  const startOfDay = (date: Date) =>
    new Date(date.getFullYear(), date.getMonth(), date.getDate()).getTime();
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

function actionTone(action: string): "success" | "warn" | "error" | "info" {
  const cat = actionCategory(action);
  if (cat === "approved" || cat === "created") return "success";
  if (cat === "rejected") return "error";
  if (cat === "updated") return "warn";
  return "info";
}

function isSystemActor(id: string) {
  return !id || id === "-" || id === "system";
}

/**
 * Compute a Pulse variant for a watchlist row based on how soon the grant
 * expires. The audit timeline is read-only, so Pulse is purely a visual cue
 * for which rows need an admin's attention next.
 */
function watchlistTone(grant: ExpiringGrant): "success" | "warn" | "error" | "info" {
  const exp = describeExpiry(grant.expires_at);
  if (!exp) return "info";
  if (exp.tone === "expired" || exp.tone === "critical") return "error";
  if (exp.tone === "warning") return "warn";
  return "info";
}

export default function AuditView() {
  const [limit, setLimit] = useState(50);
  const [actionFilter, setActionFilter] = useState<
    "all" | "approved" | "rejected" | "created" | "updated" | "other"
  >("all");
  const [actorFilter, setActorFilter] = useState<string>("");
  const [searchQuery, setSearchQuery] = useState("");
  const debouncedSearch = useDebounce(searchQuery, 200);
  const [drawerEntry, setDrawerEntry] = useState<AuditEntry | null>(null);

  const auditQuery = useAuditEntries({ limit });
  const governanceQuery = useGovernanceSummary();
  const logs = useMemo(() => auditQuery.data ?? [], [auditQuery.data]);
  const summary = governanceQuery.data ?? null;

  // Distinct actors observed in the loaded logs feed the actor select. Values
  // remain UUID-typed so the backend filter matches; only the rendered label
  // is name-resolved (see <UserName/> below).
  const actors = useMemo(() => {
    const set = new Set<string>();
    for (const log of logs) set.add(log.actor_id);
    return Array.from(set).sort();
  }, [logs]);

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

  const expiringGrants = summary?.expiring_grants ?? [];
  const cleanupHints = summary?.cleanup_hints ?? [];

  return (
    <div className="space-y-6 animate-fade-in-up">
      <header>
        <Eyebrow tone="primary">Audit &amp; Governance</Eyebrow>
        <h1 className="mt-3 font-display text-3xl font-semibold tracking-tight text-on-surface">
          Operational timeline
        </h1>
        <p className="mt-2 text-on-surface-variant">
          Track approvals, expiring access, and cleanup signals in one place. Click any row to inspect the raw payload.
        </p>
      </header>

      <div className="grid grid-cols-1 gap-6 md:grid-cols-3">
        <Card variant="glass">
          <Eyebrow>Pending Requests</Eyebrow>
          <p className="mt-3 font-display text-5xl font-semibold tracking-tight text-on-surface">
            {summary?.pending_requests.length ?? 0}
          </p>
          <p className="mt-2 text-sm text-on-surface-variant">Awaiting admin review</p>
        </Card>
        <Card variant="glass">
          <Eyebrow>Expiring Grants</Eyebrow>
          <p className="mt-3 font-display text-5xl font-semibold tracking-tight text-on-surface">
            {expiringGrants.length}
          </p>
          <p className="mt-2 text-sm text-on-surface-variant">Due within 14 days</p>
        </Card>
        <Card variant="glass">
          <Eyebrow>Cleanup Hints</Eyebrow>
          <p className="mt-3 font-display text-5xl font-semibold tracking-tight text-on-surface">
            {cleanupHints.length}
          </p>
          <p className="mt-2 text-sm text-on-surface-variant">Governance nudges</p>
        </Card>
      </div>

      <div className="grid grid-cols-1 gap-6 xl:grid-cols-[1.15fr_0.85fr]">
        <Card variant="glass">
          <div className="mb-4 flex items-center justify-between">
            <CardTitle>Recent activity</CardTitle>
            <button
              onClick={() => auditQuery.refetch()}
              className="text-sm text-on-surface-variant transition-colors hover:text-on-surface focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary-container"
            >
              Refresh
            </button>
          </div>

          <div className="mb-4 grid grid-cols-1 gap-2 md:grid-cols-3">
            <Input
              type="search"
              value={searchQuery}
              onChange={(event) => setSearchQuery(event.target.value)}
              placeholder="Search by action, actor, or resource"
              aria-label="Search audit log"
            />
            <Select
              value={actionFilter}
              onChange={(event) => setActionFilter(event.target.value as typeof actionFilter)}
              aria-label="Filter by action category"
            >
              <option value="all">All actions</option>
              <option value="approved">Approved</option>
              <option value="rejected">Rejected / Revoked</option>
              <option value="created">Created</option>
              <option value="updated">Updated</option>
              <option value="other">Other</option>
            </Select>
            {/* Actor select keeps UUID values so the backend filter matches; the
                rendered label resolves through the name resolver. */}
            <ActorSelect
              actors={actors}
              value={actorFilter}
              onChange={setActorFilter}
            />
          </div>

          {auditQuery.isLoading ? (
            <div className="space-y-2" aria-busy="true" aria-label="Loading audit trail">
              {Array.from({ length: 6 }).map((_, i) => (
                <Skeleton key={i} className="h-14 w-full" />
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
              action={{
                label: "Clear filters",
                onClick: () => {
                  setActionFilter("all");
                  setActorFilter("");
                  setSearchQuery("");
                },
              }}
            />
          ) : (
            <div className="space-y-6">
              {filteredGroups.map(([day, entries]) => (
                <section key={day}>
                  <h3 className="sticky top-0 z-10 bg-surface-container/85 px-1 py-1 text-[10px] font-semibold uppercase tracking-[0.18em] text-on-surface-variant backdrop-blur">
                    {day} · <span className="text-on-surface">{entries.length}</span>
                  </h3>
                  <ul className="mt-2 space-y-2" aria-label={`Audit entries for ${day}`}>
                    {entries.map((log) => (
                      <li key={log.id}>
                        <button
                          type="button"
                          onClick={() => setDrawerEntry(log)}
                          className="w-full rounded-card border border-outline-variant bg-surface-container-low/40 p-4 text-left transition-colors hover:border-primary-container/60 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary-container"
                        >
                          <div className="flex flex-wrap items-center gap-2">
                            <Pulse variant={actionTone(log.action)} />
                            <span className="font-medium text-on-surface">{log.action}</span>
                            <span className="text-on-surface-variant">·</span>
                            <span className="text-sm text-on-surface">
                              {isSystemActor(log.actor_id) ? "System" : <UserName id={log.actor_id} />}
                            </span>
                            {log.target_id && log.target_id !== "-" && (
                              <>
                                <span className="text-on-surface-variant">→</span>
                                <span className="text-sm text-on-surface">
                                  <UserName id={log.target_id} />
                                </span>
                              </>
                            )}
                          </div>
                          <p className="mt-2 flex items-center gap-3 text-xs text-on-surface-variant">
                            <span className="font-mono whitespace-nowrap">
                              {new Date(log.created_at).toLocaleTimeString()}
                            </span>
                            {log.resource_id && (
                              <span
                                className="truncate font-mono"
                                title={log.resource_id}
                              >
                                resource: {log.resource_id}
                              </span>
                            )}
                          </p>
                        </button>
                      </li>
                    ))}
                  </ul>
                </section>
              ))}

              {logs.length >= limit && limit < 200 && (
                <div className="flex justify-center pt-2">
                  <Button
                    size="sm"
                    variant="outline"
                    onClick={() => setLimit((current) => Math.min(current + 50, 200))}
                  >
                    Load more
                  </Button>
                </div>
              )}
            </div>
          )}
        </Card>

        <Card variant="glass">
          <CardHeader>
            <CardTitle>Governance Watchlist</CardTitle>
          </CardHeader>
          <div className="space-y-4">
            <div>
              <Eyebrow>Expiring Access</Eyebrow>
              <div className="mt-2 space-y-2">
                {expiringGrants.length === 0 ? (
                  <p className="text-xs text-on-surface-variant">
                    No grants expiring in the next 14 days.
                  </p>
                ) : (
                  [...expiringGrants]
                    .sort((a, b) => {
                      const at = a.expires_at ? new Date(a.expires_at).getTime() : Number.MAX_SAFE_INTEGER;
                      const bt = b.expires_at ? new Date(b.expires_at).getTime() : Number.MAX_SAFE_INTEGER;
                      return at - bt;
                    })
                    .map((grant) => {
                      const exp = describeExpiry(grant.expires_at);
                      const tone = exp?.tone ?? "neutral";
                      const toneClasses =
                        tone === "expired" || tone === "critical"
                          ? "border-[var(--error)]/40 bg-[var(--error)]/5"
                          : tone === "warning"
                            ? "border-[var(--warning)]/40 bg-[var(--warning)]/5"
                            : "border-outline-variant bg-surface-container-low/40";
                      const badgeVariant: "destructive" | "outline" | "secondary" =
                        tone === "expired" || tone === "critical"
                          ? "destructive"
                          : tone === "warning"
                            ? "outline"
                            : "secondary";
                      return (
                        <div key={grant.id} className={`rounded-card border p-3 ${toneClasses}`}>
                          <div className="flex items-start justify-between gap-3">
                            <div className="min-w-0">
                              <p className="flex flex-wrap items-center gap-1.5 font-medium text-on-surface">
                                <Pulse variant={watchlistTone(grant)} />
                                <UserName id={grant.user_id} />
                                <span className="text-on-surface-variant">→</span>
                                <ProjectName id={grant.project_id} />
                                <span className="text-on-surface-variant">:</span>
                                <RoleName projectId={grant.project_id} roleKey={grant.role_key} />
                              </p>
                              <p className="mt-1 text-xs text-on-surface-variant">
                                {grant.expires_at ? new Date(grant.expires_at).toLocaleString() : "no fixed expiry"}
                              </p>
                            </div>
                            {exp && (
                              <Badge
                                variant={badgeVariant}
                                className="text-[10px] uppercase tracking-[0.16em]"
                              >
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
              <Eyebrow>Cleanup Suggestions</Eyebrow>
              <div className="mt-2 space-y-2">
                {cleanupHints.length === 0 ? (
                  <p className="text-xs text-on-surface-variant">
                    No cleanup hints right now — everything looks tidy.
                  </p>
                ) : (
                  cleanupHints.map((hint) => (
                    <div
                      key={hint}
                      className="rounded-card border border-outline-variant bg-surface-container-low/40 p-3 text-sm text-on-surface-variant"
                    >
                      {hint}
                    </div>
                  ))
                )}
              </div>
            </div>
          </div>
        </Card>
      </div>

      <Drawer
        open={drawerEntry !== null}
        onClose={() => setDrawerEntry(null)}
        size="lg"
        labelledBy="audit-drawer-title"
      >
        {drawerEntry && (
          <div className="space-y-4">
            <header>
              <Eyebrow tone="primary">{actionCategory(drawerEntry.action).toUpperCase()}</Eyebrow>
              <h2 id="audit-drawer-title" className="mt-2 font-display text-2xl font-semibold text-on-surface">
                {drawerEntry.action}
              </h2>
              <p className="mt-1 text-xs text-on-surface-variant">
                {new Date(drawerEntry.created_at).toLocaleString()}
              </p>
            </header>
            <dl className="grid grid-cols-1 gap-3 text-sm">
              <div>
                <dt className="text-xs uppercase tracking-[0.16em] text-on-surface-variant">Actor</dt>
                <dd className="mt-1 text-on-surface">
                  {isSystemActor(drawerEntry.actor_id) ? "System" : <UserName id={drawerEntry.actor_id} showEmail />}
                </dd>
                <dd className="text-xs font-mono text-on-surface-variant">{drawerEntry.actor_id}</dd>
              </div>
              {drawerEntry.target_id && drawerEntry.target_id !== "-" && (
                <div>
                  <dt className="text-xs uppercase tracking-[0.16em] text-on-surface-variant">Target</dt>
                  <dd className="mt-1 text-on-surface">
                    <UserName id={drawerEntry.target_id} showEmail />
                  </dd>
                  <dd className="text-xs font-mono text-on-surface-variant">{drawerEntry.target_id}</dd>
                </div>
              )}
              {drawerEntry.resource_id && (
                <div>
                  <dt className="text-xs uppercase tracking-[0.16em] text-on-surface-variant">Resource</dt>
                  <dd className="mt-1 break-all font-mono text-xs text-on-surface">{drawerEntry.resource_id}</dd>
                </div>
              )}
            </dl>
            <div>
              <Eyebrow>Raw payload</Eyebrow>
              <div className="mt-2 rounded-card border border-outline-variant bg-surface-container-lowest p-4">
                <JsonView value={drawerEntry} />
              </div>
            </div>
            <div className="flex justify-end">
              <Button variant="outline" size="sm" onClick={() => setDrawerEntry(null)}>
                Close
              </Button>
            </div>
          </div>
        )}
      </Drawer>
    </div>
  );
}

/**
 * Native <select> renders <option> children as plain text — composing
 * <UserName/> inside <option> doesn't work. Instead we read the resolver
 * cache directly so each option label is the real display name once
 * resolution lands. Values stay UUID-typed so the backend filter matches.
 */
function ActorSelect({
  actors,
  value,
  onChange,
}: {
  actors: string[];
  value: string;
  onChange: (next: string) => void;
}) {
  const resolver = useNameResolver();
  // Force a re-render on the next tick after the resolver flushes so option
  // labels switch from UID-fallback to resolved name. We don't poll — one
  // microtask after the resolver enqueues is enough for cache hits, and the
  // resolver bumps the React Query cache (which our parent <UserName/> mounts
  // already subscribe to) for cache misses.
  const [, force] = useState(0);
  useEffect(() => {
    const t = setTimeout(() => force((n) => n + 1), 0);
    return () => clearTimeout(t);
  }, [actors.length]);

  return (
    <Select
      value={value}
      onChange={(event) => onChange(event.target.value)}
      aria-label="Filter by actor"
    >
      <option value="">All actors</option>
      {actors.map((actor) => {
        const { value: u } = resolver.resolveUser(actor);
        const label = u?.display_name
          ? u.display_name
          : actor.length > 12
            ? `${actor.slice(0, 8)}…`
            : actor;
        return (
          <option key={actor} value={actor}>
            {label}
          </option>
        );
      })}
    </Select>
  );
}
