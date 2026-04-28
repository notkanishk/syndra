"use client";

import { useEffect, useMemo, useState } from "react";

import { ProjectName, RoleName, UserName } from "@/components/names";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Card, CardTitle } from "@/components/ui/Card";
import { EmptyState } from "@/components/ui/EmptyState";
import { Eyebrow } from "@/components/ui/Eyebrow";
import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import { SkeletonCardList } from "@/components/ui/Skeleton";
import { SubmitButton } from "@/components/ui/SubmitButton";
import { useQuery } from "@tanstack/react-query";
import { request } from "@/lib/api-client";
import {
  useAddBundleRole,
  useBundleImpact,
  useBundleRoles,
  useBundles,
  useCreateBundle,
} from "@/lib/queries/useBundles";
import { toastError, toastSuccess } from "@/lib/toast";

interface ProjectCatalog {
  id: string;
  name: string;
  roles: Array<{ key: string; label: string }>;
}

function useCatalogProjects() {
  return useQuery({
    queryKey: ["catalog", "projects"],
    queryFn: async (): Promise<ProjectCatalog[]> => {
      const data = await request<{ projects?: ProjectCatalog[] }>("/catalog");
      return Array.isArray(data?.projects) ? data.projects : [];
    },
  });
}

const SAMPLE_USER_LIMIT = 5;

export default function BundlesView() {
  const bundlesQuery = useBundles();
  const projectsQuery = useCatalogProjects();
  const bundles = useMemo(() => bundlesQuery.data ?? [], [bundlesQuery.data]);
  const catalog = useMemo(() => projectsQuery.data ?? [], [projectsQuery.data]);
  const loading = bundlesQuery.isLoading;

  const createBundle = useCreateBundle();

  const [formOpen, setFormOpen] = useState(false);
  const [form, setForm] = useState({ name: "", description: "" });
  const [expandedBundleId, setExpandedBundleId] = useState<string | null>(null);
  const [impactOpen, setImpactOpen] = useState(false);
  // Defaults populate from the live catalog after it resolves; starting empty
  // avoids serializing stale identifiers into production HTML and stops the
  // submit button from posting before the catalog is ready.
  const [newRole, setNewRole] = useState({ project_id: "", role_key: "" });
  useEffect(() => {
    if (newRole.project_id || catalog.length === 0) return;
    setNewRole({
      project_id: catalog[0].id,
      role_key: catalog[0].roles[0]?.key ?? "",
    });
  }, [catalog, newRole.project_id]);

  async function handleCreate(event: React.FormEvent) {
    event.preventDefault();
    try {
      await createBundle.mutateAsync(form);
      setForm({ name: "", description: "" });
      setFormOpen(false);
      toastSuccess("Bundle created", `"${form.name}" is ready to be assigned.`);
    } catch (err) {
      toastError(err instanceof Error ? err.message : "Failed to create bundle");
    }
  }

  return (
    <div className="space-y-6 animate-fade-in-up relative z-10">
      <header>
        <Eyebrow>Bundles</Eyebrow>
        <h1 className="text-3xl font-semibold text-on-surface mt-1 font-display">
          Reusable role bundles
        </h1>
        <p className="text-on-surface-variant mt-2">
          Group role sets into reusable assignments and inspect the user pool
          currently influenced by each bundle.
        </p>
      </header>

      <Card variant="glass">
        <div className="flex items-center justify-between mb-6">
          <CardTitle>Bundle library</CardTitle>
          {!formOpen && (
            <Button variant="primary" size="sm" onClick={() => setFormOpen(true)}>
              + New bundle
            </Button>
          )}
        </div>

        {formOpen && (
          <form
            onSubmit={handleCreate}
            className="mb-6 rounded-card border border-outline-variant bg-surface-container-low p-6 animate-fade-in-up"
          >
            <Eyebrow>Create bundle</Eyebrow>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mt-3 mb-4">
              <div>
                <label className="block text-xs font-medium text-on-surface-variant mb-1.5">
                  Bundle name
                </label>
                <Input
                  required
                  placeholder="e.g. Student Access"
                  value={form.name}
                  onChange={(event) => setForm({ ...form, name: event.target.value })}
                />
              </div>
              <div>
                <label className="block text-xs font-medium text-on-surface-variant mb-1.5">
                  Description
                </label>
                <Input
                  placeholder="Role bundle purpose"
                  value={form.description}
                  onChange={(event) => setForm({ ...form, description: event.target.value })}
                />
              </div>
            </div>
            <div className="flex items-center gap-3">
              <SubmitButton
                isPending={createBundle.isPending}
                pendingLabel="Creating…"
                label="Create bundle"
              />
              <Button
                type="button"
                variant="ghost"
                size="sm"
                onClick={() => setFormOpen(false)}
              >
                Cancel
              </Button>
            </div>
          </form>
        )}

        {loading ? (
          <SkeletonCardList count={3} />
        ) : bundles.length === 0 ? (
          <EmptyState
            eyebrow="Empty library"
            title="No bundles yet"
            description="Group related roles into a bundle so you can assign access in one click instead of granting individual roles."
            action={{ label: "Create bundle", onClick: () => setFormOpen(true) }}
          />
        ) : (
          <div className="space-y-3">
            {bundles.map((bundle) => {
              const isExpanded = expandedBundleId === bundle.id;
              return (
                <BundleRowCard
                  key={bundle.id}
                  bundle={bundle}
                  expanded={isExpanded}
                  impactOpen={isExpanded && impactOpen}
                  catalog={catalog}
                  newRole={newRole}
                  setNewRole={setNewRole}
                  onToggle={() => {
                    setExpandedBundleId(isExpanded ? null : bundle.id);
                    if (!isExpanded) setImpactOpen(false);
                  }}
                  onToggleImpact={() => setImpactOpen((prev) => !prev)}
                />
              );
            })}
          </div>
        )}
      </Card>
    </div>
  );
}

