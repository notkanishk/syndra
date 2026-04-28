"use client";

import { useEffect, useMemo, useState } from "react";

import { ProjectName, RoleName, UserName } from "@/components/names";
import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Card, CardHeader, CardTitle } from "@/components/ui/Card";
import { ConfirmModal } from "@/components/ui/ConfirmModal";
import { EmptyState } from "@/components/ui/EmptyState";
import { Eyebrow } from "@/components/ui/Eyebrow";
import { Input } from "@/components/ui/Input";
import { Pulse } from "@/components/ui/Pulse";
import { Select } from "@/components/ui/Select";
import { SkeletonCardList } from "@/components/ui/Skeleton";
import { SubmitButton } from "@/components/ui/SubmitButton";
import { useQuery } from "@tanstack/react-query";
import { request } from "@/lib/api-client";
import {
  useCreateRequest,
  useDecideRequest,
  useRequestsAdmin,
  type AccessRequest,
} from "@/lib/queries/useRequests";
import { toastError, toastSuccess } from "@/lib/toast";

interface CatalogResponse {
  users: Array<{ id: string; name: string }>;
  projects: Array<{ id: string; name: string; roles: Array<{ key: string; label: string }> }>;
}

function useCatalog() {
  return useQuery({
    queryKey: ["catalog"],
    queryFn: async (): Promise<CatalogResponse> => {
      const data = await request<{ users?: CatalogResponse["users"]; projects?: CatalogResponse["projects"] }>(
        "/catalog",
      );
      return {
        users: Array.isArray(data?.users) ? data.users : [],
        projects: Array.isArray(data?.projects) ? data.projects : [],
      };
    },
  });
}

const STALE_THRESHOLD_MS = 24 * 60 * 60 * 1000;

function isStale(request: AccessRequest, now: number): boolean {
  if (request.status !== "pending") return false;
  const created = new Date(request.created_at).getTime();
  if (Number.isNaN(created)) return false;
  return now - created >= STALE_THRESHOLD_MS;
}

