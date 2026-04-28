"use client";

import { useCallback, useEffect, useMemo, useState } from "react";

import { Badge } from "@/components/ui/Badge";
import { Card, CardHeader, CardTitle } from "@/components/ui/Card";
import { ConfirmModal } from "@/components/ui/ConfirmModal";
import { Eyebrow } from "@/components/ui/Eyebrow";
import { Pulse } from "@/components/ui/Pulse";
import { useZitadelHealth } from "@/lib/queries/useZitadel";

// --- Backend response shapes (mirror backend/internal/zitadel & handlers) ---

interface HealthResponse {
  status: "ok" | "disabled" | "error";
  mode: "live" | "local-policy-only";
  domain?: string;
  projects_total?: number;
  latency_ms?: number;
  error?: string;
}

interface Paginated<T> {
  items: T[];
  total: number;
  limit: number;
  offset: number;
}

interface ZitadelUser {
  id: string;
  userName: string;
  displayName: string;
  email: string;
  state: string;
}

interface ZitadelProject {
  id: string;
  name: string;
  state: string;
}

interface ProjectRole {
  key: string;
  displayName: string;
  group: string;
}

interface UserGrant {
  id: string;
  userId: string;
  projectId: string;
  roleKeys: string[];
}

// Mirrors backend/internal/handlers/rotation_status.go:ActionRotationStatus.
// last_rotated_at and age_days are omitted by the backend when status is
// "disabled" or "unknown"; they're modelled as optional so render paths
// don't have to check magic sentinel values.
//
// Status ladder (backend-owned, precedence top to bottom):
//   - disabled : ZITADEL_ACTION_SIGNING_KEY unset on the backend —
//                signature verification is off. Any age reading would be
//                misleading, so the panel MUST render this as a misconfig.
//   - unknown  : key installed but ROTATED_AT unset/malformed/in the future.
//   - ok / warn / stale : age-based against the configured threshold.
interface RotationStatus {
  key_installed: boolean;
  last_rotated_at?: string;
  age_days?: number;
  threshold_days: number;
  status: "ok" | "warn" | "stale" | "unknown" | "disabled";
  rotate_command: string;
}

// --- HTTP helpers ---

async function apiGet<T>(path: string): Promise<T> {
  const res = await fetch(`/api/proxy/${path}`, { cache: "no-store" });
  const body = await res.json();
  if (!res.ok) throw new Error(body?.error ?? body?.message ?? `${res.status} ${res.statusText}`);
  return body as T;
}

// apiGetDiagnostic returns the parsed response body regardless of HTTP status.
// The health probe intentionally returns structured 502/503 payloads ({status:
// "disabled"} or {status: "error", error: ...}) that must be rendered, not
// surfaced as generic errors.
async function apiGetDiagnostic<T>(path: string): Promise<T> {
  const res = await fetch(`/api/proxy/${path}`, { cache: "no-store" });
  return (await res.json()) as T;
}

async function apiSend<T>(method: "POST" | "PUT" | "DELETE", path: string, body?: unknown): Promise<T> {
  const init: RequestInit = { method, headers: {}, cache: "no-store" };
  if (body !== undefined) {
    init.headers = { "Content-Type": "application/json" };
    init.body = JSON.stringify(body);
  }
  const res = await fetch(`/api/proxy/${path}`, init);
  const payload = await res.json();
  if (!res.ok) throw new Error(payload?.error ?? payload?.message ?? `${res.status} ${res.statusText}`);
  return payload as T;
}

// --- Page ---

