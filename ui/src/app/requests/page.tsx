"use client";

import { useEffect, useState } from "react";

import { Badge } from "@/components/ui/Badge";
import { Card, CardHeader, CardTitle } from "@/components/ui/Card";

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

export default function RequestsPage() {
  const [catalog, setCatalog] = useState<CatalogResponse>({ users: [], projects: [] });
  const [requests, setRequests] = useState<AccessRequest[]>([]);
  const [statusFilter, setStatusFilter] = useState("pending");
  const [message, setMessage] = useState("");
  const [form, setForm] = useState({
    requester_id: "ava_guest",
    project_id: "laser",
    role_key: "trainee",
    justification: "Needs time-bound access for a supervised residency task.",
    duration_days: "14",
  });

  async function loadCatalog() {
    const res = await fetch("/api/proxy/catalog");
    const data = await res.json();
    setCatalog({
      users: Array.isArray(data?.users) ? data.users : [],
      projects: Array.isArray(data?.projects) ? data.projects : [],
    });
  }

  async function loadRequests(filter = statusFilter) {
    const query = filter === "all" ? "" : `?status=${filter}`;
    const res = await fetch(`/api/proxy/requests${query}`);
    const data = await res.json();
    setRequests(Array.isArray(data) ? data : []);
  }

  useEffect(() => {
    loadCatalog();
    loadRequests("pending");
  }, []);

  async function submitRequest(event: React.FormEvent) {
    event.preventDefault();
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

    if (res.ok) {
      setMessage("Request submitted.");
      loadRequests(statusFilter);
      return;
    }
    const body = await res.json().catch(() => ({}));
    setMessage(body.message || "Failed to submit request.");
  }

  async function resolveRequest(id: string, status: "approved" | "rejected") {
    const res = await fetch(`/api/proxy/requests/${id}/decision`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        status,
        reviewer_id: "alice.rivera",
        review_note: status === "approved" ? "Approved through MkAuth request queue." : "Rejected during governance review.",
      }),
    });

    if (res.ok) {
      setMessage(`Request ${status}.`);
      loadRequests(statusFilter);
      return;
    }
    const body = await res.json().catch(() => ({}));
    setMessage(body.message || `Failed to mark request as ${status}.`);
  }

  const selectedProject = catalog.projects.find((project) => project.id === form.project_id);

  return (
    <div className="space-y-6 animate-fade-in-up">
      <header>
        <h1 className="text-3xl font-bold text-foreground">Access Requests</h1>
        <p className="text-muted mt-2">Test the self-service request queue, approve requests, and watch approved access turn into direct grants.</p>
      </header>

      <div className="grid grid-cols-1 xl:grid-cols-[0.9fr,1.1fr] gap-6">
        <Card>
          <CardHeader>
            <CardTitle>Submit Request</CardTitle>
          </CardHeader>
          {message && <p className="text-sm text-primary">{message}</p>}
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
            <button type="submit" className="w-full rounded-lg bg-primary px-4 py-2 text-sm font-medium text-white">
              Create Access Request
            </button>
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
                      className="rounded-lg bg-primary px-3 py-2 text-xs font-semibold uppercase tracking-[0.16em] text-white"
                    >
                      Approve
                    </button>
                    <button
                      onClick={() => resolveRequest(request.id, "rejected")}
                      className="rounded-lg bg-red-500/10 px-3 py-2 text-xs font-semibold uppercase tracking-[0.16em] text-red-500"
                    >
                      Reject
                    </button>
                  </div>
                )}
              </div>
            ))}
            {requests.length === 0 && <p className="text-sm text-muted">No requests in this filter right now.</p>}
          </div>
        </Card>
      </div>
    </div>
  );
}
