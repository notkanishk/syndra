"use client";

import { useState } from "react";

interface CreateRuleFormProps {
  onCreated: () => void;
}

export default function CreateRuleForm({ onCreated }: CreateRuleFormProps) {
  const [open, setOpen] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [form, setForm] = useState({
    source_project: "",
    source_role: "",
    target_project: "",
    target_role: "",
  });

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setError("");

    try {
      const res = await fetch("/api/proxy/rules/mapping", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(form),
      });
      if (!res.ok) {
        const body = await res.json().catch(() => ({}));
        throw new Error(body.message || "Failed to create rule");
      }
      setForm({ source_project: "", source_role: "", target_project: "", target_role: "" });
      setOpen(false);
      onCreated();
    } catch (err: any) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  if (!open) {
    return (
      <button
        onClick={() => setOpen(true)}
        className="bg-primary hover:bg-primaryHover text-white px-4 py-2 rounded-md font-medium text-sm transition-all shadow-sm hover:shadow-md"
      >
        + New Logic Rule
      </button>
    );
  }

  return (
    <form onSubmit={handleSubmit} className="w-full border border-border rounded-lg p-6 bg-surfaceHover animate-fade-in-up">
      <h3 className="text-sm font-semibold text-foreground mb-4">Create Mapping Rule</h3>
      
      {error && (
        <div className="mb-4 p-3 rounded-md bg-red-500/10 border border-red-500/20 text-red-500 text-sm">
          {error}
        </div>
      )}

      <div className="grid grid-cols-2 gap-4 mb-4">
        <div>
          <label className="block text-xs font-medium text-muted mb-1.5">Source Project ID</label>
          <input
            type="text"
            required
            placeholder="e.g. proj_printing"
            value={form.source_project}
            onChange={(e) => setForm({ ...form, source_project: e.target.value })}
            className="w-full px-3 py-2 rounded-md border border-border bg-surface text-foreground text-sm placeholder:text-muted focus:outline-none focus:ring-2 focus:ring-primary/50 focus:border-primary transition-colors"
          />
        </div>
        <div>
          <label className="block text-xs font-medium text-muted mb-1.5">Source Role Key</label>
          <input
            type="text"
            required
            placeholder="e.g. role_user"
            value={form.source_role}
            onChange={(e) => setForm({ ...form, source_role: e.target.value })}
            className="w-full px-3 py-2 rounded-md border border-border bg-surface text-foreground text-sm placeholder:text-muted focus:outline-none focus:ring-2 focus:ring-primary/50 focus:border-primary transition-colors"
          />
        </div>
        <div>
          <label className="block text-xs font-medium text-muted mb-1.5">Target Project ID</label>
          <input
            type="text"
            required
            placeholder="e.g. proj_door_access"
            value={form.target_project}
            onChange={(e) => setForm({ ...form, target_project: e.target.value })}
            className="w-full px-3 py-2 rounded-md border border-border bg-surface text-foreground text-sm placeholder:text-muted focus:outline-none focus:ring-2 focus:ring-primary/50 focus:border-primary transition-colors"
          />
        </div>
        <div>
          <label className="block text-xs font-medium text-muted mb-1.5">Target Role Key</label>
          <input
            type="text"
            required
            placeholder="e.g. 3d_lab_pin"
            value={form.target_role}
            onChange={(e) => setForm({ ...form, target_role: e.target.value })}
            className="w-full px-3 py-2 rounded-md border border-border bg-surface text-foreground text-sm placeholder:text-muted focus:outline-none focus:ring-2 focus:ring-primary/50 focus:border-primary transition-colors"
          />
        </div>
      </div>

      <div className="flex items-center gap-3">
        <button
          type="submit"
          disabled={loading}
          className="bg-primary hover:bg-primaryHover text-white px-4 py-2 rounded-md font-medium text-sm transition-all shadow-sm hover:shadow-md disabled:opacity-50"
        >
          {loading ? "Creating..." : "Create Rule"}
        </button>
        <button
          type="button"
          onClick={() => { setOpen(false); setError(""); }}
          className="px-4 py-2 rounded-md font-medium text-sm text-muted hover:text-foreground transition-colors"
        >
          Cancel
        </button>
      </div>
    </form>
  );
}
