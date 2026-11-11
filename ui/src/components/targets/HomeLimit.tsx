"use client";

import { useState } from "react";

import { ActionOutcome } from "@/components/ui/ActionOutcome";
import { StatusDot } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { FieldHint, FieldLabel, Input, Textarea } from "@/components/ui/Input";
import { Modal, ModalFooter, ModalHeader } from "@/components/ui/Modal";
import { Segmented } from "@/components/ui/Select";
import { UntilChoice, untilWords } from "@/components/targets/UntilChoice";
import { request } from "@/lib/api-client";
import { describeFailure, sentence } from "@/lib/outcome";
import { formatLimit } from "@/lib/fields";
import { useMappingFields } from "@/lib/queries/useFieldSchema";
import { formatBytes, formatShortDate } from "@/lib/format";
import { targetLabel } from "@/lib/nav";
import { useLiftHold } from "@/lib/queries/useHolds";
import { useNameResolver } from "@/lib/queries/useNameResolver";
import type { AccountHome } from "@/lib/queries/useTargets";
import { useMutation, useQueryClient } from "@tanstack/react-query";

import { invalidateAfter } from "@/lib/queries/invalidation";
import { endOfDayISO } from "@/lib/time";
import { roleName } from "@/lib/names";

/**
 * One person's home folder limit, on the system's account list (A-03 §3): the
 * one write place for it. Their roles give a limit; their own takes its place,
 * larger or smaller, until it ends or is reviewed.
 *
 * Nothing here knows anyone's pronouns, so the copy says "they".
 */

const GB = 1024 ** 3;

function gbWords(bytes: number | undefined): string {
  if (bytes === undefined || bytes === 0) return "no limit";
  return formatLimit(bytes, "bytes");
}

function useRoleLabel() {
  const resolver = useNameResolver();
  return (role?: { project_id: string; role_key: string }) =>
    role ? roleName(resolver.resolveRole(role.project_id, role.role_key).value?.display_name, role.role_key) : "";
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
  const field = useMappingFields(target).limit;
  return useMutation({
    mutationFn: (input: { subjectId: string; bytes: number; reason: string; endsOn?: string; reviewOn?: string }) =>
      request<{ id: string }>("/allowances", {
        method: "POST",
        body: {
          subject_id: input.subjectId,
          target,
          field,
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
      invalidateAfter(client, "access");
    },
  });
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
  const limitField = useMappingFields(target).limit;
  const lift = useLiftHold();
  const own = home.own;
  const [kind, setKind] = useState<"size" | "none">(own?.bytes === 0 ? "none" : "size");
  const [gb, setGb] = useState(own && own.bytes > 0 ? String(own.bytes / GB) : "");
  const [reason, setReason] = useState(own?.reason ?? "");
  const [until, setUntil] = useState<"ends" | "review">(own?.review_date && !own.expires_at ? "review" : "ends");
  const [date, setDate] = useState("");
  const reviewing = Boolean(own?.review_due);

  const bytes = kind === "none" ? 0 : Math.round(Number(gb) * GB);
  const sized = kind === "none" || (Number(gb) > 0 && Number.isFinite(Number(gb)));
  const ready = sized && reason.trim() !== "" && date !== "" && Boolean(limitField);
  const from = roleLabel(home.roles_limit_from);
  const rolesWords = home.roles_limit === undefined ? "no limit" : `up to ${gbWords(home.roles_limit)}`;
  const busy = set.isPending || lift.isPending;
  // The dialog ends on its result, with Close; it never closes onto the row.
  const [done, setDone] = useState<{ word: string; message: string } | null>(null);

  // The server replaces the limit in force with this one in a single step, so
  // there is nothing left to end afterwards.
  const save = () =>
    set.mutate(
      {
        subjectId: home.subject_id,
        bytes,
        reason: reason.trim(),
        endsOn: until === "ends" ? endOfDayISO(date) : undefined,
        reviewOn: until === "review" ? endOfDayISO(date) : undefined,
      },
      {
        onSuccess: () =>
          setDone({
            word: reviewing ? "Kept" : "Given",
            message: `${name} has their own limit of ${kind === "none" ? "no limit" : gbWords(bytes)}, and ${system} gets it now. It ${untilWords(until, endOfDayISO(date))}.`,
          }),
      },
    );

  return (
    <Modal open onClose={onClose} size="md" labelledBy="limit-title" busy={busy}>
      <ModalHeader
        title={reviewing ? `Review ${name}'s own home folder limit` : `Give ${name} their own home folder limit`}
        titleId="limit-title"
        lede={`Their roles give them ${rolesWords}${from ? `, from ${from}` : ""}. Their own limit takes the place of that, larger or smaller, until it ends.`}
      />
      {done ? (
        <div className="px-6">
          <ActionOutcome word={done.word} outcome={{ kind: "applied", message: done.message }} />
        </div>
      ) : (
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
          <UntilChoice
            until={until}
            onUntil={setUntil}
            date={date}
            onDate={setDate}
            comesBack={`their roles' ${rolesWords}`}
            endsNote="If they have more than that in their folder then, they keep it all and can't save more until they're under."
          />
        </div>
        {(set.error || lift.error) && (
          <p className="text-[13.5px] text-danger-text">{sentence(describeFailure(set.error ?? lift.error))}</p>
        )}
      </div>
      )}
      <ModalFooter
        cancel={done ? undefined : { onClick: onClose }}
        note={done ? undefined : `Syndra sends it to ${system} as soon as you save.`}
      >
        {done ? (
          <Button variant="outline" onClick={onClose}>
            Close
          </Button>
        ) : (
          <>
            {own && (
              <Button
                variant="danger"
                disabled={busy}
                onClick={() =>
                  lift.mutate(own.allowance_id, {
                    onSuccess: () =>
                      setDone({
                        word: "Ended",
                        message: `${name}'s own limit has ended. Their roles' ${rolesWords} applies again.`,
                      }),
                  })
                }
              >
                End this limit
              </Button>
            )}
            <Button variant="accent" disabled={!ready || busy} onClick={save}>
              {reviewing ? "Keep this limit" : `Give them ${kind === "none" ? "no limit" : gbWords(bytes)}`}
            </Button>
          </>
        )}
      </ModalFooter>
    </Modal>
  );
}
