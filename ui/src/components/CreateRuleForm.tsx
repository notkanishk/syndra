"use client";

import { useEffect, useMemo, useRef, useState } from "react";

import { Badge } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Eyebrow } from "@/components/ui/Eyebrow";
import { Modal } from "@/components/ui/Modal";
import { Select } from "@/components/ui/Select";
import { SubmitButton } from "@/components/ui/SubmitButton";
import { formatRoleRef } from "@/lib/format";
import { useGlobalConfirmationDefault, type ConfirmationMode } from "@/lib/queries/useConfirmationMode";
import {
  useCreateMappingRule,
  useValidateMappingRule,
  type ValidateMappingRuleResult,
} from "@/lib/queries/useMappingRules";
import { toastError, toastSuccess } from "@/lib/toast";

interface ProjectCatalog {
  id: string;
  name: string;
  roles: Array<{ key: string; label: string }>;
}

interface CreateRuleFormProps {
  /** Controls whether the modal is rendered. */
  open: boolean;
  /** Called when the modal should close (cancel, click-outside, Esc). */
  onClose: () => void;
  /** Called after a successful create — typically used to refresh the list. */
  onCreated?: () => void;
  projects: ProjectCatalog[];
}

/**
 * Mapping-rule authoring form. Wrapped in a Modal so cycle warnings, role
 * preview, and the Create button claim full focus during a high-stakes admin
 * action. Stage 3 swap moved this from an inline disclosure on /policies into
 * a true modal per the OpenSpec governance-first UX requirement.
 */
