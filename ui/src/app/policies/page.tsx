"use client";

import { useMemo, useState } from "react";

import { EmptyState, ListStates, RowSkeleton } from "@/components/states";
import { Badge, Mono } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Card, CardColumns } from "@/components/ui/Card";
import { FieldHint, FieldLabel } from "@/components/ui/Input";
import { ActionOutcome } from "@/components/ui/ActionOutcome";
import { Modal, ModalFooter, ModalHeader } from "@/components/ui/Modal";
import { outcomeFromError, type ActionOutcome as Outcome } from "@/lib/outcome";
import { PageHeader } from "@/components/ui/PageHeader";
import { Segmented, Select } from "@/components/ui/Select";
import {
  RowCheckbox,
  SelectAllRow,
  SelectModeToggle,
  SelectionAction,
  SelectionBar,
} from "@/components/ui/SelectionBar";
import { useRowSelection } from "@/lib/useRowSelection";
import { ProjectName } from "@/components/names";
import {
  useBulkSetConfirmationMode,
  type ConfirmationMode,
} from "@/lib/queries/useConfirmationMode";
import {
  useCreateMappingRule,
  useDeleteMappingRule,
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

  const rows = useMemo(() => rules.data ?? [], [rules.data]);

  /**
   * Confirmation mode is the one rule setting that is routinely wrong in bulk:
   * a batch created before the global default was decided, or a set of rules
   * being moved to Immediate together once they've been watched for a term.
   * Opening ten editors to flip ten switches is the errand this removes.
   *
   * The row click still opens the editor. Selection lives on the checkbox
   * alone, deliberately — on the other queues a row click selects because the
   * row's actions are on the row, and here the row's action is "open me".
   */
  const selection = useRowSelection(useMemo(() => rows.map((rule) => rule.id), [rows]));
  // Selection is a mode, announced by a named control, rather than a column of
  // checkboxes that is always there. Leaving the mode keeps what was chosen —
  // the operator who taps Done to read a rule properly has not changed their
  // mind about the other nine.
  const [selecting, setSelecting] = useState(false);
  const bulkMode = useBulkSetConfirmationMode();
  const [outcome, setOutcome] = useState<Outcome | null>(null);

  const chosen = rows.filter((rule) => selection.selected.has(rule.id));
  const chosenThings = `${chosen.length} ${chosen.length === 1 ? "rule" : "rules"}`;
  const immediate = chosen.filter((rule) => rule.confirmation_mode === "auto").length;
  const composition =
    chosen.length === 0
      ? ""
      : [
          immediate > 0 ? `${immediate} apply at once` : "",
          chosen.length - immediate > 0
            ? `${chosen.length - immediate} wait under Pending changes`
            : "",
        ]
          .filter(Boolean)
          .join(" · ");

  async function applyMode(mode: ConfirmationMode) {
    const ids = chosen.map((rule) => rule.id);
    const one = ids.length === 1;
    const target =
      mode === "auto"
        ? `${one ? "applies" : "apply"} at once`
        : `${one ? "waits" : "wait"} under Pending changes`;
    try {
      await bulkMode.mutateAsync({ kind: "rule", ids, mode });
      setOutcome({
        kind: "applied",
        message: `${ids.length} ${one ? "rule" : "rules"} now ${target}`,
        detail: "This changes what the rules do next, not what they have already done.",
      });
      selection.clear();
    } catch (error) {
      setOutcome(outcomeFromError(error));
    }
  }

  return (
    <div className="flex flex-col gap-[18px]">
      <PageHeader
        title="Automatic rules"
        lede="Each rule is a sentence: if someone holds the first role, they also get the second. Rules that apply at once give access with no review; rules that wait put it under Pending changes first."
        actions={
          <>
            {/* No rules, nothing to select. A mode control over an empty list
                is a button that does nothing and says otherwise. */}
            {rows.length > 0 && (
              <SelectModeToggle active={selecting} onToggle={() => setSelecting((on) => !on)} />
            )}
            <Button variant="accent" onClick={() => setEditing("new")}>
              New rule
            </Button>
          </>
        }
      />

      {outcome && <ActionOutcome outcome={outcome} />}

      <Card>
        <CardColumns>
          {selecting && <span className="w-11 shrink-0 desktop:w-[18px]" />}
          <span className="w-[110px]">Rule</span>
          <span className="flex-1">If … then</span>
          <span className="w-[130px] text-right">Given by this rule</span>
          <span className="w-[210px] text-right">When it applies</span>
        </CardColumns>

        {selecting && rows.length > 0 && (
          <SelectAllRow
            inScope={rows.length}
            noun={["rule", "rules"]}
            allSelected={selection.allSelected}
            {...selection.headerCheckboxProps}
          />
        )}

        <div data-selection-scope {...selection.containerProps}>
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
              guidance="Every role somebody holds was given to them on purpose. Add a rule when holding one role should always come with a second one."
              action={{ label: "New rule", onClick: () => setEditing("new") }}
            />
          }
        >
          {rows.map((rule) => (
            <div
              key={rule.id}
              className={`row-divider flex w-full min-h-[60px] items-center gap-3 px-5 tablet:gap-[18px] ${
                selection.isSelected(rule.id) ? "bg-accent-soft/30" : ""
              }`}
            >
              {selecting && (
                <span className="w-11 shrink-0 desktop:w-[18px]">
                  <RowCheckbox label="Select this rule" {...selection.checkboxProps(rule.id)} />
                </span>
              )}
              <button
                type="button"
                onClick={() => setEditing(rule)}
                className="flex min-w-0 flex-1 flex-col items-start gap-1.5 py-3.5 text-left motion-tint hover:bg-[var(--hover)] tablet:flex-row tablet:flex-wrap tablet:items-center tablet:gap-[18px]"
              >
                <Mono className="truncate text-faint tablet:w-[110px] tablet:shrink-0">
                  {shortRuleId(rule.id)}
                </Mono>

                <span className="w-full text-[14.5px] tablet:min-w-[320px] tablet:flex-1">
                  <span className="text-muted">
                    <ProjectName id={rule.source_project} /> / <Mono>{rule.source_role}</Mono>
                  </span>
                  <span aria-hidden className="mx-2.5 text-faint">⇒</span>
                  <span className="sr-only">then </span>
                  <span className="font-semibold">
                    <ProjectName id={rule.target_project} /> / <Mono>{rule.target_role}</Mono>
                  </span>
                </span>

                <span className="text-[15px] tablet:w-[130px] tablet:text-right">
                  {rule.holder_count ?? 0}
                  <span className="text-[13px] text-faint tablet:hidden">
                    {(rule.holder_count ?? 0) === 1 ? " person has" : " people have"} the second role from this rule
                  </span>
                </span>

                <span className="flex tablet:w-[210px] tablet:justify-end">
                  {/*
                    Amber for "Immediate": it is not an error, but it is the
                    setting where a bad rule reaches every holder before anybody
                    can look at it. Queue is the quiet default.
                  */}
                  <Badge tone={rule.confirmation_mode === "auto" ? "warn" : "neutral"}>
                    {rule.confirmation_mode === "auto"
                      ? "Applies at once"
                      : "Waits under Pending changes"}
                  </Badge>
                </span>
              </button>
            </div>
          ))}
        </ListStates>
        </div>
      </Card>

      <SelectionBar
        count={selecting ? selection.count : 0}
        noun={["rule", "rules"]}
        composition={composition}
        onClear={selection.clear}
      >
        {/*
          Immediate reads as the louder of the two because it is: a rule set to
          fire immediately reaches every holder before anybody can look at it.
        */}
        {/* These two apply on tap — there is no plan to read first — so each
            one states how many rules it is about to change rather than naming
            the setting and leaving the count on the other side of the bar. */}
        <SelectionAction
          tone="danger"
          disabled={bulkMode.isPending}
          onClick={() => applyMode("auto")}
        >
          Set {chosenThings} to apply at once
        </SelectionAction>
        <SelectionAction disabled={bulkMode.isPending} onClick={() => applyMode("manual")}>
          Set {chosenThings} to wait under Pending changes
        </SelectionAction>
      </SelectionBar>

      <p className="max-w-[900px] text-[14px] leading-[1.55] text-faint">
        One rule can set off another. Before you can save a rule, Check lists every rule yours
        would set off, and who would gain access.
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
  const [outcome, setOutcome] = useState<Outcome | null>(null);
  const setMode = useSetRuleConfirmationMode();
  const validate = useValidateMappingRule();
  /**
   * Retiring the rule takes over this same dialog rather than opening a second one on top of it.
   * The thing you are about to remove is the thing already on screen, and a modal over a modal
   * asks somebody to read a consequence through the form that produced it.
   */
  const [confirmingDelete, setConfirmingDelete] = useState(false);

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
              ? "The two roles are the same. Pick a different second role."
              : "That would make a loop: each rule would keep giving the other's role."),
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
      // "directly" is not decoration. `assigned_user_count` is
      // COUNT(DISTINCT user_id) FROM direct_role_grants — bundle holders and
      // rule-produced holders are not in it. Saying "N people hold this" over
      // that number under-counts silently, and the sentence it feeds is read
      // immediately before a Save.
      const who = `${holders} ${holders === 1 ? "person already holds" : "people already hold"} the first role`;
      notes.push(
        mode === "auto"
          ? `${who}, so saving gives them ${targetRoleName} at once.`
          : `${who}. Saving would give them ${targetRoleName}, and that would wait under Pending changes first.`,
      );
    }

    // Naming the chain matters more than any other line here: a rule that
    // triggers another rule is the single most surprising thing this system
    // does, and finding out afterwards is how people stop trusting it.
    for (const chained of existingRules) {
      if (chained.id === rule?.id) continue;
      if (chained.source_project === targetProject && chained.source_role === targetRole) {
        notes.push(
          `Also sets off ${shortRuleId(chained.id)}, which would then give them ${roleLabel(
            chained.target_project,
            chained.target_role,
          )}.`,
        );
      }
    }

    setValidated({ ok: true, notes });
    return true;
  }

  if (rule && confirmingDelete) {
    return (
      <DeleteRuleConfirm
        rule={rule}
        label={roleLabel(rule.target_project, rule.target_role)}
        onCancel={() => setConfirmingDelete(false)}
        onDeleted={onClose}
      />
    );
  }

  return (
    <Modal open onClose={onClose} busy={busy} size="md" labelledBy="rule-title">
      <ModalHeader
        title={rule ? `Editing ${shortRuleId(rule.id)}` : "New automatic rule"}
        titleId="rule-title"
        lede="Anybody who holds the first role gets the second — everyone who holds it today, and everyone who is given it later."
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
            <FieldLabel htmlFor="rule-source-role">Role they hold</FieldLabel>
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
            <FieldLabel htmlFor="rule-target-role">Role to give</FieldLabel>
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
          <FieldLabel>When this rule applies</FieldLabel>
          <Segmented<"auto" | "manual">
            label="When this rule applies"
            value={mode}
            // Through `change`, because the checked note reads the mode: a
            // verdict about "at once" says nothing about "waits".
            onChange={change(setModeValue)}
            options={[
              { value: "manual", label: "Waits under Pending changes" },
              { value: "auto", label: "Applies at once" },
            ]}
          />
          <FieldHint>
            {mode === "manual"
              ? "The access this rule gives waits under Pending changes until you send it. Nobody's access changes before then."
              : "The access this rule gives takes effect the moment the rule applies. Nobody reviews it first."}
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
                  ? `Checked — ${validated.notes.length} ${validated.notes.length === 1 ? "thing" : "things"} to know`
                  : "Checked"
                : "This rule can't be saved"}
            </div>
            {validated.notes.length ? (
              <ul className="flex flex-col gap-1 text-muted">
                {validated.notes.map((note) => (
                  <li key={note}>{note}</li>
                ))}
              </ul>
            ) : (
              <p className="text-muted">
                Nobody holds the first role, directly or through a bundle. Somebody given it by
                another rule is not counted here, so this could still reach people.
              </p>
            )}
          </div>
        )}
      </div>

      {outcome && <ActionOutcome outcome={outcome} className="mx-6 mb-1" />}

      <ModalFooter
        note={
          validated?.ok ? undefined : "Check the rule before you can save it."
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
                setOutcome({
                  kind: "applied",
                  message: "Rule updated",
                  detail: savedDetail(mode),
                });
              } else {
                await create.mutateAsync({ ...input, confirmation_mode: mode });
                setOutcome({
                  kind: "applied",
                  message: "Rule created",
                  detail: savedDetail(mode),
                });
              }
              onClose();
            } catch (error) {
              setOutcome(outcomeFromError(error));
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
              setOutcome(outcomeFromError(error));
            }
          }}
        >
          {validated ? "Check again" : "Check rule"}
        </Button>
        <Button onClick={onClose}>Cancel</Button>
        {/*
          Outline, and last. A rule is retired from the same place it is retargeted, because
          those are the two things you come here to do — but a solid red button beside Save is
          one slip away from revoking a term's worth of access.
        */}
        {rule && (
          <Button variant="danger" disabled={busy} onClick={() => setConfirmingDelete(true)}>
            Delete rule
          </Button>
        )}
      </ModalFooter>
    </Modal>
  );
}

