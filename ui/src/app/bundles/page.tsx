"use client";

import { useEffect, useState, useCallback } from "react";
import { Card, CardHeader, CardTitle } from "@/components/ui/Card";
import { Badge } from "@/components/ui/Badge";

interface BundleRole {
  zitadel_project_id: string;
  zitadel_role_key: string;
}

interface Bundle {
  id: string;
  name: string;
  description: string;
  created_at: string;
}

export default function BundlesView() {
  const [bundles, setBundles] = useState<Bundle[]>([]);
  const [loading, setLoading] = useState(true);
  const [formOpen, setFormOpen] = useState(false);
  const [creating, setCreating] = useState(false);
  const [error, setError] = useState("");
  const [form, setForm] = useState({ name: "", description: "" });
  const [expandedBundle, setExpandedBundle] = useState<string | null>(null);
  const [bundleRoles, setBundleRoles] = useState<Record<string, BundleRole[]>>({});
  const [newRole, setNewRole] = useState({ project_id: "", role_key: "" });

  const loadBundles = useCallback(async () => {
    setLoading(true);
    try {
      const res = await fetch("/api/proxy/bundles");
      const data = await res.json();
      setBundles(Array.isArray(data) ? data : []);
    } catch {
      setBundles([]);
    } finally {
      setLoading(false);
    }
  }, []);

  const loadBundleRoles = async (bundleId: string) => {
    try {
      const res = await fetch(`/api/proxy/bundles/${bundleId}/roles`);
      const data = await res.json();
      setBundleRoles(prev => ({ ...prev, [bundleId]: Array.isArray(data) ? data : [] }));
    } catch (err) {
      console.error("Failed to load bundle roles", err);
    }
  };

  useEffect(() => {
    loadBundles();
  }, [loadBundles]);

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    setCreating(true);
    setError("");
    try {
      const res = await fetch("/api/proxy/bundles", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(form),
      });
      if (!res.ok) {
        const body = await res.json().catch(() => ({}));
        throw new Error(body.message || "Failed to create bundle");
      }
      setForm({ name: "", description: "" });
      setFormOpen(false);
      loadBundles();
    } catch (err: any) {
      setError(err.message);
    } finally {
      setCreating(false);
    }
  };

  const handleAddRole = async (bundleId: string) => {
    if (!newRole.project_id || !newRole.role_key) return;
    try {
      const res = await fetch(`/api/proxy/bundles/${bundleId}/roles`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(newRole),
      });
      if (res.ok) {
        setNewRole({ project_id: "", role_key: "" });
        loadBundleRoles(bundleId);
      }
    } catch (err) {
      console.error("Failed to add role", err);
    }
  };

  return (
    <div className="space-y-6 animate-fade-in-up">
      <header>
        <h1 className="text-3xl font-bold text-foreground">Bundles</h1>
        <p className="text-muted mt-2">
          Group multiple roles into a single assignable unit. Assign a bundle to a user and all underlying roles propagate automatically.
        </p>
      </header>

      <Card>
        <div className="flex items-center justify-between mb-6">
          <CardTitle>Defined Bundles</CardTitle>
          {!formOpen && (
            <button
              onClick={() => setFormOpen(true)}
              className="bg-primary hover:bg-primaryHover text-white px-4 py-2 rounded-md font-medium text-sm transition-all shadow-sm hover:shadow-md"
            >
              + New Bundle
            </button>
          )}
        </div>

        {formOpen && (
          <form onSubmit={handleCreate} className="mb-6 border border-border rounded-lg p-6 bg-surfaceHover animate-fade-in-up">
            <h3 className="text-sm font-semibold text-foreground mb-4">Create Bundle</h3>
            {error && (
              <div className="mb-4 p-3 rounded-md bg-red-500/10 border border-red-500/20 text-red-500 text-sm">{error}</div>
            )}
            <div className="grid grid-cols-2 gap-4 mb-4">
              <div>
                <label className="block text-xs font-medium text-muted mb-1.5">Bundle Name</label>
                <input
                  type="text"
                  required
                  placeholder="e.g. Student Access"
                  value={form.name}
                  onChange={(e) => setForm({ ...form, name: e.target.value })}
                  className="w-full px-3 py-2 rounded-md border border-border bg-surface text-foreground text-sm placeholder:text-muted focus:outline-none focus:ring-2 focus:ring-primary/50 focus:border-primary transition-colors"
                />
              </div>
              <div>
                <label className="block text-xs font-medium text-muted mb-1.5">Description</label>
                <input
                  type="text"
                  placeholder="e.g. Basic access for enrolled students"
                  value={form.description}
                  onChange={(e) => setForm({ ...form, description: e.target.value })}
                  className="w-full px-3 py-2 rounded-md border border-border bg-surface text-foreground text-sm placeholder:text-muted focus:outline-none focus:ring-2 focus:ring-primary/50 focus:border-primary transition-colors"
                />
              </div>
            </div>
            <div className="flex items-center gap-3">
              <button type="submit" disabled={creating} className="bg-primary hover:bg-primaryHover text-white px-4 py-2 rounded-md font-medium text-sm transition-all disabled:opacity-50">
                {creating ? "Creating..." : "Create Bundle"}
              </button>
              <button type="button" onClick={() => { setFormOpen(false); setError(""); }} className="px-4 py-2 rounded-md font-medium text-sm text-muted hover:text-foreground transition-colors">
                Cancel
              </button>
            </div>
          </form>
        )}

        {loading ? (
          <div className="text-center py-10">
            <div className="inline-block w-6 h-6 border-2 border-primary border-t-transparent rounded-full animate-spin"></div>
            <p className="text-muted mt-3 text-sm">Loading bundles...</p>
          </div>
        ) : bundles.length === 0 ? (
          <div className="text-center py-10 border border-dashed border-border rounded-lg">
            <p className="text-muted">No bundles defined yet.</p>
            <p className="text-xs text-muted mt-1">Create your first bundle to group roles together.</p>
          </div>
        ) : (
          <div className="space-y-3">
            {bundles.map((bundle) => (
              <div key={bundle.id} className="border border-border rounded-lg bg-surfaceHover overflow-hidden transition-all hover:border-primary/30">
                <div 
                  className="p-4 flex items-center justify-between cursor-pointer hover:bg-surface"
                  onClick={() => {
                    if (expandedBundle === bundle.id) {
                      setExpandedBundle(null);
                    } else {
                      setExpandedBundle(bundle.id);
                      loadBundleRoles(bundle.id);
                    }
                  }}
                >
                  <div>
                    <h3 className="font-semibold text-foreground flex items-center gap-2">
                      {bundle.name}
                      {expandedBundle === bundle.id ? (
                        <span className="text-[10px] text-muted">▼</span>
                      ) : (
                        <span className="text-[10px] text-muted">▶</span>
                      )}
                    </h3>
                    <p className="text-xs text-muted mt-0.5">{bundle.description || "No description"}</p>
                  </div>
                  <Badge variant="secondary">
                    {new Date(bundle.created_at).toLocaleDateString()}
                  </Badge>
                </div>

                {expandedBundle === bundle.id && (
                  <div className="px-4 pb-4 pt-2 border-t border-border bg-surface/50 animate-fade-in-down">
                    <div className="mb-4">
                      <h4 className="text-[10px] font-bold text-muted uppercase tracking-widest mb-2">Roles in Bundle</h4>
                      {!bundleRoles[bundle.id] ? (
                        <p className="text-xs text-muted">Loading roles...</p>
                      ) : bundleRoles[bundle.id].length === 0 ? (
                        <p className="text-xs text-muted italic">No roles added yet.</p>
                      ) : (
                        <div className="flex flex-wrap gap-2">
                          {bundleRoles[bundle.id].map((r, idx) => (
                            <Badge key={idx} variant="outline" className="text-[10px] py-0 px-1.5 border-primary/20 bg-primary/5 text-primary">
                              {r.zitadel_project_id}:{r.zitadel_role_key}
                            </Badge>
                          ))}
                        </div>
                      )}
                    </div>

                    <div className="pt-4 border-t border-border/50">
                      <h4 className="text-[10px] font-bold text-muted uppercase tracking-widest mb-2">Add Role to Bundle</h4>
                      <div className="grid grid-cols-2 gap-2 mb-2">
                        <input
                          type="text"
                          placeholder="Project ID"
                          value={newRole.project_id}
                          onChange={(e) => setNewRole({ ...newRole, project_id: e.target.value })}
                          className="px-2 py-1.5 rounded bg-surface border border-border text-xs focus:outline-none focus:border-primary transition-colors"
                        />
                        <input
                          type="text"
                          placeholder="Role Key"
                          value={newRole.role_key}
                          onChange={(e) => setNewRole({ ...newRole, role_key: e.target.value })}
                          className="px-2 py-1.5 rounded bg-surface border border-border text-xs focus:outline-none focus:border-primary transition-colors"
                        />
                      </div>
                      <button 
                        onClick={() => handleAddRole(bundle.id)}
                        className="w-full py-1.5 bg-primary/10 hover:bg-primary/20 text-primary text-[10px] font-bold uppercase tracking-wider rounded transition-colors"
                      >
                        + Add Role
                      </button>
                    </div>
                  </div>
                )}
              </div>
            ))}
          </div>
        )}
      </Card>
    </div>
  );
}
