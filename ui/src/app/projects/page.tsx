"use client";

import { useEffect, useMemo, useState } from "react";

import { Badge } from "@/components/ui/Badge";
import { Card, CardHeader, CardTitle } from "@/components/ui/Card";
import { EmptyState } from "@/components/ui/EmptyState";
import { SkeletonCardList } from "@/components/ui/Skeleton";

interface ProjectSummary {
  project: {
    id: string;
    name: string;
    kind: string;
    description: string;
    roles: Array<{ key: string; label: string }>;
  };
  member_count: number;
  bundle_count: number;
  rule_in_count: number;
  rule_out_count: number;
  active_role_keys: string[];
  sample_members: string[];
}

interface BundleRow {
  id: string;
  name: string;
}

interface BundleRoleRow {
  zitadel_project_id: string;
  zitadel_role_key: string;
}

interface MappingRuleRow {
  id: string;
  source_project: string;
  source_role: string;
  target_project: string;
  target_role: string;
}

export default function ProjectsView() {
  const [projects, setProjects] = useState<ProjectSummary[]>([]);
  const [loading, setLoading] = useState(true);
  const [bundles, setBundles] = useState<BundleRow[]>([]);
  const [bundleRolesByBundle, setBundleRolesByBundle] = useState<Record<string, BundleRoleRow[]>>({});
  const [rules, setRules] = useState<MappingRuleRow[]>([]);
  const [expanded, setExpanded] = useState<Record<string, boolean>>({});

  useEffect(() => {
    async function load() {
      setLoading(true);
      try {
        const [projectsRes, bundlesRes, rulesRes] = await Promise.all([
          fetch("/api/proxy/projects"),
          fetch("/api/proxy/bundles"),
          fetch("/api/proxy/rules/mapping"),
        ]);
        const projectsData = await projectsRes.json();
        const bundlesData = await bundlesRes.json();
        const rulesData = await rulesRes.json();
        setProjects(Array.isArray(projectsData) ? projectsData : []);
        const bundleList: BundleRow[] = Array.isArray(bundlesData) ? bundlesData : [];
        setBundles(bundleList);
        setRules(Array.isArray(rulesData) ? rulesData : []);
        // Fan out one /bundles/{id}/roles per bundle so we can index by role.
        const roleEntries = await Promise.all(
          bundleList.map(async (bundle) => {
            try {
              const r = await fetch(`/api/proxy/bundles/${bundle.id}/roles`);
              const roles = await r.json();
              return [bundle.id, Array.isArray(roles) ? roles : []] as const;
            } catch {
              return [bundle.id, [] as BundleRoleRow[]] as const;
            }
          }),
        );
        setBundleRolesByBundle(Object.fromEntries(roleEntries));
      } finally {
        setLoading(false);
      }
    }

    load();
  }, []);

  // Index: project_id:role_key -> { bundles: [...], rulesIn: [...], rulesOut: [...] }
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
    <div className="space-y-6 animate-fade-in-up">
      <header>
        <h1 className="text-3xl font-bold text-foreground">Projects</h1>
        <p className="text-muted mt-2">See which roles exist per project, how policies touch them, and which members are currently active in each space.</p>
      </header>

      <Card>
        <CardHeader>
          <CardTitle>Project-Centric View</CardTitle>
        </CardHeader>

        {loading ? (
          <SkeletonCardList count={4} />
        ) : projects.length === 0 ? (
          <EmptyState
            title="No projects available"
            description="Create a project in your Zitadel instance, or check that ZITADEL_DOMAIN and ZITADEL_MACHINE_KEY_PATH are set so MkAuth can sync the live catalog."
          />
        ) : (
          <div className="grid grid-cols-1 xl:grid-cols-2 gap-4">
            {projects.map((entry) => (
              <div key={entry.project.id} className="rounded-xl border border-border bg-surfaceHover p-5">
                <div className="flex items-start justify-between gap-3">
                  <div>
                    <h2 className="text-lg font-semibold text-foreground">{entry.project.name}</h2>
                    <p className="mt-1 text-sm text-muted">{entry.project.description}</p>
                  </div>
                  <Badge variant="secondary">{entry.project.kind}</Badge>
                </div>

                <div className="mt-4 grid grid-cols-2 gap-3 text-sm">
                  <div className="rounded-lg border border-border bg-surface p-3">
                    <p className="text-xs uppercase tracking-[0.22em] text-muted">Members</p>
                    <p className="mt-2 text-2xl font-semibold">{entry.member_count}</p>
                  </div>
                  <div className="rounded-lg border border-border bg-surface p-3">
                    <p className="text-xs uppercase tracking-[0.22em] text-muted">Bundles</p>
                    <p className="mt-2 text-2xl font-semibold">{entry.bundle_count}</p>
                  </div>
                  <div className="rounded-lg border border-border bg-surface p-3">
                    <p className="text-xs uppercase tracking-[0.22em] text-muted">Rules In</p>
                    <p className="mt-2 text-2xl font-semibold">{entry.rule_in_count}</p>
                  </div>
                  <div className="rounded-lg border border-border bg-surface p-3">
                    <p className="text-xs uppercase tracking-[0.22em] text-muted">Rules Out</p>
                    <p className="mt-2 text-2xl font-semibold">{entry.rule_out_count}</p>
                  </div>
                </div>

                <div className="mt-4">
                  <p className="text-xs uppercase tracking-[0.22em] text-muted">Role Catalog · click to expand</p>
                  <div className="mt-2 space-y-2">
                    {entry.project.roles.map((role) => {
                      const key = `${entry.project.id}:${role.key}`;
                      const idx = roleIndex.get(key);
                      const isOpen = Boolean(expanded[key]);
                      const isActive = entry.active_role_keys.includes(role.key);
                      return (
                        <div key={role.key} className="rounded-lg border border-border bg-surface">
                          <button
                            type="button"
                            onClick={() => toggleRole(entry.project.id, role.key)}
                            aria-expanded={isOpen}
                            className="flex w-full items-center justify-between gap-3 px-3 py-2 text-left text-sm transition-colors hover:bg-surfaceHover focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
                          >
                            <div className="flex items-center gap-2">
                              <Badge variant={isActive ? "default" : "outline"}>{role.label}</Badge>
                              <code className="text-[10px] text-muted">{role.key}</code>
                            </div>
                            <span className="flex items-center gap-2 text-[11px] text-muted">
                              {idx?.bundles.length ? <span>{idx.bundles.length} bundle{idx.bundles.length === 1 ? "" : "s"}</span> : null}
                              {idx?.rulesIn.length ? <span>{idx.rulesIn.length} rule{idx.rulesIn.length === 1 ? "" : "s"} in</span> : null}
                              {idx?.rulesOut.length ? <span>{idx.rulesOut.length} rule{idx.rulesOut.length === 1 ? "" : "s"} out</span> : null}
                              <span aria-hidden="true">{isOpen ? "▾" : "▸"}</span>
                            </span>
                          </button>
                          {isOpen && (
                            <div className="border-t border-border bg-surfaceHover/40 px-3 py-2 text-xs">
                              {idx?.bundles.length ? (
                                <div className="mb-2">
                                  <p className="text-[10px] uppercase tracking-[0.18em] text-muted">Used by bundles</p>
                                  <div className="mt-1 flex flex-wrap gap-1">
                                    {idx.bundles.map((b) => (
                                      <Badge key={b.id} variant="secondary">{b.name}</Badge>
                                    ))}
                                  </div>
                                </div>
                              ) : null}
                              {idx?.rulesOut.length ? (
                                <div className="mb-2">
                                  <p className="text-[10px] uppercase tracking-[0.18em] text-muted">Triggers (this role propagates to)</p>
                                  <div className="mt-1 space-y-1">
                                    {idx.rulesOut.map((r) => (
                                      <p key={r.id} className="font-mono text-[11px]">→ {r.target_project}:{r.target_role}</p>
                                    ))}
                                  </div>
                                </div>
                              ) : null}
                              {idx?.rulesIn.length ? (
                                <div>
                                  <p className="text-[10px] uppercase tracking-[0.18em] text-muted">Inherited from</p>
                                  <div className="mt-1 space-y-1">
                                    {idx.rulesIn.map((r) => (
                                      <p key={r.id} className="font-mono text-[11px]">← {r.source_project}:{r.source_role}</p>
                                    ))}
                                  </div>
                                </div>
                              ) : null}
                              {!idx?.bundles.length && !idx?.rulesIn.length && !idx?.rulesOut.length && (
                                <p className="text-muted">No bundles or rules reference this role yet.</p>
                              )}
                            </div>
                          )}
                        </div>
                      );
                    })}
                  </div>
                </div>

                <div className="mt-4">
                  <p className="text-xs uppercase tracking-[0.22em] text-muted">Currently Active</p>
                  <div className="mt-2 flex flex-wrap gap-2">
                    {entry.active_role_keys.map((role) => (
                      <Badge key={role} variant="outline" className="border-primary/25 text-primary">
                        {role}
                      </Badge>
                    ))}
                  </div>
                </div>

                <div className="mt-4">
                  <p className="text-xs uppercase tracking-[0.22em] text-muted">Sample Members</p>
                  <p className="mt-2 text-sm text-foreground">
                    {entry.sample_members.length > 0 ? entry.sample_members.join(", ") : "No active members yet."}
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
