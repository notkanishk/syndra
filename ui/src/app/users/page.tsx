"use client";

import { useCallback, useEffect, useState } from "react";

import { Badge } from "@/components/ui/Badge";
import { Card, CardHeader, CardTitle } from "@/components/ui/Card";
import { ConfirmModal } from "@/components/ui/ConfirmModal";
import { EmptyState } from "@/components/ui/EmptyState";
import { SkeletonCardList } from "@/components/ui/Skeleton";
import { SubmitButton } from "@/components/ui/SubmitButton";
import { describeExpiry, formatRoleRef } from "@/lib/format";
import { useDebounce } from "@/lib/useDebounce";
import { toastError, toastSuccess } from "@/lib/toast";

interface Bundle {
  id: string;
  name: string;
  description: string;
}

interface ProjectCatalog {
  id: string;
  name: string;
  roles: Array<{ key: string; label: string }>;
}

interface CatalogResponse {
  projects: ProjectCatalog[];
}

interface UserListItem {
  user: {
    id: string;
    name: string;
    email: string;
    title: string;
    team: string;
    status: string;
    avatar: string;
  };
  bundle_count: number;
  effective_role_count: number;
  key_projects: string[];
}

interface AccessRole {
  role_key: string;
  reasons: Array<{
    kind: string;
    description: string;
  }>;
}

interface DirectGrant {
  id: string;
  project_id: string;
  role_key: string;
  granted_by: string;
  reason: string;
  expires_at?: string | null;
}

interface BundleRoleEntry {
  zitadel_project_id: string;
  zitadel_role_key: string;
}

interface UserAccessView {
  user: UserListItem["user"];
  bundles: Bundle[];
  projects: Array<{
    project_id: string;
    project_name: string;
    source_roles: AccessRole[];
    derived_roles: AccessRole[];
    effective_role_keys: string[];
  }>;
  cleanup_hints: string[];
}

