"use client";
import { personOr } from "@/lib/names";

import { Term } from "@/components/ui/Term";
import { useLayoutEffect, useRef, useState } from "react";
import { Crossfade, LeavingRow, Unfold, useLeavingRows } from "@/lib/motion";
import { AllClear, useEmptied } from "@/components/targets/AllClear";
import { useOneOpening } from "@/lib/useOneOpening";
import {
  CHECKED_UPDATING,
  CONFLICT_NEXT_FALLBACK,
  OTHER_PERSON,
  checkStopped,
  FINISH_AGAIN_NOTE,
  FINISH_REASON,
  closesAtNextCheck,
  conflictChoices,
  finishLabel,
  conflictDetailFallback,
  conflictReport,
  holdersLine,
  stillReported,
  type ConflictAnswer,
} from "@/components/targets/bindingConflict";
import { useShownNames } from "@/components/names/shownNames";
import { useCountdown } from "@/lib/queries/fresh";
import { cooldownLine } from "@/components/results/copy";
import { Radio } from "@/components/ui/Choice";
import { whyNotChecked } from "@/lib/format";

import { ListStates } from "@/components/states";
import { ApiError } from "@/lib/api-client";
import { describeFailure, mayHaveGoneThrough, outcomeFromError, partialSentence, sentence } from "@/lib/outcome";
import { UserName } from "@/components/names";
import { AdvancedOnly } from "@/components/ui/AdvancedOnly";
import { useIsAdvanced } from "@/lib/ui-view";
import { TechnicalDetail, TechnicalPayload } from "@/components/ui/TechnicalDetail";
import { RoleRef } from "@/components/names/RoleRef";
import { fieldNoun, limitWordsFor, membershipNoun, type EntitlementField } from "@/lib/fields";
import { schemaReason } from "@/components/states/SchemaState";
import { useFieldSchemaState, useSystemNames } from "@/lib/queries/useFieldSchema";
import { UntilChoice, untilWords, type Until } from "@/components/targets/UntilChoice";
import { fieldWords } from "@/lib/role-mapping-plan";
import { ActionOutcome } from "@/components/ui/ActionOutcome";
import { Button } from "@/components/ui/Button";
import { Card, CardHeader, CardRow } from "@/components/ui/Card";
import { Input } from "@/components/ui/Input";
import { Notice } from "@/components/ui/Notice";
import { useAccountHref } from "@/components/system/ServiceAccountTag";
import { Relative } from "@/components/ui/Time";
import {
  useMergeFindings,
  useReconcileTarget,
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
export function MergeFindings({
  target,
  onDrawn,
}: {
  target: string;
  /**
   * How many findings are drawn now, the ones still fading out included, and
   * whether the reader's own decision is what is emptying the list. The region
   * that counts them reads this and not the read, so its count changes after
   * the row has gone.
   */
  onDrawn?: (n: number, byReader: boolean) => void;
}) {
  const nameOf = useSystemNames();
  const findings = useMergeFindings(target);
  const resolve = useResolveMergeFinding(target);
  const schemaState = useFieldSchemaState(target);
  const { schema } = schemaState;
  const [deciding, setDeciding] = useState<string | null>(null);
  // What a saved limit said, kept past the refetch that moves its row to "waiting
  // for the system to match", so the result is shown where the decision was made.
  const [saved, setSaved] = useState<{ id: string; message: string } | null>(null);
  // The "Finish" button below has no onSuccess and no server field to key off
  // until the list refetches, so a second click before then re-submits the
  // same finish. Marked synchronously, same idiom as the request queues.
  const [finishing, setFinishing] = useState<Set<string>>(new Set());

  // A resolved finding fades where it was and the gap closes after, rather than the list jumping.
  const leaving = useLeavingRows(findings.data ?? [], (f) => f.id, !findings.isLoading && !findings.error);
  const rows = leaving.rows;
  const known = findings.data !== undefined;
  const drawn = known ? rows.length : undefined;
  // Emptied in front of the reader, and spent once seen: a read that fails and recovers brings the same
  // all-clear back, not a second opening.
  const opening = useOneOpening(useEmptied(drawn), !findings.isLoading && !findings.error);
  const emptied = opening.open;
  // The findings this reader decided (saved), by id. The all-clear blooms only when what is leaving is
  // theirs: nobody else's act, and no clock, decides it.
  const mine = useRef(new Set<string>());
  const byReaderRef = useRef(false);
  const goingNow = rows.filter((f) => leaving.props(f.id).leaving);
  if (goingNow.length > 0) byReaderRef.current = goingNow.every((f) => mine.current.has(f.id));
  const byReader = byReaderRef.current;
  useLayoutEffect(() => {
    if (drawn !== undefined) onDrawn?.(drawn, byReader);
  }, [drawn, byReader, onDrawn]);
  const name = nameOf(target);
  const advanced = useIsAdvanced();

  return (
    <Card {...leaving.listProps}>
      <CardHeader
        title="Waiting on a decision"
        count={drawn}
        caption={
          // The term and its definition are the machine's vocabulary: Advanced. Basic says the thing.
          advanced ? (
            <>
              <Term name="mergeFinding">Merge findings</Term>: differences between Syndra and {name}{" "}
              that Syndra will not settle on its own.
            </>
          ) : (
            <>Differences between Syndra and {name} that Syndra will not settle on its own.</>
          )
        }
      />
      <ListStates
        // Every sentence and decision here is worded from the fields, so none is drawn before they are read.
        isLoading={findings.isLoading || schemaState.loading}
        error={findings.error ?? schemaState.error}
        isEmpty={rows.length === 0}
        onRetry={() => {
          if (findings.error) void findings.refetch();
          if (schemaState.error) schemaState.refetch();
        }}
        // Says which read failed: the findings, or the system's fields they are worded from.
        errorTitle={findings.error ? "The findings could not be read" : `${name}'s fields could not be read`}
        empty={
          <AllClear
            title="Nothing disputed."
            guidance={`Every account Syndra manages on ${name} matches what Syndra expects, or differs in a way Syndra could explain.`}
            emptied={emptied}
            byReader={byReader}
            onOpened={opening.spend}
          />
        }
      >
        <>
          {rows.map((f, i) => {
            // The one control of a finding sits in the seat on its first line, whatever the
            // finding is waiting on; the seat keeps its width when there is none.
            const finish = (resolution: "unbound" | "adopted" | "kept", reason: string, label: string, settled: "error" | "settled") => (
              <Button
                variant="outline"
                size="sm"
                disabled={finishing.has(f.id)}
                isPending={resolve.isPending && (resolution === "unbound" || finishing.has(f.id))}
                onClick={() => {
                  setFinishing((prev) => new Set(prev).add(f.id));
                  const release = () =>
                    setFinishing((prev) => {
                      const copy = new Set(prev);
                      copy.delete(f.id);
                      return copy;
                    });
                  resolve.mutate(
                    { id: f.id, resolution, reason },
                    settled === "error"
                      ? { onError: release, onSuccess: () => void mine.current.add(f.id) }
                      : { onSettled: release, onSuccess: () => void mine.current.add(f.id) },
                  );
                }}
              >
                {label}
              </Button>
            );
            const action =
              f.decision === "unbound" ? (
                finish("unbound", "Finishing an unbind that did not complete", "Finish: stop tracking it", "error")
              ) : f.decision ? (
                f.outcome === "binding_conflict" && (f.decision === "adopted" || f.decision === "kept")
                  ? // A choice that reached the system and not Syndra's own records answers "choose
                    // it again"; this is that press, on the same repair route, safe to repeat.
                    finish(f.decision, FINISH_REASON, finishLabel(f.decision), "settled")
                  : undefined
              ) : (
                // The seat's word crossfades when the form opens under it, and keeps its place (D-1 M6).
                <Crossfade id={deciding === f.id ? "deciding" : "decide"} className="justify-items-end">
                  {deciding === f.id ? null : (
                    <Button variant="outline" size="sm" onClick={() => setDeciding(f.id)}>
                      Decide
                    </Button>
                  )}
                </Crossfade>
              );
            return (
            <LeavingRow key={f.id} {...leaving.props(f.id)}>
            <div>
              <CardRow first={i === 0} seat="default" action={action} detail={describe(f, name, schema)}>
                <Person id={f.subject_id} />
                {f.field ? <span className="text-small text-muted">{fieldWords(f.field, schema)}</span> : null}
                <span className="flex-1" />
                <span className="text-small text-faint">
                  first seen <Relative iso={f.detected_at} />
                </span>
              </CardRow>
              {(f.outcome === "binding_conflict" || rawSides(f)) && (
                // `empty:hidden`: in Basic, or when there is no other person to name, nothing here renders.
                <div className="px-5 pb-3 text-small text-muted empty:hidden">
                  {f.outcome === "binding_conflict" && <ConflictWho finding={f} name={name} />}
                  {rawSides(f) && (
                    <AdvancedOnly>
                      <TechnicalDetail className="mt-2" placeKey={`finding:${f.id}`}>
                        <TechnicalPayload>{rawSides(f)}</TechnicalPayload>
                      </TechnicalDetail>
                    </AdvancedOnly>
                  )}
                </div>
              )}
              {f.decision === "unbound" ? (
                // Decided as unbound and still standing means the unbind did not
                // finish: it settles immediately when it works, because nothing
                // will ever observe that subject again. So this row is a repair,
                // not a wait — and hiding its control is what turned a
                // recoverable half-write into a wedge.
                <CardRow>
                  <span className="text-small text-warn-text">
                    {name} may already have let go of this account while Syndra&rsquo;s records
                    were not updated. Pressing again changes nothing on {name}.
                  </span>
                </CardRow>
              ) : f.decision ? (
                // Decided and waiting. The convergence is queued or the policy
                // has changed; the row closes when a pass sees the target agree.
                // Saying "resolved" here would be the surface claiming the
                // difference is over while it is still on the target.
                <CardRow className="flex-wrap">
                  {saved?.id === f.id && (
                    <div className="w-full pb-1">
                      <ActionOutcome word="Saved" outcome={{ kind: "applied", message: saved.message }} />
                    </div>
                  )}
                  <span className="text-small text-faint">
                    {decisionLabel(f.decision, name)} by <Person id={f.decided_by ?? ""} /> ·{" "}
                    {f.outcome === "binding_conflict"
                      ? // Adopted or kept: the add-on has acted, and the next read closes it.
                        closesAtNextCheck(name)
                      : `waiting for ${name} to match`}
                  </span>
                  {f.outcome === "binding_conflict" && (f.decision === "adopted" || f.decision === "kept") && (
                    <span className="w-full text-small text-muted">{FINISH_AGAIN_NOTE}</span>
                  )}
                </CardRow>
              ) : null}
              {/* The form is the row's own panel, always mounted (a fragment), so the row's shape never
                  changes and focus is kept: it opens under the row and closes by fading first (D-1 M6). */}
                <Unfold open={!f.decision && deciding === f.id}>
                  <>
                    {!f.decision && deciding === f.id && (
                      <DecisionForm
                        finding={f}
                        name={name}
                        schemaState={schemaState}
                        pending={resolve.isPending}
                        error={resolve.error}
                        onCancel={() => setDeciding(null)}
                        onResolve={(input, savedMessage) =>
                          resolve.mutate(input, {
                            onSuccess: () => {
                              mine.current.add(f.id);
                              if (savedMessage) setSaved({ id: f.id, message: savedMessage });
                              setDeciding(null);
                            },
                          })
                        }
                      />
                    )}
                  </>
                </Unfold>
            </div>
            </LeavingRow>
            );
          })}
        </>
      </ListStates>
    </Card>
  );
}

/**
 * The finding as a sentence, leading with the history.
 *
 * A value is worded by its field's ROLE, never its name: a limit reads in the
 * field's unit and says "no limit" for the field's own unlimited; every other
 * role keeps the target's own words, and a structured value is left to
 * Technical detail. A renderer that knew what a `group` looks like would be a
 * second definition of the entitlement schema.
 */
function describe(f: MergeFinding, name: string, schema: EntitlementField[] | undefined): string {
  const limit = schema?.find((x) => x.name === f.field && x.role === "limit");
  const show = (v: unknown) => showValue(v, limit);
  const was = show(f.base);
  const its = f.field ? `Its ${fieldWords(f.field, schema)}` : "It";
  switch (f.outcome) {
    case "binding_conflict": {
      // The add-on's own words, as written: what happened, then what to do.
      const c = conflictReport(f.theirs);
      return `${c.detail ?? conflictDetailFallback(name)} ${c.next ?? CONFLICT_NEXT_FALLBACK}`;
    }
    case "deleted_upstream":
      return `This account is no longer on ${name}. Somebody deleted it there. Creating it again brings back what they deleted; stopping management leaves ${name} alone and Syndra forgets the account.`;
    case "theirs_only":
      return `${its} was ${was} when Syndra last saw it, and is ${show(f.theirs)} now. Syndra did not change it, so somebody changed it on ${name}.`;
    case "conflict":
      return `${its} was ${was} when Syndra last saw it. Syndra now wants ${show(f.ours)} and ${name} has ${show(f.theirs)}. Both moved, differently.`;
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
    case "adopted":
      return "Adopted the account for them";
    case "kept":
      return "Kept it for them";
    case "agreed":
      return "The two sides agree now";
    default:
      return "Recorded";
  }
}

function showValue(value: unknown, limit: EntitlementField | undefined): string {
  if (value === undefined || value === null) return "unrecorded";
  if (limit) return limitWordsFor(limit, valueOf(value));
  if (typeof value === "string") return value;
  // A structured value is the machine's words: its exact form is under Technical detail.
  return typeof value === "object" ? "a structured value" : String(value);
}

/**
 * What the add-on's words leave to the page: the other person a twice-claimed
 * account is recorded for (named, never an id), and every account an ambiguous
 * name or number could mean. The holder's id is the machine's: Technical detail.
 */
function ConflictWho({ finding, name }: { finding: MergeFinding; name: string }) {
  const c = conflictReport(finding.theirs);
  if (c.reason === "claimed_twice" && c.bound_to) {
    return (
      <p className="mt-1">
        {OTHER_PERSON} <Person id={c.bound_to} />.
      </p>
    );
  }
  if (c.reason === "ambiguous" && c.holders?.length) {
    return <p className="mt-1">{holdersLine(name, c.holders.map((h) => h.username))}</p>;
  }
  return null;
}

/** The three sides of a finding, exact, for whoever runs the server. */
function rawSides(f: MergeFinding): string | null {
  const sides = [f.base, f.ours, f.theirs];
  return sides.some((v) => v !== null && typeof v === "object") ? JSON.stringify({ base: f.base, ours: f.ours, theirs: f.theirs }, null, 2) : null;
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
  schemaState,
  pending,
  error,
  onResolve,
  onCancel,
}: {
  finding: MergeFinding;
  name: string;
  schemaState: Parameters<typeof schemaReason>[0];
  pending: boolean;
  error: unknown;
  /** `saved` is the sentence to show once it lands, for a decision that makes something new. */
  onResolve: (input: ResolveFindingInput, saved?: string) => void;
  onCancel: () => void;
}) {
  const [reason, setReason] = useState("");
  const [resolution, setResolution] = useState<string | null>(null);
  const gone = finding.outcome === "deleted_upstream";
  // A conflict over which account a person's record names has its own answers (ConflictDecision).
  const conflict = finding.outcome === "binding_conflict";
  // A limit the target changed: taking its value saves it as this person's own
  // limit, which needs an end or a review date, chosen here (design §3, 3.8).
  const schema = schemaState.schema;
  // Until the fields are known the button cannot tell a limit (a panel) from a switched-off account (one press).
  const unknown = gone || conflict ? undefined : schemaReason(schemaState, name);
  const limit = schema?.find((f) => f.name === finding.field && f.role === "limit");
  const [adopting, setAdopting] = useState(false);
  const [until, setUntil] = useState<Until>("ends");
  const [date, setDate] = useState("");
  const alreadyDecided = decidedElsewhere(error);

  return (
    <div className="row-divider grid gap-3 px-5 py-4">
      <Input
        aria-label="Why"
        placeholder="What made you decide this"
        value={reason}
        onChange={(e) => setReason(e.target.value)}
      />
      {conflict ? (
        <ConflictDecision
          finding={finding}
          name={name}
          reason={reason}
          pending={pending}
          onCancel={onCancel}
          onResolve={(answer) => {
            setResolution(answer);
            onResolve({ id: finding.id, resolution: answer, reason });
          }}
        />
      ) : (
      <div className="flex flex-wrap gap-2">
        <Button
          size="sm"
          variant="outline"
          isPending={pending}
          disabled={!reason.trim()}
          reason={!reason.trim() ? "Say why. This becomes the record of the decision." : undefined}
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
            disabled={!reason.trim() || Boolean(unknown)}
            reason={unknown ?? (!reason.trim() ? "Say why. This becomes the record of the decision." : undefined)}
            onClick={() => {
              if (limit) {
                setAdopting(true);
                return;
              }
              const pick = gone ? "unbound" : "take_theirs";
              setResolution(pick);
              onResolve({
                id: finding.id,
                resolution: pick,
                // A suspension adopted from the target has to end or be looked at
                // again, and the backend owns that date: with none sent it sets a
                // review six months on, in its own zone. The page does not work out time.
                reason,
              });
            }}
          >
            {gone ? "Stop managing it" : `Use ${name}'s value`}
          </Button>
        )}
        <span className="flex-1" />
        {/* Beside outline acts, Cancel is outline too: ghost only defers to a primary (G1). */}
        <Button size="sm" onClick={onCancel}>
          Cancel
        </Button>
      </div>
      )}
      {limit && adopting && (
        <div className="grid gap-2.5 rounded-inner bg-tint-1 px-4 py-3.5" data-adopt-limit>
          <p className="text-small leading-[1.55] text-ink [text-wrap:pretty]">
            This saves {name}&apos;s value, {limitWordsFor(limit, valueOf(finding.theirs))}, as this person&apos;s own{" "}
            {fieldNoun(limit)}. It takes the place of what their roles give until it ends or you review it.
          </p>
          <UntilChoice
            until={until}
            onUntil={setUntil}
            date={date}
            onDate={setDate}
            comesBack={`their roles' ${fieldNoun(limit)}`}
          />
          <span className="flex flex-wrap gap-2">
            <Button
              size="sm"
              variant="accent"
              isPending={pending}
              disabled={!reason.trim() || date === ""}
              reason={!reason.trim() ? "Say why. This becomes the record of the decision." : date === "" ? "Pick the day it ends or is reviewed." : undefined}
              onClick={() => {
                setResolution("take_theirs");
                const day = date; // the date itself: the backend ends it in its own zone
                onResolve(
                  {
                    id: finding.id,
                    resolution: "take_theirs",
                    reason,
                    ...(until === "ends" ? { expires_at: day } : { review_date: day }),
                  },
                  `Their own ${fieldNoun(limit)} is now ${limitWordsFor(limit, valueOf(finding.theirs))}. It ${untilWords(until, day)}. Their roles' ${fieldNoun(limit)} does not apply to them while it stands.`,
                );
              }}
            >
              Save it as their own limit
            </Button>
            <Button size="sm" onClick={() => setAdopting(false)}>
              Back
            </Button>
          </span>
        </div>
      )}
      {!gone && !conflict && !finding.adoptable && (
        // The alternative to a button that cannot work. Adopting a group value
        // has nowhere to live — it belongs to a mapping that reaches every
        // holder of that role — so the surface says which mapping and how many
        // people, and the operator goes and changes the policy.
        //
        // Rendered as text rather than as a disabled control: a disabled button
        // whose reason lives in a tooltip is a reason nobody on a keyboard or a
        // phone can find.
        <div className="grid gap-1 text-small text-muted">
          <span>{finding.why_not}</span>
          {(finding.policy ?? []).map((p) => (
            <span key={p.mapping_id} className="text-faint">
              <RoleRef projectId={p.project_id} roleKey={p.role_key} /> gives the {p.value} {membershipNoun(schema)} ·{" "}
              Syndra gives that role to {p.holders} {p.holders === 1 ? "person" : "people"}
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
          <ActionOutcome
            placement="inline"
            word="Not decided"
            outcome={{
              ...outcomeFromError(error),
              message: (() => {
                if (!(error instanceof Error)) return "That decision was not recorded. Nothing was changed.";
                // An ApiError's message is the envelope's sentence (never upstream text); anything else is a transport fault.
                const said = error instanceof ApiError ? error.message : describeFailure(error);
                const ref = outcomeFromError(error).requestId;
                const where = mayHaveGoneThrough(error);
                const told = where ? `${sentence(said)} ${partialSentence(where)}` : said;
                return ref ? `${sentence(told)} Reference ${ref}.` : told;
              })(),
            }}
          />
        )
      )}
    </div>
  );
}

