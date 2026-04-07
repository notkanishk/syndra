"use client";

import { useState } from "react";

interface ProjectCatalog {
  id: string;
  name: string;
  roles: Array<{ key: string; label: string }>;
}

interface CreateRuleFormProps {
  onCreated: () => void;
  projects: ProjectCatalog[];
}

export default function CreateRuleForm({ onCreated, projects }: CreateRuleFormProps) {
  const [open, setOpen] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [form, setForm] = useState({
    source_project: projects[0]?.id || "",
    source_role: projects[0]?.roles[0]?.key || "",
    target_project: projects[1]?.id || projects[0]?.id || "",
    target_role: projects[1]?.roles[0]?.key || projects[0]?.roles[0]?.key || "",
  });

  const sourceProject = projects.find((project) => project.id === form.source_project);
  const targetProject = projects.find((project) => project.id === form.target_project);

  const handleSubmit = async (event: React.FormEvent) => {
    event.preventDefault();
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
      setOpen(false);
      onCreated();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to create rule");
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

      <div className="grid grid-cols-1 md:grid-cols-2 gap-4 mb-4">
        <div>
          <label className="block text-xs font-medium text-muted mb-1.5">Source Project</label>
          <select
            value={form.source_project}
            onChange={(event) => {
              const project = projects.find((item) => item.id === event.target.value);
              setForm({
                ...form,
                source_project: event.target.value,
                source_role: project?.roles[0]?.key || "",
              });
            }}
            className="w-full px-3 py-2 rounded-md border border-border bg-surface text-foreground text-sm"
          >
            {projects.map((project) => (
              <option key={project.id} value={project.id}>
                {project.name}
              </option>
            ))}
          </select>
        </div>
        <div>
          <label className="block text-xs font-medium text-muted mb-1.5">Source Role</label>
          <select
            value={form.source_role}
            onChange={(event) => setForm({ ...form, source_role: event.target.value })}
            className="w-full px-3 py-2 rounded-md border border-border bg-surface text-foreground text-sm"
          >
            {(sourceProject?.roles || []).map((role) => (
              <option key={role.key} value={role.key}>
                {role.label}
              </option>
            ))}
          </select>
        </div>
        <div>
          <label className="block text-xs font-medium text-muted mb-1.5">Target Project</label>
          <select
            value={form.target_project}
            onChange={(event) => {
              const project = projects.find((item) => item.id === event.target.value);
              setForm({
                ...form,
                target_project: event.target.value,
                target_role: project?.roles[0]?.key || "",
              });
            }}
            className="w-full px-3 py-2 rounded-md border border-border bg-surface text-foreground text-sm"
          >
            {projects.map((project) => (
              <option key={project.id} value={project.id}>
                {project.name}
              </option>
            ))}
          </select>
        </div>
        <div>
          <label className="block text-xs font-medium text-muted mb-1.5">Target Role</label>
          <select
            value={form.target_role}
            onChange={(event) => setForm({ ...form, target_role: event.target.value })}
            className="w-full px-3 py-2 rounded-md border border-border bg-surface text-foreground text-sm"
          >
            {(targetProject?.roles || []).map((role) => (
              <option key={role.key} value={role.key}>
                {role.label}
              </option>
            ))}
          </select>
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
          onClick={() => {
            setOpen(false);
            setError("");
          }}
          className="px-4 py-2 rounded-md font-medium text-sm text-muted hover:text-foreground transition-colors"
        >
          Cancel
        </button>
      </div>
    </form>
  );
}
