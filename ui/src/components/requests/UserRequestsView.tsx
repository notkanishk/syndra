"use client";

import { useEffect, useMemo, useState } from "react";

import { ProjectName, RoleName, UserName } from "@/components/names";
import { Badge } from "@/components/ui/Badge";
import { Card, CardHeader, CardTitle } from "@/components/ui/Card";
import { EmptyState } from "@/components/ui/EmptyState";
import { Eyebrow } from "@/components/ui/Eyebrow";
import { Input } from "@/components/ui/Input";
import { Pulse } from "@/components/ui/Pulse";
import { Select } from "@/components/ui/Select";
import { SubmitButton } from "@/components/ui/SubmitButton";
import { useQuery } from "@tanstack/react-query";
import { request } from "@/lib/api-client";
import { useApplications } from "@/lib/queries/useApplications";
import { useCreateRequest, useRequestsMine } from "@/lib/queries/useRequests";
import { toastError, toastSuccess } from "@/lib/toast";
import type { SessionUser } from "@/lib/session";

interface CatalogResponse {
  projects: Array<{ id: string; name: string; roles: Array<{ key: string; label: string }> }>;
}

function useCatalogProjects() {
  return useQuery({
    queryKey: ["catalog", "projects"],
    queryFn: async (): Promise<CatalogResponse["projects"]> => {
      const data = await request<{ projects?: CatalogResponse["projects"] }>("/catalog");
      return Array.isArray(data?.projects) ? data.projects : [];
    },
  });
}