export default function ZitadelDiagnostics() {
  return (
    <div className="p-8 space-y-6 animate-fade-in-up relative z-10">
      <header className="flex flex-col gap-2">
        <Eyebrow>Operations</Eyebrow>
        <h1 className="text-3xl font-semibold text-on-surface mt-1 font-display">
          Zitadel diagnostics
        </h1>
        <p className="text-sm text-on-surface-variant max-w-2xl">
          Exercises the live <code className="text-on-surface">/api/v1/zitadel/*</code>{" "}
          management surface — health probe, projects &amp; roles, users &amp; grants. Use
          this to verify a new Zitadel deployment or service-account rotation before
          trusting the orchestrator.
        </p>
      </header>

      <LiveStatusTile />
      <HealthSection />
      <RotationStatusSection />
      <ProjectsSection />
      <UsersSection />
      <AllGrantsSection />
    </div>
  );
}

/**
 * Top-of-page glass tile. Polls /zitadel/health every 10s and shows the live
 * connection state through a Pulse — steady-green when ok, amber-pulse when
 * disabled (locally configured but not exercising live calls), red-pulse when
 * the management API is unreachable. The polling cadence matches the spec
 * (operations dashboards ≤10s) and pauses when the tab is hidden.
 */
function LiveStatusTile() {
  const { data, isFetching, error } = useZitadelHealth();

  const status = data?.status;
  const variant: "success" | "warn" | "error" | "info" =
    status === "ok"
      ? "success"
      : status === "disabled"
        ? "warn"
        : status === "error" || error
          ? "error"
          : "info";
  const label =
    status === "ok"
      ? "Connected"
      : status === "disabled"
        ? "Disabled (local-policy-only)"
        : status === "error"
          ? "Error"
          : error
            ? "Unreachable"
            : "Checking…";

  return (
    <Card variant="glass">
      <div className="flex flex-wrap items-center justify-between gap-4">
        <div className="flex items-center gap-3">
          <Pulse variant={variant} static={variant === "success"} ariaLabel={label} />
          <div>
            <Eyebrow>Live status</Eyebrow>
            <p className="mt-1 text-xl font-semibold text-on-surface font-display">
              {label}
            </p>
          </div>
        </div>
        <div className="flex flex-wrap items-center gap-3 text-xs text-on-surface-variant">
          {data?.domain && <span>domain: {data.domain}</span>}
          {data?.latency_ms !== undefined && <span>· {data.latency_ms}ms</span>}
          {data?.projects_total !== undefined && (
            <span>· {data.projects_total} projects</span>
          )}
          {isFetching && <span aria-hidden="true">·</span>}
          {isFetching && <span>refreshing…</span>}
        </div>
      </div>
      {data?.error && <p className="mt-3 text-sm text-[var(--error)]">{data.error}</p>}
    </Card>
  );
}

// --- Section: Actions v2 signing-key rotation status ---
//
// Read-only by design. We deliberately do NOT render a "Rotate now" button:
// rotation is a cryptographic mutation whose failure modes (Zitadel accepts
// the new key but the backend is still serving the old one) are easier to
// reason about when the operator runs the command themselves with full
// terminal context. The panel exists for observability — age, threshold,
// and a copyable snippet — not as a click-to-rotate trigger.
function RotationStatusSection() {
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState<RotationStatus | null>(null);
  const [err, setErr] = useState<string>("");
  const [copied, setCopied] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    setErr("");
    try {
      setResult(await apiGet<RotationStatus>("zitadel/action-rotation-status"));
    } catch (e) {
      setErr(e instanceof Error ? e.message : String(e));
      setResult(null);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const badge = useMemo(() => {
    if (!result) return null;
    switch (result.status) {
      case "ok":
        return <Badge>ok</Badge>;
      case "warn":
        return <Badge variant="secondary">warn</Badge>;
      case "stale":
        return <Badge variant="destructive">stale</Badge>;
      case "disabled":
        // disabled is strictly worse than unknown: verification is actively
        // off. Use the destructive styling so the operator sees it as a
        // production problem, not a missing-config nit.
        return <Badge variant="destructive">disabled</Badge>;
      default:
        return <Badge variant="secondary">unknown</Badge>;
    }
  }, [result]);

  const subtext = useMemo(() => {
    if (!result) return null;
    if (result.status === "disabled") {
      return "ZITADEL_ACTION_SIGNING_KEY is unset on the backend — signature verification is passing every Action request through unchecked. Set the env var to the value from zitadel/actions/.action-signing-key and restart the backend before trusting the rotation age.";
    }
    if (result.status === "unknown") {
      return result.key_installed
        ? "ZITADEL_ACTION_SIGNING_KEY_ROTATED_AT is unset, malformed, or in the future — run rotate.sh once or set the env var manually."
        : "Rotation timestamp could not be read.";
    }
    const age = result.age_days ?? 0;
    const threshold = result.threshold_days;
    if (result.status === "stale") {
      return `Key is ${age}d old — past 2× the ${threshold}d threshold. Rotate now.`;
    }
    if (result.status === "warn") {
      return `Key is ${age}d old, above the ${threshold}d threshold. Schedule a rotation.`;
    }
    return `Key is ${age}d old, within the ${threshold}d threshold.`;
  }, [result]);

  const onCopy = useCallback(async () => {
    if (!result) return;
    try {
      await navigator.clipboard.writeText(result.rotate_command);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      // Clipboard access may be denied (e.g. non-HTTPS dev origin); fall back
      // to letting the user select the <code> text themselves.
    }
  }, [result]);

  return (
    <Card>
      <CardHeader>
        <CardTitle>Rotation Status (Actions v2 signing key)</CardTitle>
      </CardHeader>
      <div className="flex items-center gap-3 flex-wrap">
        <button
          onClick={load}
          disabled={loading}
          className="rounded-lg bg-surfaceHover px-3 py-1.5 text-xs font-medium text-foreground disabled:opacity-50"
        >
          {loading ? "Checking..." : "Refresh"}
        </button>
        {badge}
        {result?.last_rotated_at && (
          <span className="text-xs text-muted">
            last rotated: {new Date(result.last_rotated_at).toISOString().replace("T", " ").slice(0, 19)} UTC
          </span>
        )}
        {result?.age_days !== undefined && (
          <span className="text-xs text-muted">· age {result.age_days}d</span>
        )}
        {result && <span className="text-xs text-muted">· threshold {result.threshold_days}d</span>}
      </div>
      {subtext && <p className="mt-3 text-sm text-muted max-w-3xl">{subtext}</p>}
      {err && <p className="mt-3 text-sm text-red-400">{err}</p>}
      {result && (
        <div className="mt-4 flex items-center gap-2 flex-wrap">
          <code className="rounded-lg bg-surfaceHover px-3 py-2 text-xs text-foreground">
            {result.rotate_command}
          </code>
          <button
            onClick={onCopy}
            className="rounded-lg bg-surfaceHover px-3 py-2 text-xs font-medium text-foreground"
            aria-label="Copy rotate command"
          >
            {copied ? "Copied" : "Copy"}
          </button>
          <span className="text-xs text-muted">
            Paste into your terminal — this panel intentionally does not trigger rotation.
          </span>
        </div>
      )}
    </Card>
  );
}

// --- Section 1: M2M Health ---

function HealthSection() {
  const [loading, setLoading] = useState(false);
  const [result, setResult] = useState<HealthResponse | null>(null);
  const [networkError, setNetworkError] = useState<string>("");

  const run = useCallback(async () => {
    setLoading(true);
    setNetworkError("");
    try {
      // apiGetDiagnostic: preserves structured non-2xx payloads so "disabled"
      // and "error" responses render their full diagnostic detail instead of
      // collapsing into a generic error.
      setResult(await apiGetDiagnostic<HealthResponse>("zitadel/health"));
    } catch (err) {
      // Only reached on transport-level failures (proxy unreachable, JSON parse).
      setNetworkError(err instanceof Error ? err.message : String(err));
      setResult(null);
    } finally {
      setLoading(false);
    }
  }, []);

  const badge = result?.status === "ok"
    ? <Badge>ok</Badge>
    : result?.status === "disabled"
      ? <Badge variant="secondary">disabled</Badge>
      : result?.status === "error"
        ? <Badge variant="destructive">error</Badge>
        : networkError
          ? <Badge variant="destructive">unreachable</Badge>
          : null;

  return (
    <Card>
      <CardHeader>
        <CardTitle>M2M Health</CardTitle>
      </CardHeader>
      <div className="flex items-center gap-3 flex-wrap">
        <button
          onClick={run}
          disabled={loading}
          className="rounded-lg bg-primary px-4 py-2 text-sm font-medium text-white disabled:opacity-50"
        >
          {loading ? "Checking..." : "Check connection"}
        </button>
        {badge}
        {result?.domain && <span className="text-xs text-muted">domain: {result.domain}</span>}
        {result?.latency_ms !== undefined && <span className="text-xs text-muted">· {result.latency_ms}ms</span>}
        {result?.projects_total !== undefined && (
          <span className="text-xs text-muted">· {result.projects_total} projects</span>
        )}
      </div>
      {result?.error && <p className="mt-3 text-sm text-red-400">{result.error}</p>}
      {networkError && <p className="mt-3 text-sm text-red-400">{networkError}</p>}
      {result && (
        <details className="mt-3">
          <summary className="cursor-pointer text-xs text-muted">raw response</summary>
          <pre className="mt-2 overflow-x-auto rounded-lg bg-surfaceHover p-3 text-xs text-foreground">
            {JSON.stringify(result, null, 2)}
          </pre>
        </details>
      )}
    </Card>
  );
}

// --- Section 2: Projects & Roles ---

function ProjectsSection() {
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
      const res = await apiGet<Paginated<ZitadelProject>>("zitadel/projects?limit=500");
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
      const res = await apiGet<Paginated<ProjectRole>>(`zitadel/projects/${projectId}/roles?limit=500`);
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
      await apiSend("POST", `zitadel/projects/${selectedId}/roles`, newRole);
      setNewRole({ key: "", displayName: "", group: "" });
      setFlash({ kind: "ok", msg: `Role ${newRole.key} created` });
      await loadRoles(selectedId);
    } catch (err) {
      setFlash({ kind: "err", msg: err instanceof Error ? err.message : String(err) });
    }
  };

  const onUpdate = async (key: string) => {
    try {
      await apiSend("PUT", `zitadel/projects/${selectedId}/roles/${key}`, editDraft);
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
      await apiSend("DELETE", `zitadel/projects/${selectedId}/roles/${key}`);
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
        <label className="text-xs uppercase tracking-[0.22em] text-muted">Project</label>
        <select
          value={selectedId}
          onChange={(e) => setSelectedId(e.target.value)}
          className="rounded-lg border border-border bg-surface px-3 py-2 text-sm text-foreground min-w-[18rem]"
        >
          <option value="">— select a project —</option>
          {projects.map((p) => (
            <option key={p.id} value={p.id}>{p.name} ({p.id})</option>
          ))}
        </select>
        <span className="text-xs text-muted">{total} total</span>
        <button
          onClick={() => void loadProjects()}
          className="rounded-lg border border-border px-3 py-2 text-xs text-muted hover:text-foreground"
        >
          Refresh
        </button>
      </div>

      {selectedId && (
        <div className="mt-4 space-y-3">
          <div className="flex items-center gap-2">
            <p className="text-xs uppercase tracking-[0.22em] text-muted">Roles</p>
            <span className="text-xs text-muted">{rolesTotal} total</span>
          </div>

          {loadingRoles ? (
            <p className="text-sm text-muted">Loading roles...</p>
          ) : roles.length === 0 ? (
            <p className="text-sm text-muted">No roles defined in this project.</p>
          ) : (
            <div className="space-y-2">
              {roles.map((r) => (
                <div key={r.key} className="rounded-lg border border-border bg-surfaceHover px-3 py-2 text-sm flex items-center gap-3 flex-wrap">
                  <code className="text-foreground font-mono">{r.key}</code>
                  {editing === r.key ? (
                    <>
                      <input
                        value={editDraft.displayName}
                        onChange={(e) => setEditDraft({ ...editDraft, displayName: e.target.value })}
                        placeholder="display name"
                        className="flex-1 min-w-[10rem] rounded-lg border border-border bg-surface px-2 py-1 text-sm"
                      />
                      <input
                        value={editDraft.group}
                        onChange={(e) => setEditDraft({ ...editDraft, group: e.target.value })}
                        placeholder="group"
                        className="w-32 rounded-lg border border-border bg-surface px-2 py-1 text-sm"
                      />
                      <button onClick={() => onUpdate(r.key)} className="rounded-lg bg-primary px-3 py-1 text-xs text-white">Save</button>
                      <button onClick={() => setEditing("")} className="rounded-lg border border-border px-3 py-1 text-xs text-muted">Cancel</button>
                    </>
                  ) : (
                    <>
                      <span className="text-muted">{r.displayName || "—"}</span>
                      {r.group && <Badge variant="outline">{r.group}</Badge>}
                      <span className="flex-1" />
                      <button
                        onClick={() => { setEditing(r.key); setEditDraft({ displayName: r.displayName, group: r.group }); }}
                        className="rounded-lg border border-border px-3 py-1 text-xs text-muted hover:text-foreground"
                      >
                        Edit
                      </button>
                      <button
                        onClick={() => onDelete(r.key)}
                        className="rounded-lg border border-red-500/40 px-3 py-1 text-xs text-red-400 hover:bg-red-500/10"
                      >
                        Delete
                      </button>
                    </>
                  )}
                </div>
              ))}
            </div>
          )}

          <div className="rounded-lg border border-dashed border-border px-3 py-2 text-sm flex items-center gap-2 flex-wrap">
            <span className="text-xs uppercase tracking-[0.22em] text-muted">Add role</span>
            <input
              value={newRole.key}
              onChange={(e) => setNewRole({ ...newRole, key: e.target.value })}
              placeholder="role key (required)"
              className="w-40 rounded-lg border border-border bg-surface px-2 py-1 text-sm"
            />
            <input
              value={newRole.displayName}
              onChange={(e) => setNewRole({ ...newRole, displayName: e.target.value })}
              placeholder="display name"
              className="w-40 rounded-lg border border-border bg-surface px-2 py-1 text-sm"
            />
            <input
              value={newRole.group}
              onChange={(e) => setNewRole({ ...newRole, group: e.target.value })}
              placeholder="group"
              className="w-32 rounded-lg border border-border bg-surface px-2 py-1 text-sm"
            />
            <button
              onClick={onCreate}
              disabled={!newRole.key.trim()}
              className="rounded-lg bg-primary px-3 py-1 text-xs text-white disabled:opacity-50"
            >
              Create
            </button>
          </div>
        </div>
      )}

      {flash && (
        <p className={`mt-3 text-sm ${flash.kind === "ok" ? "text-emerald-400" : "text-red-400"}`}>
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

// --- Section 3: Users & Grants ---

function UsersSection() {
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
        apiGet<Paginated<ZitadelUser>>("zitadel/users?limit=500"),
        apiGet<Paginated<ZitadelProject>>("zitadel/projects?limit=500"),
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
      const res = await apiGet<Paginated<UserGrant>>(`zitadel/users/${userId}/grants?limit=500`);
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

  const parseRoleKeys = (raw: string) =>
    raw.split(",").map((s) => s.trim()).filter(Boolean);

  const onAssign = async () => {
    const roleKeys = parseRoleKeys(newGrant.roleKeys);
    if (!selectedId || !newGrant.projectId || roleKeys.length === 0) return;
    try {
      await apiSend("POST", `zitadel/users/${selectedId}/grants`, {
        projectId: newGrant.projectId,
        roleKeys,
      });
      setNewGrant({ projectId: "", roleKeys: "" });
      setFlash({ kind: "ok", msg: "Grant assigned" });
      await loadGrants(selectedId);
    } catch (err) {
      setFlash({ kind: "err", msg: err instanceof Error ? err.message : String(err) });
    }
  };

  const onUpdate = async (grantId: string) => {
    const roleKeys = parseRoleKeys(editDraft);
    if (roleKeys.length === 0) return;
    try {
      await apiSend("PUT", `zitadel/users/${selectedId}/grants/${grantId}`, { roleKeys });
      setEditing("");
      setFlash({ kind: "ok", msg: "Grant updated" });
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
      await apiSend("DELETE", `zitadel/users/${selectedId}/grants/${id}`);
      setFlash({ kind: "ok", msg: "Grant revoked" });
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
        <label className="text-xs uppercase tracking-[0.22em] text-muted">User</label>
        <input
          value={filter}
          onChange={(e) => setFilter(e.target.value)}
          placeholder="filter by email, name, id"
          className="w-56 rounded-lg border border-border bg-surface px-3 py-2 text-sm"
        />
        <select
          value={selectedId}
          onChange={(e) => setSelectedId(e.target.value)}
          className="rounded-lg border border-border bg-surface px-3 py-2 text-sm text-foreground min-w-[20rem]"
        >
          <option value="">— select a user —</option>
          {filteredUsers.map((u) => (
            <option key={u.id} value={u.id}>
              {u.displayName || u.userName} · {u.email}
            </option>
          ))}
        </select>
        <span className="text-xs text-muted">{total} total{filter ? ` · ${filteredUsers.length} match` : ""}</span>
        <button
          onClick={() => void loadDirectory()}
          className="rounded-lg border border-border px-3 py-2 text-xs text-muted hover:text-foreground"
        >
          Refresh
        </button>
      </div>

      {selectedId && (
        <div className="mt-4 space-y-3">
          <div className="flex items-center gap-2">
            <p className="text-xs uppercase tracking-[0.22em] text-muted">Grants</p>
            <span className="text-xs text-muted">{grantsTotal} total</span>
          </div>

          {loadingGrants ? (
            <p className="text-sm text-muted">Loading grants...</p>
          ) : grants.length === 0 ? (
            <p className="text-sm text-muted">No grants for this user.</p>
          ) : (
            <div className="space-y-2">
              {grants.map((g) => (
                <div key={g.id} className="rounded-lg border border-border bg-surfaceHover px-3 py-2 text-sm flex items-center gap-3 flex-wrap">
                  <code className="text-foreground font-mono">{projectName(g.projectId)}</code>
                  {editing === g.id ? (
                    <>
                      <input
                        value={editDraft}
                        onChange={(e) => setEditDraft(e.target.value)}
                        placeholder="comma-separated role keys"
                        className="flex-1 min-w-[16rem] rounded-lg border border-border bg-surface px-2 py-1 text-sm"
                      />
                      <button onClick={() => onUpdate(g.id)} className="rounded-lg bg-primary px-3 py-1 text-xs text-white">Save</button>
                      <button onClick={() => setEditing("")} className="rounded-lg border border-border px-3 py-1 text-xs text-muted">Cancel</button>
                    </>
                  ) : (
                    <>
                      <div className="flex flex-wrap gap-1">
                        {g.roleKeys.map((rk) => (
                          <Badge key={rk} variant="outline">{rk}</Badge>
                        ))}
                      </div>
                      <span className="flex-1" />
                      <button
                        onClick={() => { setEditing(g.id); setEditDraft(g.roleKeys.join(", ")); }}
                        className="rounded-lg border border-border px-3 py-1 text-xs text-muted hover:text-foreground"
                      >
                        Edit
                      </button>
                      <button
                        onClick={() => onRevoke(g.id, `${projectName(g.projectId)} / ${g.roleKeys.join(", ")}`)}
                        className="rounded-lg border border-red-500/40 px-3 py-1 text-xs text-red-400 hover:bg-red-500/10"
                      >
                        Revoke
                      </button>
                    </>
                  )}
                </div>
              ))}
            </div>
          )}

          <div className="rounded-lg border border-dashed border-border px-3 py-2 text-sm flex items-center gap-2 flex-wrap">
            <span className="text-xs uppercase tracking-[0.22em] text-muted">Assign grant</span>
            <select
              value={newGrant.projectId}
              onChange={(e) => setNewGrant({ ...newGrant, projectId: e.target.value })}
              className="rounded-lg border border-border bg-surface px-2 py-1 text-sm min-w-[14rem]"
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
              className="flex-1 min-w-[14rem] rounded-lg border border-border bg-surface px-2 py-1 text-sm"
            />
            <button
              onClick={onAssign}
              disabled={!newGrant.projectId || !newGrant.roleKeys.trim()}
              className="rounded-lg bg-primary px-3 py-1 text-xs text-white disabled:opacity-50"
            >
              Assign
            </button>
          </div>
        </div>
      )}

      {flash && (
        <p className={`mt-3 text-sm ${flash.kind === "ok" ? "text-emerald-400" : "text-red-400"}`}>
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

// --- Section 4: All Grants ---

function AllGrantsSection() {
  const [grants, setGrants] = useState<UserGrant[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  const run = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      const res = await apiGet<Paginated<UserGrant>>("zitadel/grants?limit=500");
      setGrants(res.items ?? []);
      setTotal(res.total ?? 0);
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(false);
    }
  }, []);

  return (
    <Card>
      <CardHeader>
        <CardTitle>All Grants</CardTitle>
      </CardHeader>
      <div className="flex items-center gap-3 flex-wrap">
        <button
          onClick={run}
          disabled={loading}
          className="rounded-lg bg-primary px-4 py-2 text-sm font-medium text-white disabled:opacity-50"
        >
          {loading ? "Loading..." : "Load all grants"}
        </button>
        {total > 0 && <span className="text-xs text-muted">{total} total · showing {grants.length}</span>}
      </div>

      {error && <p className="mt-3 text-sm text-red-400">{error}</p>}

      {grants.length > 0 && (
        <div className="mt-4 overflow-x-auto">
          <table className="min-w-full text-sm">
            <thead>
              <tr className="text-left text-xs uppercase tracking-[0.22em] text-muted">
                <th className="py-2 pr-4">User ID</th>
                <th className="py-2 pr-4">Project ID</th>
                <th className="py-2 pr-4">Roles</th>
                <th className="py-2 pr-4">Grant ID</th>
              </tr>
            </thead>
            <tbody>
              {grants.map((g) => (
                <tr key={g.id} className="border-t border-border">
                  <td className="py-2 pr-4 font-mono text-xs">{g.userId}</td>
                  <td className="py-2 pr-4 font-mono text-xs">{g.projectId}</td>
                  <td className="py-2 pr-4">
                    <div className="flex flex-wrap gap-1">
                      {g.roleKeys.map((rk) => (
                        <Badge key={rk} variant="outline">{rk}</Badge>
                      ))}
                    </div>
                  </td>
                  <td className="py-2 pr-4 font-mono text-xs text-muted">{g.id}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </Card>
  );
}
