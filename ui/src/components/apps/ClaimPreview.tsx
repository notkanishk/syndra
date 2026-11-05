"use client";

import { useRef, useState } from "react";

import { ErrorState, RowSkeleton } from "@/components/states";
import { ReadGate } from "@/components/states/ReadGate";
import { StatusDot } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Card, CardHeader } from "@/components/ui/Card";
import { CheckTrail } from "@/components/ui/CheckTrail";
import { CopiedSeat, useConfirmFor } from "@/components/ui/CopiedSeat";
import { ApiError } from "@/lib/api-client";
import { Crossfade } from "@/lib/motion";
import { Select } from "@/components/ui/Select";
import { Ticks, TokenBlock } from "@/components/ui/TokenBlock";
import { Unfold } from "@/lib/motion";
import { clockOf, diffLines, tokenLines } from "@/lib/claims";
import { selectContents, useClipboardAvailable } from "@/lib/clipboard";
import type { CatalogUser } from "@/lib/queries/useCatalogUsers";
import type { Explanation, LiveCheck, Preview } from "@/lib/queries/useClaimShape";
import { sortAccounts } from "@/lib/sort";

import { check5, check6, checkWords, stepName, verdictOf, type CheckContext } from "./claim-words";

/**
 * The preview (A-11, A-14 §1, A-15 §5): "Preview for <person>", one verdict
 * line, the six checks one step away, then the token. When every check
 * passes the trail waits behind Show the checks; a failure or a warning opens
 * it by itself, compact, so only what didn't pass carries its detail. With a
 * draft the checks run on the draft and the token marks what leaves (−) and
 * joins (+). Every word is read off the backend's explain step.
 */
export function ClaimPreview({
  preview,
  isLoading,
  error,
  onRetry,
  people,
  personId,
  onPerson,
  ctx,
}: {
  preview?: Preview;
  isLoading: boolean;
  error: unknown;
  onRetry: () => void;
  people: CatalogUser[];
  personId: string;
  onPerson: (id: string) => void;
  ctx: CheckContext;
}) {
  // People A to Z, then service accounts A to Z under their own label: the
  // selected one is marked by the select, never moved to the top.
  const sorted = sortAccounts(people);
  const humans = sorted.filter((u) => !u.service_account);
  const machines = sorted.filter((u) => u.service_account);
  const one = preview?.people[0];
  const shown: Explanation | undefined = one?.draft ?? one?.now;
  const asDraft = Boolean(one?.draft);
  const [copied, setCopied] = useState(false);
  const confirmFor = useConfirmFor();
  const canCopy = useClipboardAvailable();
  const ref = useRef<HTMLDivElement>(null);

  const lines = asDraft ? diffLines(one!.now.token, one!.draft!.token) : tokenLines(shown?.token);
  const c5 = shown ? check5(shown, ctx, asDraft) : null;
  const checks = shown && c5 ? [...checkWords(shown, ctx), c5, ...check6(shown, ctx)] : [];
  const verdict = verdictOf(checks, asDraft);

  // Shown on demand it is the whole trail; opened by a failure it is compact.
  // A choice holds until whether anything is wrong changes, then the page decides again.
  const bad = verdict.tone !== "healthy";
  const [choice, setChoice] = useState<{ bad: boolean; open: boolean } | null>(null);
  const chosen = choice?.bad === bad ? choice.open : null;
  const open = chosen ?? bad;

  // A re-run failed but an earlier answer is held: show it receded with the reason.
  const stale = Boolean(error && preview && !(error instanceof ApiError && error.code === "VALIDATION_FAILED"));
  const foot = asDraft ? draftFoot(lines, preview, shown, ctx.person) : c5?.foot;

  return (
    <Card className="w-full">
      <CardHeader
        title={
          <span className="flex flex-wrap items-center gap-2.5">
            Preview
            <span className="text-[13.5px] font-normal text-faint">for</span>
            <Select
              aria-label="Preview for"
              value={personId}
              onChange={(e) => onPerson(e.target.value)}
              className="w-[200px] py-2 text-[14px]"
            >
              {humans.map((u) => (
                <option key={u.id} value={u.id}>
                  {u.name}
                </option>
              ))}
              {machines.length > 0 && (
                <optgroup label="Service accounts">
                  {machines.map((u) => (
                    <option key={u.id} value={u.id}>
                      {u.name}
                    </option>
                  ))}
                </optgroup>
              )}
            </Select>
          </span>
        }
        note={shown && `Checked ${clockOf(shown.read_at)}`}
        action={
          <Button size="sm" onClick={onRetry}>
            Check again
          </Button>
        }
      />
      <ReadGate
        loading={isLoading && !preview}
        skeleton={
          <div className="px-5 pb-5">
            <RowSkeleton rows={3} avatar={false} label="Running the checks" />
          </div>
        }
      >
        {error && !preview ? (
          <div className="px-5 pb-5">
            <ErrorState title="Couldn't run the checks." error={error} onRetry={onRetry} />
          </div>
        ) : shown ? (
          <>
            {stale && (
              <div className="px-5 pb-3">
                <ErrorState
                  title={`Couldn't run the checks again. This is from ${clockOf(shown.read_at)}.`}
                  error={error}
                  onRetry={onRetry}
                  bare
                />
              </div>
            )}
            {/* A failed re-run leaves the last good preview in place, receded: its age is in the header and the Notice above. */}
            <div className={stale ? "opacity-45 motion-tint" : "motion-tint"} {...(stale ? { inert: true, "aria-disabled": true } : {})}>
            <div className="flex flex-wrap items-center gap-x-2.5 gap-y-1.5 px-5 pb-3">
              <StatusDot tone={verdict.tone} />
              <Crossfade id={verdict.text} inline>
                <span className="text-[14.5px] font-semibold text-ink">{verdict.text}</span>
              </Crossfade>
              <span className="ml-1 inline-flex">
                <Button size="sm" aria-expanded={open} onClick={() => setChoice({ bad, open: !open })}>
                  {open ? "Hide the checks" : "Show the checks"}
                </Button>
              </span>
            </div>
            <Unfold open={open}>
              <div className="px-5 pb-4 pt-0.5">
                <CheckTrail checks={checks} compact={chosen === null} />
              </div>
            </Unfold>
            <div ref={ref} className="flex flex-col gap-2.5 px-5">
              <TokenBlock lines={lines} empty={<span className="text-muted">{"{}"}</span>} className="[overflow-wrap:anywhere]" />
              {shown.would_receive && <TokenBlock lines={tokenLines(shown.would_receive)} comment="once their claims are built" />}
            </div>
            {!asDraft && <LiveLine live={preview?.live?.[personId]} person={ctx.person} />}
            <div className="flex flex-wrap items-center gap-x-3 gap-y-2 px-5 pb-5 pt-2.5">
              <span className="min-w-0 flex-[1_1_260px] text-[13px] leading-[1.55] text-faint [text-wrap:pretty]">
                {foot && <Ticks text={foot} />}
              </span>
              <span className="ml-auto inline-flex flex-none whitespace-nowrap">
                <Button
                  size="sm"
                  onClick={async () => {
                    const text = JSON.stringify(shown.token ?? {}, null, 2);
                    if (canCopy) await navigator.clipboard.writeText(text);
                    else selectContents(ref.current);
                    confirmFor(setCopied);
                  }}
                >
                  <CopiedSeat done={copied} idle={canCopy ? "Copy preview" : "Select preview"} word={canCopy ? "Copied" : "Selected"} dot={canCopy} />
                </Button>
              </span>
            </div>
            </div>
          </>
        ) : null}
      </ReadGate>
    </Card>
  );
}

