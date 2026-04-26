"use client";

import { useCallback, useEffect, useState } from "react";

import { Badge } from "@/components/ui/Badge";
import { Card, CardHeader, CardTitle } from "@/components/ui/Card";
import { ConfirmModal } from "@/components/ui/ConfirmModal";
import { EmptyState } from "@/components/ui/EmptyState";
import { SubmitButton } from "@/components/ui/SubmitButton";
import { toastError, toastSuccess } from "@/lib/toast";

interface CatalogResponse {
  users: Array<{ id: string; name: string }>;
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

export default function AdminRequestsView() {
  const [catalog, setCatalog] = useState<CatalogResponse>({ users: [], projects: [] });
  const [requests, setRequests] = useState<AccessRequest[]>([]);
  const [statusFilter, setStatusFilter] = useState("pending");
  const [creating, setCreating] = useState(false);
  const [resolvingId, setResolvingId] = useState<string>("");
  const [pendingRejectId, setPendingRejectId] = useState<string>("");
  // Defaults populate from live catalog after loadCatalog. Empty initial
  // values avoid leaking demo identifiers into production HTML and the 400
  // that would result from submitting before the live catalog returns.
  const [form, setForm] = useState({
    requester_id: "",
    project_id: "",
    role_key: "",
    justification: "",
    duration_days: "14",
  });

  async function loadCatalog() {
    const res = await fetch("/api/proxy/catalog");
    const data = await res.json();
    const users = Array.isArray(data?.users) ? data.users : [];
    const projects = Array.isArray(data?.projects) ? data.projects : [];
    setCatalog({ users, projects });
    if (users.length > 0 && projects.length > 0) {
      setForm((current) => {
        if (current.requester_id || current.project_id) return current;
        return {
          ...current,
          requester_id: users[0].id,
          project_id: projects[0].id,
          role_key: projects[0].roles?.[0]?.key ?? "",
        };
      });
    }
  }

  const loadRequests = useCallback(async (filter = statusFilter) => {
    const query = filter === "all" ? "" : `?status=${filter}`;
    const res = await fetch(`/api/proxy/requests${query}`);
    const data = await res.json();
    setRequests(Array.isArray(data) ? data : []);
  }, [statusFilter]);

  useEffect(() => {
    async function initialize() {
      await Promise.all([loadCatalog(), loadRequests("pending")]);
    }

    initialize();
  }, [loadRequests]);

  async function submitRequest(event: React.FormEvent) {
    event.preventDefault();
    setCreating(true);
    try {
      const durationDays = Number.parseInt(form.duration_days, 10);
      const res = await fetch("/api/proxy/requests", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          requester_id: form.requester_id,
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
      loadRequests(statusFilter);
      toastSuccess("Request submitted");
    } catch (err) {
      toastError(err instanceof Error ? err.message : "Failed to submit request.");
    } finally {
      setCreating(false);
    }
  }

  async function resolveRequest(id: string, status: "approved" | "rejected") {
    setResolvingId(id);
    try {
      // reviewer_id is intentionally omitted: the backend resolves the actor
      // from the authenticated principal (Zitadel JWT) or the proxy injects
      // the demo session id in local-dev. See backend resolveActor.
      const res = await fetch(`/api/proxy/requests/${id}/decision`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          status,
          review_note: status === "approved" ? "Approved through MkAuth request queue." : "Rejected during governance review.",
        }),
      });
      if (!res.ok) {
        const body = await res.json().catch(() => ({}));
        throw new Error(body.message || `Failed to mark request as ${status}.`);
      }
      loadRequests(statusFilter);
      toastSuccess(`Request ${status}`);
    } catch (err) {
      toastError(err instanceof Error ? err.message : `Failed to mark request as ${status}.`);
    } finally {
      setResolvingId("");
    }
  }

  const selectedProject = catalog.projects.find((project) => project.id === form.project_id);

  return (
    <div className="space-y-6 animate-fade-in-up">
      <header>
        <h1 className="text-3xl font-bold text-foreground">Access Requests</h1>
        <p className="text-muted mt-2">Review the self-service queue, create requests on behalf of members, and approve or reject pending access.</p>
      </header>

      <div className="grid grid-cols-1 xl:grid-cols-[0.9fr,1.1fr] gap-6">
        <Card>
          <CardHeader>
            <CardTitle>Create Request</CardTitle>
          </CardHeader>
          <form onSubmit={submitRequest} className="space-y-3">
            <select
              value={form.requester_id}
              onChange={(event) => setForm({ ...form, requester_id: event.target.value })}
              className="w-full rounded-lg border border-border bg-surface px-3 py-2 text-sm"
            >
              {catalog.users.map((user) => (
                <option key={user.id} value={user.id}>
                  {user.name}
                </option>
              ))}
            </select>
            <select
              value={form.project_id}
              onChange={(event) => {
                const projectId = event.target.value;
                const project = catalog.projects.find((entry) => entry.id === projectId);
                setForm({
                  ...form,
                  project_id: projectId,
                  role_key: project?.roles[0]?.key || "",
                });
              }}
              className="w-full rounded-lg border border-border bg-surface px-3 py-2 text-sm"
            >
              {catalog.projects.map((project) => (
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
              placeholder="Why does this user need access?"
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
              isPending={creating}
              pendingLabel="Submitting…"
              disabled={!form.requester_id || !form.project_id || !form.role_key || !form.justification.trim()}
              className="w-full"
              label={form.requester_id ? "Create Access Request" : "Loading directory…"}
            />
          </form>
        </Card>

        <Card>
          <CardHeader>
            <div className="flex items-center justify-between gap-3">
              <CardTitle>Approval Queue</CardTitle>
              <select
                value={statusFilter}
                onChange={(event) => {
                  const next = event.target.value;
                  setStatusFilter(next);
                  loadRequests(next);
                }}
                className="rounded-lg border border-border bg-surface px-3 py-2 text-sm"
              >
                <option value="pending">Pending</option>
                <option value="approved">Approved</option>
                <option value="rejected">Rejected</option>
                <option value="all">All</option>
              </select>
            </div>
          </CardHeader>

          <div className="space-y-3">
            {requests.map((request) => (
              <div key={request.id} className="rounded-xl border border-border bg-surfaceHover p-4">
                <div className="flex items-start justify-between gap-3">
                  <div>
                    <p className="font-semibold text-foreground">
                      {request.requester_id} {"->"} {request.project_id}:{request.role_key}
                    </p>
                    <p className="mt-1 text-sm text-muted">{request.justification}</p>
                  </div>
                  <Badge variant={request.status === "pending" ? "outline" : "secondary"}>{request.status}</Badge>
                </div>
                <div className="mt-3 flex flex-wrap gap-2">
                  <Badge variant="secondary">{new Date(request.created_at).toLocaleString()}</Badge>
                  {request.duration_days ? <Badge variant="outline">{request.duration_days} day grant</Badge> : <Badge variant="outline">Permanent request</Badge>}
                </div>
                {request.status === "pending" && (
                  <div className="mt-4 flex gap-3">
                    <button
                      onClick={() => resolveRequest(request.id, "approved")}
                      disabled={resolvingId === request.id}
                      className="rounded-lg bg-emerald-500 px-3 py-2 text-xs font-semibold uppercase tracking-[0.16em] text-white disabled:opacity-50 disabled:cursor-not-allowed"
                    >
                      {resolvingId === request.id ? "Working…" : "Approve"}
                    </button>
                    <button
                      onClick={() => setPendingRejectId(request.id)}
                      disabled={resolvingId === request.id}
                      className="rounded-lg bg-red-500/10 px-3 py-2 text-xs font-semibold uppercase tracking-[0.16em] text-red-500 disabled:opacity-50 disabled:cursor-not-allowed"
                    >
                      {resolvingId === request.id ? "Working…" : "Reject"}
                    </button>
                  </div>
                )}
              </div>
            ))}
            {requests.length === 0 && (
              <EmptyState
                title="No requests in this filter"
                description={
                  statusFilter === "pending"
                    ? "When members request access, their submissions show up here for review."
                    : `No requests with status "${statusFilter}".`
                }
              />
            )}
          </div>
        </Card>
      </div>

      <ConfirmModal
        open={Boolean(pendingRejectId)}
        title="Reject this access request?"
        description="The requester will not get the role and the rejection will be recorded in the audit log. They can submit a new request."
        confirmLabel="Reject"
        variant="destructive"
        isPending={Boolean(resolvingId)}
        onCancel={() => setPendingRejectId("")}
        onConfirm={async () => {
          const id = pendingRejectId;
          setPendingRejectId("");
          await resolveRequest(id, "rejected");
        }}
      />
    </div>
  );
}
