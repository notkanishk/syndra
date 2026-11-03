"use client";

import { useRef, useState } from "react";

import { ErrorState, RowSkeleton } from "@/components/states";
import { ReadGate } from "@/components/states/ReadGate";
import { Avatar } from "@/components/ui/Avatar";
import { StatusDot } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { CheckTrail } from "@/components/ui/CheckTrail";
import { CopiedSeat, useConfirmFor } from "@/components/ui/CopiedSeat";
import { Crossfade } from "@/components/ui/Crossfade";
import { Select } from "@/components/ui/Select";
import { Ticks, TokenBlock } from "@/components/ui/TokenBlock";
import { Unfold } from "@/components/ui/Unfold";
import { clockOf, diffLines, tokenLines } from "@/lib/claims";
import { selectContents, useClipboardAvailable } from "@/lib/clipboard";
import type { CatalogUser } from "@/lib/queries/useCatalogUsers";
import type { Explanation, Preview } from "@/lib/queries/useClaimShape";

import { check5, checkWords, verdictOf, type CheckContext } from "./claim-words";

/**
 * The preview (A-11, A-14 §1, A-15 §5): "Preview for <person>", one verdict
 * line, the five checks one step away, then the token. When every check
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
  const one = preview?.people[0];
  const shown: Explanation | undefined = one?.draft ?? one?.now;
  const asDraft = Boolean(one?.draft);
  const [copied, setCopied] = useState(false);
  const confirmFor = useConfirmFor();
  const canCopy = useClipboardAvailable();
  const ref = useRef<HTMLDivElement>(null);

  const lines = asDraft ? diffLines(one!.now.token, one!.draft!.token) : tokenLines(shown?.token);
  const c5 = shown ? check5(shown, ctx, asDraft) : null;
  const checks = shown && c5 ? [...checkWords(shown, ctx), c5] : [];
  const verdict = verdictOf(checks, asDraft);

  // Shown on demand it is the whole trail; opened by a failure it is compact.
  // A choice holds until whether anything is wrong changes, then the page decides again.
  const bad = verdict.tone !== "healthy";
  const [choice, setChoice] = useState<{ bad: boolean; open: boolean } | null>(null);
  const chosen = choice?.bad === bad ? choice.open : null;
  const open = chosen ?? bad;

  const foot = asDraft ? draftFoot(lines, preview, shown, ctx.person) : c5?.foot;

  return (
    <Card className="w-full">
      <div className="flex flex-wrap items-center gap-2.5 px-[22px] pb-3.5 pt-[18px]">
        <h2 className="type-card-title m-0">Preview</h2>
        <span className="text-[13.5px] text-faint">for</span>
        <span className="flex items-center gap-2 rounded-pill border border-line-strong py-1 pl-1.5 pr-3">
          <Avatar name={ctx.person} size="row" />
          <Select
            aria-label="Preview for"
            value={personId}
            onChange={(e) => onPerson(e.target.value)}
            className="w-[160px] border-0 px-1 py-0 text-[14px]"
          >
            {people.map((u) => (
              <option key={u.id} value={u.id}>
                {u.name}
              </option>
            ))}
          </Select>
        </span>
        <span className="ml-auto flex items-center gap-2.5">
          {shown && <span className="text-[12.5px] text-faint">Checked {clockOf(shown.read_at)}</span>}
          <Button size="sm" onClick={onRetry}>
            Check again
          </Button>
        </span>
      </div>
      <ReadGate
        loading={isLoading && !preview}
        skeleton={
          <div className="px-[22px] pb-5">
            <RowSkeleton rows={3} avatar={false} label="Running the checks" />
          </div>
        }
      >
        {error && !preview ? (
          <div className="px-[22px] pb-5">
            <ErrorState title="Couldn't run the checks." error={error} onRetry={onRetry} />
          </div>
        ) : shown ? (
          <>
            <div className="flex flex-wrap items-center gap-x-2.5 gap-y-1.5 px-[22px] pb-3">
              <StatusDot tone={verdict.tone} />
              <Crossfade id={verdict.text} inline>
                <span className="text-[14.5px] font-semibold text-ink">{verdict.text}</span>
              </Crossfade>
              <span className="-ml-1 inline-flex">
                <Button variant="ghost" size="sm" aria-expanded={open} onClick={() => setChoice({ bad, open: !open })}>
                  {open ? "Hide the checks" : "Show the checks"}
                </Button>
              </span>
            </div>
            <Unfold open={open}>
              <div className="px-[22px] pb-4 pt-0.5">
                <CheckTrail checks={checks} compact={chosen === null} />
              </div>
            </Unfold>
            <div ref={ref} className="flex flex-col gap-2.5 px-[22px]">
              <TokenBlock lines={lines} empty={<span className="text-muted">{"{}"}</span>} className="[overflow-wrap:anywhere]" />
              {shown.would_receive && <TokenBlock lines={tokenLines(shown.would_receive)} comment="once their claims are built" />}
            </div>
            <div className="flex flex-wrap items-center gap-x-3 gap-y-2 px-[22px] pb-5 pt-2.5">
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
          </>
        ) : null}
      </ReadGate>
    </Card>
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
