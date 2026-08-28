"use client";

import { useState } from "react";

import {
  SourceChip,
  sourceQualifier,
  type RoleReason,
  type SourceKind,
} from "@/components/access/AccessSource";
import { ActionOutcome } from "@/components/ui/ActionOutcome";
import { Mono } from "@/components/ui/Badge";
import { Button, ButtonLink } from "@/components/ui/Button";
import { Modal, ModalFooter, ModalHeader } from "@/components/ui/Modal";
import { useRemoveDirectGrant } from "@/lib/queries/useRoleMembers";
import { useRemoveBundle } from "@/lib/queries/useBundles";
import { humanizeKey } from "@/lib/format";
import { outcomeFromError, type ActionOutcome as Outcome } from "@/lib/outcome";

/**
 * Source-specific removal.
 *
 * There is never a generic "Revoke role". A person can hold one role through
 * several sources at once, so a generic action is ambiguous at best and
 * destructive at worst. The action is named after the thing being removed, and
 * the confirmation states the residual outcome — what they are left holding.
 *
 * That sentence is not garnish. It is the difference between a safe click and
 * an outage on the laser cutter.
 */

export interface Removal {
  projectId: string;
  projectName: string;
  roleKey: string;
  sources: RoleReason[];
  /** The direct grant id, when one of the sources is direct. */
  grantId?: string;
  /** Whose access this is. Defaults to the person whose page we're on. */
  userId?: string;
  userName?: string;
}

export function RemovalDialog({
  removal,
  onClose,
  userId,
  userName,
}: {
  removal: Removal | null;
  onClose: () => void;
  userId?: string;
  userName?: string;
}) {
  const [chosen, setChosen] = useState<SourceKind | null>(null);

  if (!removal) return null;

  const subject = removal.userName ?? userName ?? "this person";
  const person = removal.userId ?? userId;
  const roleLabel = `${removal.projectName} / ${removal.roleKey}`;

  // With more than one source, the menu lists ONE entry per source, each named
  // after its own removal — never a single control that guesses.
  if (removal.sources.length > 1 && !chosen) {
    return (
      <Modal open onClose={onClose} size="sm">
        <ModalHeader
          title={`${subject} holds ${roleLabel} ${removal.sources.length} ways.`}
          lede="Each source is removed on its own terms. Pick the one you mean."
        />
        <div className="flex flex-col gap-2 px-6">
          {removal.sources.map((source) => (
            <button
              key={source.kind}
              type="button"
              onClick={() => setChosen(source.kind as SourceKind)}
              className="flex items-center gap-3 rounded-inner border border-line-strong px-4 py-3 text-left motion-tint hover:bg-[var(--hover)]"
            >
              <SourceChip kind={source.kind as SourceKind} />
              <span className="flex-1 text-[14.5px]">{actionName(source.kind as SourceKind)}</span>
              {sourceQualifier(source) && (
                <span className="text-[13px] text-faint">{sourceQualifier(source)}</span>
              )}
            </button>
          ))}
        </div>
        <ModalFooter>
          <Button onClick={onClose}>Cancel</Button>
        </ModalFooter>
      </Modal>
    );
  }

  const kind = (chosen ?? (removal.sources[0]?.kind as SourceKind)) ?? "direct";
  const source = removal.sources.find((entry) => entry.kind === kind) ?? removal.sources[0];
  const others = removal.sources.filter((entry) => entry.kind !== kind);

  const close = () => {
    setChosen(null);
    onClose();
  };

  if (kind === "mapping") {
    return <AutomaticDialog removal={removal} source={source} subject={subject} onClose={close} />;
  }
  if (kind === "bundle") {
    return (
      <BundleDialog
        removal={removal}
        source={source}
        subject={subject}
        userId={person}
        others={others}
        onClose={close}
      />
    );
  }
  return (
    <DirectDialog
      removal={removal}
      subject={subject}
      userId={person}
      others={others}
      onClose={close}
    />
  );
}

function actionName(kind: SourceKind): string {
  if (kind === "direct") return "Revoke direct access";
  if (kind === "bundle") return "Remove bundle assignment";
  return "Open the rule";
}

