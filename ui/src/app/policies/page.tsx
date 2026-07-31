"use client";

import { useState } from "react";
import { toast } from "sonner";

import { EmptyState, ListStates, RowSkeleton } from "@/components/states";
import { Badge, Mono } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Card, CardColumns } from "@/components/ui/Card";
import { FieldHint, FieldLabel } from "@/components/ui/Input";
import { Modal, ModalFooter, ModalHeader } from "@/components/ui/Modal";
import { PageHeader } from "@/components/ui/PageHeader";
import { Segmented, Select } from "@/components/ui/Select";
import { ProjectName } from "@/components/names";
import {
  useCreateMappingRule,
  useMappingRules,
  useSetRuleConfirmationMode,
  useUpdateMappingRule,
  useValidateMappingRule,
  type MappingRuleRow,
} from "@/lib/queries/useMappingRules";
import { useProjects } from "@/lib/queries/useProjects";
import { useGlobalRoleCatalog } from "@/lib/queries/useRoles";
import { humanizeKey } from "@/lib/format";

/**
 * S2 · Automation › Automatic rules.
 *
 * A rule is the reason a role shows up with a dashed chip and nobody remembers
 * clicking it, so every rule is written here as the English sentence it
 * produces — "3D Lab / operator ⇒ Laser Lab / trained" — with the antecedent
 * muted and the consequent bold. Not a form with a "source" and a "target".
 */
export default function AutomaticRulesPage() {
  const rules = useMappingRules();
  const [editing, setEditing] = useState<MappingRuleRow | "new" | null>(null);

  const rows = rules.data ?? [];

  return (
    <div className="flex flex-col gap-[18px]">
      <PageHeader
        title="Automatic rules"
        meta="Holding one role produces another, without anybody clicking."
        actions={
          <Button variant="accent" onClick={() => setEditing("new")}>
            New rule
          </Button>
        }
      />

      <Card>
        <CardColumns>
          <span className="w-[110px]">Rule</span>
          <span className="flex-1">If … then</span>
          <span className="w-[90px] text-right">Holders</span>
          <span className="w-[150px] text-right">On fire</span>
        </CardColumns>

        <ListStates
          isLoading={rules.isLoading}
          error={rules.error}
          isEmpty={rows.length === 0}
          onRetry={() => rules.refetch()}
          errorTitle="Couldn't load automatic rules."
          skeleton={<RowSkeleton rows={3} avatar={false} label="Loading rules" />}
          empty={
            <EmptyState
              title="No automatic rules."
              guidance="Every role somebody holds was given to them deliberately. Add a rule when one role should always imply another."
              action={{ label: "Create a rule", onClick: () => setEditing("new") }}
            />
          }
        >
          {rows.map((rule) => (
            <button
              key={rule.id}
              type="button"
              onClick={() => setEditing(rule)}
              className="row-divider flex w-full flex-wrap items-center gap-[18px] px-5 py-3.5 text-left transition-colors hover:bg-[var(--hover)]"
            >
              <Mono className="w-[110px] shrink-0 truncate text-faint">
                {shortRuleId(rule.id)}
              </Mono>

              <span className="min-w-[320px] flex-1 text-[14.5px]">
                <span className="text-muted">
                  <ProjectName id={rule.source_project} /> / <Mono>{rule.source_role}</Mono>
                </span>
                <span className="mx-2.5 text-faint">⇒</span>
                <span className="font-semibold">
                  <ProjectName id={rule.target_project} /> / <Mono>{rule.target_role}</Mono>
                </span>
              </span>

              <span className="w-[90px] text-right text-[15px]">{rule.holder_count ?? 0}</span>

              <span className="flex w-[150px] justify-end">
                {/*
                  Amber for "Immediate": it is not an error, but it is the
                  setting where a bad rule reaches every holder before anybody
                  can look at it. Queue is the quiet default.
                */}
                <Badge tone={rule.confirmation_mode === "auto" ? "warn" : "neutral"}>
                  {rule.confirmation_mode === "auto" ? "Immediate" : "Queue"}
                </Badge>
              </span>
            </button>
          ))}
        </ListStates>
      </Card>

      <p className="max-w-[900px] text-[14px] leading-[1.55] text-faint">
        A rule that triggers another rule is the single most surprising thing this system does, so
        validation names the chain before you save.
      </p>

      {editing && (
        <RuleEditor
          rule={editing === "new" ? null : editing}
          onClose={() => setEditing(null)}
        />
      )}
    </div>
  );
}

/**
 * One editor for both create and retarget. Two fields either side of a ⇒,
 * labelled as the sentence reads. Save is blocked until validation passes, and
 * the note says so rather than leaving a dead button.
 */
