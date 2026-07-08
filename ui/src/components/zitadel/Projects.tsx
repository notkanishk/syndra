"use client";

import { useCallback, useEffect, useState } from "react";

import { Badge } from "@/components/ui/Badge";
import { Card, CardHeader, CardTitle } from "@/components/ui/Card";
import { ConfirmModal } from "@/components/ui/ConfirmModal";
import { request } from "@/lib/api-client";
import type { Paginated, ProjectRole, ZitadelProject } from "@/components/zitadel/types";

// --- Section 2: Projects & Roles ---
export default function Projects() {
  const [projects, setProjects] = useState<ZitadelProject[]>([]);
  const [total, setTotal] = useState(0);
  const [selectedId, setSelectedId] = useState("");
  const [roles, setRoles] = useState<ProjectRole[]>([]);
  const [rolesTotal, setRolesTotal] = useState(0);
  const [loadingRoles, setLoadingRoles] = useState(false);
  const [flash, setFlash] = useState<{ kind: "ok" | "err"; msg: string } | null>(null);
  const [newRole, setNewRole] = useState({ key: "", displayName: "", group: "" });
  const [editing, setEditing] = useState<string>("");
  const [editDraft, setEditDraft] = useState({ displayName: "", group: "" });
  const [pendingDeleteKey, setPendingDeleteKey] = useState<string>("");
  const [deleting, setDeleting] = useState(false);

  const loadProjects = useCallback(async () => {
    try {
      const res = await request<Paginated<ZitadelProject>>("zitadel/projects?limit=500", { cache: "no-store" });
      setProjects(res.items ?? []);
      setTotal(res.total ?? 0);
    } catch (err) {
      setFlash({ kind: "err", msg: err instanceof Error ? err.message : String(err) });
    }
  }, []);

  const loadRoles = useCallback(async (projectId: string) => {
    if (!projectId) return;
    setLoadingRoles(true);
    try {
      const res = await request<Paginated<ProjectRole>>(`zitadel/projects/${projectId}/roles?limit=500`, { cache: "no-store" });
      setRoles(res.items ?? []);
      setRolesTotal(res.total ?? 0);
    } catch (err) {
      setFlash({ kind: "err", msg: err instanceof Error ? err.message : String(err) });
    } finally {
      setLoadingRoles(false);
    }
  }, []);

  useEffect(() => { void loadProjects(); }, [loadProjects]);
  useEffect(() => { if (selectedId) void loadRoles(selectedId); }, [selectedId, loadRoles]);

  // Auto-clear flash after 3s.
  useEffect(() => {
    if (!flash) return;
    const t = setTimeout(() => setFlash(null), 3000);
    return () => clearTimeout(t);
  }, [flash]);

  const onCreate = async () => {
    if (!selectedId || !newRole.key.trim()) return;
    try {
      await request("zitadel/projects/" + selectedId + "/roles", { method: "POST", body: newRole });
      setNewRole({ key: "", displayName: "", group: "" });
      setFlash({ kind: "ok", msg: `Role ${newRole.key} created` });
      await loadRoles(selectedId);
    } catch (err) {
      setFlash({ kind: "err", msg: err instanceof Error ? err.message : String(err) });
    }
  };

  const onUpdate = async (key: string) => {
    try {
      await request(`zitadel/projects/${selectedId}/roles/${key}`, { method: "PUT", body: editDraft });
      setEditing("");
      setFlash({ kind: "ok", msg: `Role ${key} updated` });
      await loadRoles(selectedId);
    } catch (err) {
      setFlash({ kind: "err", msg: err instanceof Error ? err.message : String(err) });
    }
  };

  const onDelete = (key: string) => {
    setPendingDeleteKey(key);
  };

  const onDeleteConfirm = async () => {
    const key = pendingDeleteKey;
    setDeleting(true);
    try {
      await request(`zitadel/projects/${selectedId}/roles/${key}`, { method: "DELETE" });
      setFlash({ kind: "ok", msg: `Role ${key} deleted` });
      setPendingDeleteKey("");
      await loadRoles(selectedId);
    } catch (err) {
      setFlash({ kind: "err", msg: err instanceof Error ? err.message : String(err) });
    } finally {
      setDeleting(false);
    }
  };

  return (
    <Card>
      <CardHeader>
        <CardTitle>Projects &amp; Roles</CardTitle>
      </CardHeader>
      <div className="flex items-center gap-3 flex-wrap">
        <label className="text-xs uppercase tracking-[0.22em] text-on-surface-variant">Project</label>
        <select
          value={selectedId}
          onChange={(e) => setSelectedId(e.target.value)}
          className="rounded-lg border border-outline-variant bg-surface px-3 py-2 text-sm text-on-surface min-w-[18rem]"
        >
          <option value="">— select a project —</option>
          {projects.map((p) => (
            <option key={p.id} value={p.id}>{p.name} ({p.id})</option>
          ))}
        </select>
        <span className="text-xs text-on-surface-variant">{total} total</span>
        <button
          onClick={() => void loadProjects()}
          className="rounded-lg border border-outline-variant px-3 py-2 text-xs text-on-surface-variant hover:text-on-surface"
        >
          Refresh
        </button>
      </div>

      {selectedId && (
        <div className="mt-4 space-y-3">
          <div className="flex items-center gap-2">
            <p className="text-xs uppercase tracking-[0.22em] text-on-surface-variant">Roles</p>
            <span className="text-xs text-on-surface-variant">{rolesTotal} total</span>
          </div>

          {loadingRoles ? (
            <p className="text-sm text-on-surface-variant">Loading roles...</p>
          ) : roles.length === 0 ? (
            <p className="text-sm text-on-surface-variant">No roles defined in this project.</p>
          ) : (
            <div className="space-y-2">
              {roles.map((r) => (
                <div key={r.key} className="rounded-lg border border-outline-variant bg-surface-container-high px-3 py-2 text-sm flex items-center gap-3 flex-wrap">
                  <code className="text-on-surface font-mono">{r.key}</code>
                  {editing === r.key ? (
                    <>
                      <input
                        value={editDraft.displayName}
                        onChange={(e) => setEditDraft({ ...editDraft, displayName: e.target.value })}
                        placeholder="display name"
                        className="flex-1 min-w-[10rem] rounded-lg border border-outline-variant bg-surface px-2 py-1 text-sm"
                      />
                      <input
                        value={editDraft.group}
                        onChange={(e) => setEditDraft({ ...editDraft, group: e.target.value })}
                        placeholder="group"
                        className="w-32 rounded-lg border border-outline-variant bg-surface px-2 py-1 text-sm"
                      />
                      <button onClick={() => onUpdate(r.key)} className="rounded-lg bg-primary px-3 py-1 text-xs text-on-primary">Save</button>
                      <button onClick={() => setEditing("")} className="rounded-lg border border-outline-variant px-3 py-1 text-xs text-on-surface-variant">Cancel</button>
                    </>
                  ) : (
                    <>
                      <span className="text-on-surface-variant">{r.displayName || "—"}</span>
                      {r.group && <Badge variant="outline">{r.group}</Badge>}
                      <span className="flex-1" />
                      <button
                        onClick={() => { setEditing(r.key); setEditDraft({ displayName: r.displayName, group: r.group }); }}
                        className="rounded-lg border border-outline-variant px-3 py-1 text-xs text-on-surface-variant hover:text-on-surface"
                      >
                        Edit
                      </button>
                      <button
                        onClick={() => onDelete(r.key)}
                        className="rounded-lg border border-error/40 px-3 py-1 text-xs text-error hover:bg-error-container/40"
                      >
                        Delete
                      </button>
                    </>
                  )}
                </div>
              ))}
            </div>
          )}

          <div className="rounded-lg border border-dashed border-outline-variant px-3 py-2 text-sm flex items-center gap-2 flex-wrap">
            <span className="text-xs uppercase tracking-[0.22em] text-on-surface-variant">Add role</span>
            <input
              value={newRole.key}
              onChange={(e) => setNewRole({ ...newRole, key: e.target.value })}
              placeholder="role key (required)"
              className="w-40 rounded-lg border border-outline-variant bg-surface px-2 py-1 text-sm"
            />
            <input
              value={newRole.displayName}
              onChange={(e) => setNewRole({ ...newRole, displayName: e.target.value })}
              placeholder="display name"
              className="w-40 rounded-lg border border-outline-variant bg-surface px-2 py-1 text-sm"
            />
            <input
              value={newRole.group}
              onChange={(e) => setNewRole({ ...newRole, group: e.target.value })}
              placeholder="group"
              className="w-32 rounded-lg border border-outline-variant bg-surface px-2 py-1 text-sm"
            />
            <button
              onClick={onCreate}
              disabled={!newRole.key.trim()}
              className="rounded-lg bg-primary px-3 py-1 text-xs text-on-primary disabled:opacity-50"
            >
              Create
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
        open={Boolean(pendingDeleteKey)}
        title={`Delete role "${pendingDeleteKey}"?`}
        description="This removes the role from the project in Zitadel. Any user grants referencing it will need re-keying. This cannot be undone."
        confirmLabel="Delete role"
        variant="destructive"
        isPending={deleting}
        onCancel={() => setPendingDeleteKey("")}
        onConfirm={onDeleteConfirm}
      />
    </Card>
  );
}