/**
 * Direct removal. The residual outcome in bold, plus the real-world
 * consequence — the sentence an operator needs before they click.
 */
function DirectDialog({
  removal,
  subject,
  userId,
  others,
  onClose,
}: {
  removal: Removal;
  subject: string;
  userId?: string;
  others: RoleReason[];
  onClose: () => void;
}) {
  const remove = useRemoveDirectGrant();
  const [outcome, setOutcome] = useState<Outcome | null>(null);
  const retained = others.length > 0;
  const roleLabel = `${removal.projectName} / ${removal.roleKey}`;

  const blocked = !removal.grantId || !userId;

  return (
    <Modal open onClose={onClose} busy={remove.isPending} size="sm" labelledBy="removal-title">
      <ModalHeader
        chip={<SourceChip kind="direct" />}
        title={`Revoke direct access to ${roleLabel}?`}
        titleId="removal-title"
        lede={`Granted directly to ${subject}.`}
      />
      <div className="px-6">
        <div className="danger-note px-4 py-3.5 text-[15px] leading-[1.5]">
          {retained ? (
            <>
              <strong className="font-semibold text-danger-text">
                {subject} will still hold this role.
              </strong>
              <br />
              <span className="text-[14px] text-muted">
                It also comes from {describeOthers(others)}, so nothing changes at the door today.
              </span>
            </>
          ) : (
            <>
              <strong className="font-semibold text-danger-text">
                {subject} will lose this role.
              </strong>
              <br />
              <span className="text-[14px] text-muted">
                No bundle and no rule gives it to them. Their access ends once the change reaches Zitadel — revocations send on their own, every few minutes.
              </span>
            </>
          )}
        </div>
      </div>
      {outcome && <ActionOutcome outcome={outcome} className="mx-6 mb-1" />}

      <ModalFooter
        note={
          blocked
            ? "Refused · this role was not given directly, so there is no direct access to revoke. Only direct access can be revoked here."
            : undefined
        }
      >
        <Button
          variant="dangerConfirm"
          disabled={blocked}
          isPending={remove.isPending}
          onClick={async () => {
            try {
              const result = await remove.mutateAsync({
                userId: userId!,
                grantId: removal.grantId!,
              });
              // The residual outcome, from the backend that computed it. A
              // role this person also holds through a bundle or a rule
              // survives the removal, and which ones those are is a closure
              // diff the server does — the UI must not hold a second opinion
              // about somebody's access.
              const retained = result?.retained_roles ?? [];
              setOutcome({
                kind: "applied",
                message: `Direct access to ${roleLabel} removed`,
                detail: retained.length
                  ? `They still hold ${retained.join(", ")}, from a bundle or a rule.`
                  : "Nothing else was supplying it, so the access is gone.",
              });
            } catch (error) {
              setOutcome(outcomeFromError(error));
            }
          }}
        >
          Revoke access
        </Button>
        <Button onClick={onClose}>Cancel</Button>
      </ModalFooter>
    </Modal>
  );
}

/**
 * Bundle removal. Two explicit lists — what they lose, and what they keep and
 * why it survives. The "why" is what stops the operator second-guessing.
 */
