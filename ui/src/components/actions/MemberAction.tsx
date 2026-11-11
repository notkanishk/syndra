"use client";

import { useQueryClient } from "@tanstack/react-query";
import { useEffect, useState } from "react";

import { CameraScanner, TargetIdInput } from "@/components/actions/TargetIdInput";
import * as copy from "@/components/actions/copy";
import { saidFromError, saidFromState, type Said } from "@/components/actions/outcomes";
import { MEMBER } from "@/components/results/copy";
import { ReadResult } from "@/components/results/ReadResult";
import { ActionOutcome } from "@/components/ui/ActionOutcome";
import { Button } from "@/components/ui/Button";
import { FieldLabel, Input } from "@/components/ui/Input";
import { Modal, ModalFooter, ModalHeader } from "@/components/ui/Modal";
import { Notice } from "@/components/ui/Notice";
import { ApiError } from "@/lib/api-client";
import { invalidateAfter } from "@/lib/queries/invalidation";
import { settled, useMemberAction, useMemberOperation, type ActionParams } from "@/lib/queries/useMemberAction";
import type { MyTargetView, ReadView } from "@/lib/queries/useMyStorage";

type Action = NonNullable<MyTargetView["actions"]>[number];

/**
 * What a member can do to the world on one system (a physical operation such as
 * item.checkout), one row each. The structure is the add-on's declared actions;
 * nothing here knows what an item is. Renders nothing for a system with none.
 */
export function MemberActions({ view }: { view: MyTargetView }) {
  return (
    <>
      {(view.actions ?? []).map((action) => (
        <ActionRow key={action.id} view={view} action={action} />
      ))}
    </>
  );
}

function ActionRow({ view, action }: { view: MyTargetView; action: Action }) {
  const [open, setOpen] = useState(false);
  // The confirm being watched: the server's (a reload keeps it) or one this page just started.
  // Kept after it settles so its result stays on the row until the next press.
  const unsettled = view.unsettled?.operation === action.id ? view.unsettled : undefined;
  const [kept, setKept] = useState<string>();
  useEffect(() => {
    if (unsettled) setKept(unsettled.id);
  }, [unsettled]);
  const watching = kept ?? unsettled?.id;
  // The server's word that one is open, or this page's own poll saying so before the list has caught up.
  const polled = useMemberOperation(view.target, watching).data?.state;
  const blocked = Boolean(unsettled) || polled === "checking";
  const label = copy.actionLabel(action.id);

  return (
    <div className="row-divider flex flex-col gap-3 px-5 py-4" data-action={action.id}>
      <div>
        <Button
          size="sm"
          variant="outline"
          disabled={blocked}
          reason={blocked ? copy.UNSETTLED_REASON : undefined}
          onClick={() => {
            setKept(undefined);
            setOpen(true);
          }}
        >
          {label}
        </Button>
      </div>
      {watching && !open && <Watching target={view.target} name={view.name} id={watching} />}
      {open && (
        <ActionDialog
          view={view}
          action={action}
          label={label}
          onWatch={setKept}
          onClose={() => setOpen(false)}
        />
      )}
    </div>
  );
}

/**
 * Where a confirm stands, asked until it settles. A settled one is shown with
 * its record when the add-on gave one, and the member's list of systems is read
 * again once, so the row's "still being checked" lifts with the truth.
 */
function Watching({ target, name, id }: { target: string; name: string; id: string }) {
  const op = useMemberOperation(target, id);
  const client = useQueryClient();
  const state = op.data?.state;
  useEffect(() => {
    if (settled(state)) invalidateAfter(client, "memberAction");
  }, [state, client]);
  const said = saidFromState(state ?? "checking", op.isError);
  const view = op.data?.view;
  return (
    <div className="flex flex-col gap-3">
      <ActionOutcome word={said.word} outcome={said.outcome} />
      {view?.readable && <ReadResult read={view} name={name} reader={MEMBER} />}
    </div>
  );
}

type Step =
  | { at: "form"; error?: unknown }
  | { at: "scan"; param: string }
  | { at: "preview"; view: ReadView; noChange: boolean }
  | { at: "result"; said: Said; view?: ReadView | null }
  | { at: "checking"; id: string };

/**
 * Preview, then confirm. Each step is the same dialog's content, never a second
 * dialog. A preview sends nothing; a confirm is sent once, and an answer that
 * does not come is checked by asking, never by sending again.
 */
