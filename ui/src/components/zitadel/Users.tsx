"use client";

import Link from "next/link";
import { useCallback, useEffect, useMemo, useState } from "react";

import { Badge } from "@/components/ui/Badge";
import { Card, CardHeader, CardTitle } from "@/components/ui/Card";
import { ConfirmModal } from "@/components/ui/ConfirmModal";
import { request } from "@/lib/api-client";
import { usePendingPropagations } from "@/lib/queries/usePropagation";
import type { Paginated, UserGrant, ZitadelProject, ZitadelUser } from "@/components/zitadel/types";

// grantFlash maps the outbox enqueue response status to operator copy. Grant
// mutations now flow through the durable outbox (B4/D3): "applied" means the
// inline drain reached Zitadel, otherwise it's queued for an explicit resume.
function grantFlash(status?: string): string {
  return status === "applied"
    ? "Grant applied"
    : "Queued — resume from the dashboard to send to Zitadel";
}

// --- Section 3: Users & Grants ---
export default function Users() {
  const [users, setUsers] = useState<ZitadelUser[]>([]);
  const [projects, setProjects] = useState<ZitadelProject[]>([]);
  const [total, setTotal] = useState(0);
  const [filter, setFilter] = useState("");
  const [selectedId, setSelectedId] = useState("");
  const [grants, setGrants] = useState<UserGrant[]>([]);
  const [grantsTotal, setGrantsTotal] = useState(0);
  const [loadingGrants, setLoadingGrants] = useState(false);
  const [flash, setFlash] = useState<{ kind: "ok" | "err"; msg: string } | null>(null);
  const [newGrant, setNewGrant] = useState({ projectId: "", roleKeys: "" });
  const [editing, setEditing] = useState<string>("");
  const [editDraft, setEditDraft] = useState("");
  const [pendingRevoke, setPendingRevoke] = useState<{ id: string; label: string } | null>(null);
  const [revoking, setRevoking] = useState(false);

  const loadDirectory = useCallback(async () => {
    try {
      const [u, p] = await Promise.all([
        request<Paginated<ZitadelUser>>("zitadel/users?limit=500", { cache: "no-store" }),
        request<Paginated<ZitadelProject>>("zitadel/projects?limit=500", { cache: "no-store" }),
      ]);
      setUsers(u.items ?? []);
      setTotal(u.total ?? 0);
      setProjects(p.items ?? []);
    } catch (err) {
      setFlash({ kind: "err", msg: err instanceof Error ? err.message : String(err) });
    }
  }, []);

  const loadGrants = useCallback(async (userId: string) => {
    if (!userId) return;
    setLoadingGrants(true);
    try {
      const res = await request<Paginated<UserGrant>>(`zitadel/users/${userId}/grants?limit=500`, { cache: "no-store" });
      setGrants(res.items ?? []);
      setGrantsTotal(res.total ?? 0);
    } catch (err) {
      setFlash({ kind: "err", msg: err instanceof Error ? err.message : String(err) });
    } finally {
      setLoadingGrants(false);
    }
  }, []);

  useEffect(() => { void loadDirectory(); }, [loadDirectory]);
  useEffect(() => { if (selectedId) void loadGrants(selectedId); }, [selectedId, loadGrants]);
  useEffect(() => {
    if (!flash) return;
    const t = setTimeout(() => setFlash(null), 3000);
    return () => clearTimeout(t);
  }, [flash]);

  const filteredUsers = useMemo(() => {
    const q = filter.trim().toLowerCase();
    if (!q) return users;
    return users.filter((u) =>
      u.userName?.toLowerCase().includes(q) ||
      u.email?.toLowerCase().includes(q) ||
      u.displayName?.toLowerCase().includes(q) ||
      u.id?.toLowerCase().includes(q)
    );
  }, [users, filter]);

  const projectName = useCallback((id: string) => {
    return projects.find((p) => p.id === id)?.name ?? id;
  }, [projects]);

  // §5.2: flag grants whose (user, project, role) is a pending outbox *add*
  // still awaiting a drain to Zitadel. Keyed on the selected user since these
  // grants are all scoped to it.
  const { data: pending } = usePendingPropagations();
  const pendingAdds = useMemo(() => {
    const s = new Set<string>();
    for (const r of pending ?? []) {
      if (r.op_type !== "add") continue;
      for (const rk of r.role_keys) s.add(`${r.user_id}|${r.project_id}|${rk}`);
    }
    return s;
  }, [pending]);

  const parseRoleKeys = (raw: string) =>
    raw.split(",").map((s) => s.trim()).filter(Boolean);

  const onAssign = async () => {
    const roleKeys = parseRoleKeys(newGrant.roleKeys);
    if (!selectedId || !newGrant.projectId || roleKeys.length === 0) return;
    try {
      const res = await request<{ status?: string }>(`zitadel/users/${selectedId}/grants`, {
        method: "POST",
        body: { projectId: newGrant.projectId, roleKeys },
      });
      setNewGrant({ projectId: "", roleKeys: "" });
      setFlash({ kind: "ok", msg: grantFlash(res?.status) });
      await loadGrants(selectedId);
    } catch (err) {
      setFlash({ kind: "err", msg: err instanceof Error ? err.message : String(err) });
    }
  };

  const onUpdate = async (grantId: string) => {
    const roleKeys = parseRoleKeys(editDraft);
    if (roleKeys.length === 0) return;
    try {
      const res = await request<{ status?: string }>(`zitadel/users/${selectedId}/grants/${grantId}`, {
        method: "PUT",
        body: { roleKeys },
      });
      setEditing("");
      setFlash({ kind: "ok", msg: grantFlash(res?.status) });
      await loadGrants(selectedId);
    } catch (err) {
      setFlash({ kind: "err", msg: err instanceof Error ? err.message : String(err) });
    }
  };

  const onRevoke = (grantId: string, label: string) => {
    setPendingRevoke({ id: grantId, label });
  };

  const onRevokeConfirm = async () => {
    if (!pendingRevoke) return;
    const { id } = pendingRevoke;
    setRevoking(true);
    try {
      const res = await request<{ status?: string }>(`zitadel/users/${selectedId}/grants/${id}`, { method: "DELETE" });
      setFlash({ kind: "ok", msg: grantFlash(res?.status) });
      setPendingRevoke(null);
      await loadGrants(selectedId);
    } catch (err) {
      setFlash({ kind: "err", msg: err instanceof Error ? err.message : String(err) });
    } finally {
      setRevoking(false);
    }
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle>Users &amp; Grants</CardTitle>
      </CardHeader>
      <div className="flex items-center gap-3 flex-wrap">
        <label className="text-xs uppercase tracking-[0.22em] text-on-surface-variant">User</label>
        <input
          value={filter}
          onChange={(e) => setFilter(e.target.value)}
          placeholder="filter by email, name, id"
          className="w-56 rounded-lg border border-outline-variant bg-surface px-3 py-2 text-sm"
        />
        <select
          value={selectedId}
          onChange={(e) => setSelectedId(e.target.value)}
          className="rounded-lg border border-outline-variant bg-surface px-3 py-2 text-sm text-on-surface min-w-[20rem]"
        >
          <option value="">— select a user —</option>
          {filteredUsers.map((u) => (
            <option key={u.id} value={u.id}>
              {u.displayName || u.userName} · {u.email}
            </option>
          ))}
        </select>
        <span className="text-xs text-on-surface-variant">{total} total{filter ? ` · ${filteredUsers.length} match` : ""}</span>
        <button
          onClick={() => void loadDirectory()}
          className="rounded-lg border border-outline-variant px-3 py-2 text-xs text-on-surface-variant hover:text-on-surface"
        >
          Refresh
        </button>
      </div>

      {selectedId && (
        <div className="mt-4 space-y-3">
          <div className="flex items-center gap-2">
            <p className="text-xs uppercase tracking-[0.22em] text-on-surface-variant">Grants</p>
            <span className="text-xs text-on-surface-variant">{grantsTotal} total</span>
          </div>

          {loadingGrants ? (
            <p className="text-sm text-on-surface-variant">Loading grants...</p>
          ) : grants.length === 0 ? (
            <p className="text-sm text-on-surface-variant">No grants for this user.</p>
          ) : (
            <div className="space-y-2">
              {grants.map((g) => (
                <div key={g.id} className="rounded-lg border border-outline-variant bg-surface-container-high px-3 py-2 text-sm flex items-center gap-3 flex-wrap">
                  <code className="text-on-surface font-mono">{projectName(g.projectId)}</code>
                  {editing === g.id ? (
                    <>
                      <input
                        value={editDraft}
                        onChange={(e) => setEditDraft(e.target.value)}
                        placeholder="comma-separated role keys"
                        className="flex-1 min-w-[16rem] rounded-lg border border-outline-variant bg-surface px-2 py-1 text-sm"
                      />
                      <button onClick={() => onUpdate(g.id)} className="rounded-lg bg-primary px-3 py-1 text-xs text-on-primary">Save</button>
                      <button onClick={() => setEditing("")} className="rounded-lg border border-outline-variant px-3 py-1 text-xs text-on-surface-variant">Cancel</button>
                    </>
                  ) : (
                    <>
                      <div className="flex flex-wrap gap-1">
                        {g.roleKeys.map((rk) => (
                          <Badge key={rk} variant="outline">{rk}</Badge>
                        ))}
                      </div>
                      {g.roleKeys.some((rk) => pendingAdds.has(`${selectedId}|${g.projectId}|${rk}`)) && (
                        <span className="inline-flex items-center gap-1 rounded px-2 py-0.5 text-xs bg-tertiary-container text-on-tertiary-container">
                          ⏱ Awaiting Zitadel
                          <Link href="/" className="underline">Resume</Link>
                        </span>
                      )}
                      <span className="flex-1" />
                      <button
                        onClick={() => { setEditing(g.id); setEditDraft(g.roleKeys.join(", ")); }}
                        className="rounded-lg border border-outline-variant px-3 py-1 text-xs text-on-surface-variant hover:text-on-surface"
                      >
                        Edit
                      </button>
                      <button
                        onClick={() => onRevoke(g.id, `${projectName(g.projectId)} / ${g.roleKeys.join(", ")}`)}
                        className="rounded-lg border border-error/40 px-3 py-1 text-xs text-error hover:bg-error-container/40"
                      >
                        Revoke
                      </button>
                    </>
                  )}
                </div>
              ))}
            </div>
          )}

          <div className="rounded-lg border border-dashed border-outline-variant px-3 py-2 text-sm flex items-center gap-2 flex-wrap">
            <span className="text-xs uppercase tracking-[0.22em] text-on-surface-variant">Assign grant</span>
            <select
              value={newGrant.projectId}
              onChange={(e) => setNewGrant({ ...newGrant, projectId: e.target.value })}
              className="rounded-lg border border-outline-variant bg-surface px-2 py-1 text-sm min-w-[14rem]"
            >
              <option value="">— project —</option>
              {projects.map((p) => (
                <option key={p.id} value={p.id}>{p.name}</option>
              ))}
            </select>
            <input
              value={newGrant.roleKeys}
              onChange={(e) => setNewGrant({ ...newGrant, roleKeys: e.target.value })}
              placeholder="role1, role2"
              className="flex-1 min-w-[14rem] rounded-lg border border-outline-variant bg-surface px-2 py-1 text-sm"
            />
            <button
              onClick={onAssign}
              disabled={!newGrant.projectId || !newGrant.roleKeys.trim()}
              className="rounded-lg bg-primary px-3 py-1 text-xs text-on-primary disabled:opacity-50"
            >
              Assign
            </button>
          </div>
        </div>
      )}

      {flash && (
        <p className={`mt-3 text-sm ${flash.kind === "ok" ? "text-success" : "text-error"}`}>
          {flash.msg}
        </p>
      )}

      <ConfirmModal
        open={Boolean(pendingRevoke)}
        title="Revoke this grant?"
        description={`This removes ${pendingRevoke?.label ?? "the user's"} access immediately. The user will lose any roles tied to this grant.`}
        confirmLabel="Revoke"
        variant="destructive"
        isPending={revoking}
        onCancel={() => setPendingRevoke(null)}
        onConfirm={onRevokeConfirm}
      />
    </Card>
  );
}
