"use client";

import { useId, useState } from "react";

import { UserName } from "@/components/names/UserName";
import { ActionOutcome } from "@/components/ui/ActionOutcome";
import { Button } from "@/components/ui/Button";
import { FieldHint, FieldLabel, Textarea } from "@/components/ui/Input";
import { Modal, ModalFooter, ModalHeader } from "@/components/ui/Modal";
import { Segmented } from "@/components/ui/Select";
import { formatWhen } from "@/lib/format";
import { outcomeFromError } from "@/lib/outcome";
import { useApplications } from "@/lib/queries/useApplications";
import { useLookUpSignIns, type LookupRange } from "@/lib/queries/useSignIns";

/**
 * "Who signed in, and when" (owner, 3 Nov). Syndra keeps no such record; this
 * asks Zitadel's own log once, for one person or one app, with a reason. The
 * lookup is in Change history before Zitadel is asked, and the answer is shown
 * here and kept nowhere. The dialog ends on its own result (96b).
 */
export function SignInLookup({
  userId,
  appId,
  name,
  onClose,
}: {
  userId?: string;
  appId?: string;
  /** The person's or the app's name, as the page shows it. */
  name: string;
  onClose: () => void;
}) {
  const ids = useId();
  const lookup = useLookUpSignIns();
  const apps = useApplications();
  const [days, setDays] = useState<LookupRange>(30);
  const [reason, setReason] = useState("");
  const appName = (id: string) => apps.data?.find((a) => a.application.id === id)?.application.name;
  const result = lookup.data;
  const forPerson = Boolean(userId);
  const run = () => lookup.mutate({ user_id: userId, app_id: appId, days, reason: reason.trim() });

  const found = result?.sign_ins ?? [];
  const range = `the last ${days} days`;
  const message = forPerson
    ? found.length === 0
      ? `Zitadel's log has no sign-in by ${name} in ${range}.`
      : `${found.length} ${found.length === 1 ? "sign-in" : "sign-ins"} by ${name} in ${range}.`
    : found.length === 0
      ? `Zitadel's log has no sign-in to ${name} in ${range}.`
      : `${found.length} ${found.length === 1 ? "sign-in" : "sign-ins"} to ${name} in ${range}.`;

  return (
    <Modal open onClose={onClose} busy={lookup.isPending} size="md" labelledBy={`${ids}-title`}>
      <ModalHeader
        title={forPerson ? `Look up ${name}'s sign-ins` : `Look up sign-ins to ${name}`}
        titleId={`${ids}-title`}
        lede="Syndra keeps only each person's last sign-in. This reads who signed in, and when, from Zitadel's own log."
      />
      <div className="flex flex-col gap-4 px-6">
        {result ? (
          <>
            <ActionOutcome
              word="Looked up"
              outcome={{ kind: "applied", message, detail: "It's on the Audit page with your reason." }}
            />
            {found.length > 0 && (
              <ul className="m-0 flex max-h-[320px] list-none flex-col overflow-auto rounded-inner border border-line p-0">
                {found.map((s, i) => (
                  <li
                    key={`${s.user_id}-${s.at}-${i}`}
                    className={`flex flex-wrap items-baseline gap-x-3 gap-y-0.5 px-4 py-2.5 text-[14px] ${i > 0 ? "row-divider" : ""}`}
                  >
                    <span className="w-[130px] flex-none tabular-nums text-muted">{formatWhen(s.at)}</span>
                    {forPerson ? (
                      <span>{appName(s.app_id) ?? <span className="text-faint">App unknown</span>}</span>
                    ) : (
                      <UserName id={s.user_id} className="font-semibold text-ink" />
                    )}
                  </li>
                ))}
              </ul>
            )}
            {result.truncated && (
              <p className="m-0 text-[13px] leading-[1.5] text-faint [text-wrap:pretty]">
                Zitadel&apos;s log held more sign-ins in these days than one lookup reads, so older ones may be missing. A
                shorter range reads further back within it.
              </p>
            )}
          </>
        ) : (
          <>
            <div className="flex flex-col gap-1.5">
              <span className="type-label text-label">Over</span>
              <Segmented
                label="Over"
                value={String(days) as "7" | "30" | "90"}
                onChange={(v) => setDays(Number(v) as LookupRange)}
                options={[
                  { value: "7", label: "7 days" },
                  { value: "30", label: "30 days" },
                  { value: "90", label: "90 days" },
                ]}
              />
            </div>
            <div className="flex flex-col gap-1.5">
              <FieldLabel htmlFor={`${ids}-why`}>Why</FieldLabel>
              <Textarea id={`${ids}-why`} value={reason} maxLength={500} onChange={(e) => setReason(e.target.value)} />
              <FieldHint>Recorded with the lookup, for whoever reads Change history.</FieldHint>
            </div>
            {lookup.error && (
              <ActionOutcome
                word="Not looked up"
                outcome={{ ...outcomeFromError(lookup.error), next: "Check the address and look it up again." }}
              />
            )}
          </>
        )}
      </div>
      <ModalFooter
        note={result ? undefined : "This finishes now. Zitadel's log is read once, and the lookup is recorded with your reason."}
        cancel={result ? undefined : { onClick: onClose, disabled: lookup.isPending }}
      >
        {result ? (
          <Button variant="outline" onClick={onClose}>
            Close
          </Button>
        ) : (
          <Button
            variant="accent"
            isPending={lookup.isPending}
            disabled={reason.trim() === ""}
            reason={reason.trim() === "" ? "Say why first." : undefined}
            onClick={run}
          >
            Look up sign-ins
          </Button>
        )}
      </ModalFooter>
    </Modal>
  );
}
