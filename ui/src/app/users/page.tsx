"use client";

import { useEffect, useMemo, useState } from "react";

import { ProjectName } from "@/components/names/ProjectName";
import { RoleName } from "@/components/names/RoleName";
import { UserName } from "@/components/names/UserName";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Card, CardHeader, CardTitle } from "@/components/ui/Card";
import { ConfirmModal } from "@/components/ui/ConfirmModal";
import { EmptyState } from "@/components/ui/EmptyState";
import { Eyebrow } from "@/components/ui/Eyebrow";
import { Input } from "@/components/ui/Input";
import { Select } from "@/components/ui/Select";
import { SkeletonCardList } from "@/components/ui/Skeleton";
import { SubmitButton } from "@/components/ui/SubmitButton";
import { describeExpiry } from "@/lib/format";
import {
  type AccessRole,
  useAssignBundle,
  useCreateGrant,
  useUserAccess,
  useUserGrants,
  useUsers,
} from "@/lib/queries/useUsers";
import { useBundles, useBundleRolesByBundle } from "@/lib/queries/useBundles";
import { useProjects } from "@/lib/queries/useProjects";
import { useDebounce } from "@/lib/useDebounce";
import { toastError, toastSuccess } from "@/lib/toast";

interface GrantFormState {
  project_id: string;
  role_key: string;
  reason: string;
  duration_days: string;
}

const DURATION_OPTIONS = [
  { label: "1 week", value: "7" },
  { label: "1 month", value: "30" },
  { label: "1 semester", value: "120" },
  { label: "Permanent", value: "0" },
] as const;

/**
 * Renders the source/derived column of the lineage tree. Source roles are
 * directly granted; derived roles flow in via bundles or mapping rules. The
 * column is purely structural — names are resolved by the surrounding Name
 * components.
 */
function LineageRoleColumn({
  projectId,
  roles,
  variant,
}: {
  projectId: string;
  roles: AccessRole[];
  variant: "source" | "derived";
}) {
  const eyebrowText = variant === "source" ? "Source · directly granted" : "Derived · via bundles & rules";
  const accent =
    variant === "source"
      ? "border-l-2 border-primary-container"
      : "border-l-2 border-[var(--success)]";
  return (
    <div>
      <Eyebrow tone={variant === "source" ? "primary" : "muted"}>{eyebrowText}</Eyebrow>
      <div className={`mt-2 space-y-2 pl-3 ${accent}`}>
        {roles.length === 0 ? (
          <p className="text-sm text-on-surface-variant">
            No {variant === "source" ? "direct grants" : "derived roles"} in this project.
          </p>
        ) : (
          roles.map((role) => (
            <div
              key={role.role_key}
              className="rounded-card border border-outline-variant bg-surface-container-low/40 p-3"
            >
              <p className="font-medium text-on-surface">
                <RoleName projectId={projectId} roleKey={role.role_key} />
              </p>
              {role.reasons.map((reason) => (
                <p
                  key={`${role.role_key}-${reason.description}`}
                  className="mt-1 text-xs text-on-surface-variant"
                  title={reason.kind ? `Inherited via ${reason.kind}` : undefined}
                >
                  {variant === "derived" && (
                    <span className="font-semibold text-[var(--success)]">↳ </span>
                  )}
                  {reason.description}
                </p>
              ))}
            </div>
          ))
        )}
      </div>
    </div>
  );
}