export default function CreateRuleForm({ open, onClose, onCreated, projects }: CreateRuleFormProps) {
  const createRule = useCreateMappingRule();
  const globalDefault = useGlobalConfirmationDefault();
  // null until the operator touches the selector — the displayed (and submitted) value falls
  // back to the fetched global default, else "auto", so the form is always a WYSIWYG submit.
  const [modeOverride, setModeOverride] = useState<ConfirmationMode | null>(null);
  const confirmationMode: ConfirmationMode = modeOverride ?? globalDefault.data ?? "auto";
  // Destructure the stable callback from the mutation result. The mutation
  // result object's reference changes on every state transition (idle →
  // pending → success), so depending on `validateRule` directly would cause
  // the debounced effect below to re-run after every settled validation,
  // re-firing /rules/mapping/validate continuously while the modal is open
  // even with a stable form. `mutateAsync` is internally memoized by
  // TanStack Query and is the stable handle we depend on.
  const { mutateAsync: validateMappingRule, isPending: validatePending } =
    useValidateMappingRule();
  const [validation, setValidation] = useState<ValidateMappingRuleResult | null>(null);
  const validateTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const [form, setForm] = useState({
    source_project: projects[0]?.id || "",
    source_role: projects[0]?.roles[0]?.key || "",
    target_project: projects[1]?.id || projects[0]?.id || "",
    target_role: projects[1]?.roles[0]?.key || projects[0]?.roles[0]?.key || "",
  });

  // Defaults populate from the live catalog if it lands after open. This
  // happens when /policies is the first page hit and the catalog still
  // resolves a few hundred ms in.
  useEffect(() => {
    if (form.source_project || projects.length === 0) return;
    setForm({
      source_project: projects[0].id,
      source_role: projects[0].roles[0]?.key || "",
      target_project: projects[1]?.id || projects[0].id,
      target_role: projects[1]?.roles[0]?.key || projects[0].roles[0]?.key || "",
    });
  }, [projects, form.source_project]);

  const sourceProject = useMemo(
    () => projects.find((project) => project.id === form.source_project),
    [projects, form.source_project],
  );
  const targetProject = useMemo(
    () => projects.find((project) => project.id === form.target_project),
    [projects, form.target_project],
  );
  const sourceRef = formatRoleRef(form.source_project, form.source_role, projects);
  const targetRef = formatRoleRef(form.target_project, form.target_role, projects);

  // Debounced validation: each form change schedules a validate POST after
  // 250ms of quiet so the operator sees the cycle warning before clicking
  // Create. Backend short-circuits on partial input so blank fields don't
  // crash the validator.
  useEffect(() => {
    if (!open) return;
    if (validateTimer.current) clearTimeout(validateTimer.current);
    validateTimer.current = setTimeout(async () => {
      try {
        const result = await validateMappingRule(form);
        setValidation(result);
      } catch {
        setValidation(null);
      }
    }, 250);
    return () => {
      if (validateTimer.current) clearTimeout(validateTimer.current);
    };
  }, [form, open, validateMappingRule]);

  const handleSubmit = async (event: React.FormEvent) => {
    event.preventDefault();
    try {
      await createRule.mutateAsync({ ...form, confirmation_mode: confirmationMode });
      toastSuccess("Mapping rule created");
      onCreated?.();
      onClose();
    } catch (err) {
      toastError(err instanceof Error ? err.message : "Failed to create rule");
    }
  };

  if (!open) return null;

  const blocking = validation?.would_cycle || validation?.self_reference;

  return (
    <Modal
      open={open}
      onClose={onClose}
      busy={createRule.isPending}
      labelledBy="create-rule-title"
      describedBy="create-rule-preview"
      size="lg"
    >
      <h2 id="create-rule-title" className="text-lg font-semibold text-on-surface">
        Create mapping rule
      </h2>
      <p className="mt-1 text-sm text-on-surface-variant">
        Define how role grants in one project propagate to roles in another.
      </p>

      <form onSubmit={handleSubmit} className="mt-5 space-y-4">
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <div>
            <label className="block text-xs font-medium text-on-surface-variant mb-1.5">
              Source project
            </label>
            <Select
              value={form.source_project}
              onChange={(event) => {
                const project = projects.find((item) => item.id === event.target.value);
                setForm({
                  ...form,
                  source_project: event.target.value,
                  source_role: project?.roles[0]?.key || "",
                });
              }}
            >
              {projects.map((project) => (
                <option key={project.id} value={project.id}>
                  {project.name}
                </option>
              ))}
            </Select>
          </div>
          <div>
            <label className="block text-xs font-medium text-on-surface-variant mb-1.5">
              Source role
            </label>
            <Select
              value={form.source_role}
              onChange={(event) => setForm({ ...form, source_role: event.target.value })}
            >
              {(sourceProject?.roles || []).map((role) => (
                <option key={role.key} value={role.key}>
                  {role.label}
                </option>
              ))}
            </Select>
          </div>
          <div>
            <label className="block text-xs font-medium text-on-surface-variant mb-1.5">
              Target project
            </label>
            <Select
              value={form.target_project}
              onChange={(event) => {
                const project = projects.find((item) => item.id === event.target.value);
                setForm({
                  ...form,
                  target_project: event.target.value,
                  target_role: project?.roles[0]?.key || "",
                });
              }}
            >
              {projects.map((project) => (
                <option key={project.id} value={project.id}>
                  {project.name}
                </option>
              ))}
            </Select>
          </div>
          <div>
            <label className="block text-xs font-medium text-on-surface-variant mb-1.5">
              Target role
            </label>
            <Select
              value={form.target_role}
              onChange={(event) => setForm({ ...form, target_role: event.target.value })}
            >
              {(targetProject?.roles || []).map((role) => (
                <option key={role.key} value={role.key}>
                  {role.label}
                </option>
              ))}
            </Select>
          </div>
          <div>
            <label className="block text-xs font-medium text-on-surface-variant mb-1.5">
              Confirmation mode
            </label>
            <Select
              value={confirmationMode}
              onChange={(event) => setModeOverride(event.target.value as ConfirmationMode)}
              aria-label="Confirmation mode"
            >
              <option value="auto">Auto — drains immediately</option>
              <option value="manual">Manual — waits for operator resume</option>
            </Select>
          </div>
        </div>

        <div
          id="create-rule-preview"
          className="rounded-card border border-dashed border-outline-variant bg-surface-container-low p-4"
        >
          <Eyebrow>Live preview</Eyebrow>
          <div className="mt-2 flex flex-wrap items-center gap-2 text-sm">
            <span className="font-mono text-on-surface-variant font-semibold">IF</span>
            <Badge variant="outline" className="border-primary-container/40 text-primary-container">
              {sourceRef.label}
            </Badge>
            <span className="font-mono text-on-surface-variant font-semibold">THEN ADD</span>
            <Badge variant="outline" className="border-[var(--success)]/40 text-[var(--success)]">
              {targetRef.label}
            </Badge>
          </div>
          {validatePending && (
            <p className="mt-2 text-[11px] text-on-surface-variant">Checking for cycles…</p>
          )}
          {!validatePending && validation?.would_cycle && (
            <p className="mt-2 text-xs text-[var(--warning)]">
              ⚠ Would create a cycle:{" "}
              {validation.reason ?? "downstream rule already feeds back into the source."}
            </p>
          )}
          {!validatePending && validation?.self_reference && (
            <p className="mt-2 text-xs text-[var(--warning)]">
              ⚠ Source and target are the same role; rule would be a no-op.
            </p>
          )}
          {!validatePending &&
            validation &&
            !validation.would_cycle &&
            !validation.self_reference && (
              <p className="mt-2 text-xs text-[var(--success)]">
                ✓ No cycle detected — safe to create.
              </p>
            )}
        </div>

        <div className="flex items-center justify-end gap-3 pt-1">
          <Button type="button" variant="ghost" size="sm" onClick={onClose}>
            Cancel
          </Button>
          <SubmitButton
            isPending={createRule.isPending}
            disabled={blocking}
            pendingLabel="Creating…"
            label="Create rule"
          />
        </div>
      </form>
    </Modal>
  );
}