/**
 * A binding conflict's answers in the decision form (addon-contract 7t-7): one at a time, safest
 * first and preselected (conflictChoices), the sentence beside it saying what happens on the
 * system and in Syndra, and one button that does it. Check again is a read, not a decision: it
 * needs no reason, and the add-on's cooldown (Retry-After) is waited out with the time said.
 */
function ConflictDecision({
  finding,
  name,
  reason,
  pending,
  onResolve,
  onCancel,
}: {
  finding: MergeFinding;
  name: string;
  reason: string;
  pending: boolean;
  onResolve: (answer: Exclude<ConflictAnswer, "check">) => void;
  onCancel: () => void;
}) {
  const names = useShownNames();
  const report = conflictReport(finding.theirs);
  // Names as shown, never ids: the two people a twice-claimed account is recorded for.
  const choices = conflictChoices(
    report,
    name,
    names.user(finding.subject_id, personOr(null)),
    report.bound_to ? names.user(report.bound_to, "the other person") : "the other person",
  );
  const [answer, setAnswer] = useState<ConflictAnswer>(choices[0].answer);
  const chosen = choices.find((c) => c.answer === answer) ?? choices[0];
  const run = useReconcileTarget(finding.target);
  // A check's answer: the row's last sighting when it was pressed (the sweep refreshes it while
  // the add-on still reports the conflict, and closes the row when it stops), a cooldown, or a stop.
  const [checked, setChecked] = useState<{ before: string; at: number; retry?: number; stopped?: string } | null>(null);
  const left = useCountdown(checked?.retry, checked?.at);
  const checking = chosen.answer === "check";
  const needsWhy = !checking && !reason.trim();
  const cooling = checking && left > 0;

  function check() {
    const before = finding.last_seen_at;
    run.mutate(undefined, {
      onSuccess: (pass) =>
        setChecked({
          before,
          at: Date.now(),
          retry: pass.halted && pass.retry_after ? pass.retry_after : undefined,
          stopped:
            pass.halted && !pass.retry_after
              ? checkStopped(pass.detail ?? (pass.reason ? whyNotChecked(pass.reason, name) : `${name} could not be read.`))
              : undefined,
        }),
    });
  }

  const confirm = () => (chosen.answer === "check" ? check() : onResolve(chosen.answer));
  const said = run.error
    ? sentence(describeFailure(run.error))
    : cooling
      ? sentence(cooldownLine(name, left))
      : checked?.stopped
        ? checked.stopped
        : checked && finding.last_seen_at !== checked.before
          ? stillReported(name)
          : checked && !checked.retry
            ? CHECKED_UPDATING
            : null;

  return (
    <>
      <fieldset className="grid gap-1.5">
        <legend className="sr-only">What to do</legend>
        {choices.map((c) => (
          <label key={c.answer} className="flex items-center gap-2 text-small text-ink">
            <Radio name={`answer-${finding.id}`} checked={c.answer === chosen.answer} onChange={() => setAnswer(c.answer)} />
            {c.label}
          </label>
        ))}
      </fieldset>
      {/* What pressing does, beside the button that does it. */}
      <p className="text-small text-muted">{chosen.says}</p>
      <div className="flex flex-wrap gap-2">
        <Button size="sm" variant="accent" isPending={checking ? run.isPending : pending} disabled={needsWhy || cooling}
          reason={needsWhy ? "Say why. This becomes the record of the decision." : undefined} onClick={confirm}>
          {chosen.label}
        </Button>
        <span className="flex-1" />
        <Button size="sm" variant="ghost" onClick={onCancel}>
          Cancel
        </Button>
      </div>
      {said && <p className="text-small text-warn-text">{said}</p>}
    </>
  );
}