export default function UsersView() {
  const [query, setQuery] = useState("");
  const debouncedQuery = useDebounce(query, 300);
  const [selectedProjectFilters, setSelectedProjectFilters] = useState<string[]>([]);
  const [selectedUser, setSelectedUser] = useState<string>("");
  const [pendingAssignBundleId, setPendingAssignBundleId] = useState<string | null>(null);
  const [grantForm, setGrantForm] = useState<GrantFormState>({
    project_id: "",
    role_key: "",
    reason: "",
    duration_days: "14",
  });

  const usersQuery = useUsers(debouncedQuery);
  const bundlesQuery = useBundles();
  const projectsQuery = useProjects();
  const accessQuery = useUserAccess(selectedUser);
  const grantsQuery = useUserGrants(selectedUser);
  const assignBundle = useAssignBundle(selectedUser);
  const createGrant = useCreateGrant(selectedUser);

  const users = useMemo(() => usersQuery.data ?? [], [usersQuery.data]);
  const allBundles = useMemo(() => bundlesQuery.data ?? [], [bundlesQuery.data]);
  const projects = useMemo(() => projectsQuery.data ?? [], [projectsQuery.data]);

  // Fan out one /bundles/{id}/roles per bundle so the assignment UI can preview
  // the exact role list before the admin confirms — same pattern Stage 1 already
  // uses on /projects.
  const bundleIds = useMemo(() => allBundles.map((b) => b.id), [allBundles]);
  const { byId: bundleRolesById } = useBundleRolesByBundle(bundleIds);

  // Auto-select first user once loaded.
  useEffect(() => {
    if (!selectedUser && users.length > 0) {
      setSelectedUser(users[0].user.id);
    }
  }, [users, selectedUser]);

  // Populate grant form defaults from the live project list once loaded so the
  // Direct Grant form shows a real project rather than an empty placeholder.
  useEffect(() => {
    if (grantForm.project_id) return;
    if (projects.length === 0) return;
    const first = projects[0].project;
    setGrantForm((current) => ({
      ...current,
      project_id: first.id,
      role_key: first.roles?.[0]?.key ?? "",
    }));
  }, [projects, grantForm.project_id]);

  const selectedProject = projects.find((p) => p.project.id === grantForm.project_id)?.project;

  // Project pill list reflects the union of projects appearing in any user's
  // key_projects so admins can filter the list to "people with X access".
  // Toggling a pill is purely a client-side narrowing.
  const projectPillSet = useMemo(() => {
    const set = new Set<string>();
    for (const entry of users) {
      for (const project of entry.key_projects ?? []) set.add(project);
    }
    return Array.from(set).sort();
  }, [users]);

  const filteredUsers = useMemo(() => {
    if (selectedProjectFilters.length === 0) return users;
    return users.filter((entry) =>
      selectedProjectFilters.every((p) => entry.key_projects?.includes(p)),
    );
  }, [users, selectedProjectFilters]);

  const access = accessQuery.data ?? null;
  const grants = grantsQuery.data ?? [];

  function toggleProjectPill(name: string) {
    setSelectedProjectFilters((prev) =>
      prev.includes(name) ? prev.filter((p) => p !== name) : [...prev, name],
    );
  }

  async function handleConfirmAssign() {
    if (!pendingAssignBundleId) return;
    try {
      await assignBundle.mutateAsync(pendingAssignBundleId);
      toastSuccess("Bundle assigned");
    } catch (err) {
      toastError(err instanceof Error ? err.message : "Failed to assign bundle.");
    } finally {
      setPendingAssignBundleId(null);
    }
  }

  async function handleGrantSubmit(event: React.FormEvent) {
    event.preventDefault();
    if (!selectedUser) return;
    const durationDays = Number.parseInt(grantForm.duration_days, 10);
    try {
      await createGrant.mutateAsync({
        project_id: grantForm.project_id,
        role_key: grantForm.role_key,
        reason: grantForm.reason,
        duration_days: Number.isNaN(durationDays) ? 0 : durationDays,
      });
      toastSuccess("Direct grant saved");
    } catch (err) {
      toastError(err instanceof Error ? err.message : "Failed to save direct grant.");
    }
  }

  const pendingAssignBundle = pendingAssignBundleId
    ? allBundles.find((b) => b.id === pendingAssignBundleId) ?? null
    : null;
  const pendingAssignRoles = pendingAssignBundle
    ? bundleRolesById[pendingAssignBundle.id] ?? []
    : [];

  return (
    <div className="space-y-6 animate-fade-in-up">
      <header>
        <Eyebrow tone="primary">Users &amp; Access</Eyebrow>
        <h1 className="mt-3 font-display text-3xl font-semibold tracking-tight text-on-surface">
          Trace lineage and assign access
        </h1>
        <p className="mt-2 text-on-surface-variant">
          Filter the directory, inspect source vs derived roles, and issue temporary direct grants when an exception is needed.
        </p>
      </header>

      {/* 3-column shell: filter rail / user list / lineage. Stacks below xl
          so narrower viewports stay legible. */}
      <div className="grid grid-cols-1 gap-6 xl:grid-cols-[280px_1fr_1.4fr]">
        <Card variant="glass" className="xl:sticky xl:top-6 xl:self-start">
          <Eyebrow>Filter</Eyebrow>
          <div className="mt-4 space-y-4">
            <Input
              type="search"
              value={query}
              onChange={(event) => setQuery(event.target.value)}
              placeholder="Search users, teams, emails"
              aria-label="Search users by name, team, or email"
            />
            <div>
              <Eyebrow>Projects</Eyebrow>
              {projectPillSet.length === 0 ? (
                <p className="mt-2 text-xs text-on-surface-variant">
                  No project memberships found.
                </p>
              ) : (
                <div className="mt-2 flex flex-wrap gap-2">
                  {projectPillSet.map((name) => {
                    const active = selectedProjectFilters.includes(name);
                    return (
                      <button
                        type="button"
                        key={name}
                        onClick={() => toggleProjectPill(name)}
                        aria-pressed={active}
                        className={`rounded-full border px-3 py-1 text-xs transition-colors focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary-container ${
                          active
                            ? "border-primary-container bg-primary-container/15 text-on-surface"
                            : "border-outline-variant text-on-surface-variant hover:text-on-surface"
                        }`}
                      >
                        {name}
                      </button>
                    );
                  })}
                </div>
              )}
              {selectedProjectFilters.length > 0 && (
                <button
                  type="button"
                  onClick={() => setSelectedProjectFilters([])}
                  className="mt-3 text-xs text-on-surface-variant underline-offset-2 hover:text-on-surface hover:underline"
                >
                  Clear filters
                </button>
              )}
            </div>
          </div>
        </Card>

        <Card variant="glass">
          <CardHeader>
            <CardTitle>Directory</CardTitle>
          </CardHeader>
          <div className="space-y-3 max-h-[36rem] overflow-y-auto pr-1">
            {usersQuery.isLoading ? (
              <SkeletonCardList count={4} />
            ) : filteredUsers.length === 0 ? (
              <EmptyState
                title={query || selectedProjectFilters.length > 0 ? "No users match those filters" : "No users found"}
                description={
                  query || selectedProjectFilters.length > 0
                    ? "Try a different search term, or clear the filters to see everyone."
                    : "Confirm that ZITADEL_DOMAIN + ZITADEL_MACHINE_KEY_PATH are set so MkAuth can fetch the live user directory."
                }
              />
            ) : (
              filteredUsers.map((entry) => (
                <button
                  key={entry.user.id}
                  onClick={() => setSelectedUser(entry.user.id)}
                  className={`w-full rounded-card border p-4 text-left transition-colors focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary-container ${
                    selectedUser === entry.user.id
                      ? "border-primary-container bg-primary-container/10"
                      : "border-outline-variant bg-surface-container-low/40 hover:border-primary-container/50"
                  }`}
                >
                  <div className="flex items-start justify-between gap-3">
                    <div className="flex items-center gap-3">
                      <div className="flex h-10 w-10 items-center justify-center rounded-full bg-primary-container/15 text-sm font-semibold text-on-primary-container">
                        {entry.user.avatar}
                      </div>
                      <div className="min-w-0">
                        <p className="font-semibold text-on-surface truncate">{entry.user.name}</p>
                        <p className="text-sm text-on-surface-variant truncate">{entry.user.title}</p>
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
          <Card variant="glass" className="xl:sticky xl:top-6 xl:self-start">
            <CardHeader>
              <CardTitle>Access Lineage</CardTitle>
            </CardHeader>
            {accessQuery.isLoading || !access ? (
              <p className="text-sm text-on-surface-variant">Loading access lineage…</p>
            ) : (
              <div className="space-y-4">
                <div className="rounded-card border border-outline-variant bg-surface-container-low/40 p-4">
                  <div className="flex items-center justify-between gap-3">
                    <div>
                      <p className="text-xl font-semibold text-on-surface">
                        <UserName id={access.user.id} />
                      </p>
                      <p className="text-sm text-on-surface-variant">
                        {access.user.email} · {access.user.team}
                      </p>
                    </div>
                    <Badge variant="secondary">{access.user.status}</Badge>
                  </div>
                  <div className="mt-3 flex flex-wrap gap-2">
                    {(access.bundles || []).map((bundle) => (
                      <Badge key={bundle.id} variant="outline" className="border-primary-container text-on-primary-container">
                        {bundle.name}
                      </Badge>
                    ))}
                  </div>
                </div>

                {(access.projects || []).map((project) => (
                  <div
                    key={project.project_id}
                    className="rounded-card border border-outline-variant bg-surface-container-low/40 p-4"
                  >
                    <div className="flex items-center justify-between gap-3">
                      <h3 className="text-base font-semibold text-on-surface">
                        <ProjectName id={project.project_id} fallback={project.project_name} />
                      </h3>
                      <Badge variant="secondary">{project.effective_role_keys.length} effective roles</Badge>
                    </div>

                    <div className="mt-4 grid grid-cols-1 gap-4 lg:grid-cols-2">
                      <LineageRoleColumn
                        projectId={project.project_id}
                        roles={project.source_roles ?? []}
                        variant="source"
                      />
                      <LineageRoleColumn
                        projectId={project.project_id}
                        roles={project.derived_roles ?? []}
                        variant="derived"
                      />
                    </div>
                  </div>
                ))}

                {(access.cleanup_hints?.length ?? 0) > 0 && (
                  <div className="rounded-card border border-primary-container/40 bg-primary-container/10 p-4">
                    <Eyebrow tone="primary">Least-Privilege Hints</Eyebrow>
                    {access.cleanup_hints.map((hint) => (
                      <p key={hint} className="mt-2 text-sm text-on-surface-variant">
                        {hint}
                      </p>
                    ))}
                  </div>
                )}
              </div>
            )}
          </Card>

          <div className="grid grid-cols-1 gap-6 2xl:grid-cols-2">
            <Card variant="glass">
              <CardHeader>
                <CardTitle>Assign Bundle</CardTitle>
              </CardHeader>
              <p className="text-sm text-on-surface-variant">
                Normal admin flow for reusable access sets. Selecting a bundle previews the exact role list before you confirm.
              </p>
              <div className="mt-4 space-y-3">
                {allBundles.map((bundle) => {
                  const isAssigned = Boolean(access?.bundles.some((entry) => entry.id === bundle.id));
                  const isSubmitting =
                    assignBundle.isPending && pendingAssignBundleId === bundle.id;
                  const roles = bundleRolesById[bundle.id] ?? [];
                  return (
                    <div
                      key={bundle.id}
                      className="rounded-card border border-outline-variant bg-surface-container-low/40 p-4"
                    >
                      <div className="flex items-start justify-between gap-3">
                        <div>
                          <div className="flex items-center gap-2">
                            <p className="font-semibold text-on-surface">{bundle.name}</p>
                            <Badge variant="outline" className="text-[10px]">
                              {roles.length} {roles.length === 1 ? "role" : "roles"}
                            </Badge>
                          </div>
                          <p className="mt-1 text-sm text-on-surface-variant">{bundle.description}</p>
                          {roles.length > 0 && (
                            <div className="mt-2 flex flex-wrap gap-1">
                              {roles.slice(0, 4).map((r) => (
                                <Badge
                                  key={`${r.zitadel_project_id}-${r.zitadel_role_key}`}
                                  variant="secondary"
                                  className="text-[10px]"
                                >
                                  <RoleName
                                    projectId={r.zitadel_project_id}
                                    roleKey={r.zitadel_role_key}
                                    fallback={`${r.zitadel_project_id}:${r.zitadel_role_key}`}
                                  />
                                </Badge>
                              ))}
                              {roles.length > 4 && (
                                <Badge variant="outline" className="text-[10px]">
                                  +{roles.length - 4} more
                                </Badge>
                              )}
                            </div>
                          )}
                        </div>
                        <Button
                          size="sm"
                          variant={isAssigned ? "ghost" : "primary"}
                          disabled={isAssigned || isSubmitting}
                          isPending={isSubmitting}
                          onClick={() => setPendingAssignBundleId(bundle.id)}
                        >
                          {isAssigned ? "Assigned" : isSubmitting ? "Assigning…" : "Assign"}
                        </Button>
                      </div>
                    </div>
                  );
                })}
              </div>
            </Card>

            <Card variant="glass">
              <CardHeader>
                <CardTitle>Advanced Direct Grant</CardTitle>
              </CardHeader>
              <p className="text-sm text-on-surface-variant">
                Issue a raw role directly, optionally with an expiry for temporary access.
              </p>
              <form onSubmit={handleGrantSubmit} className="mt-4 space-y-3">
                <Select
                  value={grantForm.project_id}
                  onChange={(event) => {
                    const projectId = event.target.value;
                    const project = projects.find((entry) => entry.project.id === projectId)?.project;
                    setGrantForm({
                      ...grantForm,
                      project_id: projectId,
                      role_key: project?.roles?.[0]?.key ?? "",
                    });
                  }}
                  aria-label="Project for direct grant"
                >
                  {projects.map((entry) => (
                    <option key={entry.project.id} value={entry.project.id}>
                      {entry.project.name}
                    </option>
                  ))}
                </Select>
                <Select
                  value={grantForm.role_key}
                  onChange={(event) => setGrantForm({ ...grantForm, role_key: event.target.value })}
                  aria-label="Role for direct grant"
                >
                  {(selectedProject?.roles ?? []).map((role) => (
                    <option key={role.key} value={role.key}>
                      {role.label}
                    </option>
                  ))}
                </Select>
                <Input
                  type="text"
                  value={grantForm.reason}
                  onChange={(event) => setGrantForm({ ...grantForm, reason: event.target.value })}
                  placeholder="Why this direct grant exists"
                  aria-label="Reason for direct grant"
                />
                <div>
                  <Eyebrow>Duration</Eyebrow>
                  <div className="mt-2 flex flex-wrap gap-2">
                    {DURATION_OPTIONS.map((opt) => {
                      const selected = grantForm.duration_days === opt.value;
                      return (
                        <button
                          type="button"
                          key={opt.value}
                          onClick={() => setGrantForm({ ...grantForm, duration_days: opt.value })}
                          aria-pressed={selected}
                          className={`rounded-full border px-3 py-1 text-xs font-medium transition-colors focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary-container ${
                            selected
                              ? "border-primary-container bg-primary-container/15 text-on-surface"
                              : "border-outline-variant text-on-surface-variant hover:text-on-surface"
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
                      className="w-28 rounded-full border border-outline-variant bg-surface-container px-3 py-1 text-xs text-on-surface focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary-container"
                      aria-label="Custom duration in days"
                    />
                  </div>
                  <p className="mt-1 text-xs text-on-surface-variant">
                    {grantForm.duration_days === "0"
                      ? "Permanent grant — no expiry."
                      : `Expires after ${grantForm.duration_days} day${grantForm.duration_days === "1" ? "" : "s"}.`}
                  </p>
                </div>
                <SubmitButton
                  isPending={createGrant.isPending}
                  pendingLabel="Saving…"
                  disabled={!grantForm.project_id || !grantForm.role_key}
                  className="w-full"
                  label={grantForm.project_id ? "Save Direct Grant" : "Loading project catalog…"}
                />
              </form>

              <div className="mt-5 space-y-3">
                {grants.map((grant) => {
                  const exp = describeExpiry(grant.expires_at);
                  const tone = exp?.tone;
                  const variant: "destructive" | "outline" | "secondary" = !grant.expires_at
                    ? "secondary"
                    : tone === "expired" || tone === "critical"
                      ? "destructive"
                      : "outline";
                  return (
                    <div
                      key={grant.id}
                      className="rounded-card border border-outline-variant bg-surface-container-low/40 p-4"
                    >
                      <div className="flex items-center justify-between gap-3">
                        <p className="font-semibold text-on-surface">
                          <RoleName
                            projectId={grant.project_id}
                            roleKey={grant.role_key}
                            fallback={`${grant.project_id}:${grant.role_key}`}
                          />
                        </p>
                        <Badge variant={variant}>
                          {!grant.expires_at
                            ? "Permanent"
                            : exp
                              ? exp.countdown
                              : `Expires ${new Date(grant.expires_at).toLocaleDateString()}`}
                        </Badge>
                      </div>
                      <p className="mt-2 text-sm text-on-surface-variant">{grant.reason || "No reason recorded"}</p>
                      <p className="mt-1 text-xs text-on-surface-variant">
                        Granted by <UserName id={grant.granted_by} />
                      </p>
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
          if (pendingAssignRoles.length === 0) {
            return "This bundle has no roles defined yet — assigning it grants nothing until roles are added.";
          }
          const shown = pendingAssignRoles.slice(0, 6);
          const restCount = pendingAssignRoles.length - shown.length;
          // Inline <RoleName/> so the modal copy shows resolved display names,
          // not raw `project_id:role_key` pairs that would leak UUID-like
          // strings into the admin flow. The user-management spec requires
          // this listing to be name-resolved.
          return (
            <>
              This adds {pendingAssignRoles.length} role{pendingAssignRoles.length === 1 ? "" : "s"}:{" "}
              {shown.map((r, idx) => (
                <span key={`${r.zitadel_project_id}-${r.zitadel_role_key}`}>
                  <RoleName
                    projectId={r.zitadel_project_id}
                    roleKey={r.zitadel_role_key}
                  />
                  {idx < shown.length - 1 ? ", " : ""}
                </span>
              ))}
              {restCount > 0 ? ` and ${restCount} more` : ""}.
            </>
          );
        })()}
        confirmLabel="Assign bundle"
        isPending={assignBundle.isPending}
        onCancel={() => setPendingAssignBundleId(null)}
        onConfirm={handleConfirmAssign}
      />
    </div>
  );
}