export default function UserRequestsView({ session }: { session: SessionUser }) {
  const appsQuery = useApplications();
  const projectsQuery = useCatalogProjects();
  const requestsQuery = useRequestsMine();
  const createRequest = useCreateRequest();

  const apps = useMemo(() => appsQuery.data ?? [], [appsQuery.data]);
  const projects = useMemo(() => projectsQuery.data ?? [], [projectsQuery.data]);
  const requests = useMemo(() => requestsQuery.data ?? [], [requestsQuery.data]);

  const [form, setForm] = useState({
    project_id: "",
    role_key: "",
    justification: "",
    duration_days: "14",
  });

  useEffect(() => {
    if (form.project_id || projects.length === 0) return;
    setForm((current) => ({
      ...current,
      project_id: projects[0].id,
      role_key: projects[0].roles?.[0]?.key ?? "",
    }));
  }, [projects, form.project_id]);

  async function submitRequest(event: React.FormEvent) {
    event.preventDefault();
    try {
      const durationDays = Number.parseInt(form.duration_days, 10);
      await createRequest.mutateAsync({
        project_id: form.project_id,
        role_key: form.role_key,
        justification: form.justification,
        duration_days: Number.isNaN(durationDays) ? 0 : durationDays,
      });
      toastSuccess("Request submitted", "We'll notify your administrator for review.");
    } catch (err) {
      toastError(err instanceof Error ? err.message : "Failed to submit request.");
    }
  }

  const selectedProject = projects.find((project) => project.id === form.project_id);

  return (
    <div className="space-y-6 animate-fade-in-up relative z-10">
      <header>
        <Eyebrow>My access requests</Eyebrow>
        <h1 className="text-3xl font-semibold text-on-surface mt-1 font-display">
          Request &amp; track service access
        </h1>
        <p className="text-on-surface-variant mt-2">
          {session.name}, request service access here and watch the status
          timeline without exposing the admin review queue.
        </p>
      </header>

      <div className="grid grid-cols-1 xl:grid-cols-[0.95fr,1.05fr] gap-6 items-start">
        <Card>
          <CardHeader>
            <CardTitle>Request a service</CardTitle>
          </CardHeader>

          <div className="rounded-card border border-outline-variant bg-surface-container-low p-4">
            <Eyebrow>Published services</Eyebrow>
            <div className="mt-2 space-y-2">
              {apps.length === 0 ? (
                <p className="text-sm text-on-surface-variant">
                  No services have been published yet. Ask your admin which
                  apps are available.
                </p>
              ) : (
                apps.map((entry) => (
                  <div
                    key={entry.application.id}
                    className="rounded-card border border-outline-variant bg-surface-container-lowest p-3"
                  >
                    <p className="font-semibold text-on-surface">
                      {entry.application.name}
                    </p>
                    <p className="mt-1 text-sm text-on-surface-variant">
                      {entry.application.description}
                    </p>
                  </div>
                ))
              )}
            </div>
          </div>

          <form onSubmit={submitRequest} className="mt-4 space-y-3">
            <div>
              <label className="block text-xs font-medium text-on-surface-variant mb-1">
                Project
              </label>
              <Select
                value={form.project_id}
                onChange={(event) => {
                  const projectId = event.target.value;
                  const project = projects.find((entry) => entry.id === projectId);
                  setForm({
                    ...form,
                    project_id: projectId,
                    role_key: project?.roles[0]?.key || "",
                  });
                }}
              >
                {projects.map((project) => (
                  <option key={project.id} value={project.id}>
                    {project.name}
                  </option>
                ))}
              </Select>
            </div>
            <div>
              <label className="block text-xs font-medium text-on-surface-variant mb-1">
                Role
              </label>
              <Select
                value={form.role_key}
                onChange={(event) => setForm({ ...form, role_key: event.target.value })}
              >
                {(selectedProject?.roles || []).map((role) => (
                  <option key={role.key} value={role.key}>
                    {role.label}
                  </option>
                ))}
              </Select>
            </div>
            <div>
              <label className="block text-xs font-medium text-on-surface-variant mb-1">
                Justification
              </label>
              <textarea
                value={form.justification}
                onChange={(event) => setForm({ ...form, justification: event.target.value })}
                placeholder="Why do you need this access?"
                className="min-h-28 block w-full rounded-card bg-surface-container px-4 py-2 text-sm text-on-surface placeholder:text-on-surface-variant shadow-[inset_0_1px_2px_rgba(0,0,0,0.4)] focus-visible:outline-2 focus-visible:outline-primary-container focus-visible:outline-offset-1"
              />
            </div>
            <div>
              <label className="block text-xs font-medium text-on-surface-variant mb-1">
                Duration (days)
              </label>
              <Input
                type="number"
                min="0"
                value={form.duration_days}
                onChange={(event) => setForm({ ...form, duration_days: event.target.value })}
              />
            </div>
            <SubmitButton
              isPending={createRequest.isPending}
              pendingLabel="Submitting…"
              disabled={
                !form.project_id || !form.role_key || !form.justification.trim()
              }
              className="w-full"
              label={form.project_id ? "Submit my request" : "Loading services…"}
            />
          </form>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>My request history</CardTitle>
          </CardHeader>
          <div className="space-y-3">
            {requests.length === 0 ? (
              <EmptyState
                eyebrow="Nothing yet"
                title="No requests submitted yet"
                description="Submit a request above and you'll see your status timeline here."
              />
            ) : (
              requests.map((entry) => {
                const variant: "outline" | "secondary" | "destructive" =
                  entry.status === "pending"
                    ? "outline"
                    : entry.status === "rejected"
                      ? "destructive"
                      : "secondary";
                const pulseVariant =
                  entry.status === "approved"
                    ? "success"
                    : entry.status === "rejected"
                      ? "error"
                      : "info";
                return (
                  <div
                    key={entry.id}
                    className="rounded-card border border-outline-variant bg-surface-container-low p-4"
                  >
                    <div className="flex items-start justify-between gap-3">
                      <div className="min-w-0">
                        <div className="flex items-center gap-2">
                          <Pulse variant={pulseVariant} static={entry.status !== "pending"} />
                          <p className="font-semibold text-on-surface truncate">
                            <ProjectName id={entry.project_id} /> ·{" "}
                            <RoleName projectId={entry.project_id} roleKey={entry.role_key} />
                          </p>
                        </div>
                        <p className="mt-1 text-sm text-on-surface-variant">
                          {entry.justification}
                        </p>
                      </div>
                      <Badge variant={variant}>{entry.status}</Badge>
                    </div>
                    <div className="mt-3 flex flex-wrap gap-2 text-xs">
                      <Badge variant="secondary">
                        {new Date(entry.created_at).toLocaleString()}
                      </Badge>
                      {entry.duration_days ? (
                        <Badge variant="outline">{entry.duration_days} day grant</Badge>
                      ) : (
                        <Badge variant="outline">Permanent request</Badge>
                      )}
                    </div>
                    {entry.status !== "pending" && (entry.reviewer_id || entry.review_note) && (
                      <div className="mt-3 rounded-card border border-outline-variant/60 bg-surface-container-lowest p-3 text-xs text-on-surface-variant">
                        <p>
                          <span className="font-semibold text-on-surface">
                            Reviewed
                            {entry.reviewer_id ? (
                              <>
                                {" "}by <UserName id={entry.reviewer_id} />
                              </>
                            ) : null}
                          </span>
                          {entry.review_note ? `: ${entry.review_note}` : ""}
                        </p>
                      </div>
                    )}
                  </div>
                );
              })
            )}
          </div>
        </Card>
      </div>
    </div>
  );
}
