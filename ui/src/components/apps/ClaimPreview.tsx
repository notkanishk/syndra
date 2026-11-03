"use client";

import { useRef, useState } from "react";

import { ErrorState, RowSkeleton } from "@/components/states";
import { ReadGate } from "@/components/states/ReadGate";
import { Avatar } from "@/components/ui/Avatar";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { CheckTrail } from "@/components/ui/CheckTrail";
import { CopiedSeat, useConfirmFor } from "@/components/ui/CopiedSeat";
import { Select } from "@/components/ui/Select";
import { Ticks, TokenBlock } from "@/components/ui/TokenBlock";
import { clockOf, diffLines, tokenLines } from "@/lib/claims";
import { selectContents, useClipboardAvailable } from "@/lib/clipboard";
import type { CatalogUser } from "@/lib/queries/useCatalogUsers";
import type { Explanation, Preview } from "@/lib/queries/useClaimShape";

import { check5, checkWords, type CheckContext } from "./claim-words";

/**
 * The preview (A-11, A-14 §1): the five checks for one person, ending in the
 * token Zitadel receives. With a draft, the checks run on the draft and the
 * token marks what leaves (−) and joins (+). Every word is read off the
 * backend's explain step, which runs the one shaper.
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
  lede,
}: {
  preview?: Preview;
  isLoading: boolean;
  error: unknown;
  onRetry: () => void;
  people: CatalogUser[];
  personId: string;
  onPerson: (id: string) => void;
  ctx: CheckContext;
  lede: string;
}) {
  const one = preview?.people[0];
  const shown: Explanation | undefined = one?.draft ?? one?.now;
  const asDraft = Boolean(one?.draft);
  const [copied, setCopied] = useState(false);
  const confirmFor = useConfirmFor();
  const canCopy = useClipboardAvailable();
  const ref = useRef<HTMLDivElement>(null);

  const lines = asDraft ? diffLines(one!.now.token, one!.draft!.token) : tokenLines(shown?.token);
  const changed = new Set(lines.filter((l) => l.mark !== " ").map((l) => l.key)).size;
  const c5 = shown ? check5(shown, ctx, asDraft) : null;
  const note = asDraft
    ? changed === 0
      ? `Nothing changes for ${ctx.person} yet.`
      : `${changed === 1 ? "One difference" : `${changed} differences`} for ${ctx.person}. A line marked − leaves the token; a line marked + joins it.`
    : undefined;

  return (
    <Card className="w-full">
      <div className="px-[22px] pb-3.5 pt-5">
        <h2 className="type-card-title">Preview</h2>
        <p className="mt-1 text-[14px] leading-[1.5] text-faint">{lede}</p>
      </div>
      <div className="flex flex-wrap items-center gap-2.5 px-[22px] pb-[18px]">
        <span className="text-[13.5px] text-faint">for</span>
        <span className="flex items-center gap-2 rounded-pill border border-line-strong py-1 pl-1.5 pr-3">
          <Avatar name={ctx.person} size="row" />
          <Select
            aria-label="Preview for"
            value={personId}
            onChange={(e) => onPerson(e.target.value)}
            className="w-[180px] border-0 px-1 py-0 text-[14px]"
          >
            {people.map((u) => (
              <option key={u.id} value={u.id}>
                {u.name}
              </option>
            ))}
          </Select>
        </span>
        <span className="flex-1" />
        {shown && <span className="text-[12.5px] text-faint">Checked {clockOf(shown.read_at)}</span>}
        <Button size="sm" onClick={onRetry}>
          Check again
        </Button>
      </div>
      <div className="px-[22px] pb-5">
        <ReadGate loading={isLoading && !preview} skeleton={<RowSkeleton rows={4} avatar={false} label="Running the checks" />}>
          {error && !preview ? (
            <ErrorState title="Couldn't run the checks." error={error} onRetry={onRetry} />
          ) : shown && c5 ? (
            <CheckTrail
              checks={[
                ...checkWords(shown, ctx),
                {
                  tone: c5.tone,
                  sentence: c5.sentence,
                  children: (
                    <div ref={ref} className="flex flex-col gap-2.5">
                      <TokenBlock lines={lines} empty={<span className="text-muted">{"{}"}</span>} />
                      {shown.would_receive && (
                        <TokenBlock lines={tokenLines(shown.would_receive)} comment="once their claims are built" />
                      )}
                      <p className="max-w-[62ch] text-[13.5px] leading-[1.55] text-muted [text-wrap:pretty]">
                        <Ticks text={[note, ...(c5.faint ?? [])].filter(Boolean).join(" ")} />
                      </p>
                      <span>
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
                  ),
                },
              ]}
            />
          ) : null}
        </ReadGate>
      </div>
    </Card>
  );
}
