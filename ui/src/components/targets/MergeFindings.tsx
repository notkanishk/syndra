"use client";

import { Term } from "@/components/ui/Term";
import { useState } from "react";

import { EmptyState, ListStates } from "@/components/states";
import { ApiError } from "@/lib/api-client";
import { UserName } from "@/components/names";
import { Mono } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Card, CardHeader, CardRow } from "@/components/ui/Card";
import { Input } from "@/components/ui/Input";
import { Relative } from "@/components/ui/Time";
import { targetLabel } from "@/lib/nav";
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
  const name = targetLabel(target);

  return (
    <Card>
      <CardHeader
        title="Waiting on a decision"
        count={rows.length}
        note={
          <>
            <Term name="mergeFinding">Merge findings</Term> — differences between Syndra and {name}{" "}
            that reconciliation will not settle on its own.
          </>
        }
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
            guidance={`Every account Syndra manages on ${name} matches what Syndra expects, or differs in a way Syndra could explain.`}
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
                {describe(f, name)}
              </div>
              {f.decision === "unbound" ? (
                // Decided as unbound and still standing means the unbind did not
                // finish: it settles immediately when it works, because nothing
                // will ever observe that subject again. So this row is a repair,
                // not a wait — and hiding its control is what turned a
                // recoverable half-write into a wedge.
                <CardRow className="flex-wrap">
                  <span className="text-[13px] text-warn-text">
                    {name} may already have let go of this account while Syndra&rsquo;s records
                    were not updated. Pressing again changes nothing on {name}.
                  </span>
                  <span className="flex-1" />
                  <Button
                    variant="outline"
                    size="sm"
                    isPending={resolve.isPending}
                    onClick={() =>
                      resolve.mutate({
                        id: f.id,
                        resolution: "unbound",
                        reason: "Finishing an unbind that did not complete",
                      })
                    }
                  >
                    Finish: stop managing it
                  </Button>
                </CardRow>
              ) : f.decision ? (
                // Decided and waiting. The convergence is queued or the policy
                // has changed; the row closes when a pass sees the target agree.
                // Saying "resolved" here would be the surface claiming the
                // difference is over while it is still on the target.
                <CardRow>
                  <span className="text-[13px] text-faint">
                    {decisionLabel(f.decision, name)} by <UserName id={f.decided_by ?? ""} /> ·
                    waiting for {name} to match
                  </span>
                </CardRow>
              ) : deciding === f.id ? (
                <DecisionForm
                  finding={f}
                  name={name}
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
function describe(f: MergeFinding, name: string): string {
  const was = show(f.base);
  const its = f.field ? `Its ${f.field}` : "It";
  switch (f.outcome) {
    case "deleted_upstream":
      return `This account is no longer on ${name} — somebody deleted it there. Creating it again brings back what they deleted; stopping management leaves ${name} alone and Syndra forgets the account.`;
    case "theirs_only":
      return `${its} was ${was} when Syndra last saw it, and is ${show(f.theirs)} now. Syndra did not change it, so somebody changed it on ${name}.`;
    case "conflict":
      return `${its} was ${was} when Syndra last saw it. Syndra now wants ${show(f.ours)} and ${name} has ${show(f.theirs)} — both moved, differently.`;
  }
}

/**
 * A decision, in the words an operator used to make it.
 *
 * The switch is the boundary between the wire and the console. `unbound` was
 * missing and fell through to the default, which rendered the literal string
 * "unbound" on the page — the backend's vocabulary reaching somebody who never
 * agreed to learn it. `agreed` is here for the same reason even though nobody
 * chooses it: a finding can close because the two sides stopped disagreeing,
 * and that resolution appears on the row like any other.
 *
 * The default no longer echoes the code. A resolution this does not know about
 * is a deploy skew, and "recorded" is the honest thing to say about one —
 * `merge-vocabulary.test.ts` fails when the backend grows a resolution this
 * has not been taught.
 */
function decisionLabel(decision: string, name: string): string {
  switch (decision) {
    case "keep_ours":
      return "Keeping Syndra's value";
    case "take_theirs":
      return `Using ${name}'s value`;
    case "reprovisioned":
      return "Creating it again";
    case "unbound":
      return "No longer managing it";
    case "agreed":
      return "The two sides agree now";
    default:
      return "Recorded";
  }
}

function show(value: unknown): string {
  if (value === undefined || value === null) return "unrecorded";
  if (typeof value === "string") return value;
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
  name,
  pending,
  error,
  onResolve,
  onCancel,
}: {
  finding: MergeFinding;
  name: string;
  pending: boolean;
  error: unknown;
  onResolve: (input: ResolveFindingInput) => void;
  onCancel: () => void;
}) {
  const [reason, setReason] = useState("");
  const [resolution, setResolution] = useState<string | null>(null);
  const gone = finding.outcome === "deleted_upstream";
  const alreadyDecided = decidedElsewhere(error);

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
          onClick={() => {
            const pick = gone ? "reprovisioned" : "keep_ours";
            setResolution(pick);
            onResolve({ id: finding.id, resolution: pick, reason });
          }}
        >
          {gone ? "Create it again" : "Keep Syndra's value"}
        </Button>
        {(gone || finding.adoptable) && (
          <Button
            size="sm"
            variant="outline"
            isPending={pending}
            disabled={!reason.trim()}
            onClick={() => {
              const pick = gone ? "unbound" : "take_theirs";
              setResolution(pick);
              onResolve({
                id: finding.id,
                resolution: pick,
                // A suspension adopted from the target is a decision that has to
                // end or be looked at again — the schema underneath refuses one
                // with neither. Six months is the review interval, not an expiry:
                // this is somebody else's decision being written down, and
                // guessing when it should lapse would be inventing policy.
                ...(gone ? {} : { review_date: sixMonthsOut() }),
                reason,
              });
            }}
          >
            {gone ? "Stop managing it" : `Use ${name}'s value`}
          </Button>
        )}
        <span className="flex-1" />
        <Button size="sm" variant="ghost" onClick={onCancel}>
          Cancel
        </Button>
      </div>
      {!gone && !finding.adoptable && (
        // The alternative to a button that cannot work. Adopting a group value
        // has nowhere to live — it belongs to a mapping that reaches every
        // holder of that role — so the surface says which mapping and how many
        // people, and the operator goes and changes the policy.
        //
        // Rendered as text rather than as a disabled control: a disabled button
        // whose reason lives in a tooltip is a reason nobody on a keyboard or a
        // phone can find.
        <div className="grid gap-1 text-[13px] text-muted">
          <span>{finding.why_not}</span>
          {(finding.policy ?? []).map((p) => (
            <span key={p.mapping_id} className="text-faint">
              <Mono>{p.role_key}</Mono> → {p.value} ·{" "}
              {p.holders} {p.holders === 1 ? "person holds" : "people hold"} that role
            </span>
          ))}
        </div>
      )}
      {alreadyDecided ? (
        <AlreadyDecided taken={alreadyDecided} picked={resolution} name={name} />
      ) : (
        Boolean(error) && (
          // The refusals are the useful half. Adopting a group value has nowhere
          // to live — it belongs to a mapping that reaches every holder of that
          // role — and the backend says so with the policy named. Rendered in
          // full rather than replaced with "could not resolve", which would leave
          // an operator pressing a button that never works.
          <span className="text-[13.5px] text-warn-text">
            {error instanceof Error
              ? error.message
              : "That decision did not go through. Nothing was changed."}
          </span>
        )
      )}
    </div>
  );
}

function sixMonthsOut(): string {
  const d = new Date();
  d.setMonth(d.getMonth() + 6);
  return d.toISOString();
}

/** What the backend sends back when somebody decided first. */
interface DecidedElsewhere {
  decision: string;
  decided_by: string;
  decision_reason?: string;
  decided_at?: string;
}

/**
 * A finding takes one decision, so the second operator to press is refused.
 *
 * Read as fields rather than as a sentence: the API's own message carries a
 * UUID and a snake_case resolution, which is raw material and not copy.
 */
function decidedElsewhere(error: unknown): DecidedElsewhere | null {
  if (!(error instanceof ApiError) || error.code !== "ALREADY_DECIDED") return null;
  const details = error.details ?? {};
  if (!details.decision) return null;
  return {
    decision: details.decision,
    decided_by: details.decided_by ?? "",
    decision_reason: details.decision_reason,
    decided_at: details.decided_at,
  };
}

/**
 * Somebody decided this while the form was open (design B1).
 *
 * Accent, and not shaped like an error. What happened is that the finding
 * became DECIDED — which is a state the product already has a colour for, and
 * the same accent the decided-and-waiting row wears. The operator did nothing
 * wrong and nothing was changed by pressing.
 *
 * Two answers side by side, so a reader can tell in one glance whether they
 * even disagree. Most of the time they will not, and that is the common case
 * this is written for.
 *
 * The other operator's reason is quoted in full rather than linked. The reason
 * is mandatory on every resolution and exists for exactly the person who
 * arrives second — putting it one click away would be putting the only thing
 * they need one click away.
 */
function AlreadyDecided({
  taken,
  picked,
  name,
}: {
  taken: DecidedElsewhere;
  picked: string | null;
  name: string;
}) {
  const agree = picked !== null && picked === taken.decision;

  return (
    <div className="grid gap-2.5 rounded-inner border border-accent-line bg-accent-soft px-4 py-3">
      <p className="text-[13.5px] font-semibold text-accent-text">
        <UserName id={taken.decided_by} fallback="Somebody" /> decided this
        {taken.decided_at ? (
          <>
            {" "}
            <Relative iso={taken.decided_at} />
          </>
        ) : null}
        , while this page was open.
      </p>

      <p className="text-[13.5px] text-muted">
        They chose <span className="font-semibold text-ink">{decisionLabel(taken.decision, name)}</span>
        {taken.decision_reason ? (
          <>
            {" "}
            — &ldquo;{taken.decision_reason}&rdquo;
          </>
        ) : null}
        . Your choice was not applied and nothing was changed by pressing.
      </p>

      {picked !== null && (
        <p className="text-[13.5px] text-muted">
          {agree ? (
            <>
              You had picked{" "}
              <span className="font-semibold text-ink">{decisionLabel(picked, name)}</span> too,
              so there is nothing to disagree about.
            </>
          ) : (
            <>
              You had picked{" "}
              <span className="font-semibold text-ink">{decisionLabel(picked, name)}</span>. Only
              one decision can stand, because the two answers undo each other.
            </>
          )}
        </p>
      )}

      <p className="text-[13px] text-faint">
        The finding has not gone anywhere. It stays here as decided-and-waiting until Syndra
        next checks the account and finds {name} matches.
      </p>
    </div>
  );
}
