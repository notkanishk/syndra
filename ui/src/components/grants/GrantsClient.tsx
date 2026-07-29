"use client";

import { useMemo, useState } from "react";

import { ProjectName, RoleName, UserName } from "@/components/names";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { Drawer } from "@/components/ui/Drawer";
import { Eyebrow } from "@/components/ui/Eyebrow";
import { JsonView } from "@/components/ui/JsonView";
import { Pulse } from "@/components/ui/Pulse";
import {
  useReconciliationDiff,
  useZitadelAllGrants,
  type ReconciliationDriftEntry,
  type ReconciliationGrant,
  type ZitadelGrantRow,
} from "@/lib/queries/useGrants";
import { useMappingRules } from "@/lib/queries/useMappingRules";
import { useNameResolver } from "@/lib/queries/useNameResolver";

type TabKey = "all" | "reconciliation";

type GrantSource = "mkauth" | "zitadel-only" | "derived" | "mkauth-only";

interface AllGrantsRow {
  user_id: string;
  project_id: string;
  role_key: string;
  source: GrantSource;
  /** Optional Zitadel grant id for drill-in. */
  grant_id?: string;
}

const SOURCE_LABEL: Record<GrantSource, string> = {
  mkauth: "MkAuth + Zitadel",
  "zitadel-only": "Zitadel only",
  derived: "Derived from rule",
  "mkauth-only": "MkAuth only (sync gap)",
};

const SOURCE_BADGE_CLASS: Record<GrantSource, string> = {
  mkauth: "border-primary-container/40 text-primary-container bg-primary-container/10",
  "zitadel-only": "border-[var(--warning)]/40 text-[var(--warning)] bg-[var(--warning)]/10",
  derived: "border-secondary-container/40 text-secondary-container bg-secondary-container/10",
  "mkauth-only": "border-[var(--error)]/40 text-[var(--error)] bg-[var(--error)]/10",
};

type ReconFilterKey = "all" | "drift" | "only_in_mkauth" | "only_in_zitadel";

/**
 * Cross-source grants console. Tab 1 surfaces every (user, project, role)
 * known to either side with a source pill so operators can see the universe
 * at a glance; Tab 2 is the reconciliation diff with drill-in Drawer for
 * the full record. Strictly read-only — no remediation actions per
 * obsidian-clarity-redesign.
 */
export default function GrantsClient() {
  const [activeTab, setActiveTab] = useState<TabKey>("all");

  const zitadelQuery = useZitadelAllGrants();
  const reconQuery = useReconciliationDiff();
  // Mapping rules let us tag derived (mapping-rule target) grants with a
  // distinct source pill so operators don't mistake them for unmanaged
  // Zitadel-only grants.
  const rulesQuery = useMappingRules();

  return (
    <div className="space-y-6 animate-fade-in-up relative z-10">
      <header>
        <Eyebrow tone="primary">Grants</Eyebrow>
        <h1 className="text-3xl font-semibold text-on-surface mt-1 font-display">
          Cross-source grants ledger
        </h1>
        <p className="text-on-surface-variant mt-2 max-w-2xl">
          The union of MkAuth-direct grants, mapping-rule derivatives, and
          Zitadel-side grants — reconciled into a single inventory. Use the
          Reconciliation tab to spot drift before it widens.
        </p>
      </header>

      <Card variant="glass">
        <div role="tablist" aria-label="Grants views" className="flex flex-wrap gap-1 border-b border-outline-variant pb-3 mb-4">
          <TabButton active={activeTab === "all"} onClick={() => setActiveTab("all")}>
            All grants
          </TabButton>
          <TabButton active={activeTab === "reconciliation"} onClick={() => setActiveTab("reconciliation")}>
            Reconciliation
            {reconQuery.data && (
              <span className="ml-1.5 text-[10px]">
                ({(reconQuery.data.only_in_mkauth.length +
                  reconQuery.data.only_in_zitadel.length +
                  reconQuery.data.drift.length)})
              </span>
            )}
          </TabButton>
        </div>

        {activeTab === "all" ? (
          <AllGrantsTab
            zitadelQuery={zitadelQuery}
            reconQuery={reconQuery}
            rulesTargets={
              rulesQuery.data?.map((r) => `${r.target_project}:${r.target_role}`) ?? []
            }
          />
        ) : (
          <ReconciliationTab reconQuery={reconQuery} />
        )}
      </Card>
    </div>
  );
}

