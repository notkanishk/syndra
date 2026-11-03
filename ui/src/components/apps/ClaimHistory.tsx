"use client";

import { useState } from "react";

import { UserName } from "@/components/names";
import { ErrorState, RowSkeleton } from "@/components/states";
import { ActionOutcome } from "@/components/ui/ActionOutcome";
import { StatusDot } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { CardColumns } from "@/components/ui/Card";
import { Checkbox } from "@/components/ui/Choice";
import { Modal, ModalFooter, ModalHeader } from "@/components/ui/Modal";
import { Ticks } from "@/components/ui/TokenBlock";
import { Unfold } from "@/components/ui/Unfold";
import { ApiError } from "@/lib/api-client";
import { changeLine, type SayContext } from "@/lib/claims";
import { formatShortDate } from "@/lib/format";
import { outcomeFromError, type ActionOutcome as Outcome } from "@/lib/outcome";
import { useApplications } from "@/lib/queries/useApplications";
import {
  useClaimShapeVersions,
  useRestoreClaimShape,
  type ClaimTemplate,
  type HistoryEntry,
} from "@/lib/queries/useClaimShape";

const SEED = "As Actions v1 sent it";

/**
 * History (A-11 §5, A-14 §7): every save of what the app is told, kept as it
 * was; Go back saves one again. A settings row of the tools card (A-15 §6):
 * the version in use in one line, and with two or more, Show all N unfolds
 * the table. With one there is nothing to go back to, so no button.
 */
export function ClaimHistory({
  appId,
  ctx,
  project,
  templates,
}: {
  appId: string;
  ctx: SayContext;
  project: string;
  templates: ClaimTemplate[];
}) {
  const versions = useClaimShapeVersions(appId);
  const apps = useApplications();
  const [restoring, setRestoring] = useState<HistoryEntry | null>(null);
  const [all, setAll] = useState(false);
  const list = versions.data ?? [];

  const what = (v: HistoryEntry): string => {
    if (v.note.startsWith(SEED)) return "As Actions v1 sent it: `groups` with every role, and `custom_claims`.";
    if (v.restored_from) return `Went back to version ${v.restored_from}.`;
    if (v.copied_from_application_id) {
      const from = apps.data?.find((a) => a.application.id === v.copied_from_application_id)?.application.name ?? "another app";
      return `Copied from ${from}'s version ${v.copied_from_version}.`;
    }
    if (v.changes.length === 0) {
      const t = templates.find((x) => x.id === v.template_id);
      return t ? `Started from the ${t.name} template.` : "A first shape of its own.";
    }
    const lines = v.changes.map((c) => changeLine(c, ctx));
    return lines.length > 3 ? `${lines.slice(0, 2).join(" ")} And ${lines.length - 2} more changes.` : lines.join(" ");
  };

  const current = list[0];
  return (
    <>
      <div className="row-divider flex flex-wrap items-center gap-x-[18px] gap-y-2 px-[22px] pb-4 pt-3.5">
        <span className="w-[170px] flex-none text-[14px] font-semibold">History</span>
        <span className="flex min-w-0 flex-[1_1_260px] flex-col gap-0.5">
          {versions.isLoading ? (
            <RowSkeleton rows={1} avatar={false} label="Loading history" />
          ) : versions.error ? (
            <ErrorState title="Couldn't load the history." error={versions.error} onRetry={() => versions.refetch()} />
          ) : !current ? (
            <>
              <span className="text-[14.5px]">Nothing saved for {ctx.app} yet</span>
              <span className="text-[13px] text-faint">It uses {project}&rsquo;s default.</span>
            </>
          ) : (
            <>
              <span className="text-[14.5px]">
                Version {current.version}
                {current.note.startsWith(SEED) ? ", as Actions v1 sent it" : ""}
              </span>
              <span className="text-[13px] text-faint">
                Saved {formatShortDate(current.saved_at)} by <UserName id={current.saved_by} fallback="Syndra" />.
              </span>
            </>
          )}
        </span>
        {list.length > 1 && (
          <span className="ml-auto flex justify-end whitespace-nowrap">
            <Button variant="ghost" size="sm" aria-expanded={all} onClick={() => setAll(!all)}>
              Show all {list.length}
              <span aria-hidden className={`motion-turn ml-1.5 inline-block ${all ? "rotate-90" : ""}`}>
                ›
              </span>
            </Button>
          </span>
        )}
      </div>
      <Unfold open={all && list.length > 1}>
        <>
          <CardColumns>
            <span className="w-[130px] flex-none">Version ↓</span>
            <span className="min-w-[220px] flex-1">What changed</span>
            <span className="w-[150px] flex-none text-right">Saved</span>
            <span className="w-[210px] flex-none" />
          </CardColumns>
          {list.map((v, i) => (
            <div key={v.version} className="row-divider flex flex-wrap items-start gap-x-[18px] gap-y-1.5 px-5 py-3.5">
              <span className="w-[130px] flex-none text-[15px] font-semibold">Version {v.version}</span>
              <span className="flex min-w-[220px] flex-1 flex-col gap-0.5">
                <span className="text-[14.5px] leading-[1.5] [text-wrap:pretty]">
                  <Ticks text={what(v)} />
                </span>
                {v.note && !v.note.startsWith(SEED) && <span className="text-[13px] text-faint">&ldquo;{v.note}&rdquo;</span>}
                {v.note.startsWith(SEED) && <span className="text-[13px] text-faint">&ldquo;Carried over when Syndra took over from Actions v1.&rdquo;</span>}
              </span>
              <span className="flex w-[150px] flex-none flex-col text-[14px] tablet:items-end">
                {formatShortDate(v.saved_at)}
                <span className="text-[13px] text-muted">
                  <UserName id={v.saved_by} fallback="Syndra" />
                </span>
              </span>
              <span className="flex w-[210px] flex-none justify-end whitespace-nowrap">
                {i === 0 ? (
                  <span className="inline-flex items-center gap-2 text-[13.5px] font-semibold">
                    <StatusDot tone="healthy" />
                    In use
                  </span>
                ) : (
                  <Button size="sm" onClick={() => setRestoring(v)}>
                    Go back to this version
                  </Button>
                )}
              </span>
            </div>
          ))}
        </>
      </Unfold>
      {restoring && (
        <GoBack appId={appId} entry={restoring} next={(list[0]?.version ?? 0) + 1} app={ctx.app} onClose={() => setRestoring(null)} />
      )}
    </>
  );
}

