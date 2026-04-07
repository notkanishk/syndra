"use client";

import { useEffect, useState } from "react";

import { Badge } from "@/components/ui/Badge";
import { Card, CardHeader, CardTitle } from "@/components/ui/Card";

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
  const [users, setUsers] = useState<UserListItem[]>([]);
  const [allBundles, setAllBundles] = useState<Bundle[]>([]);
  const [projects, setProjects] = useState<ProjectCatalog[]>([]);
  const [selectedUser, setSelectedUser] = useState<string>("");
  const [access, setAccess] = useState<UserAccessView | null>(null);
  const [grants, setGrants] = useState<DirectGrant[]>([]);
  const [loadingUsers, setLoadingUsers] = useState(true);
  const [loadingAccess, setLoadingAccess] = useState(false);
  const [message, setMessage] = useState("");
  const [grantForm, setGrantForm] = useState({
    project_id: "printing",
    role_key: "member",
    reason: "Advanced admin override",
    duration_days: "14",
  });

  async function loadUsers(search = "") {
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
  }

  async function loadReferenceData() {
    const [bundleRes, catalogRes] = await Promise.all([
      fetch("/api/proxy/bundles"),
      fetch("/api/proxy/catalog"),
    ]);

    const bundles = await bundleRes.json();
    const catalog: CatalogResponse = await catalogRes.json();
    setAllBundles(Array.isArray(bundles) ? bundles : []);
    setProjects(Array.isArray(catalog?.projects) ? catalog.projects : []);
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
    loadUsers();
    loadReferenceData();
  }, []);

  useEffect(() => {
    loadAccess(selectedUser);
  }, [selectedUser]);

  async function handleAssignBundle(bundleId: string) {
    if (!selectedUser) {
      return;
    }
    const res = await fetch(`/api/proxy/users/${selectedUser}/bundles`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ bundle_id: bundleId }),
    });

    if (res.ok) {
      setMessage("Bundle assigned successfully.");
      await Promise.all([loadUsers(query), loadAccess(selectedUser)]);
      return;
    }

    const body = await res.json().catch(() => ({}));
    setMessage(body.message || "Failed to assign bundle.");
  }

  async function handleGrantSubmit(event: React.FormEvent) {
    event.preventDefault();
    if (!selectedUser) {
      return;
    }
    const durationDays = Number.parseInt(grantForm.duration_days, 10);
    const res = await fetch(`/api/proxy/users/${selectedUser}/grants`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        project_id: grantForm.project_id,
        role_key: grantForm.role_key,
        granted_by: "alice.rivera",
        reason: grantForm.reason,
        duration_days: Number.isNaN(durationDays) ? 0 : durationDays,
      }),
    });

    if (res.ok) {
      setMessage("Direct grant saved.");
      await Promise.all([loadUsers(query), loadAccess(selectedUser)]);
      return;
    }

    const body = await res.json().catch(() => ({}));
    setMessage(body.message || "Failed to save direct grant.");
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
              type="text"
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              placeholder="Search users, teams, or emails"
              className="flex-1 rounded-lg border border-border bg-surface px-4 py-2 text-sm text-foreground"
            />
            <button
              onClick={() => loadUsers(query)}
              className="rounded-lg bg-primary px-4 py-2 text-sm font-medium text-white"
            >
              Search
            </button>
          </div>

          <div className="mt-4 space-y-3">
            {loadingUsers ? (
              <p className="text-sm text-muted">Loading users...</p>
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
                      <Badge variant="secondary">{project.effective_role_keys.length} total roles</Badge>
                    </div>

                    <div className="mt-4 grid grid-cols-1 lg:grid-cols-2 gap-4">
                      <div>
                        <p className="text-xs uppercase tracking-[0.22em] text-muted">Source</p>
                        <div className="mt-2 space-y-2">
                          {(project.source_roles || []).length === 0 ? (
                            <p className="text-sm text-muted">No source grants in this project.</p>
                          ) : (
                            (project.source_roles || []).map((role) => (
                              <div key={role.role_key} className="rounded-lg border border-border bg-surface p-3">
                                <p className="font-medium text-foreground">{role.role_key}</p>
                                {role.reasons.map((reason) => (
                                  <p key={`${role.role_key}-${reason.description}`} className="mt-1 text-xs text-muted">
                                    {reason.description}
                                  </p>
                                ))}
                              </div>
                            ))
                          )}
                        </div>
                      </div>

                      <div>
                        <p className="text-xs uppercase tracking-[0.22em] text-muted">Derived</p>
                        <div className="mt-2 space-y-2">
                          {(project.derived_roles || []).length === 0 ? (
                            <p className="text-sm text-muted">No derived roles in this project.</p>
                          ) : (
                            (project.derived_roles || []).map((role) => (
                              <div key={role.role_key} className="rounded-lg border border-border bg-surface p-3">
                                <p className="font-medium text-foreground">{role.role_key}</p>
                                {role.reasons.map((reason) => (
                                  <p key={`${role.role_key}-${reason.description}`} className="mt-1 text-xs text-muted">
                                    {reason.description}
                                  </p>
                                ))}
                              </div>
                            ))
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
              <p className="text-sm text-muted">Normal admin flow for reusable access sets.</p>
              {message && <p className="mt-3 text-sm text-primary">{message}</p>}
              <div className="mt-4 space-y-3">
                {allBundles.map((bundle) => {
                  const isAssigned = Boolean(access?.bundles.some((entry) => entry.id === bundle.id));
                  return (
                    <div key={bundle.id} className="rounded-xl border border-border bg-surfaceHover p-4">
                      <div className="flex items-start justify-between gap-3">
                        <div>
                          <p className="font-semibold text-foreground">{bundle.name}</p>
                          <p className="mt-1 text-sm text-muted">{bundle.description}</p>
                        </div>
                        <button
                          onClick={() => handleAssignBundle(bundle.id)}
                          disabled={isAssigned}
                          className={`rounded-lg px-3 py-2 text-xs font-semibold uppercase tracking-[0.16em] ${
                            isAssigned
                              ? "bg-muted/10 text-muted"
                              : "bg-primary/10 text-primary hover:bg-primary hover:text-white"
                          }`}
                        >
                          {isAssigned ? "Assigned" : "Assign"}
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
                <input
                  type="number"
                  min="0"
                  value={grantForm.duration_days}
                  onChange={(event) => setGrantForm({ ...grantForm, duration_days: event.target.value })}
                  placeholder="Duration in days (0 = permanent)"
                  className="w-full rounded-lg border border-border bg-surface px-3 py-2 text-sm"
                />
                <button type="submit" className="w-full rounded-lg bg-primary px-4 py-2 text-sm font-medium text-white">
                  Save Direct Grant
                </button>
              </form>

              <div className="mt-5 space-y-3">
                {grants.map((grant) => (
                  <div key={grant.id} className="rounded-xl border border-border bg-surfaceHover p-4">
                    <div className="flex items-center justify-between gap-3">
                      <p className="font-semibold text-foreground">
                        {grant.project_id}:{grant.role_key}
                      </p>
                      <Badge variant={grant.expires_at ? "outline" : "secondary"}>
                        {grant.expires_at ? `Expires ${new Date(grant.expires_at).toLocaleDateString()}` : "Permanent"}
                      </Badge>
                    </div>
                    <p className="mt-2 text-sm text-muted">{grant.reason || "No reason recorded"}</p>
                    <p className="mt-1 text-xs text-muted">Granted by {grant.granted_by}</p>
                  </div>
                ))}
              </div>
            </Card>
          </div>
        </div>
      </div>
    </div>
  );
}
