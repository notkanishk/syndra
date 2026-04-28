"use client";

import { useMemo, useState } from "react";

import { Badge } from "@/components/ui/Badge";
import { Card, CardHeader, CardTitle } from "@/components/ui/Card";
import { EmptyState } from "@/components/ui/EmptyState";
import { Eyebrow } from "@/components/ui/Eyebrow";
import { SkeletonCardList } from "@/components/ui/Skeleton";
import { useBundleRolesByBundle, useBundles, type BundleRow } from "@/lib/queries/useBundles";
import { useMappingRules, type MappingRuleRow } from "@/lib/queries/useMappingRules";
import { useProjects } from "@/lib/queries/useProjects";

export default function ProjectsView() {
  const projectsQuery = useProjects();
  const bundlesQuery = useBundles();
  const rulesQuery = useMappingRules();
  const projects = projectsQuery.data ?? [];
  const bundles = useMemo(() => bundlesQuery.data ?? [], [bundlesQuery.data]);
  const rules = useMemo(() => rulesQuery.data ?? [], [rulesQuery.data]);
  const bundleIds = useMemo(() => bundles.map((b) => b.id), [bundles]);
  const { byId: bundleRolesByBundle } = useBundleRolesByBundle(bundleIds);
  const loading = projectsQuery.isLoading || bundlesQuery.isLoading;

  const [expanded, setExpanded] = useState<Record<string, boolean>>({});

  // Index: project_id:role_key → { bundles, rulesIn, rulesOut }
  const roleIndex = useMemo(() => {
    const idx = new Map<
      string,
      { bundles: BundleRow[]; rulesIn: MappingRuleRow[]; rulesOut: MappingRuleRow[] }
    >();
    const get = (key: string) => {
      let entry = idx.get(key);
      if (!entry) {
        entry = { bundles: [], rulesIn: [], rulesOut: [] };
        idx.set(key, entry);
      }
      return entry;
    };
    for (const bundle of bundles) {
      for (const role of bundleRolesByBundle[bundle.id] ?? []) {
        get(`${role.zitadel_project_id}:${role.zitadel_role_key}`).bundles.push(bundle);
      }
    }
    for (const rule of rules) {
      get(`${rule.source_project}:${rule.source_role}`).rulesOut.push(rule);
      get(`${rule.target_project}:${rule.target_role}`).rulesIn.push(rule);
    }
    return idx;
  }, [bundles, bundleRolesByBundle, rules]);

  function toggleRole(projectId: string, roleKey: string) {
    const key = `${projectId}:${roleKey}`;
    setExpanded((prev) => ({ ...prev, [key]: !prev[key] }));
  }

  return (
    <div className="space-y-6 animate-fade-in-up relative z-10">
      <header>
        <Eyebrow>Projects</Eyebrow>
        <h1 className="text-3xl font-semibold text-on-surface mt-1 font-display">Project topology</h1>
        <p className="text-on-surface-variant mt-2">
          See which roles exist per project, how policies touch them, and which members are
          currently active in each space.
        </p>
      </header>

      <Card variant="glass">
        <CardHeader>
          <CardTitle>Project-Centric View</CardTitle>
        </CardHeader>

        {loading ? (
          <SkeletonCardList count={4} />
        ) : projects.length === 0 ? (
          <EmptyState
            eyebrow="Empty catalog"
            title="No projects available"
            description="Create a project in your Zitadel instance, or check that ZITADEL_DOMAIN and ZITADEL_MACHINE_KEY_PATH are set so MkAuth can sync the live catalog."
          />
        ) : (
          <div className="grid grid-cols-1 lg:grid-cols-2 2xl:grid-cols-3 gap-4">
            {projects.map((entry) => (
              <div
                key={entry.project.id}
                className="rounded-card bg-surface-container-high p-5 flex flex-col"
              >
                <div className="flex items-start justify-between gap-3">
                  <div>
                    <Eyebrow tone="primary">{entry.project.kind}</Eyebrow>
                    <h2 className="text-xl font-semibold text-on-surface mt-1 font-display">
                      {entry.project.name}
                    </h2>
                    <p className="mt-1 text-sm text-on-surface-variant">
                      {entry.project.description || "No description provided."}
                    </p>
                  </div>
                  <Badge variant="secondary">{entry.project.kind}</Badge>
                </div>

                <div className="mt-4 grid grid-cols-2 sm:grid-cols-4 gap-3 text-sm">
                  <div className="rounded-card bg-surface-container-low p-3">
                    <Eyebrow>Members</Eyebrow>
                    <p className="mt-2 text-2xl font-semibold">{entry.member_count}</p>
                  </div>
                  <div className="rounded-card bg-surface-container-low p-3">
                    <Eyebrow>Bundles</Eyebrow>
                    <p className="mt-2 text-2xl font-semibold">{entry.bundle_count}</p>
                  </div>
                  <div className="rounded-card bg-surface-container-low p-3">
                    <Eyebrow>Rules In</Eyebrow>
                    <p className="mt-2 text-2xl font-semibold">{entry.rule_in_count}</p>
                  </div>
                  <div className="rounded-card bg-surface-container-low p-3">
                    <Eyebrow>Rules Out</Eyebrow>
                    <p className="mt-2 text-2xl font-semibold">{entry.rule_out_count}</p>
                  </div>
                </div>

                <div className="mt-4">
                  <Eyebrow>Role Catalog · click to expand</Eyebrow>
                  <div className="mt-2 space-y-2">
                    {entry.project.roles.map((role) => {
                      const key = `${entry.project.id}:${role.key}`;
                      const idx = roleIndex.get(key);
                      const isOpen = Boolean(expanded[key]);
                      const isActive = entry.active_role_keys.includes(role.key);
                      return (
                        <div
                          key={role.key}
                          className="rounded-card bg-surface-container-low border border-outline-variant"
                        >
                          <button
                            type="button"
                            onClick={() => toggleRole(entry.project.id, role.key)}
                            aria-expanded={isOpen}
                            className="flex w-full items-center justify-between gap-3 px-3 py-2 text-left text-sm transition-colors hover:bg-surface-container-high focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary-container"
                          >
                            <div className="flex items-center gap-2">
                              <Badge variant={isActive ? "default" : "outline"}>
                                {role.label}
                              </Badge>
                              <code className="text-[10px] text-on-surface-variant">
                                {role.key}
                              </code>
                            </div>
                            <span className="flex items-center gap-2 text-[11px] text-on-surface-variant">
                              {idx?.bundles.length ? (
                                <span>
                                  {idx.bundles.length} bundle
                                  {idx.bundles.length === 1 ? "" : "s"}
                                </span>
                              ) : null}
                              {idx?.rulesIn.length ? (
                                <span>
                                  {idx.rulesIn.length} rule
                                  {idx.rulesIn.length === 1 ? "" : "s"} in
                                </span>
                              ) : null}
                              {idx?.rulesOut.length ? (
                                <span>
                                  {idx.rulesOut.length} rule
                                  {idx.rulesOut.length === 1 ? "" : "s"} out
                                </span>
                              ) : null}
                              <span aria-hidden="true">{isOpen ? "▾" : "▸"}</span>
                            </span>
                          </button>
                          {isOpen && (
                            <div className="border-t border-outline-variant bg-surface-container px-3 py-2 text-xs">
                              {idx?.bundles.length ? (
                                <div className="mb-2">
                                  <Eyebrow>Used by bundles</Eyebrow>
                                  <div className="mt-1 flex flex-wrap gap-1">
                                    {idx.bundles.map((b) => (
                                      <Badge key={b.id} variant="secondary">
                                        {b.name}
                                      </Badge>
                                    ))}
                                  </div>
                                </div>
                              ) : null}
                              {idx?.rulesOut.length ? (
                                <div className="mb-2">
                                  <Eyebrow>Triggers (this role propagates to)</Eyebrow>
                                  <div className="mt-1 space-y-1">
                                    {idx.rulesOut.map((r) => (
                                      <p key={r.id} className="font-mono text-[11px]">
                                        → {r.target_project}:{r.target_role}
                                      </p>
                                    ))}
                                  </div>
                                </div>
                              ) : null}
                              {idx?.rulesIn.length ? (
                                <div>
                                  <Eyebrow>Inherited from</Eyebrow>
                                  <div className="mt-1 space-y-1">
                                    {idx.rulesIn.map((r) => (
                                      <p key={r.id} className="font-mono text-[11px]">
                                        ← {r.source_project}:{r.source_role}
                                      </p>
                                    ))}
                                  </div>
                                </div>
                              ) : null}
                              {!idx?.bundles.length &&
                                !idx?.rulesIn.length &&
                                !idx?.rulesOut.length && (
                                  <p className="text-on-surface-variant">
                                    No bundles or rules reference this role yet.
                                  </p>
                                )}
                            </div>
                          )}
                        </div>
                      );
                    })}
                  </div>
                </div>

                <div className="mt-4">
                  <Eyebrow>Currently active</Eyebrow>
                  <div className="mt-2 flex flex-wrap gap-2">
                    {entry.active_role_keys.map((role) => (
                      <Badge
                        key={role}
                        variant="outline"
                        className="border-primary-container/40 text-primary-container"
                      >
                        {role}
                      </Badge>
                    ))}
                  </div>
                </div>

                <div className="mt-4">
                  <Eyebrow>Sample members</Eyebrow>
                  <p className="mt-2 text-sm text-on-surface">
                    {entry.sample_members.length > 0
                      ? entry.sample_members.join(", ")
                      : "No active members yet."}
                  </p>
                </div>
              </div>
            ))}
          </div>
        )}
      </Card>
    </div>
  );
}