function TabButton({
  active,
  onClick,
  children,
}: {
  active: boolean;
  onClick: () => void;
  children: React.ReactNode;
}) {
  return (
    <button
      role="tab"
      type="button"
      aria-selected={active}
      onClick={onClick}
      className={`rounded-full px-3 py-1.5 text-xs font-medium transition-colors focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary-container ${
        active
          ? "bg-primary-container/20 text-primary-container"
          : "text-on-surface-variant hover:bg-surface-container"
      }`}
    >
      {children}
    </button>
  );
}

interface AllGrantsTabProps {
  zitadelQuery: ReturnType<typeof useZitadelAllGrants>;
  reconQuery: ReturnType<typeof useReconciliationDiff>;
  rulesTargets: string[];
}

function AllGrantsTab({ zitadelQuery, reconQuery, rulesTargets }: AllGrantsTabProps) {
  const [projectFilter, setProjectFilter] = useState<Set<string>>(new Set());
  const [sourceFilter, setSourceFilter] = useState<Set<GrantSource>>(new Set());
  const [userFilter, setUserFilter] = useState("");

  const resolver = useNameResolver();

  const recon = reconQuery.data;
  const zitadelData = zitadelQuery.data;

  // Compose the union of (user, project, role) rows tagged with source.
  // Zitadel-side grants are the canonical surface (what the system actually
  // serves); MkAuth-only grants are surfaced separately so the operator sees
  // the sync gap. Drift entries fan out into per-role rows so each row's
  // source tag is unambiguous.
  const rows: AllGrantsRow[] = useMemo(() => {
    const ruleTargetSet = new Set(rulesTargets);
    const onlyInZitadelPairs = new Set<string>();
    const driftZitadelOnly = new Map<string, Set<string>>();

    if (recon) {
      for (const g of recon.only_in_zitadel) {
        onlyInZitadelPairs.add(`${g.user_id}|${g.project_id}`);
      }
      for (const d of recon.drift) {
        const k = `${d.user_id}|${d.project_id}`;
        driftZitadelOnly.set(k, new Set(d.only_in_zitadel));
      }
    }

    const out: AllGrantsRow[] = [];
    const zitadelItems = zitadelData?.items ?? [];

    // Zitadel-side rows
    for (const grant of zitadelItems) {
      const pairKey = `${grant.userId}|${grant.projectId}`;
      const driftSet = driftZitadelOnly.get(pairKey);
      const isPairOnlyInZitadel = onlyInZitadelPairs.has(pairKey);
      for (const role of grant.roleKeys) {
        const fullPairRole = `${grant.projectId}:${role}`;
        const isDerived = ruleTargetSet.has(fullPairRole);
        let source: GrantSource;
        if (isPairOnlyInZitadel || driftSet?.has(role)) {
          source = isDerived ? "derived" : "zitadel-only";
        } else {
          source = "mkauth";
        }
        out.push({
          user_id: grant.userId,
          project_id: grant.projectId,
          role_key: role,
          source,
          grant_id: grant.id,
        });
      }
    }

    // MkAuth-only rows (the operator must see these because they signal a
    // sync gap — MkAuth thinks the grant exists but Zitadel does not).
    if (recon) {
      for (const g of recon.only_in_mkauth) {
        for (const role of g.role_keys) {
          out.push({
            user_id: g.user_id,
            project_id: g.project_id,
            role_key: role,
            source: "mkauth-only",
          });
        }
      }
      // Drift's only-in-mkauth roles are also gaps from Zitadel's view.
      for (const d of recon.drift) {
        for (const role of d.only_in_mkauth) {
          out.push({
            user_id: d.user_id,
            project_id: d.project_id,
            role_key: role,
            source: "mkauth-only",
          });
        }
      }
    }

    return out.sort((a, b) => {
      if (a.user_id !== b.user_id) return a.user_id.localeCompare(b.user_id);
      if (a.project_id !== b.project_id) return a.project_id.localeCompare(b.project_id);
      return a.role_key.localeCompare(b.role_key);
    });
  }, [zitadelData, recon, rulesTargets]);

  // Filter rail values — derive distinct projects and sources from the rows.
  const distinctProjects = useMemo(
    () => Array.from(new Set(rows.map((r) => r.project_id))),
    [rows],
  );

  const filteredRows = useMemo(() => {
    const userQ = userFilter.trim().toLowerCase();
    return rows.filter((row) => {
      if (projectFilter.size > 0 && !projectFilter.has(row.project_id)) return false;
      if (sourceFilter.size > 0 && !sourceFilter.has(row.source)) return false;
      if (!userQ) return true;
      // Match against the resolved display name + email first (what the
      // operator actually sees), then fall back to a UUID-prefix match for
      // power users searching by id. Resolver returns `{value, resolved}`
      // tri-state; an unresolved id (still loading) only contributes the UID
      // path until its name lands.
      const lookup = resolver.resolveUser(row.user_id);
      const display = lookup.value?.display_name?.toLowerCase() ?? "";
      const email = lookup.value?.email?.toLowerCase() ?? "";
      if (display.includes(userQ) || email.includes(userQ)) return true;
      return row.user_id.toLowerCase().includes(userQ);
    });
  }, [rows, projectFilter, sourceFilter, userFilter, resolver]);

  function toggleProject(id: string) {
    setProjectFilter((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }

  function toggleSource(source: GrantSource) {
    setSourceFilter((prev) => {
      const next = new Set(prev);
      if (next.has(source)) next.delete(source);
      else next.add(source);
      return next;
    });
  }

  if (zitadelQuery.isLoading || reconQuery.isLoading) {
    return <p className="text-sm text-on-surface-variant">Loading grants ledger…</p>;
  }
  if (zitadelQuery.isError || reconQuery.isError) {
    return (
      <p className="text-sm text-[var(--error)]">
        Failed to load grants ledger.{" "}
        {zitadelQuery.error instanceof Error
          ? zitadelQuery.error.message
          : reconQuery.error instanceof Error
            ? reconQuery.error.message
            : "Unknown error"}
      </p>
    );
  }

  return (
    <div className="grid grid-cols-1 lg:grid-cols-[260px_1fr] gap-4">
      <aside className="space-y-4">
        <div>
          <Eyebrow tone="muted">Filter by user</Eyebrow>
          <input
            type="search"
            value={userFilter}
            onChange={(event) => setUserFilter(event.target.value)}
            placeholder="Name, email, or ID prefix…"
            className="mt-2 w-full rounded-full bg-surface-container-low border border-outline-variant px-3 py-1.5 text-sm focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary-container"
          />
        </div>
        <div>
          <Eyebrow tone="muted">Source</Eyebrow>
          <div className="mt-2 flex flex-wrap gap-1.5">
            {(Object.keys(SOURCE_LABEL) as GrantSource[]).map((src) => (
              <button
                key={src}
                type="button"
                aria-pressed={sourceFilter.has(src)}
                onClick={() => toggleSource(src)}
                className={`rounded-full border px-2.5 py-1 text-[11px] transition-colors ${
                  sourceFilter.has(src)
                    ? SOURCE_BADGE_CLASS[src]
                    : "border-outline-variant text-on-surface-variant hover:border-primary-container/50"
                }`}
              >
                {SOURCE_LABEL[src]}
              </button>
            ))}
          </div>
        </div>
        {distinctProjects.length > 0 && (
          <div>
            <Eyebrow tone="muted">Project</Eyebrow>
            <div className="mt-2 flex flex-wrap gap-1.5">
              {distinctProjects.map((id) => (
                <button
                  key={id}
                  type="button"
                  aria-pressed={projectFilter.has(id)}
                  onClick={() => toggleProject(id)}
                  className={`rounded-full border px-2.5 py-1 text-[11px] transition-colors ${
                    projectFilter.has(id)
                      ? "border-primary-container bg-primary-container/15 text-primary-container"
                      : "border-outline-variant text-on-surface-variant hover:border-primary-container/50"
                  }`}
                  title={id}
                >
                  <ProjectName id={id} fallback={id} />
                </button>
              ))}
            </div>
          </div>
        )}
        {(projectFilter.size > 0 || sourceFilter.size > 0 || userFilter) && (
          <button
            type="button"
            onClick={() => {
              setProjectFilter(new Set());
              setSourceFilter(new Set());
              setUserFilter("");
            }}
            className="text-xs text-primary-container hover:underline"
          >
            Clear filters
          </button>
        )}
      </aside>

      <div>
        <p className="mb-3 text-xs text-on-surface-variant">
          Showing {filteredRows.length} of {rows.length} grants
          {zitadelData?.truncated || recon?.truncated
            ? " · partial snapshot — inventory exceeded the safety cap"
            : ""}
        </p>
        {(zitadelData?.truncated || recon?.truncated) && (
          <p className="mb-3 rounded-card border border-[var(--warning)]/40 bg-[var(--warning)]/10 px-3 py-2 text-xs text-[var(--warning)]">
            The grants ledger is incomplete — the Zitadel inventory exceeded the
            client-side safety cap{zitadelData?.total ? ` (loaded ${zitadelData.items.length} of ${zitadelData.total})` : ""}.
            Drill into specific users for full attribution; rows for users beyond
            the cap may be missing or appear as &ldquo;MkAuth only&rdquo; false-positives.
          </p>
        )}
        {filteredRows.length === 0 ? (
          <div className="rounded-card border border-dashed border-outline-variant bg-surface-container-low px-4 py-8 text-center">
            <p className="text-sm text-on-surface-variant">No grants match the current filters.</p>
          </div>
        ) : (
          <ul className="divide-y divide-outline-variant rounded-card border border-outline-variant bg-surface-container-low">
            {filteredRows.map((row) => (
              <li key={`${row.user_id}-${row.project_id}-${row.role_key}-${row.source}`} className="px-4 py-2.5 flex items-center gap-3">
                <div className="flex-1 min-w-0">
                  <span className="text-sm text-on-surface">
                    <UserName id={row.user_id} fallback={row.user_id} />
                    <span className="text-on-surface-variant"> · </span>
                    <ProjectName id={row.project_id} fallback={row.project_id} />
                    <span className="text-on-surface-variant"> · </span>
                    <RoleName projectId={row.project_id} roleKey={row.role_key} />
                  </span>
                </div>
                <Badge variant="outline" className={`text-[10px] ${SOURCE_BADGE_CLASS[row.source]}`}>
                  {SOURCE_LABEL[row.source]}
                </Badge>
              </li>
            ))}
          </ul>
        )}
      </div>
    </div>
  );
}

interface ReconciliationTabProps {
  reconQuery: ReturnType<typeof useReconciliationDiff>;
}

function ReconciliationTab({ reconQuery }: ReconciliationTabProps) {
  const [scope, setScope] = useState<ReconFilterKey>("all");
  const [drillIn, setDrillIn] = useState<
    | { kind: "drift"; entry: ReconciliationDriftEntry }
    | { kind: "only_in_mkauth"; entry: ReconciliationGrant }
    | { kind: "only_in_zitadel"; entry: ReconciliationGrant }
    | null
  >(null);

  if (reconQuery.isLoading) {
    return <p className="text-sm text-on-surface-variant">Loading reconciliation snapshot…</p>;
  }
  if (reconQuery.isError) {
    return (
      <p className="text-sm text-[var(--error)]">
        Failed to load reconciliation diff.{" "}
        {reconQuery.error instanceof Error ? reconQuery.error.message : ""}
      </p>
    );
  }
  const recon = reconQuery.data;
  if (!recon) return null;

  const driftCount = recon.drift.length;
  const onlyMkCount = recon.only_in_mkauth.length;
  const onlyZitadelCount = recon.only_in_zitadel.length;

  return (
    <div className="space-y-5">
      {/* Drift summary card — clicking a count scopes the table below. */}
      <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
        <DriftCount
          label="Role mismatch"
          value={driftCount}
          tone={driftCount > 0 ? "warn" : "info"}
          active={scope === "drift"}
          onClick={() => setScope(scope === "drift" ? "all" : "drift")}
        />
        <DriftCount
          label="Only in MkAuth"
          value={onlyMkCount}
          tone={onlyMkCount > 0 ? "error" : "info"}
          active={scope === "only_in_mkauth"}
          onClick={() => setScope(scope === "only_in_mkauth" ? "all" : "only_in_mkauth")}
        />
        <DriftCount
          label="Only in Zitadel"
          value={onlyZitadelCount}
          tone={onlyZitadelCount > 0 ? "warn" : "info"}
          active={scope === "only_in_zitadel"}
          onClick={() => setScope(scope === "only_in_zitadel" ? "all" : "only_in_zitadel")}
        />
      </div>

      {recon.truncated && (
        <p className="rounded-card border border-[var(--warning)]/40 bg-[var(--warning)]/10 px-3 py-2 text-xs text-[var(--warning)]">
          Partial reconciliation — the Zitadel inventory exceeded the server-side safety cap so the
          backend stopped paging. Drift buckets may contain false positives for grants on
          un-fetched pages; drill into specific users for authoritative state.
        </p>
      )}

      {(scope === "all" || scope === "drift") && driftCount > 0 && (
        <Section title="Role mismatch" eyebrow="Drift">
          <ul className="divide-y divide-outline-variant rounded-card border border-[var(--warning)]/40 bg-surface-container-low">
            {recon.drift.map((d) => (
              <li key={`${d.user_id}-${d.project_id}`}>
                <button
                  type="button"
                  onClick={() => setDrillIn({ kind: "drift", entry: d })}
                  className="w-full text-left px-4 py-3 flex items-center gap-3 transition-colors hover:bg-surface-container focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary-container"
                >
                  <Pulse variant="warn" ariaLabel="role mismatch" />
                  <div className="flex-1 min-w-0">
                    <p className="text-sm text-on-surface">
                      <UserName id={d.user_id} fallback={d.user_id} />
                      <span className="text-on-surface-variant"> · </span>
                      <ProjectName id={d.project_id} fallback={d.project_id} />
                    </p>
                    <p className="mt-1 text-[11px] text-on-surface-variant">
                      MkAuth: <code className="font-mono">{d.mkauth_roles.join(", ")}</code> · Zitadel:{" "}
                      <code className="font-mono">{d.zitadel_roles.join(", ")}</code>
                    </p>
                  </div>
                  <span className="text-on-surface-variant text-xs">View ▸</span>
                </button>
              </li>
            ))}
          </ul>
        </Section>
      )}

      {(scope === "all" || scope === "only_in_mkauth") && onlyMkCount > 0 && (
        <Section title="Only in MkAuth (sync gap)" eyebrow="Drift">
          <SimpleList
            entries={recon.only_in_mkauth}
            tone="error"
            onView={(entry) => setDrillIn({ kind: "only_in_mkauth", entry })}
          />
        </Section>
      )}

      {(scope === "all" || scope === "only_in_zitadel") && onlyZitadelCount > 0 && (
        <Section title="Only in Zitadel" eyebrow="Drift">
          <SimpleList
            entries={recon.only_in_zitadel}
            tone="warn"
            onView={(entry) => setDrillIn({ kind: "only_in_zitadel", entry })}
          />
        </Section>
      )}

      {driftCount === 0 && onlyMkCount === 0 && onlyZitadelCount === 0 && (
        <div className="rounded-card border border-[var(--success)]/40 bg-[var(--success)]/10 px-4 py-6 text-center">
          <p className="text-sm text-[var(--success)]">No drift detected — MkAuth and Zitadel are in sync.</p>
          {recon.generated_at && (
            <p className="mt-1 text-[11px] text-on-surface-variant">
              Snapshot taken {new Date(recon.generated_at).toLocaleString()}
            </p>
          )}
        </div>
      )}

      {drillIn && (
        <Drawer open onClose={() => setDrillIn(null)} labelledBy="recon-drawer-title" size="lg">
          <Eyebrow>Reconciliation detail</Eyebrow>
          <h3 id="recon-drawer-title" className="text-base font-semibold text-on-surface mt-1">
            {drillIn.kind === "drift" ? "Role mismatch" : drillIn.kind === "only_in_mkauth" ? "Only in MkAuth" : "Only in Zitadel"}
          </h3>
          <p className="mt-1 text-xs text-on-surface-variant">
            <UserName
              id={drillIn.kind === "drift" ? drillIn.entry.user_id : drillIn.entry.user_id}
              fallback={drillIn.kind === "drift" ? drillIn.entry.user_id : drillIn.entry.user_id}
            />
            {" · "}
            <ProjectName
              id={drillIn.kind === "drift" ? drillIn.entry.project_id : drillIn.entry.project_id}
              fallback={drillIn.kind === "drift" ? drillIn.entry.project_id : drillIn.entry.project_id}
            />
          </p>

          {drillIn.kind === "drift" ? (
            <div className="mt-4 grid grid-cols-1 md:grid-cols-2 gap-3">
              <div className="rounded-card border border-outline-variant bg-surface-container-lowest p-3">
                <Eyebrow tone="primary">MkAuth-side</Eyebrow>
                <JsonView
                  value={{
                    user_id: drillIn.entry.user_id,
                    project_id: drillIn.entry.project_id,
                    roles: drillIn.entry.mkauth_roles,
                    only_in_mkauth: drillIn.entry.only_in_mkauth,
                  }}
                />
              </div>
              <div className="rounded-card border border-outline-variant bg-surface-container-lowest p-3">
                <Eyebrow tone="primary">Zitadel-side</Eyebrow>
                <JsonView
                  value={{
                    grant_id: drillIn.entry.grant_id ?? null,
                    user_id: drillIn.entry.user_id,
                    project_id: drillIn.entry.project_id,
                    roles: drillIn.entry.zitadel_roles,
                    only_in_zitadel: drillIn.entry.only_in_zitadel,
                  }}
                />
              </div>
            </div>
          ) : (
            <div className="mt-4 rounded-card border border-outline-variant bg-surface-container-lowest p-3">
              <JsonView value={drillIn.entry} />
            </div>
          )}

          <div className="mt-4 flex justify-end">
            <Button type="button" variant="ghost" size="sm" onClick={() => setDrillIn(null)}>
              Close
            </Button>
          </div>
        </Drawer>
      )}
    </div>
  );
}

function DriftCount({
  label,
  value,
  tone,
  active,
  onClick,
}: {
  label: string;
  value: number;
  tone: "warn" | "error" | "info";
  active: boolean;
  onClick: () => void;
}) {
  const valueClass =
    tone === "error"
      ? "text-[var(--error)]"
      : tone === "warn"
        ? "text-[var(--warning)]"
        : "text-on-surface";
  return (
    <button
      type="button"
      onClick={onClick}
      aria-pressed={active}
      className={`text-left rounded-card border bg-surface-container-low p-4 transition-colors focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary-container ${
        active
          ? "border-primary-container/60 bg-primary-container/10"
          : "border-outline-variant hover:border-primary-container/40"
      }`}
    >
      <Eyebrow tone="muted">{label}</Eyebrow>
      <p className={`mt-2 font-display text-4xl font-semibold ${valueClass}`}>{value}</p>
    </button>
  );
}

function Section({
  title,
  eyebrow,
  children,
}: {
  title: string;
  eyebrow: string;
  children: React.ReactNode;
}) {
  return (
    <section>
      <Eyebrow>{eyebrow}</Eyebrow>
      <h3 className="text-sm font-semibold text-on-surface mt-1 mb-2">{title}</h3>
      {children}
    </section>
  );
}

function SimpleList({
  entries,
  tone,
  onView,
}: {
  entries: ReconciliationGrant[];
  tone: "warn" | "error";
  onView: (entry: ReconciliationGrant) => void;
}) {
  const borderClass =
    tone === "error" ? "border-[var(--error)]/40" : "border-[var(--warning)]/40";
  return (
    <ul className={`divide-y divide-outline-variant rounded-card border ${borderClass} bg-surface-container-low`}>
      {entries.map((g) => (
        <li key={`${g.user_id}-${g.project_id}`}>
          <button
            type="button"
            onClick={() => onView(g)}
            className="w-full text-left px-4 py-3 flex items-center gap-3 transition-colors hover:bg-surface-container focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary-container"
          >
            <Pulse variant={tone} ariaLabel={tone} />
            <div className="flex-1 min-w-0">
              <p className="text-sm text-on-surface">
                <UserName id={g.user_id} fallback={g.user_id} />
                <span className="text-on-surface-variant"> · </span>
                <ProjectName id={g.project_id} fallback={g.project_id} />
              </p>
              <p className="mt-1 text-[11px] text-on-surface-variant font-mono">
                {g.role_keys.join(", ")}
              </p>
            </div>
            <span className="text-on-surface-variant text-xs">View ▸</span>
          </button>
        </li>
      ))}
    </ul>
  );
}

// Reference type-only imports kept on the export edge so consumers can pull
// the inferred row shape if they need it (currently internal-only, but keeps
// the API surface tidy if the page ever needs to reuse the row format).
export type { AllGrantsRow, GrantSource, ZitadelGrantRow };