function GoBack({ appId, entry, next, app, onClose }: { appId: string; entry: HistoryEntry; next: number; app: string; onClose: () => void }) {
  const restore = useRestoreClaimShape(appId);
  const [breaking, setBreaking] = useState(false);
  const [understood, setUnderstood] = useState(false);
  const [done, setDone] = useState(false);
  const [outcome, setOutcome] = useState<Outcome | null>(null);
  const go = async () => {
    setOutcome(null);
    try {
      await restore.mutateAsync({ version: entry.version, acknowledge: understood });
      setDone(true);
    } catch (e) {
      // The app will act on going back: ask once, then send it with the tick.
      if (e instanceof ApiError && e.code === "BREAKING_UNACKNOWLEDGED") setBreaking(true);
      else setOutcome(outcomeFromError(e));
    }
  };
  return (
    <Modal open onClose={onClose} busy={restore.isPending} size="sm">
      <ModalHeader title={`Go back to version ${entry.version}`} lede={done ? undefined : `Saves version ${entry.version} again, as version ${next}. Nothing in between is lost.`} />
      <div className="flex flex-col gap-3 px-6 pb-2">
        {done ? (
          <div role="status" className="lift-in flex flex-col gap-2">
            <span className="flex items-center gap-2.5">
              <StatusDot tone="healthy" bloom />
              <span className="font-display text-[22px] font-semibold">Went back</span>
            </span>
            <p className="text-[15px]">
              {app}&rsquo;s claims are now version {next}, as version {entry.version} had them.
            </p>
            <p className="text-[13px] text-faint">It reaches {app} at each person&rsquo;s next sign-in.</p>
          </div>
        ) : (
          breaking && (
            <label className="flex items-start gap-2.5 text-[14.5px] leading-[1.5]">
              <Checkbox className="mt-1" checked={understood} onChange={(e) => setUnderstood(e.target.checked)} />
              <span>I understand {app} will act on going back: people&rsquo;s groups change at their next sign-in.</span>
            </label>
          )
        )}
        {outcome && <ActionOutcome outcome={outcome} />}
      </div>
      <ModalFooter note={done ? undefined : `Takes effect at each person's next sign-in to ${app}.`}>
        {done ? (
          <Button onClick={onClose}>Close</Button>
        ) : (
          <>
            <Button onClick={onClose}>Cancel</Button>
            <Button variant="accent" isPending={restore.isPending} disabled={breaking && !understood} onClick={go}>
              Go back to version {entry.version}
            </Button>
          </>
        )}
      </ModalFooter>
    </Modal>
  );
}