/** What a saved rule does next, in the rule's own mode. */
function savedDetail(mode: "auto" | "manual"): string {
  return mode === "auto"
    ? "People who already hold the first role get the second at once; anyone given the first role later gets the second too."
    : "The access this rule gives waits under Pending changes; send it from there. Anyone given the first role later is handled the same way.";
}

/**
 * The consequence, before the click.
 *
 * The holder count is the number that matters and the one the index already shows: a rule with
 * eleven holders is eleven people who may lose the target role. "May" is exact — anybody who
 * also holds it via a bundle or a direct grant keeps it, and the backend works that out per
 * person. Promising a precise number here would mean recomputing the closure in the browser and
 * being wrong about it.
 */
function DeleteRuleConfirm({
  rule,
  label,
  onCancel,
  onDeleted,
}: {
  rule: MappingRuleRow;
  label: string;
  onCancel: () => void;
  onDeleted: () => void;
}) {
  const remove = useDeleteMappingRule();
  const [outcome, setOutcome] = useState<Outcome | null>(null);
  const holders = rule.holder_count ?? 0;
  // The rule is gone in both: `queued` describes the withdrawals it caused,
  // not the delete.
  const gone = outcome?.kind === "applied" || outcome?.kind === "queued";

  return (
    <Modal
      open
      onClose={gone ? onDeleted : onCancel}
      busy={remove.isPending}
      size="md"
      labelledBy="rule-delete-title"
    >
      <ModalHeader
        title={`Delete ${shortRuleId(rule.id)}?`}
        titleId="rule-delete-title"
        lede={`Some people may hold ${label} only because of this rule.`}
      />

      <div className="px-6">
        <div className="danger-note px-4 py-3.5 text-[14px] leading-[1.55]">
          <div className="type-label mb-1 text-danger-text">What happens</div>
          <ul className="flex flex-col gap-1 text-muted">
            <li>The rule stops giving {label} to anybody.</li>
            <li>
              {holders === 0
                ? "This rule is giving the second role to nobody today, so deleting it revokes nothing."
                : `${holders} ${holders === 1 ? "person has" : "people have"} ${label} from this rule. ` +
                  `It is revoked for them unless a bundle or a direct grant also gives it.`}
            </li>
            <li>
              {rule.confirmation_mode === "auto"
                ? "Their access ends at once, with no review."
                : "Revoking waits under Pending changes until you send it. Until then, their access stays."}
            </li>
          </ul>
        </div>
      </div>

      {outcome && <ActionOutcome outcome={outcome} className="mx-6 mb-1" />}

      <ModalFooter note="If you only want the rule to give a different role, edit it instead of deleting it.">
        {!gone && (
        <Button
          variant="dangerConfirm"
          isPending={remove.isPending}
          onClick={async () => {
            try {
              const result = await remove.mutateAsync(rule.id);
              const n = result.cascade?.enqueued ?? 0;
              const auto = result.cascade?.mode === "auto";
              setOutcome(
                n === 0
                  ? {
                      kind: "applied",
                      message: `${shortRuleId(rule.id)} deleted`,
                      detail: "Nobody's access changed — the rule was giving nothing.",
                    }
                  : {
                      // Queued unless the rule fired unattended: the
                      // withdrawals it caused are recorded and wait for
                      // somebody to resume the queue, and calling them applied
                      // would say access is gone that is still there.
                      kind: auto ? "applied" : "queued",
                      message: `${shortRuleId(rule.id)} deleted — ${n} ${
                        n === 1 ? "revocation" : "revocations"
                      } ${auto ? "sent" : "waiting to be sent"}`,
                      detail: auto
                        ? "The rule applied without review, so the access it gave was revoked without review too."
                        : "The revocations wait under Pending changes until you send them. Until then, that access stays.",
                    },
              );
            } catch (error) {
              setOutcome(outcomeFromError(error));
            }
          }}
        >
          Delete rule and revoke access
        </Button>
        )}
        {/* `onDeleted` closes the editor this dialog opened over, and it was
            never called — so deleting a rule left the operator looking at an
            editor for a rule that no longer exists. Called from the dismiss
            button rather than from the mutation, because closing the editor
            takes the outcome with it. */}
        <Button disabled={remove.isPending} onClick={gone ? onDeleted : onCancel}>
          {gone ? "Done" : "Keep the rule"}
        </Button>
      </ModalFooter>
    </Modal>
  );
}

/** "R-014" reads as a rule; a raw UUID reads as noise. */
function shortRuleId(id: string): string {
  return `R-${id.replace(/-/g, "").slice(0, 4)}`;
}