interface BundleRowCardProps {
  bundle: { id: string; name: string; description?: string; created_at?: string };
  expanded: boolean;
  impactOpen: boolean;
  catalog: ProjectCatalog[];
  newRole: { project_id: string; role_key: string };
  setNewRole: (next: { project_id: string; role_key: string }) => void;
  onToggle: () => void;
  onToggleImpact: () => void;
}

function BundleRowCard({
  bundle,
  expanded,
  impactOpen,
  catalog,
  newRole,
  setNewRole,
  onToggle,
  onToggleImpact,
}: BundleRowCardProps) {
  // Roles are fetched eagerly on expand so the role chips render immediately;
  // impact is deferred behind its own accordion to avoid the N×users payload
  // until the operator asks for it.
  const rolesQuery = useBundleRoles(expanded ? bundle.id : null);
  const impactQuery = useBundleImpact(expanded && impactOpen ? bundle.id : null);
  const addRole = useAddBundleRole(bundle.id);

  const roles = rolesQuery.data ?? [];
  const distinctProjects = Array.from(new Set(roles.map((r) => r.zitadel_project_id)));
  const impactUsers = impactQuery.data?.users ?? [];
  const visibleUsers = impactUsers.slice(0, SAMPLE_USER_LIMIT);
  const remainingUsers = Math.max(0, impactUsers.length - visibleUsers.length);

  const selectedProject = catalog.find((p) => p.id === newRole.project_id);

  async function handleAddRole() {
    try {
      await addRole.mutateAsync(newRole);
      toastSuccess("Role added to bundle");
    } catch (err) {
      toastError(err instanceof Error ? err.message : "Failed to add role");
    }
  }

  return (
    <div
      className={`rounded-card border bg-surface-container-low transition-all ${
        expanded ? "border-primary-container/60" : "border-outline-variant hover:border-primary-container/40"
      }`}
    >
      <button
        type="button"
        onClick={onToggle}
        aria-expanded={expanded}
        className="flex w-full items-center justify-between gap-3 px-4 py-3 text-left transition-colors hover:bg-surface-container focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary-container rounded-card"
      >
        <div>
          <h3 className="font-semibold text-on-surface">{bundle.name}</h3>
          <p className="mt-0.5 text-xs text-on-surface-variant">
            {bundle.description || "No description provided."}
          </p>
        </div>
        <div className="flex items-center gap-2">
          {bundle.created_at ? (
            <Badge variant="outline">
              {new Date(bundle.created_at).toLocaleDateString()}
            </Badge>
          ) : null}
          <span aria-hidden="true" className="text-on-surface-variant">
            {expanded ? "▾" : "▸"}
          </span>
        </div>
      </button>

      {expanded && (
        <div className="px-4 pb-4 pt-2 border-t border-outline-variant bg-surface-container/40 animate-fade-in-up space-y-4">
          {distinctProjects.length > 0 && (
            <div>
              <Eyebrow>Affected projects ({distinctProjects.length})</Eyebrow>
              <div className="mt-2 flex flex-wrap gap-2">
                {distinctProjects.map((projectId) => (
                  <Badge key={projectId} variant="secondary" title={projectId}>
                    <ProjectName id={projectId} />
                  </Badge>
                ))}
              </div>
            </div>
          )}

          <div>
            <Eyebrow>Contained roles</Eyebrow>
            <div className="mt-2 flex flex-wrap gap-2">
              {roles.length === 0 && rolesQuery.isLoading ? (
                <p className="text-xs text-on-surface-variant italic">Loading roles…</p>
              ) : roles.length === 0 ? (
                <p className="text-xs text-on-surface-variant italic">No roles in this bundle yet.</p>
              ) : (
                roles.map((role) => (
                  <Badge
                    key={`${role.zitadel_project_id}-${role.zitadel_role_key}`}
                    variant="outline"
                    className="border-primary-container/40 bg-primary-container/10 text-primary-container"
                    title={`${role.zitadel_project_id}:${role.zitadel_role_key}`}
                  >
                    <ProjectName id={role.zitadel_project_id} /> ·{" "}
                    <RoleName
                      projectId={role.zitadel_project_id}
                      roleKey={role.zitadel_role_key}
                    />
                  </Badge>
                ))
              )}
            </div>
          </div>

          {/*
           * Impact accordion. Stays collapsed until the admin opens it so we
           * never trigger the role-fanout/user-scan unless they explicitly
           * ask for it. Stage 4 will surface a richer impact view (delta
           * visualisation); Stage 3 ships the data path and the CTA.
           */}
          <div className="rounded-card border border-outline-variant">
            <button
              type="button"
              onClick={onToggleImpact}
              aria-expanded={impactOpen}
              className="flex w-full items-center justify-between gap-3 px-3 py-2 text-left transition-colors hover:bg-surface-container focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary-container rounded-card"
            >
              <div className="flex items-center gap-2">
                <Eyebrow tone="primary">Impact preview</Eyebrow>
                {impactQuery.data && (
                  <span className="text-xs text-on-surface-variant">
                    {impactQuery.data.users.length} user
                    {impactQuery.data.users.length === 1 ? "" : "s"} · {impactQuery.data.role_count}{" "}
                    role{impactQuery.data.role_count === 1 ? "" : "s"}
                  </span>
                )}
              </div>
              <span aria-hidden="true" className="text-on-surface-variant">
                {impactOpen ? "▾" : "▸"}
              </span>
            </button>

            {impactOpen && (
              <div className="border-t border-outline-variant px-3 py-3 animate-fade-in-up">
                {impactQuery.isLoading ? (
                  <p className="text-xs text-on-surface-variant italic">Loading impact…</p>
                ) : impactUsers.length === 0 ? (
                  <p className="text-xs text-on-surface-variant italic">
                    No users would be affected by this bundle yet.
                  </p>
                ) : (
                  <div className="flex flex-wrap items-center gap-2">
                    {visibleUsers.map((user) => (
                      <Badge key={user.id} variant="secondary">
                        <UserName id={user.id} fallback={user.name} />
                      </Badge>
                    ))}
                    {remainingUsers > 0 && (
                      <Badge variant="outline">+{remainingUsers} more</Badge>
                    )}
                  </div>
                )}
              </div>
            )}
          </div>

          <div className="pt-3 border-t border-outline-variant/60">
            <Eyebrow>Add role to bundle</Eyebrow>
            <div className="mt-2 grid grid-cols-1 md:grid-cols-2 gap-2 mb-2">
              <Select
                value={newRole.project_id}
                onChange={(event) => {
                  const projectId = event.target.value;
                  const project = catalog.find((item) => item.id === projectId);
                  setNewRole({
                    project_id: projectId,
                    role_key: project?.roles[0]?.key || "",
                  });
                }}
              >
                {catalog.map((project) => (
                  <option key={project.id} value={project.id}>
                    {project.name}
                  </option>
                ))}
              </Select>
              <Select
                value={newRole.role_key}
                onChange={(event) =>
                  setNewRole({ ...newRole, role_key: event.target.value })
                }
              >
                {(selectedProject?.roles || []).map((role) => (
                  <option key={role.key} value={role.key}>
                    {role.label}
                  </option>
                ))}
              </Select>
            </div>
            <Button
              type="button"
              variant="secondary"
              onClick={handleAddRole}
              isPending={addRole.isPending}
              disabled={!newRole.project_id || !newRole.role_key}
              className="w-full"
            >
              {newRole.project_id ? "Add role" : "Loading project catalog…"}
            </Button>
          </div>
        </div>
      )}
    </div>
  );
}
