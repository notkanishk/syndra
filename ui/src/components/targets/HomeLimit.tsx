"use client";

import { useState } from "react";

import { StatusDot } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { FieldHint, FieldLabel, Input, Textarea } from "@/components/ui/Input";
import { Modal, ModalFooter, ModalHeader } from "@/components/ui/Modal";
import { Segmented } from "@/components/ui/Select";
import { request } from "@/lib/api-client";
import { describeFailure, sentence } from "@/lib/outcome";
import { formatBytes, formatShortDate, humanizeKey } from "@/lib/format";
import { targetLabel } from "@/lib/nav";
import { useLiftHold } from "@/lib/queries/useHolds";
import { useNameResolver } from "@/lib/queries/useNameResolver";
import type { AccountHome } from "@/lib/queries/useTargets";
import { useMutation, useQueryClient } from "@tanstack/react-query";

/**
 * One person's home folder limit, on the system's account list (A-03 §3): the
 * one write place for it. Their roles give a limit; their own takes its place,
 * larger or smaller, until it ends or is reviewed.
 *
 * Nothing here knows anyone's pronouns, so the copy says "they".
 */

const GB = 1024 ** 3;
const FIELD = "home_quota";

function gbWords(bytes: number | undefined): string {
  if (bytes === undefined || bytes === 0) return "no limit";
  const n = bytes / GB;
  return `${Number.isInteger(n) ? n : n.toFixed(1)} GB`;
}

function useRoleLabel() {
  const resolver = useNameResolver();
  return (role?: { project_id: string; role_key: string }) =>
    role ? resolver.resolveRole(role.project_id, role.role_key).value?.display_name || humanizeKey(role.role_key) : "";
}

/** "Home folder up to 20 GB, from Fabrication lead", and what is owed about it. */
export function HomeLine({ home }: { home: AccountHome }) {
  const roleLabel = useRoleLabel();
  if (!home.gives_home && !home.own) return null;
  const from = roleLabel(home.roles_limit_from);
  const roles = home.roles_limit === undefined ? "no limit" : `up to ${gbWords(home.roles_limit)}`;

  if (home.own) {
    const own = home.own;
    return (
      <span className="flex w-full flex-col gap-0.5 pl-0.5">
        <span className="text-[13.5px] text-ink">
          Home folder {own.bytes === 0 ? "with no limit" : `up to ${gbWords(own.bytes)}`}, their own
        </span>
        {own.review_due ? (
          <span className="flex items-baseline gap-2 text-[13px] text-warn-text">
            <StatusDot tone="warn" />
            Review was due {formatShortDate(own.review_date)}. It stays at {gbWords(own.bytes)} until you keep it or end it.
          </span>
        ) : (
          <span className="text-[13px] text-faint">
            {own.expires_at ? `Ends ${formatShortDate(own.expires_at)}` : `Review on ${formatShortDate(own.review_date)}`} · “
            {own.reason}”
          </span>
        )}
      </span>
    );
  }
  return (
    <span className="flex w-full flex-col gap-0.5 pl-0.5">
      <span className="text-[13.5px] text-muted">
        Home folder {home.roles_limit === undefined ? "with no limit" : roles}
        {from ? `, from ${from}` : ""}
      </span>
      {home.own_ended && home.roles_limit !== undefined && (
        <span className="flex items-baseline gap-2 text-[13px] text-warn-text [text-wrap:pretty]">
          <StatusDot tone="warn" />
          Their own {gbWords(home.own_ended.bytes)} ended on {formatShortDate(home.own_ended.ended_at)} with{" "}
          {formatBytes(home.used_bytes ?? 0)} in their folder. They can open and delete files but can&apos;t save new
          ones until they&apos;re under {gbWords(home.roles_limit)}. Nothing was deleted.
        </span>
      )}
    </span>
  );
}

/** The row's first verb, by what is true of the limit now. */
export function limitVerb(home: AccountHome | undefined): string | null {
  if (!home || (!home.gives_home && !home.own)) return null;
  if (home.own?.review_due) return "Review limit";
  return home.own ? "Change limit" : "Set a limit";
}

function useSetOwnLimit(target: string) {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (input: { subjectId: string; bytes: number; reason: string; endsOn?: string; reviewOn?: string }) =>
      request<{ id: string }>("/allowances", {
        method: "POST",
        body: {
          subject_id: input.subjectId,
          target,
          field: FIELD,
          value: String(input.bytes),
          // One person's own value in place of what their roles give: the
          // allowance's `set` direction, bounded like every hold.
          direction: "set",
          reason: input.reason,
          expires_at: input.endsOn,
          review_date: input.reviewOn,
        },
      }),
    onSuccess: () => {
      client.invalidateQueries({ queryKey: ["targets", target, "homes"] });
      client.invalidateQueries({ queryKey: ["governance", "holds", "review-due"] });
      client.invalidateQueries({ queryKey: ["users"] });
    },
  });
}

/** End of the chosen day, so "ends 30 Nov" means all of 30 Nov. */
function endOfDay(date: string): string {
  return new Date(`${date}T23:59:59`).toISOString();
}