function RuleEditor({ rule, onClose }: { rule: MappingRuleRow | null; onClose: () => void }) {
  const projects = useProjects();
  const catalog = useGlobalRoleCatalog();
  const create = useCreateMappingRule();
  const update = useUpdateMappingRule();
  const setMode = useSetRuleConfirmationMode();
  const validate = useValidateMappingRule();

  const [sourceProject, setSourceProject] = useState(rule?.source_project ?? "");
  const [sourceRole, setSourceRole] = useState(rule?.source_role ?? "");
  const [targetProject, setTargetProject] = useState(rule?.target_project ?? "");
  const [targetRole, setTargetRole] = useState(rule?.target_role ?? "");
  const [mode, setModeValue] = useState<"auto" | "manual">(rule?.confirmation_mode ?? "manual");
  const [validated, setValidated] = useState<{ ok: boolean; notes: string[] } | null>(null);

  const existingRules = useMappingRules().data ?? [];

  const rolesFor = (projectId: string) =>
    (catalog.data ?? []).filter((role) => role.project_id === projectId);

  // Names, not ids, in every validation sentence: "Chains into R-022, which
  // would also give them Studio Access / door" is readable; the same line
  // built from UUIDs is not.
  const roleLabel = (projectId: string, roleKey: string) => {
    const entry = (catalog.data ?? []).find(
      (role) => role.project_id === projectId && role.role_key === roleKey,
    );
    const project =
      entry?.project_name ||
      (projects.data ?? []).find((p) => p.project.id === projectId)?.project.name ||
      projectId;
    return `${project} / ${roleKey}`;
  };

  const complete = Boolean(sourceProject && sourceRole && targetProject && targetRole);
  const busy = create.isPending || update.isPending || validate.isPending;

  // Any edit invalidates a previous verdict: a rule validated against different
  // roles has told you nothing about this one.
  function change<T>(setter: (value: T) => void) {
    return (value: T) => {
      setValidated(null);
      setter(value);
    };
  }

  async function runValidation() {
    const input = {
      source_project: sourceProject,
      source_role: sourceRole,
      target_project: targetProject,
      target_role: targetRole,
    };
    const check = await validate.mutateAsync(input);
    if (check.would_cycle || check.self_reference) {
      setValidated({
        ok: false,
        notes: [
          check.reason ??
            (check.self_reference
              ? "A role can't produce itself."
              : "That would close a loop — the rules would keep firing."),
        ],
      });
      return false;
    }

    const notes: string[] = [];
    const targetRoleName = roleLabel(targetProject, targetRole);
    const holders = (catalog.data ?? []).find(
      (role) => role.project_id === sourceProject && role.role_key === sourceRole,
    )?.assigned_user_count;
    if (holders) {
      notes.push(
        `Would grant ${targetRoleName} to ${holders} ${holders === 1 ? "person" : "people"} immediately on save.`,
      );
    }

    // Naming the chain matters more than any other line here: a rule that
    // triggers another rule is the single most surprising thing this system
    // does, and finding out afterwards is how people stop trusting it.
    for (const chained of existingRules) {
      if (chained.id === rule?.id) continue;
      if (chained.source_project === targetProject && chained.source_role === targetRole) {
        notes.push(
          `Chains into ${shortRuleId(chained.id)}, which would then also give them ${roleLabel(
            chained.target_project,
            chained.target_role,
          )}.`,
        );
      }
    }

    setValidated({ ok: true, notes });
    return true;
  }

  return (
    <Modal open onClose={onClose} busy={busy} size="md" labelledBy="rule-title">
      <ModalHeader
        title={rule ? `Editing ${shortRuleId(rule.id)}` : "New automatic rule"}
        titleId="rule-title"
        lede="Anybody who holds the first role gets the second, from now on and retroactively."
      />

      <div className="flex flex-col gap-3.5 px-6">
        <div className="grid grid-cols-2 gap-3">
          <div>
            <FieldLabel htmlFor="rule-source-project">If someone holds — project</FieldLabel>
            <Select
              id="rule-source-project"
              value={sourceProject}
              onChange={(event) => {
                change(setSourceProject)(event.target.value);
                setSourceRole("");
              }}
            >
              <option value="">Choose…</option>
              {(projects.data ?? []).map((entry) => (
                <option key={entry.project.id} value={entry.project.id}>
                  {entry.project.name}
                </option>
              ))}
            </Select>
          </div>
          <div>
            <FieldLabel htmlFor="rule-source-role">Role</FieldLabel>
            <Select
              id="rule-source-role"
              value={sourceRole}
              disabled={!sourceProject}
              onChange={(event) => change(setSourceRole)(event.target.value)}
            >
              <option value="">{sourceProject ? "Choose…" : "Pick a project"}</option>
              {rolesFor(sourceProject).map((role) => (
                <option key={role.role_key} value={role.role_key}>
                  {role.display_name || humanizeKey(role.role_key)}
                </option>
              ))}
            </Select>
          </div>
        </div>

        <div aria-hidden className="text-center text-[18px] text-faint">
          ⇒
        </div>

        <div className="grid grid-cols-2 gap-3">
          <div>
            <FieldLabel htmlFor="rule-target-project">then also give them — project</FieldLabel>
            <Select
              id="rule-target-project"
              value={targetProject}
              onChange={(event) => {
                change(setTargetProject)(event.target.value);
                setTargetRole("");
              }}
            >
              <option value="">Choose…</option>
              {(projects.data ?? []).map((entry) => (
                <option key={entry.project.id} value={entry.project.id}>
                  {entry.project.name}
                </option>
              ))}
            </Select>
          </div>
          <div>
            <FieldLabel htmlFor="rule-target-role">Role</FieldLabel>
            <Select
              id="rule-target-role"
              value={targetRole}
              disabled={!targetProject}
              onChange={(event) => change(setTargetRole)(event.target.value)}
            >
              <option value="">{targetProject ? "Choose…" : "Pick a project"}</option>
              {rolesFor(targetProject).map((role) => (
                <option key={role.role_key} value={role.role_key}>
                  {role.display_name || humanizeKey(role.role_key)}
                </option>
              ))}
            </Select>
          </div>
        </div>

        <div>
          <FieldLabel>When this rule fires</FieldLabel>
          <Segmented<"auto" | "manual">
            label="Confirmation mode"
            value={mode}
            onChange={setModeValue}
            options={[
              { value: "manual", label: "Queue for review" },
              { value: "auto", label: "Apply immediately" },
            ]}
          />
          <FieldHint>
            {mode === "manual"
              ? "Its writes wait under Pending changes until somebody confirms them."
              : "Its writes reach the identity provider the moment the rule fires."}
          </FieldHint>
        </div>

        {validated && (
          <div
            className={`${validated.ok ? "warn-note" : "danger-note"} px-4 py-3.5 text-[14px] leading-[1.55]`}
          >
            <div
              className={`type-label mb-1 ${validated.ok ? "text-warn-text" : "text-danger-text"}`}
            >
              {validated.ok
                ? validated.notes.length
                  ? `Validated — ${validated.notes.length} ${validated.notes.length === 1 ? "thing" : "things"} to know`
                  : "Validated"
                : "Not valid"}
            </div>
            {validated.notes.length ? (
              <ul className="flex flex-col gap-1 text-muted">
                {validated.notes.map((note) => (
                  <li key={note}>{note}</li>
                ))}
              </ul>
            ) : (
              <p className="text-muted">Nothing changes for anybody who already holds the role.</p>
            )}
          </div>
        )}
      </div>

      <ModalFooter
        note={
          validated?.ok ? undefined : "Save is blocked until validation passes."
        }
      >
        <Button
          variant="accent"
          disabled={!complete || !validated?.ok}
          isPending={busy}
          onClick={async () => {
            try {
              const input = {
                source_project: sourceProject,
                source_role: sourceRole,
                target_project: targetProject,
                target_role: targetRole,
              };
              if (rule) {
                await update.mutateAsync({ id: rule.id, ...input });
                if (mode !== rule.confirmation_mode) {
                  await setMode.mutateAsync({ id: rule.id, mode });
                }
                toast.success("Rule updated.");
              } else {
                await create.mutateAsync({ ...input, confirmation_mode: mode });
                toast.success("Rule created.");
              }
              onClose();
            } catch (error) {
              toast.error(error instanceof Error ? error.message : "The rule wasn't saved.");
            }
          }}
        >
          {rule ? "Save rule" : "Create rule"}
        </Button>
        <Button
          disabled={!complete}
          isPending={validate.isPending}
          onClick={async () => {
            try {
              await runValidation();
            } catch (error) {
              toast.error(error instanceof Error ? error.message : "Validation didn't run.");
            }
          }}
        >
          {validated ? "Re-validate" : "Validate"}
        </Button>
        <Button onClick={onClose}>Cancel</Button>
      </ModalFooter>
    </Modal>
  );
}

/** "R-014" reads as a rule; a raw UUID reads as noise. */
function shortRuleId(id: string): string {
  return `R-${id.replace(/-/g, "").slice(0, 4)}`;
}
