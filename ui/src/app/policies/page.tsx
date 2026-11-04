"use client";

import { useMemo, useState } from "react";

import { RolePickerOne, roleId, type PickerPlace } from "@/components/bundles/RolePickerOne";
import { splitRoleId } from "@/components/bundles/RolePicker";
import { RetiredRulesFold } from "@/components/rules/RetiredRules";
import { EmptyState, ListStates, RowSkeleton } from "@/components/states";
import { Mono } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { KEPT, PushedStep, useCancelStep } from "@/components/ui/CancelQueuedStep";
import { Card, CardColumns } from "@/components/ui/Card";
import { FieldLabel } from "@/components/ui/Input";
import { ActionOutcome } from "@/components/ui/ActionOutcome";
import { Modal, ModalFooter, ModalHeader } from "@/components/ui/Modal";
import { Notice } from "@/components/ui/Notice";
import { ReadFreshness, readStateOfQuery } from "@/components/ui/ReadFreshness";
import { outcomeFromError, type ActionOutcome as Outcome } from "@/lib/outcome";
import { PageHeader } from "@/components/ui/PageHeader";
import { WaitsMark } from "@/components/ui/WaitsMark";
import { ProjectName, RoleRef } from "@/components/names";
import {
  useCreateMappingRule,
  useDeleteMappingRule,
  useMappingRules,
  useUpdateMappingRule,
  useValidateMappingRule,
  type MappingRuleRow,
} from "@/lib/queries/useMappingRules";
import { useProjects } from "@/lib/queries/useProjects";
import { useRolesThatWait } from "@/lib/queries/useRolesThatWait";
import { useGlobalRoleCatalog } from "@/lib/queries/useRoles";
import { humanizeKey } from "@/lib/format";
import { UNKNOWN_PROJECT } from "@/components/names/Unresolved";
import { sortByName } from "@/lib/sort";
import { useShownNames } from "@/components/names/shownNames";

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

  const names = useShownNames();
  // A to Z by the rule as it reads: "Project / role ⇒ Project / role".
  const rows = useMemo(
    () =>
      sortByName(
        rules.data ?? [],
        (r) => `${names.roleRef(r.source_project, r.source_role)} ⇒ ${names.roleRef(r.target_project, r.target_role)}`,
      ),
    [rules.data, names],
  );

  const catalog = useGlobalRoleCatalog();
  const waits = (projectId: string, roleKey: string) =>
    Boolean(
      (catalog.data ?? []).find(
        (r) => r.project_id === projectId && r.role_key === roleKey,
      )?.waits_for_send,
    );

  return (
    <div className="flex flex-col gap-4">
      <PageHeader
        title="Automatic rules"
        lede="Each rule is a sentence: if someone holds the first role, they also get the second."
        actions={
          <Button variant="accent" onClick={() => setEditing("new")}>
            New rule
          </Button>
        }
      />

      {rows.length > 0 && (
        <ReadFreshness state={readStateOfQuery(rules)} subject="Automatic rules and who has them" className="px-5" />
      )}

      <Card>
        {rows.length > 0 && (
          <CardColumns>
            <span className="flex-1">If &hellip; then</span>
            <span className="w-[110px] text-right">People</span>
          </CardColumns>
        )}

        <ListStates
          isLoading={rules.isLoading}
          error={rules.error}
          isEmpty={rows.length === 0}
          onRetry={() => rules.refetch()}
          errorTitle="Couldn't load automatic rules."
          skeleton={
            <RowSkeleton rows={3} avatar={false} label="Loading rules" />
          }
          empty={
            <EmptyState
              title="No automatic rules."
              guidance="Every role somebody holds was given to them on purpose. Add a rule when holding one role should always come with a second one."
            />
          }
        >
          {rows.map((rule) => (
            <div
              key={rule.id}
              className="row-divider flex w-full min-h-[60px] items-center gap-3 px-5 tablet:gap-[18px]"
            >
              <button
                type="button"
                onClick={() => setEditing(rule)}
                className="flex min-w-0 flex-1 flex-col items-start gap-1.5 py-3.5 text-left hover-tint tablet:flex-row tablet:flex-wrap tablet:items-center tablet:gap-[18px]"
              >
                <span className="w-full text-[14.5px] tablet:min-w-[320px] tablet:flex-1">
                  <span className="text-muted">
                    <ProjectName id={rule.source_project} /> /{" "}
                    <Mono>{rule.source_role}</Mono>
                  </span>
                  <span aria-hidden className="mx-2.5 text-faint">
                    ⇒
                  </span>
                  <span className="sr-only">then </span>
                  <span className="font-semibold">
                    <ProjectName id={rule.target_project} /> /{" "}
                    <Mono>{rule.target_role}</Mono>
                    {waits(rule.target_project, rule.target_role) && (
                      <WaitsMark className="ml-2" />
                    )}
                  </span>
                </span>

                <span className="text-[15px] tablet:w-[110px] tablet:text-right">
                  {rule.holder_count ?? 0}
                  <span className="text-[13px] text-faint tablet:hidden">
                    {(rule.holder_count ?? 0) === 1
                      ? " person has"
                      : " people have"}{" "}
                    the second role from this rule
                  </span>
                </span>
              </button>
            </div>
          ))}
        </ListStates>
      </Card>

      <RetiredRulesFold />

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
function RuleEditor({
  rule,
  onClose,
}: {
  rule: MappingRuleRow | null;
  onClose: () => void;
}) {
  const projects = useProjects();
  const catalog = useGlobalRoleCatalog();
  const create = useCreateMappingRule();
  const update = useUpdateMappingRule();
  const [outcome, setOutcome] = useState<Outcome | null>(null);
  // Saving a rule gives the role to everyone it covers, which can cancel a
  // removal still waiting for you to send: the sheet turns to the step first.
  const step = useCancelStep({
    host: rule ? "Edit this rule" : "New automatic rule",
    goAhead: "save the rule",
  });
  const validate = useValidateMappingRule();
  /**
   * Retiring the rule takes over this same dialog rather than opening a second one on top of it.
   * The thing you are about to remove is the thing already on screen, and a modal over a modal
   * asks somebody to read a consequence through the form that produced it.
   */
  const [confirmingDelete, setConfirmingDelete] = useState(false);

  const [sourceProject, setSourceProject] = useState(
    rule?.source_project ?? "",
  );
  const [sourceRole, setSourceRole] = useState(rule?.source_role ?? "");
  const [targetProject, setTargetProject] = useState(
    rule?.target_project ?? "",
  );
  const [targetRole, setTargetRole] = useState(rule?.target_role ?? "");
  const [validated, setValidated] = useState<{
    ok: boolean;
    notes: string[];
  } | null>(null);

  const existingRules = useMappingRules().data ?? [];
  // Saved used to close the dialog in the same tick as setOutcome, which
  // discarded `savedDetail()` — the operator's only account of whether the
  // rule waits under Pending changes or already applied — before it ever
  // painted. Now the footer swaps to Close, same as DeleteRuleConfirm below.
  // Both `applied` (role sends at once) and `queued` (role waits) mean the rule
  // itself saved; only a thrown error leaves the form armed to retry.
  const saved = outcome?.kind === "applied" || outcome?.kind === "queued";

  // Every project that has roles, with the roles a rule can name.
  const places = useMemo(() => {
    const byProject = new Map<string, PickerPlace>();
    for (const role of catalog.data ?? []) {
      const place = byProject.get(role.project_id) ?? {
        id: role.project_id,
        name:
          role.project_name ||
          (projects.data ?? []).find((p) => p.project.id === role.project_id)?.project.name ||
          UNKNOWN_PROJECT,
        roles: [],
      };
      place.roles.push({
        key: role.role_key,
        name: role.display_name || humanizeKey(role.role_key),
        description: role.description,
        people: role.assigned_user_count,
        waits: role.waits_for_send,
        note: { kind: "none" },
      });
      byProject.set(role.project_id, place);
    }
    return Array.from(byProject.values());
  }, [catalog.data, projects.data]);

  // Names, not ids, in every validation sentence: "Chains into R-022, which
  // would also give them Studio Access / door" is readable; the same line
  // built from UUIDs is not.
  const roleLabel = (projectId: string, roleKey: string) => {
    const entry = (catalog.data ?? []).find(
      (role) => role.project_id === projectId && role.role_key === roleKey,
    );
    const project =
      entry?.project_name ||
      (projects.data ?? []).find((p) => p.project.id === projectId)?.project
        .name ||
      UNKNOWN_PROJECT;
    return `${project} / ${roleKey}`;
  };

  const roleEntry = (projectId: string, roleKey: string) =>
    (catalog.data ?? []).find(
      (r) => r.project_id === projectId && r.role_key === roleKey,
    );
  const targetEntry = roleEntry(targetProject, targetRole);
  const targetWaits = Boolean(targetEntry?.waits_for_send);
  const targetName = targetEntry?.display_name || humanizeKey(targetRole);

  const complete = Boolean(
    sourceProject && sourceRole && targetProject && targetRole,
  );
  const busy = create.isPending || update.isPending || validate.isPending;

  // Any edit invalidates a previous verdict: a rule validated against different
  // roles has told you nothing about this one.
  function change<T>(setter: (value: T) => void) {
    return (value: T) => {
      setValidated(null);
      setter(value);
    };
  }

  // Choosing or removing a role sets both halves (project and role) at once.
  function pick(id: string | null, setProject: (v: string) => void, setRole: (v: string) => void) {
    const [project, role] = id ? splitRoleId(id) : ["", ""];
    change(setProject)(project);
    setRole(role);
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
      (role) =>
        role.project_id === sourceProject && role.role_key === sourceRole,
    )?.assigned_user_count;
    if (holders) {
      // "directly" is not decoration. `assigned_user_count` is
      // COUNT(DISTINCT user_id) FROM direct_role_grants — bundle holders and
      // rule-produced holders are not in it. Saying "N people hold this" over
      // that number under-counts silently, and the sentence it feeds is read
      // immediately before a Save.
      const who = `${holders} ${holders === 1 ? "person already holds" : "people already hold"} the first role`;
      notes.push(`${who}. Saving would give them ${targetRoleName}.`);
    }

    // Naming the chain matters more than any other line here: a rule that
    // triggers another rule is the single most surprising thing this system
    // does, and finding out afterwards is how people stop trusting it.
    for (const chained of existingRules) {
      if (chained.id === rule?.id) continue;
      if (
        chained.source_project === targetProject &&
        chained.source_role === targetRole
      ) {
        notes.push(
          `Also sets off another rule, which would then give them ${roleLabel(
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
      <PushedStep step={step}>
      <ModalHeader
        title={rule ? "Edit this rule" : "New automatic rule"}
        titleId="rule-title"
        chip={rule && ruleChip(rule)}
        lede="Anybody who holds the first role gets the second: everyone who holds it today, and everyone who is given it later."
      />

      <div className="flex flex-col gap-3.5 px-6">
        <div>
          <FieldLabel id="rule-source-label">If someone holds</FieldLabel>
          <RolePickerOne
            mode="one"
            audience="operator"
            places={places}
            value={sourceProject && sourceRole ? roleId(sourceProject, sourceRole) : null}
            onChange={(id) => pick(id, setSourceProject, setSourceRole)}
            labelledBy="rule-source-label"
            group="rule-source"
            searchLabel="Search the role they hold"
          />
        </div>

        <div aria-hidden className="text-center text-[18px] text-faint">
          &rArr;
        </div>

        <div>
          <FieldLabel id="rule-target-label">then also give them</FieldLabel>
          <RolePickerOne
            mode="one"
            audience="operator"
            places={places}
            value={targetProject && targetRole ? roleId(targetProject, targetRole) : null}
            onChange={(id) => pick(id, setTargetProject, setTargetRole)}
            labelledBy="rule-target-label"
            group="rule-target"
            searchLabel="Search the role to give"
          />
        </div>

        {sourceRole && targetRole && (
          <p className="text-[14px] leading-[1.55] text-muted">
            {targetWaits
              ? `${targetName} always waits for you, so what this rule gives will wait for you to send.`
              : "What this rule gives goes straight to Zitadel."}
          </p>
        )}

        {validated &&
          (validated.ok && validated.notes.length === 0 ? (
            // Nothing is wrong: a result, said as a caption, not a box.
            <p className="text-[14px] leading-[1.55] text-muted">
              <strong className="font-semibold text-ink">Checked.</strong> Nobody holds the first role,
              directly or through a bundle. Somebody given it by another rule is not counted here, so
              this could still reach people.
            </p>
          ) : (
            <Notice
              tone={validated.ok ? "warn" : "danger"}
              blocking={!validated.ok}
              title={
                validated.ok
                  ? `Checked: ${validated.notes.length} ${validated.notes.length === 1 ? "thing" : "things"} to know`
                  : "This rule can't be saved"
              }
            >
              <ul className="flex flex-col gap-1">
                {validated.notes.map((note) => (
                  <li key={note}>{note}</li>
                ))}
              </ul>
            </Notice>
          ))}
      </div>

      {outcome && <ActionOutcome outcome={outcome} word={saved ? "Saved" : "Not saved"} className="mx-6 mb-1" />}

      {saved && step.after}

      <ModalFooter
        cancel={saved ? undefined : { onClick: onClose }}
        note={
          step.note ??
          // Two different unmet conditions used to share one sentence: an
          // incomplete form was told to "check the rule" while Check itself
          // sat disabled right below, recommending an action it had just
          // refused.
          (!complete
            ? "Fill in every field above, then check the rule."
            : !validated
              ? "Check the rule before you can save it."
              : undefined)
        }
      >
        {saved && (
          <Button variant="outline" onClick={onClose}>
            Close
          </Button>
        )}
        {!saved && (
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
                const written = await step.run(() =>
                  rule ? update.mutateAsync({ id: rule.id, ...input }) : create.mutateAsync(input),
                );
                if (written === KEPT) return;
                if (rule) {
                  setOutcome({
                    // "Applied" over the rule itself is always true — it is a
                    // Syndra-side row, not a Zitadel mutation. What the pill
                    // must not contradict is `savedDetail(targetWaits)` just below: a
                    // manual rule's access still waits under Pending changes,
                    // so the outcome for that half is `queued`.
                    kind: targetWaits ? "queued" : "applied",
                    message: "Rule updated",
                    detail: savedDetail(targetWaits),
                  });
                } else {
                  setOutcome({
                    kind: targetWaits ? "queued" : "applied",
                    message: "Rule created",
                    detail: savedDetail(targetWaits),
                  });
                }
              } catch (error) {
                setOutcome(outcomeFromError(error));
              }
            }}
          >
            {rule ? "Save rule" : "Create rule"}
          </Button>
        )}
        {!saved && (
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
        )}
        {/*
          Outline, and last. A rule is retired from the same place it is retargeted, because
          those are the two things you come here to do — but a solid red button beside Save is
          one slip away from revoking a term's worth of access.
        */}
        {!saved && rule && (
          <Button
            variant="danger"
            disabled={busy}
            onClick={() => setConfirmingDelete(true)}
          >
            Retire rule
          </Button>
        )}
      </ModalFooter>
      </PushedStep>
    </Modal>
  );
}

/** What a saved rule does next, by whether the role it gives waits. */
function savedDetail(targetWaits: boolean): string {
  return targetWaits
    ? "The access this rule gives waits under Pending changes; send it from there. Anyone given the first role later is handled the same way."
    : "People who already hold the first role get the second at once; anyone given the first role later gets the second too.";
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
  const step = useCancelStep({ host: "Retire this rule", goAhead: "retire it", tone: "danger" });
  const holders = rule.holder_count ?? 0;
  // What the backend decides (services.Deliver): the role waits, or, until
  // the switch over to roles that wait, the rule's own mode did.
  const catalog = useGlobalRoleCatalog();
  const switched = useRolesThatWait().data?.switched_over ?? false;
  const roleWaits = Boolean(
    (catalog.data ?? []).find(
      (r) =>
        r.project_id === rule.target_project && r.role_key === rule.target_role,
    )?.waits_for_send,
  );
  const waitsForYou =
    roleWaits || (!switched && rule.confirmation_mode === "manual");
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
      <PushedStep step={step}>
      <ModalHeader
        title="Retire this rule?"
        titleId="rule-delete-title"
        chip={ruleChip(rule)}
        lede={`Some people may hold ${label} only because of this rule.`}
      />

      <div className="px-6">
        <div className="danger-note px-4 py-3.5 text-[14px] leading-[1.55]">
          <div className="type-label mb-1 text-danger-text">What happens</div>
          <ul className="flex flex-col gap-1 text-muted">
            <li>The rule stops giving {label} to anybody, and can&rsquo;t be used again. It stays in the history.</li>
            <li>
              {holders === 0
                ? "This rule is giving the second role to nobody today, so retiring it revokes nothing."
                : `${holders} ${holders === 1 ? "person has" : "people have"} ${label} from this rule. ` +
                  `It is revoked for them unless a bundle or a direct grant also gives it.`}
            </li>
            <li>
              {waitsForYou
                ? "Revoking waits under Pending changes until you send it. Until then, their access stays."
                : "Their access ends at once, with no review."}
            </li>
          </ul>
        </div>
      </div>

      {outcome && <ActionOutcome outcome={outcome} word={gone ? "Retired" : "Not retired"} className="mx-6 mb-1" />}
      {gone && step.after}

      <ModalFooter
        note={step.note ?? "If you only want the rule to give a different role, edit it instead of retiring it."}
        cancel={gone ? undefined : { onClick: onCancel, disabled: remove.isPending }}
      >
        {/* `onDeleted` closes the editor this dialog opened over, and it was
            never called, so deleting a rule left the operator looking at an
            editor for a rule that no longer exists. Called from Close rather
            than from the mutation, because closing the editor takes the
            outcome with it. */}
        {gone && (
          <Button variant="outline" onClick={onDeleted}>
            Close
          </Button>
        )}
        {!gone && (
          <Button
            // A dialog must not open with the cursor on the button that revokes access.
            data-skip-initial-focus
            variant="dangerConfirm"
            isPending={remove.isPending}
            onClick={async () => {
              try {
                const result = await step.run(() => remove.mutateAsync(rule.id));
                if (result === KEPT) return;
                const n = result.cascade?.enqueued ?? 0;
                const auto = result.cascade?.mode === "auto";
                setOutcome(
                  n === 0
                    ? {
                        kind: "applied",
                        message: "Rule retired",
                        detail:
                          "Nobody's access changed. The rule was giving nothing.",
                      }
                    : {
                        // Queued unless the rule fired unattended: the
                        // withdrawals it caused are recorded and wait for
                        // somebody to resume the queue, and calling them applied
                        // would say access is gone that is still there.
                        kind: auto ? "applied" : "queued",
                        message: `Rule retired, ${n} ${
                          n === 1 ? "revocation" : "revocations"
                        } ${auto ? "sent" : "waiting for you to send"}`,
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
            {holders === 0 ? "Retire rule" : "Retire rule and revoke access"}
          </Button>
        )}
      </ModalFooter>
      </PushedStep>
    </Modal>
  );
}

/**
 * The rule, as the sentence it produces — above the dialog title, because the
 * title alone is only ever this rule's short id. `R-014` means nothing to the
 * person deciding whether to edit or delete it; the project/role pair does.
 */
function ruleChip(rule: MappingRuleRow) {
  return (
    <span className="text-[13px]">
      <RoleRef
        projectId={rule.source_project}
        roleKey={rule.source_role}
        className="text-muted"
      />
      <span aria-hidden className="mx-2 text-faint">
        ⇒
      </span>
      <RoleRef
        projectId={rule.target_project}
        roleKey={rule.target_role}
        className="font-semibold"
      />
    </span>
  );
}
