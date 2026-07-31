"use client";

import { useState } from "react";
import { toast } from "sonner";

import { EmptyState, ListStates, RowSkeleton } from "@/components/states";
import { Badge, Mono } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Card, CardHeader } from "@/components/ui/Card";
import { FieldHint, FieldLabel } from "@/components/ui/Input";
import { Modal, ModalFooter, ModalHeader } from "@/components/ui/Modal";
import { PageHeader } from "@/components/ui/PageHeader";
import { Segmented, Select } from "@/components/ui/Select";
import {
  useCreateMappingRule,
  useMappingRules,
  useValidateMappingRule,
} from "@/lib/queries/useMappingRules";
import { useProjects } from "@/lib/queries/useProjects";
import { useGlobalRoleCatalog } from "@/lib/queries/useRoles";
import { humanizeKey } from "@/lib/format";

/**
 * S2 · Automation › Automatic rules.
 *
 * "Role A in project X ⇒ role B in project Y." A rule is the reason a role
 * shows up with a dashed chip and nobody remembers clicking it, so every rule
 * is written here as the sentence it produces.
 */
export default function AutomaticRulesPage() {
  const rules = useMappingRules();
  const [creating, setCreating] = useState(false);

  const rows = rules.data ?? [];

  return (
    <div className="flex flex-col gap-[18px]">
      <PageHeader
        title="Automatic rules"
        meta="Holding one role produces another, without anybody clicking."
        actions={
          <Button variant="accent" onClick={() => setCreating(true)}>
            New rule
          </Button>
        }
      />

      <Card>
        <CardHeader title="Active rules" count={rows.length} />
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
              action={{ label: "Create a rule", onClick: () => setCreating(true) }}
            />
          }
        >
          {rows.map((rule) => (
            <div key={rule.id} className="row-divider flex flex-wrap items-center gap-4 px-5 py-3.5">
              <div className="min-w-[320px] flex-1 text-[14.5px]">
                <span className="text-muted">
                  {rule.source_project} / <Mono>{rule.source_role}</Mono>
                </span>
                <span className="mx-2.5 text-faint">⇒</span>
                <span className="font-semibold">
                  {rule.target_project} / <Mono>{rule.target_role}</Mono>
                </span>
              </div>
              <Badge tone={rule.confirmation_mode === "auto" ? "accent" : "neutral"}>
                {rule.confirmation_mode === "auto" ? "Applies immediately" : "Queues for review"}
              </Badge>
              <Mono className="text-faint">{rule.id.slice(0, 8)}</Mono>
            </div>
          ))}
        </ListStates>
      </Card>

      <NewRuleDialog open={creating} onClose={() => setCreating(false)} />
    </div>
  );
}

function NewRuleDialog({ open, onClose }: { open: boolean; onClose: () => void }) {
  const projects = useProjects();
  const catalog = useGlobalRoleCatalog();
  const create = useCreateMappingRule();
  const validate = useValidateMappingRule();

  const [sourceProject, setSourceProject] = useState("");
  const [sourceRole, setSourceRole] = useState("");
  const [targetProject, setTargetProject] = useState("");
  const [targetRole, setTargetRole] = useState("");
  const [mode, setMode] = useState<"auto" | "manual">("manual");

  const rolesFor = (projectId: string) =>
    (catalog.data ?? []).filter((role) => role.project_id === projectId);

  const complete = sourceProject && sourceRole && targetProject && targetRole;

  if (!open) return null;

  return (
    <Modal open onClose={onClose} busy={create.isPending} size="md" labelledBy="new-rule-title">
      <ModalHeader
        title="New automatic rule"
        titleId="new-rule-title"
        lede="Anybody who holds the first role gets the second, from now on and retroactively."
      />
      <div className="flex flex-col gap-3.5 px-6">
        <div className="grid grid-cols-2 gap-3">
          <div>
            <FieldLabel htmlFor="rule-source-project">When somebody holds — project</FieldLabel>
            <Select
              id="rule-source-project"
              value={sourceProject}
              onChange={(event) => {
                setSourceProject(event.target.value);
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
              onChange={(event) => setSourceRole(event.target.value)}
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

        <div className="grid grid-cols-2 gap-3">
          <div>
            <FieldLabel htmlFor="rule-target-project">They also get — project</FieldLabel>
            <Select
              id="rule-target-project"
              value={targetProject}
              onChange={(event) => {
                setTargetProject(event.target.value);
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
              onChange={(event) => setTargetRole(event.target.value)}
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
          <FieldLabel>When it fires</FieldLabel>
          <Segmented<"auto" | "manual">
            label="Confirmation mode"
            value={mode}
            onChange={setMode}
            options={[
              { value: "manual", label: "Queue for review" },
              { value: "auto", label: "Apply immediately" },
            ]}
          />
          <FieldHint>
            {mode === "manual"
              ? "Its writes wait under Pending changes until somebody resumes them."
              : "Its writes go straight to the identity provider as the rule fires."}
          </FieldHint>
        </div>

        {complete && (
          <div className="accent-note px-4 py-3.5 text-[14px] leading-[1.55] text-ink/[.78]">
            Everybody holding{" "}
            <strong className="font-semibold text-ink">
              {sourceProject} / {sourceRole}
            </strong>{" "}
            — now and in future — also gets{" "}
            <strong className="font-semibold text-ink">
              {targetProject} / {targetRole}
            </strong>
            . Their rows will read <em>Automatic</em>, because nobody clicked it.
          </div>
        )}
      </div>

      <ModalFooter>
        <Button
          variant="accent"
          disabled={!complete}
          isPending={create.isPending || validate.isPending}
          onClick={async () => {
            try {
              // Validate first. A rule that points at itself or closes a cycle
              // would fire forever; catching that before the write means the
              // operator never has to undo a live cascade.
              const check = await validate.mutateAsync({
                source_project: sourceProject,
                source_role: sourceRole,
                target_project: targetProject,
                target_role: targetRole,
              });
              if (check.would_cycle || check.self_reference) {
                toast.error(
                  check.reason ??
                    (check.self_reference
                      ? "A role can't produce itself."
                      : "That would close a loop — the rules would keep firing."),
                );
                return;
              }
              await create.mutateAsync({
                source_project: sourceProject,
                source_role: sourceRole,
                target_project: targetProject,
                target_role: targetRole,
                confirmation_mode: mode,
              });
              toast.success("Rule created.");
              onClose();
            } catch (error) {
              toast.error(error instanceof Error ? error.message : "The rule wasn't created.");
            }
          }}
        >
          Create rule
        </Button>
        <Button onClick={onClose}>Cancel</Button>
      </ModalFooter>
    </Modal>
  );
}
