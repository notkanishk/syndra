"use client";

import { useEffect, useRef, useState } from "react";

import { Badge } from "@/components/ui/Badge";
import { SubmitButton } from "@/components/ui/SubmitButton";
import { formatRoleRef } from "@/lib/format";
import { toastError, toastSuccess } from "@/lib/toast";

interface ProjectCatalog {
  id: string;
  name: string;
  roles: Array<{ key: string; label: string }>;
}

interface CreateRuleFormProps {
  onCreated: () => void;
  projects: ProjectCatalog[];
}

interface ValidationResult {
  would_cycle: boolean;
  self_reference: boolean;
  reason?: string;
}

export default function CreateRuleForm({ onCreated, projects }: CreateRuleFormProps) {
  const [open, setOpen] = useState(false);
  const [loading, setLoading] = useState(false);
  const [validation, setValidation] = useState<ValidationResult | null>(null);
  const [validating, setValidating] = useState(false);
  const validateTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const [form, setForm] = useState({
    source_project: projects[0]?.id || "",
    source_role: projects[0]?.roles[0]?.key || "",
    target_project: projects[1]?.id || projects[0]?.id || "",
    target_role: projects[1]?.roles[0]?.key || projects[0]?.roles[0]?.key || "",
  });

  const sourceProject = projects.find((project) => project.id === form.source_project);
  const targetProject = projects.find((project) => project.id === form.target_project);
  const sourceRef = formatRoleRef(form.source_project, form.source_role, projects);
  const targetRef = formatRoleRef(form.target_project, form.target_role, projects);

  // Debounced validation: each form change schedules a validate POST after
  // 250ms of quiet so the operator sees the cycle warning before clicking
  // Create. Backend short-circuits on partial input so no fields blank UI.
  useEffect(() => {
    if (!open) return;
    if (validateTimer.current) clearTimeout(validateTimer.current);
    validateTimer.current = setTimeout(async () => {
      setValidating(true);
      try {
        const res = await fetch("/api/proxy/rules/mapping/validate", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(form),
        });
        if (!res.ok) {
          setValidation(null);
          return;
        }
        const data = (await res.json()) as ValidationResult;
        setValidation(data);
      } catch {
        setValidation(null);
      } finally {
        setValidating(false);
      }
    }, 250);
    return () => {
      if (validateTimer.current) clearTimeout(validateTimer.current);
    };
  }, [form, open]);

  const handleSubmit = async (event: React.FormEvent) => {
    event.preventDefault();
    setLoading(true);
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
      toastSuccess("Mapping rule created");
    } catch (err) {
      toastError(err instanceof Error ? err.message : "Failed to create rule");
    } finally {
      setLoading(false);
    }
  };

  if (!open) {
    return (
      <button
        onClick={() => setOpen(true)}
        className="bg-primary hover:bg-primary-hover text-white px-4 py-2 rounded-md font-medium text-sm transition-all shadow-sm hover:shadow-md focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
      >
        + New Logic Rule
      </button>
    );
  }

  const blocking = validation?.would_cycle || validation?.self_reference;

  return (
    <form onSubmit={handleSubmit} className="w-full border border-border rounded-lg p-6 bg-surfaceHover animate-fade-in-up">
      <h3 className="text-sm font-semibold text-foreground mb-4">Create Mapping Rule</h3>

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

      <div className="mb-4 rounded-lg border border-dashed border-border bg-background/40 p-4">
        <p className="text-[10px] uppercase tracking-[0.22em] text-muted">Live preview</p>
        <div className="mt-2 flex flex-wrap items-center gap-2 text-sm">
          <span className="font-mono text-muted font-semibold">IF</span>
          <Badge variant="outline" className="border-primary/40 text-primary">{sourceRef.label}</Badge>
          <span className="font-mono text-muted font-semibold">THEN ADD</span>
          <Badge variant="outline" className="border-emerald-500/40 text-emerald-600 dark:text-emerald-400">{targetRef.label}</Badge>
        </div>
        {validating && (
          <p className="mt-2 text-[11px] text-muted">Checking for cycles…</p>
        )}
        {!validating && validation?.would_cycle && (
          <p className="mt-2 text-xs text-amber-600 dark:text-amber-400">
            ⚠ Would create a cycle: {validation.reason ?? "downstream rule already feeds back into the source."}
          </p>
        )}
        {!validating && validation?.self_reference && (
          <p className="mt-2 text-xs text-amber-600 dark:text-amber-400">
            ⚠ Source and target are the same role; rule would be a no-op.
          </p>
        )}
        {!validating && validation && !validation.would_cycle && !validation.self_reference && (
          <p className="mt-2 text-xs text-emerald-600 dark:text-emerald-400">
            ✓ No cycle detected — safe to create.
          </p>
        )}
      </div>

      <div className="flex items-center gap-3">
        <SubmitButton isPending={loading} disabled={blocking} pendingLabel="Creating…" label="Create Rule" />
        <button
          type="button"
          onClick={() => {
            setOpen(false);
            setValidation(null);
          }}
          className="px-4 py-2 rounded-md font-medium text-sm text-muted hover:text-foreground transition-colors"
        >
          Cancel
        </button>
      </div>
    </form>
  );
}
