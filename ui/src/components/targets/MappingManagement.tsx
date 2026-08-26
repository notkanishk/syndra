"use client";

import { useState } from "react";

import { EmptyState, ListStates } from "@/components/states";
import { Button } from "@/components/ui/Button";
import { Card, CardHeader, CardRow } from "@/components/ui/Card";
import { Input } from "@/components/ui/Input";
import { RehearsalDialog } from "@/components/ui/RehearsalDialog";
import { ConvergeButton } from "@/components/targets/ConvergeEntitlements";
import { Relative } from "@/components/ui/Time";
import { RoleRef, UserName } from "@/components/names";
import type { BulkPlan } from "@/lib/queries/useBulkGrants";
import {
  applyMappingDelete,
  applyMappingEdit,
  rehearseMappingDelete,
  rehearseMappingEdit,
  useMappingHistory,
  useMappingHolders,
  useMappings,
  useRollbackMappingVersion,
  rehearseMappingRollback,
  type MappingApplyResult,
  type MappingRehearsal,
  type MappingVersion,
  type RoleMapping,
} from "@/lib/queries/useMappings";
import { targetLabel } from "@/lib/nav";
import { ActionOutcome } from "@/components/ui/ActionOutcome";
import { type ActionOutcome as Outcome } from "@/lib/outcome";
import { useIsTouch } from "@/lib/useViewport";


/**
 * What a role reaches on this target, and every version of that answer
 * (9.7/9.8; design §24).
 *
 * A mapping ties a role to something on an add-on, so editing one moves access
 * for everyone holding that role and deleting one does it invisibly. Edit,
 * delete and rollback therefore all rehearse first — through the same dialog
 * bulk grants, request decisions, drift resolution and bundle publishing use,
 * because "show me what this does before it does it" must be one shape in this
 * product rather than five.
 *
 * The blast-radius acknowledgement is deliberately NOT a checkbox drawn upfront.
 * The rehearsal asks with `acknowledge_scope: false`, the backend refuses a
 * change above the cohort limit, and the refusal becomes the dialog's scope
 * step carrying the number it computed. The threshold lives in one place and an
 * operator meets the ceremony only when it is warranted.
 *
 * And it is rung 2, never type-to-confirm. A mapping edit is routine work whose
 * reach is larger than it looks; copying digits trains an operator not to look,
 * and rung 3 stays reserved for taking access from a named person.
 */
export function MappingManagement({ target }: { target: string }) {
  const mappings = useMappings(target);
  const [editing, setEditing] = useState<RoleMapping | null>(null);
  const [deleting, setDeleting] = useState<RoleMapping | null>(null);

  return (
    <>
      <Card>
        <CardHeader
          title="What roles reach here"
          count={mappings.data?.length}
          note="Editing one moves access for everybody holding that role"
        />
        <ListStates
          isLoading={mappings.isLoading}
          error={mappings.error}
          isEmpty={(mappings.data ?? []).length === 0}
          onRetry={() => mappings.refetch()}
          errorTitle="The mappings could not be read"
          empty={
            <EmptyState
              title="Nothing maps here yet"
              guidance={`No role confers anything on ${targetLabel(target)}, so nobody is entitled to it.`}
            />
          }
        >
          <>
            {(mappings.data ?? []).map((mapping, i) => (
              <MappingRow
                key={mapping.id}
                mapping={mapping}
                first={i === 0}
                onEdit={() => setEditing(mapping)}
                onDelete={() => setDeleting(mapping)}
              />
            ))}
          </>
        </ListStates>
      </Card>

      <VersionHistory target={target} />

      {editing && (
        <EditMappingDialog mapping={editing} onClose={() => { setEditing(null); void mappings.refetch(); }} />
      )}
      {deleting && (
        <DeleteMappingDialog mapping={deleting} onClose={() => { setDeleting(null); void mappings.refetch(); }} />
      )}
    </>
  );
}

/**
 * One mapping, and the three things an operator does with it.
 *
 * The holder count is read here rather than at the top, because it is per
 * mapping and it is the number every one of those three actions is about: how
 * many people a change moves, and who a convergence would be for. It is also
 * the product's only honest cohort source — the entitlement endpoints take an
 * explicit list of subjects, and this row is a surface that already knows one.
 */
