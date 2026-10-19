"use client";

import { useEffect, useState } from "react";

import { ProjectName, UserName } from "@/components/names";
import { Mono } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Modal, ModalFooter, ModalHeader } from "@/components/ui/Modal";
import { Relative } from "@/components/ui/Time";
import { setCancelConfirmer, type QueuedChange } from "@/lib/cancel-confirm";

/**
 * The one question before an action cancels changes still waiting for you to send
 * (owner, 15 Oct: never invisibly). Whoever queued them may not be the person
 * acting now, and the reason they were set in motion isn't on this screen, so
 * it lists each one: what, for whom, who queued it, when, and why.
 *
 * Mounted once at the root, and opened by `request()` over whatever dialog ran
 * the action; that dialog stays put underneath and reports the outcome. Keep
 * them is first, so it has focus, and Escape means keep them.
 */
export function CancelQueuedConfirm() {
  const [asking, setAsking] = useState<{ changes: QueuedChange[]; answer: (yes: boolean) => void } | null>(null);

  useEffect(
    () =>
      setCancelConfirmer(
        (changes) => new Promise<boolean>((answer) => setAsking({ changes, answer })),
      ),
    [],
  );

  // The dialog underneath also closes on Escape; this question answers it first.
  useEffect(() => {
    if (!asking) return;
    function keep(event: KeyboardEvent) {
      if (event.key !== "Escape") return;
      event.preventDefault();
      event.stopImmediatePropagation();
      settle(false);
    }
    window.addEventListener("keydown", keep, true);
    return () => window.removeEventListener("keydown", keep, true);
  });

  function settle(yes: boolean) {
    asking?.answer(yes);
    setAsking(null);
  }

  if (!asking) return null;
  const many = asking.changes.length > 1;

  return (
    <Modal open onClose={() => settle(false)} labelledBy="cancel-queued-title" size="sm">
      <ModalHeader
        title={many ? `This cancels ${asking.changes.length} changes that haven't been sent` : "This cancels a change that hasn't been sent"}
        titleId="cancel-queued-title"
        lede={
          many
            ? "They're waiting under Pending changes. Going ahead cancels them, so they're never sent, and Change history records that you did."
            : "It's waiting under Pending changes. Going ahead cancels it, so it's never sent, and Change history records that you did."
        }
      />
      <ul className="flex flex-col gap-2 px-6">
        {asking.changes.map((change) => (
          <li key={change.id} className="flex flex-col gap-1 rounded-inner border border-line px-3.5 py-2.5 text-[14px]">
            <span>
              <span className="font-semibold">{change.op_type === "revoke" ? "Removing" : "Giving"}</span>{" "}
              <ProjectName id={change.project_id} /> / <Mono>{change.role_keys.join(", ")}</Mono> for{" "}
              <UserName id={change.user_id} />
            </span>
            <span className="text-[13px] text-faint">
              Queued by <UserName id={change.initiated_by} />, <Relative iso={change.queued_at} />
              {change.reason ? ` · “${change.reason}”` : ""}
            </span>
          </li>
        ))}
      </ul>
      <ModalFooter>
        <Button variant="outline" onClick={() => settle(false)}>
          {many ? "Keep them" : "Keep it"}
        </Button>
        <Button variant="accent" onClick={() => settle(true)}>
          {many ? "Cancel them and go ahead" : "Cancel it and go ahead"}
        </Button>
      </ModalFooter>
    </Modal>
  );
}