function BundleDialog({
  removal,
  source,
  subject,
  userId,
  others,
  onClose,
}: {
  removal: Removal;
  source: RoleReason;
  subject: string;
  userId?: string;
  others: RoleReason[];
  onClose: () => void;
}) {
  const bundleName = source.bundle_name ?? "this bundle";
  const removeBundle = useRemoveBundle(userId ?? "");
  const [outcome, setOutcome] = useState<Outcome | null>(null);

  return (
    <Modal open onClose={onClose} busy={removeBundle.isPending} size="sm" labelledBy="removal-title">
      <ModalHeader
        chip={<SourceChip kind="bundle" />}
        title={`Remove the ${bundleName} bundle from ${subject}?`}
        titleId="removal-title"
        lede={`Everything ${bundleName} carries goes with it, except what another source also gives them.`}
      />
      <div className="flex flex-col gap-2 px-6">
        <div className="text-[12.5px] font-semibold uppercase tracking-[0.1em] text-danger-text">
          They will lose
        </div>
        <div className="rounded-nav bg-danger-soft px-3.5 py-2.5 text-[14px]">
          {removal.projectName} / <Mono>{removal.roleKey}</Mono>
          {others.length === 0 && (
            <span className="text-[13px] text-muted"> — no other source gives it</span>
          )}
        </div>

        {others.length > 0 && (
          <>
            <div className="mt-2 type-label">They will keep</div>
            <div className="flex items-center gap-2.5 rounded-nav bg-accent-soft px-3.5 py-2.5 text-[14px]">
              {removal.projectName} / <Mono>{removal.roleKey}</Mono>
              <span className="text-[13px] text-muted">— still {describeOthers(others)}</span>
            </div>
          </>
        )}
      </div>
      {outcome && <ActionOutcome outcome={outcome} className="mx-6 mb-1" />}

      <ModalFooter note="Every other role this bundle carries is removed too. Manage bundles shows the full list before you commit.">
        <Button
          variant="dangerConfirm"
          isPending={removeBundle.isPending}
          disabled={!userId || !source.bundle_id}
          onClick={async () => {
            try {
              await removeBundle.mutateAsync(source.bundle_id!);
              setOutcome({
                kind: "queued",
                message: `${bundleName} removed from ${subject}`,
                detail:
                  "The roles it supplied are revoked once you send Pending changes, except any that another source still gives.",
              });
            } catch (error) {
              setOutcome(outcomeFromError(error));
            }
          }}
        >
          Remove bundle
        </Button>
        <Button onClick={onClose}>Cancel</Button>
      </ModalFooter>
    </Modal>
  );
}

/**
 * Automatic — no removal offered, because nothing here is the operator's to
 * remove. Two real ways forward instead, and no destructive colour anywhere:
 * nothing is being destroyed.
 */
function AutomaticDialog({
  removal,
  source,
  subject,
  onClose,
}: {
  removal: Removal;
  source: RoleReason;
  subject: string;
  onClose: () => void;
}) {
  const input = sourceQualifier(source);

  return (
    <Modal open onClose={onClose} size="sm" labelledBy="removal-title">
      <ModalHeader
        chip={<SourceChip kind="mapping" />}
        title="This one isn't yours to remove."
        titleId="removal-title"
        lede={`${subject} holds ${removal.projectName} / ${removal.roleKey} because an automatic rule produced it.`}
      />
      <div className="flex flex-col gap-2.5 px-6">
        <div className="rounded-inner bg-tint-1 px-4 py-3.5 text-[14.5px] leading-[1.5]">
          <span className="text-muted">{input ?? "an input role"}</span>
          &nbsp;⇒&nbsp;
          <span className="font-semibold">
            {removal.projectName} / {humanizeKey(removal.roleKey)}
          </span>
          <div className="mt-1.5 text-[13.5px] text-faint">
            Editing the rule changes access for everyone it applies to.
          </div>
        </div>
        <p className="text-[14px] leading-[1.55] text-muted">
          To revoke this from {subject} alone, remove their{" "}
          <span className="text-ink">{input ?? "input role"}</span> — that is the input the rule
          reads.
        </p>
      </div>
      <ModalFooter>
        <ButtonLink href="/policies" variant="accentSoft">
          Open the rule →
        </ButtonLink>
        <Button onClick={onClose}>Close</Button>
      </ModalFooter>
    </Modal>
  );
}

/** "via Lab Tech and automatic from 3D Lab / operator" — the residual sources. */
function describeOthers(others: RoleReason[]): string {
  if (others.length === 0) return "no other source";
  return others
    .map((source) => {
      const qualifier = sourceQualifier(source);
      if (source.kind === "bundle") return `via ${qualifier ?? "a bundle"}`;
      if (source.kind === "mapping") return `automatic${qualifier ? ` from ${qualifier}` : ""}`;
      return "granted directly";
    })
    .join(" and ");
}