/**
 * The live check: the person's last real sign-in that debug mode saw, against
 * this preview. The backend compares the two; this only says the answer.
 * Nothing seen: one faint line saying how to see one. A sign-in is never faked.
 */
export function LiveLine({ live, person }: { live?: LiveCheck; person: string }) {
  if (!live) {
    return (
      <p className="px-5 pt-2.5 text-[13px] leading-[1.55] text-faint [text-wrap:pretty]">
        No sign-in of {person}&apos;s has been seen to compare with this. Turn on debug mode for them below, and their next sign-in is
        checked against it.
      </p>
    );
  }
  const where = stepName(live.function);
  const at = clockOf(live.at);
  const sentence = live.match
    ? live.expected.length
      ? `${person}'s last sign-in, at ${at}, carried exactly this in ${where}.`
      : `${person}'s last sign-in, at ${at}, carried nothing in ${where}, as this shape says.`
    : `${person}'s last sign-in, at ${at}, carried something different in ${where}.`;
  return (
    <div className="flex flex-col gap-2 px-5 pt-2.5">
      <span className="flex items-baseline gap-2.5 text-[13.5px] leading-[1.55] text-ink">
        <StatusDot tone={live.match ? "healthy" : "warn"} />
        <span>
          <span className="sr-only">{live.match ? "Matches. " : "Doesn't match. "}</span>
          {sentence}
        </span>
      </span>
      {!live.match && (
        <>
          <TokenBlock
            lines={tokenLines(Object.fromEntries(live.served.map((c) => [c.key, c.value])))}
            comment={`what Syndra sent at ${at}`}
            empty={<span className="text-muted">{"{}"}</span>}
            className="[overflow-wrap:anywhere]"
          />
          <span className="text-[13px] leading-[1.55] text-faint">
            If the shape or their roles changed after {at}, have them sign in again to compare.
          </span>
        </>
      )}
    </div>
  );
}

/**
 * The faint line under a draft's token: "One difference for Ada Lovelace:
 * `staff` becomes `netbird-staff`, in both claims." when the draft's one change
 * is a rename of a role they hold; otherwise how many lines differ.
 */
function draftFoot(
  lines: { key: string; mark: string }[],
  preview: Preview | undefined,
  shown: Explanation | undefined,
  person: string,
): string {
  const keys = new Set(lines.filter((l) => l.mark !== " ").map((l) => l.key)).size;
  if (keys === 0) return `Nothing changes for ${person} yet.`;
  const held = (shown?.checks[1]?.facts?.roles as string[] | undefined) ?? [];
  const changes = preview?.changes ?? [];
  const rename = changes.length === 1 && changes[0].kind === "rename" && changes[0].role && held.includes(changes[0].role) ? changes[0] : null;
  if (rename) {
    const where = keys === 1 ? "" : keys === 2 ? ", in both claims" : `, in all ${keys} claims`;
    return `One difference for ${person}: \`${rename.from}\` becomes \`${rename.to}\`${where}.`;
  }
  return `${keys === 1 ? "One difference" : `${keys} differences`} for ${person}.`;
}
