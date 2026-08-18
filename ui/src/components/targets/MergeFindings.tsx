"use client";

import { useState } from "react";

import { EmptyState, ListStates } from "@/components/states";
import { UserName } from "@/components/names";
import { Mono } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Card, CardHeader, CardRow } from "@/components/ui/Card";
import { Input } from "@/components/ui/Input";
import { Relative } from "@/components/ui/Time";
import {
  useMergeFindings,
  useResolveMergeFinding,
  type MergeFinding,
  type ResolveFindingInput,
} from "@/lib/queries/useTargets";

/**
 * The differences a reconciliation refused to resolve (change
 * `reconciliation-as-merge`).
 *
 * Every row here is a question a sweep declined to answer, and the three kinds
 * want different actions — so they are rendered as three sentences rather than
 * as one "out of step" count with a button. "Somebody changed this on the NAS",
 * "we each changed it differently" and "the account is gone" read the same only
 * to somebody who does not have to act on them.
 *
 * What each row leads with is WHAT IT USED TO BE. That is the question an
 * operator asks first, it is the thing no surface could previously answer, and
 * it is the entire reason the merge base exists.
 */
export function MergeFindings({ target }: { target: string }) {
  const findings = useMergeFindings(target);
  const resolve = useResolveMergeFinding(target);
  const [deciding, setDeciding] = useState<string | null>(null);

  const rows = findings.data ?? [];

  return (
    <Card>
      <CardHeader
        title="Waiting on a decision"
        count={rows.length}
        note="Differences the reconciliation found and is not entitled to resolve."
      />
      <ListStates
        isLoading={findings.isLoading}
        error={findings.error}
        isEmpty={rows.length === 0}
        onRetry={() => findings.refetch()}
        errorTitle="The findings could not be read"
        empty={
          <EmptyState
            title="Nothing disputed"
            guidance="Every managed account on this target matches what Syndra decided, or differs in a way the sweep could account for."
          />
        }
      >
        <>
          {rows.map((f, i) => (
            <div key={f.id}>
              <CardRow first={i === 0} className="flex-wrap">
                <UserName id={f.subject_id} />
                {f.field ? <Mono>{f.field}</Mono> : null}
                <span className="flex-1" />
                <span className="text-[13px] text-faint">
                  first seen <Relative iso={f.detected_at} />
                </span>
              </CardRow>
              <div className="px-5 pb-3 text-[13.5px] text-muted">
                {describe(f)}
              </div>
              {deciding === f.id ? (
                <DecisionForm
                  finding={f}
                  pending={resolve.isPending}
                  error={resolve.error}
                  onCancel={() => setDeciding(null)}
                  onResolve={(input) =>
                    resolve.mutate(input, { onSuccess: () => setDeciding(null) })
                  }
                />
              ) : (
                <CardRow>
                  <span className="flex-1" />
                  <Button variant="outline" size="sm" onClick={() => setDeciding(f.id)}>
                    Decide
                  </Button>
                </CardRow>
              )}
            </div>
          ))}
        </>
      </ListStates>
    </Card>
  );
}

/**
 * The finding as a sentence, leading with the history.
 *
 * `value` is rendered as JSON rather than prettified per field: the shapes are
 * the target's, not this component's, and a renderer that knows what a `group`
 * looks like is a second definition of the entitlement schema.
 */
function describe(f: MergeFinding): string {
  const was = show(f.base);
  switch (f.outcome) {
    case "deleted_upstream":
      return "The account this binding names is no longer on the target. Provisioning it again recreates what somebody deleted; unbinding leaves the target alone and stops Syndra managing it.";
    case "theirs_only":
      return `It was ${was} when Syndra last saw it, and is ${show(f.theirs)} now. Syndra did not change it, so somebody changed it on the target.`;
    case "conflict":
      return `It was ${was} when Syndra last saw it. Syndra now wants ${show(f.ours)} and the target has ${show(f.theirs)} — both moved, differently.`;
  }
}

function show(value: unknown): string {
  if (value === undefined || value === null) return "unrecorded";
  return JSON.stringify(value);
}

/**
 * The decision, with its reason.
 *
 * A reason is required for every one of these, and not as ceremony: an adopted
 * value becomes that person's policy, and a suspension with no stated reason is
 * exactly what the allowance layer exists to replace. The backend refuses
 * without one regardless — this is where somebody types it, not what enforces
 * it.
 */
function DecisionForm({
  finding,
  pending,
  error,
  onResolve,
  onCancel,
}: {
  finding: MergeFinding;
  pending: boolean;
  error: unknown;
  onResolve: (input: ResolveFindingInput) => void;
  onCancel: () => void;
}) {
  const [reason, setReason] = useState("");
  const gone = finding.outcome === "deleted_upstream";

  return (
    <div className="row-divider grid gap-3 px-5 py-4">
      <Input
        aria-label="Why"
        placeholder="Why — this becomes the record of the decision"
        value={reason}
        onChange={(e) => setReason(e.target.value)}
      />
      <div className="flex flex-wrap gap-2">
        <Button
          size="sm"
          variant="outline"
          isPending={pending}
          disabled={!reason.trim()}
          onClick={() =>
            onResolve({
              id: finding.id,
              resolution: gone ? "reprovisioned" : "keep_ours",
              reason,
            })
          }
        >
          {gone ? "Provision it again" : "Keep Syndra's"}
        </Button>
        <Button
          size="sm"
          variant="outline"
          isPending={pending}
          disabled={!reason.trim()}
          onClick={() =>
            onResolve({
              id: finding.id,
              resolution: gone ? "unbound" : "take_theirs",
              // A suspension adopted from the target is a decision that has to
              // end or be looked at again — the schema underneath refuses one
              // with neither. Six months is the review interval, not an expiry:
              // this is somebody else's decision being written down, and
              // guessing when it should lapse would be inventing policy.
              ...(gone ? {} : { review_date: sixMonthsOut() }),
              reason,
            })
          }
        >
          {gone ? "Stop managing it" : "Take the target's"}
        </Button>
        <span className="flex-1" />
        <Button size="sm" variant="ghost" onClick={onCancel}>
          Cancel
        </Button>
      </div>
      {Boolean(error) && (
        // The refusals are the useful half. Adopting a group value has nowhere
        // to live — it belongs to a mapping that reaches every holder of that
        // role — and the backend says so with the policy named. Rendered in
        // full rather than replaced with "could not resolve", which would leave
        // an operator pressing a button that never works.
        <span className="text-[13.5px] text-warn-text">
          {error instanceof Error ? error.message : "That decision could not be carried out."}
        </span>
      )}
    </div>
  );
}

function sixMonthsOut(): string {
  const d = new Date();
  d.setMonth(d.getMonth() + 6);
  return d.toISOString();
}