function ActionDialog({
  view,
  action,
  label,
  onWatch,
  onClose,
}: {
  view: MyTargetView;
  action: Action;
  label: string;
  onWatch: (id: string) => void;
  onClose: () => void;
}) {
  const send = useMemberAction(view.target, action.id);
  const [params, setParams] = useState<ActionParams>({});
  const [step, setStep] = useState<Step>({ at: "form" });
  const ready = action.params.every((p) => !p.required || (params[p.name] ?? "").trim() !== "");
  const sent = () => Object.fromEntries(Object.entries(params).map(([k, v]) => [k, v.trim()]));

  const preview = () =>
    send.mutate(
      { params: sent(), mode: "preview" },
      {
        onSuccess: (a) => setStep(a.state === "preview" ? { at: "preview", view: a.view, noChange: a.outcome === "no_change" } : { at: "form" }),
        onError: (error) => setStep({ at: "form", error }),
      },
    );

  const confirm = () =>
    send.mutate(
      { params: sent(), mode: "confirm" },
      {
        onSuccess: (a) => {
          if (a.state === "checking") {
            onWatch(a.operation);
            setStep({ at: "checking", id: a.operation });
          } else if (a.state === "no_change") setStep({ at: "result", said: saidFromState("no_change"), view: a.view });
          else setStep({ at: "result", said: saidFromState("done"), view: a.view });
        },
        onError: (error) => {
          // Their last one is still open: show it and ask after it. Nothing was sent.
          const held = error instanceof ApiError && error.code === "CALL_UNSETTLED" ? error.details?.operation : undefined;
          if (held) {
            onWatch(held);
            setStep({ at: "checking", id: held });
          } else setStep({ at: "result", said: saidFromError(error) });
        },
      },
    );

  const toForm = () => setStep({ at: "form" });
  const titleId = `action-${action.id}-title`;
  const lede =
    step.at === "form" ? copy.FORM_LEDE : step.at === "preview" && !step.noChange && step.view.readable ? copy.PREVIEW_LEDE : undefined;

  return (
    <Modal open onClose={onClose} busy={send.isPending} size="md" labelledBy={titleId}>
      <ModalHeader titleId={titleId} title={step.at === "scan" ? copy.SCAN_TITLE : label} lede={step.at === "scan" ? undefined : lede} />
      <div className="flex flex-col gap-4 px-6">
        {step.at === "scan" && (
          <CameraScanner
            closeLabel="Close"
            onClose={toForm}
            onScan={(value) => {
              setParams((p) => ({ ...p, [step.param]: value }));
              toForm();
            }}
          />
        )}
        {step.at === "form" && (
          <form
            id="action-form"
            className="flex flex-col gap-4"
            onSubmit={(e) => {
              e.preventDefault();
              if (ready && !send.isPending) preview();
            }}
          >
            {step.error !== undefined && <ActionOutcome word={saidFromError(step.error).word} outcome={saidFromError(step.error).outcome} />}
            {action.params.map((p, i) =>
              p.format === "target_id" ? (
                <TargetIdInput
                  key={p.name}
                  id={`param-${p.name}`}
                  label={copy.paramLabel(p.name)}
                  hint={copy.TARGET_ID_HINT}
                  value={params[p.name] ?? ""}
                  onChange={(v) => setParams((all) => ({ ...all, [p.name]: v }))}
                  onScan={() => setStep({ at: "scan", param: p.name })}
                  disabled={send.isPending}
                  autoFocus={i === 0}
                />
              ) : (
                <div key={p.name}>
                  <FieldLabel htmlFor={`param-${p.name}`}>{copy.paramLabel(p.name)}</FieldLabel>
                  <Input
                    id={`param-${p.name}`}
                    value={params[p.name] ?? ""}
                    onChange={(e) => setParams((all) => ({ ...all, [p.name]: e.target.value }))}
                    disabled={send.isPending}
                    autoComplete="off"
                    enterKeyHint="go"
                  />
                </div>
              ),
            )}
          </form>
        )}
        {step.at === "preview" &&
          (step.view.readable ? (
            <>
              <ReadResult read={step.view} name={view.name} reader={MEMBER} />
              {step.noChange && <p className="m-0 text-[14px] text-muted">{copy.PREVIEW_NO_CHANGE}</p>}
            </>
          ) : (
            <Notice tone="warn" title={copy.PREVIEW_UNREADABLE_TITLE} next={copy.PREVIEW_UNREADABLE_NEXT}>
              {copy.previewUnreadableBody(view.name)}
            </Notice>
          ))}
        {step.at === "result" && (
          <>
            <ActionOutcome word={step.said.word} outcome={step.said.outcome} />
            {step.view?.readable && <ReadResult read={step.view} name={view.name} reader={MEMBER} />}
          </>
        )}
        {step.at === "checking" && <Watching target={view.target} name={view.name} id={step.id} />}
      </div>
      {step.at !== "scan" && (
        <ModalFooter
          cancel={
            step.at === "form"
              ? { onClick: onClose, disabled: send.isPending }
              : step.at === "preview" && !step.noChange && step.view.readable
                ? { label: "Back", onClick: toForm, disabled: send.isPending }
                : step.at === "result" && step.said.outcome.kind === "refused"
                  ? { label: "Back", onClick: toForm }
                  : undefined
          }
          note={step.at === "preview" && !step.noChange && step.view.readable ? copy.previewNote(view.name) : undefined}
        >
          {step.at === "form" ? (
            <Button type="submit" form="action-form" variant="accent" disabled={!ready} isPending={send.isPending}>
              {copy.PREVIEW_BUTTON}
            </Button>
          ) : step.at === "preview" && !step.noChange && step.view.readable ? (
            <Button variant="accent" isPending={send.isPending} onClick={confirm}>
              {copy.CONFIRM_BUTTON}
            </Button>
          ) : (
            <Button variant="outline" onClick={step.at === "preview" && !step.noChange ? toForm : onClose}>
              {step.at === "preview" && !step.noChange ? "Back" : "Close"}
            </Button>
          )}
        </ModalFooter>
      )}
    </Modal>
  );
}