export default function AdminRequestsView() {
  const catalogQuery = useCatalog();
  const catalog = useMemo(
    () => catalogQuery.data ?? { users: [], projects: [] },
    [catalogQuery.data],
  );

  const [statusFilter, setStatusFilter] = useState<string>("pending");
  const requestsQuery = useRequestsAdmin(statusFilter);
  const requests = useMemo(() => requestsQuery.data ?? [], [requestsQuery.data]);
  const createRequest = useCreateRequest();
  const decideRequest = useDecideRequest();

  const [pendingApproveId, setPendingApproveId] = useState<string>("");
  const [pendingRejectId, setPendingRejectId] = useState<string>("");

  // The new-request form defaults populate from the live catalog so we never
  // serialize a stale demo identifier into production HTML.
  const [form, setForm] = useState({
    requester_id: "",
    project_id: "",
    role_key: "",
    justification: "",
    duration_days: "14",
  });
  useEffect(() => {
    if (form.requester_id || catalog.users.length === 0 || catalog.projects.length === 0) return;
    setForm((current) => ({
      ...current,
      requester_id: catalog.users[0].id,
      project_id: catalog.projects[0].id,
      role_key: catalog.projects[0].roles?.[0]?.key ?? "",
    }));
  }, [catalog, form.requester_id]);

  const selectedProject = catalog.projects.find((project) => project.id === form.project_id);

  const now = Date.now();

  async function submitRequest(event: React.FormEvent) {
    event.preventDefault();
    try {
      const durationDays = Number.parseInt(form.duration_days, 10);
      await createRequest.mutateAsync({
        requester_id: form.requester_id,
        project_id: form.project_id,
        role_key: form.role_key,
        justification: form.justification,
        duration_days: Number.isNaN(durationDays) ? 0 : durationDays,
      });
      toastSuccess("Request submitted");
    } catch (err) {
      toastError(err instanceof Error ? err.message : "Failed to submit request.");
    }
  }

  async function decide(id: string, status: "approved" | "rejected") {
    try {
      await decideRequest.mutateAsync({
        id,
        status,
        review_note:
          status === "approved"
            ? "Approved through MkAuth request queue."
            : "Rejected during governance review.",
      });
      toastSuccess(`Request ${status}`);
    } catch (err) {
      toastError(err instanceof Error ? err.message : `Failed to mark request as ${status}.`);
    }
  }

  return (
    <div className="space-y-6 animate-fade-in-up relative z-10">
      <header>
        <Eyebrow>Access requests</Eyebrow>
        <h1 className="text-3xl font-semibold text-on-surface mt-1 font-display">
          Approval queue
        </h1>
        <p className="text-on-surface-variant mt-2">
          Review the self-service queue, create requests on behalf of members,
          and approve or reject pending access. Requests pending more than 24
          hours pulse to flag review SLAs.
        </p>
      </header>

      <div className="grid grid-cols-1 xl:grid-cols-[0.9fr,1.1fr] gap-6 items-start">
        <Card>
          <CardHeader>
            <CardTitle>Create request</CardTitle>
          </CardHeader>
          <form onSubmit={submitRequest} className="space-y-3">
            <div>
              <label className="block text-xs font-medium text-on-surface-variant mb-1">
                Requester
              </label>
              <Select
                value={form.requester_id}
                onChange={(event) => setForm({ ...form, requester_id: event.target.value })}
              >
                {catalog.users.map((user) => (
                  <option key={user.id} value={user.id}>
                    {user.name}
                  </option>
                ))}
              </Select>
            </div>
            <div>
              <label className="block text-xs font-medium text-on-surface-variant mb-1">
                Project
              </label>
              <Select
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
              >
                {catalog.projects.map((project) => (
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
                placeholder="Why does this user need access?"
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
                !form.requester_id ||
                !form.project_id ||
                !form.role_key ||
                !form.justification.trim()
              }
              className="w-full"
              label={form.requester_id ? "Create access request" : "Loading directory…"}
            />
          </form>
        </Card>

        <Card>
          <CardHeader>
            <div className="flex items-center justify-between gap-3">
              <CardTitle>Approval queue</CardTitle>
              <Select
                value={statusFilter}
                onChange={(event) => setStatusFilter(event.target.value)}
                className="max-w-[10rem]"
              >
                <option value="pending">Pending</option>
                <option value="approved">Approved</option>
                <option value="rejected">Rejected</option>
                <option value="all">All</option>
              </Select>
            </div>
          </CardHeader>

          <div className="space-y-3">
            {requestsQuery.isLoading ? (
              <SkeletonCardList count={3} />
            ) : requests.length === 0 ? (
              <EmptyState
                eyebrow={statusFilter === "pending" ? "Inbox zero" : "Empty filter"}
                title="No requests in this filter"
                description={
                  statusFilter === "pending"
                    ? "When members request access, their submissions show up here for review."
                    : `No requests with status "${statusFilter}".`
                }
              />
            ) : (
              requests.map((entry) => {
                const stale = isStale(entry, now);
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
                      : stale
                        ? "warn"
                        : "info";
                return (
                  <div
                    key={entry.id}
                    className="rounded-card border border-outline-variant bg-surface-container-low p-4 transition-colors hover:border-primary-container/50"
                  >
                    <div className="flex items-start justify-between gap-3">
                      <div className="min-w-0">
                        <div className="flex items-center gap-2 text-sm">
                          <Pulse variant={pulseVariant} static={!stale && entry.status !== "pending"} />
                          <p className="font-semibold text-on-surface truncate">
                            <UserName id={entry.requester_id} /> →{" "}
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
                      {stale && (
                        <Badge variant="outline" className="border-[var(--warning)] text-[var(--warning)]">
                          Pending &gt;24h
                        </Badge>
                      )}
                    </div>
                    {entry.status === "pending" && (
                      <div className="mt-4 flex gap-3">
                        <Button
                          type="button"
                          variant="success"
                          size="sm"
                          onClick={() => setPendingApproveId(entry.id)}
                        >
                          Approve
                        </Button>
                        <Button
                          type="button"
                          variant="destructive"
                          size="sm"
                          onClick={() => setPendingRejectId(entry.id)}
                        >
                          Reject
                        </Button>
                      </div>
                    )}
                    {entry.status !== "pending" && entry.reviewer_id ? (
                      <p className="mt-3 text-xs text-on-surface-variant">
                        Reviewed by <UserName id={entry.reviewer_id} />
                        {entry.review_note ? ` — ${entry.review_note}` : ""}
                      </p>
                    ) : null}
                  </div>
                );
              })
            )}
          </div>
        </Card>
      </div>

      <ConfirmModal
        open={Boolean(pendingApproveId)}
        title="Approve this access request?"
        description="The grant will be created immediately and the approval recorded in the audit log."
        confirmLabel="Approve"
        isPending={decideRequest.isPending}
        onCancel={() => setPendingApproveId("")}
        onConfirm={async () => {
          const id = pendingApproveId;
          setPendingApproveId("");
          await decide(id, "approved");
        }}
      />

      <ConfirmModal
        open={Boolean(pendingRejectId)}
        title="Reject this access request?"
        description="The requester will not get the role and the rejection will be recorded in the audit log. They can submit a new request."
        confirmLabel="Reject"
        variant="destructive"
        isPending={decideRequest.isPending}
        onCancel={() => setPendingRejectId("")}
        onConfirm={async () => {
          const id = pendingRejectId;
          setPendingRejectId("");
          await decide(id, "rejected");
        }}
      />
    </div>
  );
}
