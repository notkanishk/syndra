"use client";

import { useEffect, useState } from "react";

import { Badge } from "@/components/ui/Badge";
import { Card, CardHeader, CardTitle } from "@/components/ui/Card";
import { EmptyState } from "@/components/ui/EmptyState";
import { SubmitButton } from "@/components/ui/SubmitButton";
import { toastError, toastSuccess } from "@/lib/toast";
import type { SessionUser } from "@/lib/session";

interface AppView {
  application: {
    id: string;
    name: string;
    project_id: string;
    description: string;
  };
}

interface CatalogResponse {
  projects: Array<{ id: string; name: string; roles: Array<{ key: string; label: string }> }>;
}

interface AccessRequest {
  id: string;
  requester_id: string;
  project_id: string;
  role_key: string;
  justification: string;
  duration_days?: number | null;
  status: string;
  reviewer_id?: string;
  review_note?: string;
  created_at: string;
}

export default function UserRequestsView({ session }: { session: SessionUser }) {
  const [apps, setApps] = useState<AppView[]>([]);
  const [projects, setProjects] = useState<CatalogResponse["projects"]>([]);
  const [requests, setRequests] = useState<AccessRequest[]>([]);
  const [submitting, setSubmitting] = useState(false);
  // Defaults populate from the live catalog after `load()`. Empty initial
  // values avoid leaking demo identifiers into production HTML and surface
  // an explicit "loading" state until the catalog returns.
  const [form, setForm] = useState({
    project_id: "",
    role_key: "",
    justification: "",
    duration_days: "14",
  });

  async function load() {
    const [appsRes, catalogRes, requestsRes] = await Promise.all([
      fetch("/api/proxy/applications"),
      fetch("/api/proxy/catalog"),
      fetch("/api/proxy/requests"),
    ]);

    const appData = await appsRes.json();
    const catalogData = await catalogRes.json();
    const requestData = await requestsRes.json();

    setApps(Array.isArray(appData) ? appData : []);
    const catalogProjects = Array.isArray(catalogData?.projects) ? catalogData.projects : [];
    setProjects(catalogProjects);
    setRequests(Array.isArray(requestData) ? requestData : []);

    if (catalogProjects.length > 0) {
      setForm((current) => ({
        ...current,
        project_id: current.project_id || catalogProjects[0].id,
        role_key: current.role_key || catalogProjects[0].roles?.[0]?.key || "",
      }));
    }
  }

  useEffect(() => {
    load();
  }, []);

  async function submitRequest(event: React.FormEvent) {
    event.preventDefault();
    setSubmitting(true);
    try {
      const durationDays = Number.parseInt(form.duration_days, 10);
      const res = await fetch("/api/proxy/requests", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          project_id: form.project_id,
          role_key: form.role_key,
          justification: form.justification,
          duration_days: Number.isNaN(durationDays) ? 0 : durationDays,
        }),
      });
      if (!res.ok) {
        const body = await res.json().catch(() => ({}));
        throw new Error(body.message || "Failed to submit request.");
      }
      load();
      toastSuccess("Request submitted", "We'll notify your administrator for review.");
    } catch (err) {
      toastError(err instanceof Error ? err.message : "Failed to submit request.");
    } finally {
      setSubmitting(false);
    }
  }

  const selectedProject = projects.find((project) => project.id === form.project_id);
  const projectNameById = new Map(projects.map((project) => [project.id, project.name]));

  return (
    <div className="space-y-6 animate-fade-in-up">
      <header>
        <h1 className="text-3xl font-bold text-foreground">My Access Requests</h1>
        <p className="mt-2 text-muted">
          {session.name}, request service access here and track your own approval status without exposing the admin review queue.
        </p>
      </header>

      <div className="grid grid-cols-1 xl:grid-cols-[0.95fr,1.05fr] gap-6">
        <Card>
          <CardHeader>
            <CardTitle>Request a service</CardTitle>
          </CardHeader>
          <div className="space-y-3 rounded-2xl border border-border bg-surfaceHover p-4">
            <p className="text-xs uppercase tracking-[0.24em] text-muted">Published services</p>
            <div className="space-y-2">
              {apps.length === 0 ? (
                <p className="text-sm text-muted">No services have been published yet. Ask your admin which apps are available.</p>
              ) : (
                apps.map((entry) => (
                  <div key={entry.application.id} className="rounded-xl border border-border bg-background p-3">
                    <p className="font-semibold text-foreground">{entry.application.name}</p>
                    <p className="mt-1 text-sm text-muted">{entry.application.description}</p>
                  </div>
                ))
              )}
            </div>
          </div>

          <form onSubmit={submitRequest} className="mt-4 space-y-3">
            <select
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
              className="w-full rounded-lg border border-border bg-surface px-3 py-2 text-sm"
            >
              {projects.map((project) => (
                <option key={project.id} value={project.id}>
                  {project.name}
                </option>
              ))}
            </select>
            <select
              value={form.role_key}
              onChange={(event) => setForm({ ...form, role_key: event.target.value })}
              className="w-full rounded-lg border border-border bg-surface px-3 py-2 text-sm"
            >
              {(selectedProject?.roles || []).map((role) => (
                <option key={role.key} value={role.key}>
                  {role.label}
                </option>
              ))}
            </select>
            <textarea
              value={form.justification}
              onChange={(event) => setForm({ ...form, justification: event.target.value })}
              placeholder="Why do you need this access?"
              className="min-h-28 w-full rounded-lg border border-border bg-surface px-3 py-2 text-sm"
            />
            <input
              type="number"
              min="0"
              value={form.duration_days}
              onChange={(event) => setForm({ ...form, duration_days: event.target.value })}
              className="w-full rounded-lg border border-border bg-surface px-3 py-2 text-sm"
              placeholder="Duration in days"
            />
<SubmitButton
              isPending={submitting}
              pendingLabel="Submitting…"
              disabled={!form.project_id || !form.role_key || !form.justification.trim()}
              className="w-full"
              label={form.project_id ? "Submit My Request" : "Loading services…"}
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
                title="No requests submitted yet"
                description="Submit a request above and you'll see your status timeline here."
              />
            ) : (
              requests.map((request) => {
                const statusVariant: "outline" | "secondary" | "destructive" =
                  request.status === "pending"
                    ? "outline"
                    : request.status === "rejected"
                      ? "destructive"
                      : "secondary";
                return (
                  <div key={request.id} className="rounded-xl border border-border bg-surfaceHover p-4">
                    <div className="flex items-start justify-between gap-3">
                      <div>
                        <p className="font-semibold text-foreground">
                          {projectNameById.get(request.project_id) || request.project_id} • {request.role_key}
                        </p>
                        <p className="mt-1 text-sm text-muted">{request.justification}</p>
                      </div>
                      <Badge variant={statusVariant}>{request.status}</Badge>
                    </div>
                    <div className="mt-3 flex flex-wrap gap-2">
                      <Badge variant="secondary">{new Date(request.created_at).toLocaleString()}</Badge>
                      {request.duration_days ? <Badge variant="outline">{request.duration_days} day grant</Badge> : <Badge variant="outline">Permanent request</Badge>}
                    </div>
                    {request.status !== "pending" && (request.reviewer_id || request.review_note) && (
                      <div className="mt-3 rounded-lg border border-border bg-background/40 p-3 text-xs text-muted">
                        <p>
                          <span className="font-semibold text-foreground">Reviewed{request.reviewer_id ? ` by ${request.reviewer_id}` : ""}</span>
                          {request.review_note ? `: ${request.review_note}` : ""}
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

