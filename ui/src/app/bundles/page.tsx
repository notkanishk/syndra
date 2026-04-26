"use client";

import { useEffect, useState } from "react";

import { Badge } from "@/components/ui/Badge";
import { Card, CardTitle } from "@/components/ui/Card";
import { EmptyState } from "@/components/ui/EmptyState";
import { SkeletonCardList } from "@/components/ui/Skeleton";
import { SubmitButton } from "@/components/ui/SubmitButton";
import { toastError, toastSuccess } from "@/lib/toast";

interface BundleRole {
  zitadel_project_id: string;
  zitadel_role_key: string;
}

interface BundleImpact {
  role_count: number;
  users: Array<{ id: string; name: string }>;
}

interface Bundle {
  id: string;
  name: string;
  description: string;
  created_at: string;
}

interface ProjectCatalog {
  id: string;
  name: string;
  roles: Array<{ key: string; label: string }>;
}

interface CatalogResponse {
  projects: ProjectCatalog[];
}

export default function BundlesView() {
  const [bundles, setBundles] = useState<Bundle[]>([]);
  const [catalog, setCatalog] = useState<ProjectCatalog[]>([]);
  const [loading, setLoading] = useState(true);
  const [formOpen, setFormOpen] = useState(false);
  const [creating, setCreating] = useState(false);
  const [form, setForm] = useState({ name: "", description: "" });
  const [expandedBundle, setExpandedBundle] = useState<string | null>(null);
  const [bundleRoles, setBundleRoles] = useState<Record<string, BundleRole[]>>({});
  const [bundleImpact, setBundleImpact] = useState<Record<string, BundleImpact>>({});
  // Defaults are populated from the live catalog once loadCatalog resolves.
  // Starting empty avoids serializing demo identifiers ("printing", "member")
  // into production HTML, and avoids a 400 if the form is submitted before
  // the live catalog returns.
  const [newRole, setNewRole] = useState({ project_id: "", role_key: "" });

  async function loadBundles() {
    setLoading(true);
    try {
      const res = await fetch("/api/proxy/bundles");
      const data = await res.json();
      setBundles(Array.isArray(data) ? data : []);
    } finally {
      setLoading(false);
    }
  }

  async function loadCatalog() {
    const res = await fetch("/api/proxy/catalog");
    const data: CatalogResponse = await res.json();
    const projects = Array.isArray(data?.projects) ? data.projects : [];
    setCatalog(projects);
    // Populate form defaults from the first available project once the
    // catalog resolves, but only if the user hasn't already picked something.
    if (projects.length > 0) {
      setNewRole((current) => {
        if (current.project_id) return current;
        return {
          project_id: projects[0].id,
          role_key: projects[0].roles[0]?.key ?? "",
        };
      });
    }
  }

  async function loadBundleDetails(bundleId: string) {
    const [rolesRes, impactRes] = await Promise.all([
      fetch(`/api/proxy/bundles/${bundleId}/roles`),
      fetch(`/api/proxy/bundles/${bundleId}/impact`),
    ]);

    const roles = await rolesRes.json();
    const impact = await impactRes.json();
    setBundleRoles((current) => ({ ...current, [bundleId]: Array.isArray(roles) ? roles : [] }));
    setBundleImpact((current) => ({ ...current, [bundleId]: impact }));
  }

  useEffect(() => {
    loadBundles();
    loadCatalog();
  }, []);

  async function handleCreate(event: React.FormEvent) {
    event.preventDefault();
    setCreating(true);
    try {
      const res = await fetch("/api/proxy/bundles", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(form),
      });
      if (!res.ok) {
        const body = await res.json().catch(() => ({}));
        throw new Error(body.message || "Failed to create bundle");
      }
      setForm({ name: "", description: "" });
      setFormOpen(false);
      await loadBundles();
      toastSuccess("Bundle created", `"${form.name}" is ready to be assigned.`);
    } catch (err) {
      toastError(err instanceof Error ? err.message : "Failed to create bundle");
    } finally {
      setCreating(false);
    }
  }

  async function handleAddRole(bundleId: string) {
    try {
      const res = await fetch(`/api/proxy/bundles/${bundleId}/roles`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(newRole),
      });
      if (!res.ok) {
        const body = await res.json().catch(() => ({}));
        throw new Error(body.message || "Failed to add role to bundle");
      }
      await loadBundleDetails(bundleId);
      toastSuccess("Role added to bundle");
    } catch (err) {
      toastError(err instanceof Error ? err.message : "Failed to add role");
    }
  }

  const selectedProject = catalog.find((project) => project.id === newRole.project_id);

  return (
    <div className="space-y-6 animate-fade-in-up">
      <header>
        <h1 className="text-3xl font-bold text-foreground">Bundles</h1>
        <p className="text-muted mt-2">Group role sets into reusable assignments and inspect the user pool currently influenced by each bundle.</p>
      </header>

      <Card>
        <div className="flex items-center justify-between mb-6">
          <CardTitle>Bundle / Policy View</CardTitle>
          {!formOpen && (
            <button
              onClick={() => setFormOpen(true)}
              className="bg-primary hover:bg-primaryHover text-white px-4 py-2 rounded-md font-medium text-sm transition-all shadow-sm hover:shadow-md"
            >
              + New Bundle
            </button>
          )}
        </div>

        {formOpen && (
          <form onSubmit={handleCreate} className="mb-6 border border-border rounded-lg p-6 bg-surfaceHover animate-fade-in-up">
            <h3 className="text-sm font-semibold text-foreground mb-4">Create Bundle</h3>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mb-4">
              <div>
                <label className="block text-xs font-medium text-muted mb-1.5">Bundle Name</label>
                <input
                  type="text"
                  required
                  placeholder="e.g. Student Access"
                  value={form.name}
                  onChange={(event) => setForm({ ...form, name: event.target.value })}
                  className="w-full px-3 py-2 rounded-md border border-border bg-surface text-foreground text-sm"
                />
              </div>
              <div>
                <label className="block text-xs font-medium text-muted mb-1.5">Description</label>
                <input
                  type="text"
                  placeholder="Role bundle purpose"
                  value={form.description}
                  onChange={(event) => setForm({ ...form, description: event.target.value })}
                  className="w-full px-3 py-2 rounded-md border border-border bg-surface text-foreground text-sm"
                />
              </div>
            </div>
            <div className="flex items-center gap-3">
              <SubmitButton isPending={creating} pendingLabel="Creating…" label="Create Bundle" />
              <button type="button" onClick={() => setFormOpen(false)} className="px-4 py-2 rounded-md font-medium text-sm text-muted hover:text-foreground">
                Cancel
              </button>
            </div>
          </form>
        )}

        {loading ? (
          <SkeletonCardList count={3} />
        ) : bundles.length === 0 ? (
          <EmptyState
            title="No bundles yet"
            description="Group related roles into a bundle so you can assign access in one click instead of granting individual roles."
            action={{ label: "Create Bundle", onClick: () => setFormOpen(true) }}
          />
        ) : (
          <div className="space-y-3">
            {bundles.map((bundle) => (
              <div key={bundle.id} className="border border-border rounded-lg bg-surfaceHover overflow-hidden transition-all hover:border-primary/30">
                <div
                  className="p-4 flex items-center justify-between cursor-pointer hover:bg-surface"
                  onClick={() => {
                    if (expandedBundle === bundle.id) {
                      setExpandedBundle(null);
                      return;
                    }
                    setExpandedBundle(bundle.id);
                    loadBundleDetails(bundle.id);
                  }}
                >
                  <div>
                    <h3 className="font-semibold text-foreground">{bundle.name}</h3>
                    <p className="text-xs text-muted mt-0.5">{bundle.description || "No description"}</p>
                  </div>
                  <div className="flex items-center gap-2">
                    <Badge variant="secondary">
                      {bundleImpact[bundle.id]?.users?.length || 0} users
                    </Badge>
                    <Badge variant="outline">{new Date(bundle.created_at).toLocaleDateString()}</Badge>
                  </div>
                </div>

                {expandedBundle === bundle.id && (
                  <div className="px-4 pb-4 pt-2 border-t border-border bg-surface/50 animate-fade-in-up space-y-4">
                    {(() => {
                      const roles = bundleRoles[bundle.id] || [];
                      const distinctProjects = Array.from(
                        new Set(roles.map((r) => r.zitadel_project_id)),
                      );
                      return distinctProjects.length > 0 ? (
                        <div>
                          <h4 className="text-[10px] font-bold text-muted uppercase tracking-widest mb-2">
                            Affected projects ({distinctProjects.length})
                          </h4>
                          <div className="flex flex-wrap gap-2">
                            {distinctProjects.map((projectId) => {
                              const project = catalog.find((p) => p.id === projectId);
                              return (
                                <Badge key={projectId} variant="secondary" title={projectId}>
                                  {project?.name ?? projectId}
                                </Badge>
                              );
                            })}
                          </div>
                        </div>
                      ) : null;
                    })()}

                    <div>
                      <h4 className="text-[10px] font-bold text-muted uppercase tracking-widest mb-2">Contained Roles</h4>
                      <div className="flex flex-wrap gap-2">
                        {(bundleRoles[bundle.id] || []).map((role) => {
                          const project = catalog.find((p) => p.id === role.zitadel_project_id);
                          const projectLabel = project?.name ?? role.zitadel_project_id;
                          const roleLabel =
                            project?.roles.find((r) => r.key === role.zitadel_role_key)?.label
                            ?? role.zitadel_role_key;
                          return (
                            <Badge
                              key={`${role.zitadel_project_id}-${role.zitadel_role_key}`}
                              variant="outline"
                              className="border-primary/20 bg-primary/5 text-primary"
                              title={`${role.zitadel_project_id}:${role.zitadel_role_key}`}
                            >
                              {projectLabel} · {roleLabel}
                            </Badge>
                          );
                        })}
                      </div>
                    </div>

                    <div>
                      <h4 className="text-[10px] font-bold text-muted uppercase tracking-widest mb-2">Impacted Users</h4>
                      <div className="flex flex-wrap gap-2">
                        {(bundleImpact[bundle.id]?.users || []).map((user) => (
                          <Badge key={user.id} variant="secondary">
                            {user.name}
                          </Badge>
                        ))}
                        {(bundleImpact[bundle.id]?.users || []).length === 0 && (
                          <p className="text-xs text-muted italic">No users assigned yet.</p>
                        )}
                      </div>
                    </div>

                    <div className="pt-4 border-t border-border/50">
                      <h4 className="text-[10px] font-bold text-muted uppercase tracking-widest mb-2">Add Role to Bundle</h4>
                      <div className="grid grid-cols-1 md:grid-cols-2 gap-2 mb-2">
                        <select
                          value={newRole.project_id}
                          onChange={(event) => {
                            const projectId = event.target.value;
                            const project = catalog.find((item) => item.id === projectId);
                            setNewRole({
                              project_id: projectId,
                              role_key: project?.roles[0]?.key || "",
                            });
                          }}
                          className="px-2 py-2 rounded bg-surface border border-border text-sm"
                        >
                          {catalog.map((project) => (
                            <option key={project.id} value={project.id}>
                              {project.name}
                            </option>
                          ))}
                        </select>
                        <select
                          value={newRole.role_key}
                          onChange={(event) => setNewRole({ ...newRole, role_key: event.target.value })}
                          className="px-2 py-2 rounded bg-surface border border-border text-sm"
                        >
                          {(selectedProject?.roles || []).map((role) => (
                            <option key={role.key} value={role.key}>
                              {role.label}
                            </option>
                          ))}
                        </select>
                      </div>
                      <button
                        onClick={() => handleAddRole(bundle.id)}
                        disabled={!newRole.project_id || !newRole.role_key}
                        className="w-full py-2 bg-primary/10 hover:bg-primary/20 text-primary text-xs font-bold uppercase tracking-wider rounded transition-colors disabled:cursor-not-allowed disabled:opacity-50"
                      >
                        {newRole.project_id ? "Add Role" : "Loading project catalog…"}
                      </button>
                    </div>
                  </div>
                )}
              </div>
            ))}
          </div>
        )}
      </Card>
    </div>
  );
}