function MappingRow({
  mapping,
  first,
  onEdit,
  onDelete,
}: {
  mapping: RoleMapping;
  first: boolean;
  onEdit: () => void;
  onDelete: () => void;
}) {
  const holders = useMappingHolders(mapping.id);
  const cohort = holders.data?.holders ?? [];

  return (
    <CardRow first={first} className="flex-wrap">
      <span className="text-[14px]">
        <RoleRef projectId={mapping.project_id} roleKey={mapping.role_key} />
      </span>
      <span aria-hidden className="text-faint">
        →
      </span>
      <span className="font-mono text-[13.5px]">
        {mapping.field} = {mapping.value}
      </span>
      <span className="text-[13px] text-faint">
        {holders.isLoading
          ? "…"
          : `${cohort.length} ${cohort.length === 1 ? "person" : "people"}`}
      </span>
      <span className="flex-1" />
      <ConvergeButton
        target={mapping.target}
        subjectIds={cohort}
        label={`everybody holding ${mapping.role_key}`}
        disabled={cohort.length === 0}
        disabledReason="Nobody holds this role"
      />
      <Button variant="outline" size="sm" onClick={onEdit}>
        Change
      </Button>
      <Button variant="danger" size="sm" onClick={onDelete}>
        Remove
      </Button>
    </CardRow>
  );
}

/**
 * An apply on this surface returns a count, not a plan.
 *
 * Adapted rather than re-shaped at the endpoint, because what the endpoint
 * reports is the truth about it: the mapping changed and the people it moves
 * are QUEUED. The adaptation puts that count in `queued` and leaves `succeeded`
 * at zero, so the dialog's own result copy — which reads those two fields —
 * cannot put a tick on a change that has not reached the target.
 */
function asPlan(op: string, previous: BulkPlan, result: MappingApplyResult): BulkPlan {
  return {
    op: previous.op,
    applied: true,
    outcomes: previous.outcomes.map((outcome) => ({ ...outcome, effect: "queued" as const })),
    summary: {
      total: previous.summary.total,
      apply: 0,
      no_change: previous.summary.no_change,
      blocked: previous.summary.blocked,
      failed: 0,
      succeeded: 0,
      queued: result.queued_convergences,
    },
  };
}

function EditMappingDialog({ mapping, onClose }: { mapping: RoleMapping; onClose: () => void }) {
  const [value, setValue] = useState(mapping.value);
  const [rehearsed, setRehearsed] = useState<MappingRehearsal | null>(null);
  const touch = useIsTouch();

  return (
    <RehearsalDialog
      title="Change what this role reaches"
      lede={`${mapping.role_key} currently confers ${mapping.field} = ${mapping.value} on ${targetLabel(mapping.target)}. Everybody holding the role moves with it.`}
      noun={["person", "people"]}
      ready={value.trim() !== "" && value !== mapping.value}
      compose={
        <label className="grid gap-1.5 text-[14px]">
          <span>New value for {mapping.field}</span>
          {/* Same reason as the take-away dialog: on touch, focusing on mount
              raises the keyboard over the lede saying that everybody holding
              this role moves with the change. */}
          <Input value={value} onChange={(e) => setValue(e.target.value)} autoFocus={!touch} />
          <span className="text-[13px] text-faint">
            Checked against {targetLabel(mapping.target)} before anything is planned — a
            value it does not recognise is refused here rather than at apply.
          </span>
        </label>
      }
      // The consequence no count implies, and the one an operator is most
      // likely to discover afterwards. Syndra moves the group; it does not move
      // what the old group owns, and it has no way to.
      //
      // The second half is the other card of design M4's pair: when the add-on
      // could not be asked, the edit is allowed through on purpose and the
      // screen has to say so — a check that did not run must not read as one
      // that passed. Amber and not red, because nothing is wrong.
      consequence={
        <>
          Files owned by <span className="type-mono">{mapping.value}</span> stay owned by it.
          Everybody moving loses access to those files unless somebody re-owns them on{" "}
          {targetLabel(mapping.target)}.
          {rehearsed?.value_checked === false && (
            <>
              {" "}
              <span className="font-semibold">
                {targetLabel(mapping.target)} could not be asked whether{" "}
                <span className="type-mono">{value}</span> exists,
              </span>{" "}
              so the value was not checked and the edit is allowed through — refusing it
              while a target is unreachable would make an outage look like your mistake. If
              the value turns out not to exist, the convergence that applies this mapping
              fails, and it arrives as a queued change that will not settle.
            </>
          )}
        </>
      }
      onRehearse={async (acknowledgeScope) => {
        const plan = await rehearseMappingEdit(mapping.id, value, acknowledgeScope);
        setRehearsed(plan);
        return plan;
      }}
      onApply={async (planId) => {
        const result = await applyMappingEdit(mapping.id, value, planId);
        return asPlan("edit_mapping", rehearsed ?? emptyPlan("edit_mapping"), result);
      }}
      onClose={onClose}
    />
  );
}