/** A limit's value as the finding carries it: a string, or a list of one. */
function valueOf(v: unknown): string | undefined {
  const one = Array.isArray(v) ? v[v.length - 1] : v;
  return one === undefined || one === null ? undefined : String(one);
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
    <Notice tone="neutral" title="This was decided while the page was open." nothingToDo="Syndra checks the account again every 6 hours by default, and clears the finding when it matches.">
      <p>
        <Person id={taken.decided_by} fallback={personOr(null, true)} />
        {taken.decided_at ? (
          <>
            {" "}
            <Relative iso={taken.decided_at} />
          </>
        ) : null}{" "}
        chose <span className="font-semibold">{decisionLabel(taken.decision, name)}</span>
        {taken.decision_reason ? <>: &ldquo;{taken.decision_reason}&rdquo;</> : null}. Your choice was
        not applied and nothing was changed by pressing.
      </p>

      {picked !== null && (
        <p className="mt-1 text-muted">
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

      <p className="mt-1 text-small text-faint">
        The finding has not gone anywhere. It stays here as decided and waiting until Syndra next
        checks the account and finds {name} matches.
      </p>
    </Notice>
  );
}

/** A person's name, linked to their page like every other name. */
function Person({ id, fallback }: { id: string; fallback?: string }) {
  const href = useAccountHref();
  return (
    <UserName id={id} fallback={fallback} href={href(id)} className="font-semibold text-ink" />
  );
}