export default function UsersView() {
  const [query, setQuery] = useState("");
  const debouncedQuery = useDebounce(query, 300);
  const [users, setUsers] = useState<UserListItem[]>([]);
  const [allBundles, setAllBundles] = useState<Bundle[]>([]);
  const [projects, setProjects] = useState<ProjectCatalog[]>([]);
  const [selectedUser, setSelectedUser] = useState<string>("");
  const [access, setAccess] = useState<UserAccessView | null>(null);
  const [grants, setGrants] = useState<DirectGrant[]>([]);
  const [loadingUsers, setLoadingUsers] = useState(true);
  const [loadingAccess, setLoadingAccess] = useState(false);
  const [submittingGrant, setSubmittingGrant] = useState(false);
  const [assigningBundleId, setAssigningBundleId] = useState<string>("");
  const [bundleRoleCounts, setBundleRoleCounts] = useState<Record<string, BundleRoleEntry[]>>({});
  const [pendingAssignBundle, setPendingAssignBundle] = useState<Bundle | null>(null);
  // Form defaults are populated from the live catalog after loadReferenceData
  // resolves. Starting empty avoids leaking demo identifiers and produces a
  // 400 only on intentional empty submission rather than the catalog still
  // loading.
  const [grantForm, setGrantForm] = useState({
    project_id: "",
    role_key: "",
    reason: "",
    duration_days: "14",
  });

  const loadUsers = useCallback(async (search = "") => {
    setLoadingUsers(true);
    try {
      const res = await fetch(`/api/proxy/users${search ? `?q=${encodeURIComponent(search)}` : ""}`);
      const data = await res.json();
      const items = Array.isArray(data) ? data : [];
      setUsers(items);
      if (!selectedUser && items.length > 0) {
        setSelectedUser(items[0].user.id);
      }
    } finally {
      setLoadingUsers(false);
    }
  }, [selectedUser]);

  async function loadReferenceData() {
    const [bundleRes, catalogRes] = await Promise.all([
      fetch("/api/proxy/bundles"),
      fetch("/api/proxy/catalog"),
    ]);

    const bundles = await bundleRes.json();
    const catalog: CatalogResponse = await catalogRes.json();
    const catalogProjects = Array.isArray(catalog?.projects) ? catalog.projects : [];
    const bundleList: Bundle[] = Array.isArray(bundles) ? bundles : [];
    setAllBundles(bundleList);
    setProjects(catalogProjects);

    // Fan out one /bundles/{id}/roles fetch per bundle so the assignment UI
    // can preview the exact role list before the admin confirms. Cheap
    // because the backend already caches these.
    const counts = await Promise.all(
      bundleList.map(async (bundle) => {
        try {
          const r = await fetch(`/api/proxy/bundles/${bundle.id}/roles`);
          const roles = await r.json();
          return [bundle.id, Array.isArray(roles) ? roles : []] as const;
        } catch {
          return [bundle.id, [] as BundleRoleEntry[]] as const;
        }
      }),
    );
    setBundleRoleCounts(Object.fromEntries(counts));
    // Populate grant form defaults from the live catalog if the user hasn't
    // already picked something.
    if (catalogProjects.length > 0) {
      setGrantForm((current) => {
        if (current.project_id) return current;
        return {
          ...current,
          project_id: catalogProjects[0].id,
          role_key: catalogProjects[0].roles[0]?.key ?? "",
        };
      });
    }
  }

  async function loadAccess(userId: string) {
    if (!userId) {
      return;
    }
    setLoadingAccess(true);
    try {
      const [accessRes, grantsRes] = await Promise.all([
        fetch(`/api/proxy/users/${userId}/access`),
        fetch(`/api/proxy/users/${userId}/grants`),
      ]);
      const accessData = await accessRes.json();
      const grantData = await grantsRes.json();
      setAccess(accessData);
      setGrants(Array.isArray(grantData) ? grantData : []);
    } finally {
      setLoadingAccess(false);
    }
  }

  useEffect(() => {
    async function initialize() {
      await Promise.all([loadUsers(), loadReferenceData()]);
    }

    initialize();
  }, [loadUsers]);

  // React to debounced query changes — re-run the search after a quiet pause.
  useEffect(() => {
    loadUsers(debouncedQuery);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [debouncedQuery]);

  useEffect(() => {
    loadAccess(selectedUser);
  }, [selectedUser]);

  async function handleAssignBundle(bundleId: string) {
    if (!selectedUser) {
      return;
    }
    setAssigningBundleId(bundleId);
    try {
      const res = await fetch(`/api/proxy/users/${selectedUser}/bundles`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ bundle_id: bundleId }),
      });
      if (!res.ok) {
        const body = await res.json().catch(() => ({}));
        throw new Error(body.message || "Failed to assign bundle.");
      }
      await Promise.all([loadUsers(query), loadAccess(selectedUser)]);
      toastSuccess("Bundle assigned");
    } catch (err) {
      toastError(err instanceof Error ? err.message : "Failed to assign bundle.");
    } finally {
      setAssigningBundleId("");
    }
  }

  async function handleGrantSubmit(event: React.FormEvent) {
    event.preventDefault();
    if (!selectedUser) {
      return;
    }
    setSubmittingGrant(true);
    try {
      const durationDays = Number.parseInt(grantForm.duration_days, 10);
      // granted_by is intentionally omitted: the backend derives authorship
      // from the authenticated principal (Zitadel JWT subject) or the proxy
      // injects the demo session id in local-dev. See backend resolveActor.
      const res = await fetch(`/api/proxy/users/${selectedUser}/grants`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          project_id: grantForm.project_id,
          role_key: grantForm.role_key,
          reason: grantForm.reason,
          duration_days: Number.isNaN(durationDays) ? 0 : durationDays,
        }),
      });
      if (!res.ok) {
        const body = await res.json().catch(() => ({}));
        throw new Error(body.message || "Failed to save direct grant.");
      }
      await Promise.all([loadUsers(query), loadAccess(selectedUser)]);
      toastSuccess("Direct grant saved");
    } catch (err) {
      toastError(err instanceof Error ? err.message : "Failed to save direct grant.");
    } finally {
      setSubmittingGrant(false);
    }
  }

  const selectedProject = projects.find((project) => project.id === grantForm.project_id);

  return (
    <div className="space-y-6 animate-fade-in-up">
      <header>
        <h1 className="text-3xl font-bold text-foreground">Users & Access</h1>
        <p className="text-muted mt-2">Trace lineage, assign bundles, and issue temporary direct grants for advanced operators.</p>
      </header>

      <div className="grid grid-cols-1 xl:grid-cols-[0.95fr,1.25fr] gap-6">
        <Card>
          <CardHeader>
            <CardTitle>User-Centric View</CardTitle>
          </CardHeader>
          <div className="flex gap-3">
            <input
              type="search"
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === "Enter") loadUsers(query);
              }}
              placeholder="Search users, teams, or emails"
              aria-label="Search users by name, team, or email"
              className="flex-1 rounded-lg border border-border bg-surface px-4 py-2 text-sm text-foreground focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
            />
            <button
              onClick={() => loadUsers(query)}
              aria-label="Run search"
              className="rounded-lg bg-primary px-4 py-2 text-sm font-medium text-white focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
            >
              Search
            </button>
          </div>

          <div className="mt-4 space-y-3">
            {loadingUsers ? (
              <SkeletonCardList count={4} />
            ) : users.length === 0 ? (
              <EmptyState
                title={query ? "No users match that search" : "No users found"}
                description={
                  query
                    ? "Try a different search term, or clear the field to see everyone."
                    : "Confirm that ZITADEL_DOMAIN + ZITADEL_MACHINE_KEY_PATH are set so MkAuth can fetch the live user directory."
                }
              />
            ) : (
              users.map((entry) => (
                <button
                  key={entry.user.id}
                  onClick={() => setSelectedUser(entry.user.id)}
                  className={`w-full rounded-xl border p-4 text-left transition-colors ${
                    selectedUser === entry.user.id
                      ? "border-primary bg-primary/5"
                      : "border-border bg-surfaceHover hover:border-primary/35"
                  }`}
                >
                  <div className="flex items-start justify-between gap-3">
                    <div className="flex items-center gap-3">
                      <div className="flex h-10 w-10 items-center justify-center rounded-full bg-primary/10 text-sm font-semibold text-primary">
                        {entry.user.avatar}
                      </div>
                      <div>
                        <p className="font-semibold text-foreground">{entry.user.name}</p>
                        <p className="text-sm text-muted">{entry.user.title}</p>
                      </div>
                    </div>
                    <Badge variant="secondary">{entry.user.status}</Badge>
                  </div>
                  <div className="mt-3 flex flex-wrap gap-2">
                    <Badge variant="outline">{entry.bundle_count} bundles</Badge>
                    <Badge variant="outline">{entry.effective_role_count} effective roles</Badge>
                    {entry.key_projects.slice(0, 2).map((project) => (
                      <Badge key={project} variant="secondary">
                        {project}
                      </Badge>
                    ))}
                  </div>
                </button>
              ))
            )}
          </div>
        </Card>

        <div className="space-y-6">
          <Card>
            <CardHeader>
              <CardTitle>Access Lineage</CardTitle>
            </CardHeader>
            {loadingAccess || !access ? (
              <p className="text-sm text-muted">Loading access lineage...</p>
            ) : (
              <div className="space-y-4">
                <div className="rounded-xl border border-border bg-surfaceHover p-4">
                  <div className="flex items-center justify-between gap-3">
                    <div>
                      <p className="text-xl font-semibold text-foreground">{access.user.name}</p>
                      <p className="text-sm text-muted">{access.user.email} · {access.user.team}</p>
                    </div>
                    <Badge variant="secondary">{access.user.status}</Badge>
                  </div>
                  <div className="mt-3 flex flex-wrap gap-2">
                    {(access.bundles || []).map((bundle) => (
                      <Badge key={bundle.id} variant="outline" className="border-primary/30 text-primary">
                        {bundle.name}
                      </Badge>
                    ))}
                  </div>
                </div>

                {(access.projects || []).map((project) => (
                  <div key={project.project_id} className="rounded-xl border border-border bg-surfaceHover p-4">
                    <div className="flex items-center justify-between gap-3">
                      <h3 className="text-base font-semibold">{project.project_name}</h3>
                      <Badge variant="secondary">{project.effective_role_keys.length} effective roles</Badge>
                    </div>

                    <div className="mt-4 grid grid-cols-1 lg:grid-cols-2 gap-4">
                      <div>
                        <p className="text-xs font-semibold uppercase tracking-[0.22em] text-primary">Source · directly granted</p>
                        <div className="mt-2 space-y-2">
                          {(project.source_roles || []).length === 0 ? (
                            <p className="text-sm text-muted">No direct grants in this project.</p>
                          ) : (
                            (project.source_roles || []).map((role) => {
                              const ref = formatRoleRef(project.project_id, role.role_key, projects);
                              return (
                                <div
                                  key={role.role_key}
                                  className="rounded-lg border border-primary/30 bg-primary/5 p-3"
                                >
                                  <div className="flex items-baseline justify-between gap-3">
                                    <p className="font-medium text-foreground">{ref.label}</p>
                                    <code className="text-[10px] text-muted">{ref.raw}</code>
                                  </div>
                                  {role.reasons.map((reason) => (
                                    <p key={`${role.role_key}-${reason.description}`} className="mt-1 text-xs text-muted">
                                      {reason.description}
                                    </p>
                                  ))}
                                </div>
                              );
                            })
                          )}
                        </div>
                      </div>

                      <div>
                        <p className="text-xs font-semibold uppercase tracking-[0.22em] text-emerald-600 dark:text-emerald-400">Derived · via bundles &amp; rules</p>
                        <div className="mt-2 space-y-2">
                          {(project.derived_roles || []).length === 0 ? (
                            <p className="text-sm text-muted">No derived roles in this project.</p>
                          ) : (
                            (project.derived_roles || []).map((role) => {
                              const ref = formatRoleRef(project.project_id, role.role_key, projects);
                              return (
                                <div
                                  key={role.role_key}
                                  className="rounded-lg border border-emerald-500/30 bg-emerald-500/5 p-3"
                                >
                                  <div className="flex items-baseline justify-between gap-3">
                                    <p className="font-medium text-foreground">{ref.label}</p>
                                    <code className="text-[10px] text-muted">{ref.raw}</code>
                                  </div>
                                  {role.reasons.map((reason) => (
                                    <p
                                      key={`${role.role_key}-${reason.description}`}
                                      className="mt-1 text-xs text-muted"
                                      title={`Inherited via ${reason.kind ?? "rule"}`}
                                    >
                                      <span className="font-semibold text-emerald-600 dark:text-emerald-400">↳ </span>
                                      {reason.description}
                                    </p>
                                  ))}
                                </div>
                              );
                            })
                          )}
                        </div>
                      </div>
                    </div>
                  </div>
                ))}

                {access.cleanup_hints.length > 0 && (
                  <div className="rounded-xl border border-primary/20 bg-primary/5 p-4">
                    <p className="text-xs uppercase tracking-[0.22em] text-primary">Least-Privilege Hints</p>
                    {access.cleanup_hints.map((hint) => (
                      <p key={hint} className="mt-2 text-sm text-muted">
                        {hint}
                      </p>
                    ))}
                  </div>
                )}
              </div>
            )}
          </Card>

          <div className="grid grid-cols-1 2xl:grid-cols-2 gap-6">
            <Card>
              <CardHeader>
                <CardTitle>Assign Bundle</CardTitle>
              </CardHeader>
              <p className="text-sm text-muted">Normal admin flow for reusable access sets. Selecting a bundle previews exactly which roles will apply before you confirm.</p>
              <div className="mt-4 space-y-3">
                {allBundles.map((bundle) => {
                  const isAssigned = Boolean(access?.bundles.some((entry) => entry.id === bundle.id));
                  const isSubmitting = assigningBundleId === bundle.id;
                  const roles = bundleRoleCounts[bundle.id] ?? [];
                  return (
                    <div key={bundle.id} className="rounded-xl border border-border bg-surfaceHover p-4">
                      <div className="flex items-start justify-between gap-3">
                        <div>
                          <div className="flex items-center gap-2">
                            <p className="font-semibold text-foreground">{bundle.name}</p>
                            <Badge variant="outline" className="text-[10px]">
                              {roles.length} {roles.length === 1 ? "role" : "roles"}
                            </Badge>
                          </div>
                          <p className="mt-1 text-sm text-muted">{bundle.description}</p>
                          {roles.length > 0 && (
                            <div className="mt-2 flex flex-wrap gap-1">
                              {roles.slice(0, 4).map((r) => {
                                const ref = formatRoleRef(r.zitadel_project_id, r.zitadel_role_key, projects);
                                return (
                                  <Badge
                                    key={`${r.zitadel_project_id}-${r.zitadel_role_key}`}
                                    variant="secondary"
                                    className="text-[10px]"
                                    title={ref.raw}
                                  >
                                    {ref.label}
                                  </Badge>
                                );
                              })}
                              {roles.length > 4 && (
                                <Badge variant="outline" className="text-[10px]">
                                  +{roles.length - 4} more
                                </Badge>
                              )}
                            </div>
                          )}
                        </div>
                        <button
                          onClick={() => setPendingAssignBundle(bundle)}
                          disabled={isAssigned || isSubmitting}
                          aria-busy={isSubmitting || undefined}
                          className={`inline-flex items-center gap-2 rounded-lg px-3 py-2 text-xs font-semibold uppercase tracking-[0.16em] disabled:cursor-not-allowed ${
                            isAssigned
                              ? "bg-muted/10 text-muted"
                              : "bg-primary/10 text-primary hover:bg-primary hover:text-white"
                          }`}
                        >
                          {isSubmitting && (
                            <span aria-hidden="true" className="h-3 w-3 animate-spin rounded-full border-2 border-current/40 border-t-current" />
                          )}
                          {isAssigned ? "Assigned" : isSubmitting ? "Assigning…" : "Assign"}
                        </button>
                      </div>
                    </div>
                  );
                })}
              </div>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle>Advanced Direct Grant</CardTitle>
              </CardHeader>
              <p className="text-sm text-muted">Issue a raw role directly, optionally with an expiry for temporary access.</p>
              <form onSubmit={handleGrantSubmit} className="mt-4 space-y-3">
                <select
                  value={grantForm.project_id}
                  onChange={(event) => {
                    const projectId = event.target.value;
                    const project = projects.find((entry) => entry.id === projectId);
                    setGrantForm({
                      ...grantForm,
                      project_id: projectId,
                      role_key: project?.roles[0]?.key || "",
                    });
                  }}
                  className="w-full rounded-lg border border-border bg-surface px-3 py-2 text-sm"
                >
                  {projects.map((project) => (
                    <option key={project.id} value={project.id}>
                      {project.name}
                    </option>
                  ))}
                </select>
                <select
                  value={grantForm.role_key}
                  onChange={(event) => setGrantForm({ ...grantForm, role_key: event.target.value })}
                  className="w-full rounded-lg border border-border bg-surface px-3 py-2 text-sm"
                >
                  {(selectedProject?.roles || []).map((role) => (
                    <option key={role.key} value={role.key}>
                      {role.label}
                    </option>
                  ))}
                </select>
                <input
                  type="text"
                  value={grantForm.reason}
                  onChange={(event) => setGrantForm({ ...grantForm, reason: event.target.value })}
                  placeholder="Why this direct grant exists"
                  className="w-full rounded-lg border border-border bg-surface px-3 py-2 text-sm"
                />
                <div>
                  <p className="text-xs uppercase tracking-[0.18em] text-muted">Duration</p>
                  <div className="mt-2 flex flex-wrap gap-2">
                    {([
                      { label: "1 week", value: "7" },
                      { label: "1 month", value: "30" },
                      { label: "1 semester", value: "120" },
                      { label: "Permanent", value: "0" },
                    ] as const).map((opt) => {
                      const selected = grantForm.duration_days === opt.value;
                      return (
                        <button
                          type="button"
                          key={opt.value}
                          onClick={() => setGrantForm({ ...grantForm, duration_days: opt.value })}
                          className={`rounded-full border px-3 py-1 text-xs font-medium transition-colors ${
                            selected
                              ? "border-primary bg-primary/10 text-primary"
                              : "border-border text-muted hover:text-foreground hover:border-primary/40"
                          }`}
                        >
                          {opt.label}
                        </button>
                      );
                    })}
                    <input
                      type="number"
                      min="0"
                      value={grantForm.duration_days}
                      onChange={(event) => setGrantForm({ ...grantForm, duration_days: event.target.value })}
                      placeholder="Custom days"
                      className="w-28 rounded-full border border-border bg-surface px-3 py-1 text-xs"
                      aria-label="Custom duration in days"
                    />
                  </div>
                  <p className="mt-1 text-xs text-muted">{grantForm.duration_days === "0" ? "Permanent grant — no expiry." : `Expires after ${grantForm.duration_days} day${grantForm.duration_days === "1" ? "" : "s"}.`}</p>
                </div>