function DeleteMappingDialog({ mapping, onClose }: { mapping: RoleMapping; onClose: () => void }) {
  const [rehearsed, setRehearsed] = useState<BulkPlan | null>(null);

  return (
    <RehearsalDialog
      title="Stop this role reaching that"
      lede={`${mapping.role_key} will no longer confer ${mapping.field} = ${mapping.value} on ${targetLabel(mapping.target)}. They keep the role and lose what it reached.`}
      noun={["person", "people"]}
      // Destructive, because it takes access away — but still rung 2: the
      // cohort is a role's holders rather than a person somebody named, and the
      // dialog's own scope step is the ceremony.
      destructive
      onRehearse={async (acknowledgeScope) => {
        const plan = await rehearseMappingDelete(mapping.id, acknowledgeScope);
        setRehearsed(plan);
        return plan;
      }}
      onApply={async (planId) => {
        const result = await applyMappingDelete(mapping.id, planId);
        return asPlan("delete_mapping", rehearsed ?? emptyPlan("delete_mapping"), result);
      }}
      onClose={onClose}
    />
  );
}

function emptyPlan(op: string): BulkPlan {
  return {
    op: op as BulkPlan["op"],
    applied: false,
    outcomes: [],
    summary: { total: 0, apply: 0, no_change: 0, blocked: 0, failed: 0, succeeded: 0, queued: 0 },
  };
}

/**
 * Published versions, newest first.
 *
 * Every one carries who published it and why, because that is what an operator
 * choosing a rollback target is actually choosing between. The current version
 * is TINTED rather than badged so the eye finds it without reading, and an
 * unpublished working copy is called out on its own — "current version 4" is
 * true and misleading when what is live is version 4 plus three edits.
 */
function VersionHistory({ target }: { target: string }) {
  const history = useMappingHistory(target);
  // Publishing and rolling back both report here: the version spine is the
  // thing they change, and it is what the operator is looking at.

  return (
    <Card>
      {/* No `note` about unpublished changes. The version band above says it,
          and says it far better — it names the edits. Two statements of one
          fact on one screen is a reader wondering which is authoritative. */}
      <CardHeader
        title="Published versions"
        count={history.data?.versions.length}
      />

      {/* The note and the Publish control live in the version band above, and
          only there. Two of each on one screen was two things that could
          disagree — and they did: this one refused a blank note and the band
          did not. A reader working out which is authoritative has already
          lost. What stays here is the list, and the sentence saying what a
          version is for. */}
      <div className="px-5 pb-4">
        <p className="text-[13.5px] text-muted">
          A version is a snapshot of what roles reach here, so a later change can be rolled
          back to it. The note is the only record of why this set was the right one — it is
          what somebody reads when they are deciding whether to come back.
        </p>
      </div>

      <ListStates
        isLoading={history.isLoading}
        error={history.error}
        isEmpty={(history.data?.versions ?? []).length === 0}
        onRetry={() => history.refetch()}
        errorTitle="The version history could not be read"
        empty={
          <EmptyState
            title="Nothing published yet"
            guidance="Publish the current set to give a future change something to roll back to."
          />
        }
      >
        <>
          {(history.data?.versions ?? []).map((version) => (
            <VersionRow
              key={version.version}
              target={target}
              version={version}
              current={version.version === history.data?.current_version}
              unpublished={Boolean(history.data?.unpublished)}
            />
          ))}
        </>
      </ListStates>
    </Card>
  );
}