export function LimitDialog({
  target,
  name,
  home,
  onClose,
}: {
  target: string;
  /** The person's name, as the row shows it. */
  name: string;
  home: AccountHome;
  onClose: () => void;
}) {
  const system = targetLabel(target);
  const roleLabel = useRoleLabel();
  const set = useSetOwnLimit(target);
  const lift = useLiftHold();
  const own = home.own;
  const [kind, setKind] = useState<"size" | "none">(own?.bytes === 0 ? "none" : "size");
  const [gb, setGb] = useState(own && own.bytes > 0 ? String(own.bytes / GB) : "");
  const [reason, setReason] = useState(own?.reason ?? "");
  const [until, setUntil] = useState<"ends" | "review">(own?.review_date && !own.expires_at ? "review" : "ends");
  const [date, setDate] = useState("");
  const reviewing = Boolean(own?.review_due);
  // The new limit saved and the one it replaces didn't end. Saving again would
  // give a third; only the ending is retried.
  const [savedNew, setSavedNew] = useState(false);

  const bytes = kind === "none" ? 0 : Math.round(Number(gb) * GB);
  const sized = kind === "none" || (Number(gb) > 0 && Number.isFinite(Number(gb)));
  const ready = sized && reason.trim() !== "" && date !== "";
  const from = roleLabel(home.roles_limit_from);
  const rolesWords = home.roles_limit === undefined ? "no limit" : `up to ${gbWords(home.roles_limit)}`;
  const busy = set.isPending || lift.isPending;

  const save = () =>
    set.mutate(
      {
        subjectId: home.subject_id,
        bytes,
        reason: reason.trim(),
        endsOn: until === "ends" ? endOfDay(date) : undefined,
        reviewOn: until === "review" ? endOfDay(date) : undefined,
      },
      {
        // The newest own limit is the one in force; the one it replaces ends,
        // so nothing stays due for review behind it.
        onSuccess: () =>
          own ? lift.mutate(own.allowance_id, { onSuccess: onClose, onError: () => setSavedNew(true) }) : onClose(),
      },
    );

  return (
    <Modal open onClose={onClose} size="md" labelledBy="limit-title" busy={busy}>
      <ModalHeader
        title={reviewing ? `Review ${name}'s own home folder limit` : `Give ${name} their own home folder limit`}
        titleId="limit-title"
        lede={`Their roles give them ${rolesWords}${from ? `, from ${from}` : ""}. Their own limit takes the place of that, larger or smaller, until it ends.`}
      />
      <div className="flex flex-col gap-4 px-6">
        <div className="flex flex-col gap-1.5">
          <span className="type-label text-label">Limit</span>
          <span className="flex flex-wrap items-center gap-3">
            <Segmented
              label="Limit"
              value={kind}
              onChange={setKind}
              options={[
                { value: "size", label: "Up to a size" },
                { value: "none", label: "No limit" },
              ]}
            />
            {kind === "size" && (
              <span className="flex items-center gap-2">
                <span className="w-[110px]">
                  <Input aria-label="Limit in GB" type="number" min={1} value={gb} onChange={(e) => setGb(e.target.value)} />
                </span>
                <span className="text-[14px] text-muted">GB</span>
              </span>
            )}
          </span>
          {home.used_bytes !== undefined && (
            <FieldHint>They have {formatBytes(home.used_bytes)} in their home folder now.</FieldHint>
          )}
        </div>
        <div className="flex flex-col gap-1.5">
          <FieldLabel htmlFor="limit-why">Why</FieldLabel>
          <Textarea id="limit-why" value={reason} onChange={(e) => setReason(e.target.value)} />
          <FieldHint>Whoever looks at this limit next reads this and nothing else.</FieldHint>
        </div>
        <div className="flex flex-col gap-1.5">
          <span className="type-label text-label">Until</span>
          <span className="flex flex-wrap items-center gap-3">
            <Segmented
              label="Until"
              value={until}
              onChange={setUntil}
              options={[
                { value: "ends", label: "Ends on" },
                { value: "review", label: "Review on" },
              ]}
            />
            <span className="w-[170px]">
              <Input aria-label={until === "ends" ? "Ends on" : "Review on"} type="date" value={date} onChange={(e) => setDate(e.target.value)} />
            </span>
          </span>
          <FieldHint>
            {until === "ends"
              ? `On that day their roles' ${rolesWords} comes back by itself. If they have more than that in their folder then, they keep it all and can't save more until they're under.`
              : "It stays until you keep it or end it. On that day it lists under Holds due."}
          </FieldHint>
        </div>
        {savedNew ? (
          <p className="text-[13.5px] text-danger-text">
            The new limit is saved. The one it replaces couldn&apos;t be ended, so it still shows as theirs:{" "}
            {sentence(describeFailure(lift.error))}
          </p>
        ) : (
          (set.error || lift.error) && (
            <p className="text-[13.5px] text-danger-text">{sentence(describeFailure(set.error ?? lift.error))}</p>
          )
        )}
      </div>
      <ModalFooter note={`Syndra sends it to ${system} as soon as you save.`}>
        <Button variant="ghost" onClick={onClose}>
          Cancel
        </Button>
        {own && savedNew ? (
          <Button variant="accent" disabled={busy} onClick={() => lift.mutate(own.allowance_id, { onSuccess: onClose })}>
            End the previous one
          </Button>
        ) : (
          <>
            {own && (
              <Button variant="outline" disabled={busy} onClick={() => lift.mutate(own.allowance_id, { onSuccess: onClose })}>
                End it
              </Button>
            )}
            <Button variant="accent" disabled={!ready || busy} onClick={save}>
              {reviewing ? "Keep it" : `Give them ${kind === "none" ? "no limit" : gbWords(bytes)}`}
            </Button>
          </>
        )}
      </ModalFooter>
    </Modal>
  );
}