<SubmitButton
                  isPending={submittingGrant}
                  pendingLabel="Saving…"
                  disabled={!grantForm.project_id || !grantForm.role_key}
                  className="w-full"
                  label={grantForm.project_id ? "Save Direct Grant" : "Loading project catalog…"}
                />
              </form>

              <div className="mt-5 space-y-3">
                {grants.map((grant) => {
                  const ref = formatRoleRef(grant.project_id, grant.role_key, projects);
                  const exp = describeExpiry(grant.expires_at);
                  const tone = exp?.tone;
                  const variant: "destructive" | "outline" | "secondary" = !grant.expires_at
                    ? "secondary"
                    : tone === "expired" || tone === "critical"
                      ? "destructive"
                      : "outline";
                  return (
                    <div key={grant.id} className="rounded-xl border border-border bg-surfaceHover p-4">
                      <div className="flex items-center justify-between gap-3">
                        <div>
                          <p className="font-semibold text-foreground">{ref.label}</p>
                          <code className="text-[10px] text-muted">{ref.raw}</code>
                        </div>
                        <Badge variant={variant}>
                          {!grant.expires_at
                            ? "Permanent"
                            : exp
                              ? exp.countdown
                              : `Expires ${new Date(grant.expires_at).toLocaleDateString()}`}
                        </Badge>
                      </div>
                      <p className="mt-2 text-sm text-muted">{grant.reason || "No reason recorded"}</p>
                      <p className="mt-1 text-xs text-muted">Granted by {grant.granted_by}</p>
                    </div>
                  );
                })}
              </div>
            </Card>
          </div>
        </div>
      </div>

      <ConfirmModal
        open={Boolean(pendingAssignBundle)}
        title={`Assign "${pendingAssignBundle?.name ?? ""}"?`}
        description={(() => {
          const roles = pendingAssignBundle ? bundleRoleCounts[pendingAssignBundle.id] ?? [] : [];
          if (roles.length === 0) {
            return "This bundle has no roles defined yet — assigning it grants nothing until roles are added.";
          }
          const refs = roles
            .slice(0, 6)
            .map((r) => formatRoleRef(r.zitadel_project_id, r.zitadel_role_key, projects).label);
          const rest = roles.length > 6 ? ` and ${roles.length - 6} more` : "";
          return `This adds ${roles.length} role${roles.length === 1 ? "" : "s"}: ${refs.join(", ")}${rest}.`;
        })()}
        confirmLabel="Assign bundle"
        isPending={Boolean(assigningBundleId)}
        onCancel={() => setPendingAssignBundle(null)}
        onConfirm={async () => {
          const id = pendingAssignBundle?.id ?? "";
          setPendingAssignBundle(null);
          if (id) await handleAssignBundle(id);
        }}
      />
    </div>
  );
}