function VersionRow({
  target,
  version,
  current,
  unpublished,
}: {
  target: string;
  version: MappingVersion;
  current: boolean;
  unpublished: boolean;
}) {
  const rollback = useRollbackMappingVersion(target);
  const [rehearsing, setRehearsing] = useState(false);
  const [rehearsed, setRehearsed] = useState<BulkPlan | null>(null);
  const [outcome, setOutcome] = useState<Outcome | null>(null);

  return (
    <div className={`row-divider px-5 py-3.5 ${current ? "bg-accent-soft" : ""}`}>
      <div className="flex flex-wrap items-baseline gap-2 text-[14px]">
        <span className="font-semibold">Version {version.version}</span>
        {current && <span className="text-[13px] text-accent-text">current</span>}
        <span className="text-faint">
          <UserName id={version.published_by} /> · <Relative iso={version.published_at} />
        </span>
        <span className="flex-1" />
        {current ? (
          // Rolling back to what is already live does nothing, and offering it
          // would be offering a no-op ceremony. The reason is text, not a
          // disabled control with a tooltip.
          <span className="text-[13px] text-faint">
            {unpublished ? "Restores this set, discarding unpublished changes" : "Already live"}
          </span>
        ) : null}
        {(!current || unpublished) && (
          <Button variant="outline" size="sm" onClick={() => setRehearsing(true)}>
            Roll back to this
          </Button>
        )}
      </div>

      <p className="mt-1 text-[13.5px] text-muted">
        {version.note || <span className="text-faint">No reason recorded.</span>}
      </p>
      <p className="mt-1 text-[13px] text-faint">
        {version.entries.length} binding{version.entries.length === 1 ? "" : "s"}
        {version.entries.length > 0 && (
          <>
            {" · "}
            {version.entries
              .slice(0, 3)
              .map((e) => `${e.role_key} → ${e.value}`)
              .join(", ")}
            {version.entries.length > 3 && `, and ${version.entries.length - 3} more`}
          </>
        )}
      </p>

      {/* On the version row that was rolled back to, which is the row the
          operator is looking at and the thing that changed. */}
      {outcome && <ActionOutcome outcome={outcome} className="mt-3" />}

      {rehearsing && (
        <RehearsalDialog
          title={`Roll back to version ${version.version}`}
          lede={`This replaces what roles reach ${targetLabel(target)} with the ${
            version.entries.length
          } binding${version.entries.length === 1 ? "" : "s"} in version ${version.version}.`}
          noun={["person", "people"]}
          consequence={
            <>
              Anything added since version {version.version} is removed — a rollback restores
              a set, it does not merge one. The people it moves include everybody who loses a
              mapping, not only those who gain one.
            </>
          }
          onRehearse={async (acknowledgeScope) => {
            const plan = await rehearseMappingRollback(target, version.version, acknowledgeScope);
            setRehearsed(plan);
            return plan;
          }}
          onApply={async (planId) =>
            new Promise((resolve, reject) => {
              rollback.mutate({ version: version.version, planId }, {
                onSuccess: (result) => {
                  setOutcome({
                    kind: "queued",
                    message: `Rolled back to version ${version.version} — ${
                      result.queued_convergences
                    } ${result.queued_convergences === 1 ? "person is" : "people are"} queued for convergence`,
                    detail:
                      "Nothing has reached the target yet — resume the queue on Pending changes.",
                  });
                  // The plan the dialog showed is the one being reported back,
                  // with every row now queued — `asPlan` carries the outcomes
                  // through so the result step lists the same people the
                  // rehearsal did, rather than an empty summary.
                  resolve(asPlan("rollback_mappings", rehearsed ?? emptyPlan("rollback_mappings"), result));
                },
                onError: reject,
              });
            })
          }
          onClose={() => setRehearsing(false)}
        />
      )}
    </div>
  );
}

