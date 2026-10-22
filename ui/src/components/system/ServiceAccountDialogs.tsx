"use client";

import { useId, useState } from "react";

import { UserName } from "@/components/names/UserName";
import { ActionOutcome } from "@/components/ui/ActionOutcome";
import { Button } from "@/components/ui/Button";
import { ChoiceRow } from "@/components/ui/ChoiceRow";
import { FieldHint, FieldLabel, Input } from "@/components/ui/Input";
import { Modal, ModalFooter, ModalHeader } from "@/components/ui/Modal";
import { Notice } from "@/components/ui/Notice";
import { personLine } from "@/lib/format";
import { describeFailure, sentence } from "@/lib/outcome";
import {
  SERVICE_ACCOUNT_KINDS,
  useSaveServiceAccount,
  useUnmarkServiceAccount,
} from "@/lib/queries/useServiceAccounts";
import { useUsers } from "@/lib/queries/useUsers";
import type { ServiceAccountKind } from "@/lib/types";

/**
 * Marking an account as a service account, changing what it is for or who
 * answers for it, and putting it back (A-07 §28, route-service-accounts, 23 Oct
 * 2026). One dialog for the first three, because they are one PUT and the same
 * fields; each ends on its own result (96b) rather than closing on a toast.
 */

const MAX_PURPOSE = 200;

export interface DialogAccount {
  id: string;
  name: string;
  /** Absent while it is still a person: nothing to keep. */
  kind?: ServiceAccountKind;
  purpose?: string;
  owner_id?: string;
}

interface Owner {
  id: string;
  name: string;
  title: string;
}

export function ServiceAccountDialog({
  mode,
  account,
  onClose,
}: {
  /** `mark` asks all three; the other two change one and keep the rest as they are. */
  mode: "mark" | "purpose" | "owner";
  account: DialogAccount;
  onClose: () => void;
}) {
  const save = useSaveServiceAccount(account.id);
  const [kind, setKind] = useState<ServiceAccountKind | null>(account.kind ?? null);
  const [purpose, setPurpose] = useState(account.purpose ?? "");
  const [owner, setOwner] = useState<Owner | null>(
    account.owner_id ? { id: account.owner_id, name: "", title: "" } : null,
  );
  const [failure, setFailure] = useState<string | null>(null);
  const [done, setDone] = useState(false);
  const ids = useId();

  const marking = mode === "mark";
  const title = marking ? `Mark ${account.name} as a service account` : mode === "purpose" ? "Change purpose" : "Change owner";
  const lede = marking
    ? "An account made for one purpose, not one person. It keeps everything it holds."
    : mode === "purpose"
      ? `What ${account.name} is for, in a few words.`
      : `The person who answers for what ${account.name} holds.`;
  const tooLong = purpose.trim().length > MAX_PURPOSE;
  const reason = !kind ? "Choose what it is first." : tooLong ? `Purpose is ${MAX_PURPOSE} characters at most.` : undefined;

  async function send() {
    if (!kind || tooLong) return;
    setFailure(null);
    try {
      await save.mutateAsync({ kind, purpose: purpose.trim(), owner_id: owner?.id ?? "" });
      setDone(true);
    } catch (e) {
      setFailure(sentence(describeFailure(e)));
    }
  }

  const who = owner?.name || (owner ? "The owner" : "");
  const outcome = marking
    ? {
        word: "Marked",
        message: owner
          ? `${account.name} is a service account now, and ${who} answers for it.`
          : `${account.name} is a service account now.`,
        detail: "It's in Service accounts. If this was wrong, Mark as a person on its page puts it back.",
      }
    : mode === "purpose"
      ? { word: "Saved", message: `The purpose now shows under ${account.name} everywhere.`, detail: undefined }
      : {
          word: "Saved",
          message: owner ? `${who} answers for ${account.name}.` : `${account.name} has no owner now.`,
          detail: owner ? undefined : "It shows on Home until someone answers for it.",
        };

  return (
    <Modal open onClose={onClose} busy={save.isPending} size="md" labelledBy={`${ids}-title`}>
      <ModalHeader title={title} titleId={`${ids}-title`} lede={lede} />
      <div className="flex flex-col gap-4 px-6">
        {done ? (
          <ActionOutcome word={outcome.word} outcome={{ kind: "applied", message: outcome.message, detail: outcome.detail }} />
        ) : (
          <>
            {marking && (
              <div>
                <FieldLabel id={`${ids}-kind`}>What it is</FieldLabel>
                <div role="radiogroup" aria-labelledby={`${ids}-kind`} className="flex flex-col gap-1">
                  {SERVICE_ACCOUNT_KINDS.map((k) => (
                    <ChoiceRow
                      key={k.value}
                      single
                      name={`${ids}-kind`}
                      selected={kind === k.value}
                      onToggle={() => setKind(k.value)}
                      title={k.label}
                      description={k.description}
                    />
                  ))}
                </div>
              </div>
            )}
            {mode !== "owner" && (
              <div>
                <FieldLabel htmlFor={`${ids}-purpose`}>Purpose</FieldLabel>
                <Input
                  id={`${ids}-purpose`}
                  value={purpose}
                  aria-invalid={tooLong}
                  onChange={(e) => {
                    setPurpose(e.target.value);
                    setFailure(null);
                  }}
                />
                <FieldHint>What it&apos;s for, in a few words. It shows under its name everywhere.</FieldHint>
              </div>
            )}
            {mode !== "purpose" && (
              <div>
                <FieldLabel htmlFor={`${ids}-owner`}>Owner</FieldLabel>
                <OwnerPicker id={`${ids}-owner`} value={owner} exclude={account.id} onChange={setOwner} />
                <FieldHint>The person who answers for what it holds.</FieldHint>
              </div>
            )}
            {failure && (
              <Notice tone="danger" title="That didn't save.">
                {failure} Nothing was changed.
              </Notice>
            )}
          </>
        )}
      </div>
      <ModalFooter
        note={done ? undefined : marking ? "This finishes now. It leaves People, the people counts and the welcome bundle; nothing about its access changes." : undefined}
      >
        {done ? (
          <Button onClick={onClose}>Close</Button>
        ) : (
          <>
            <Button disabled={save.isPending} onClick={onClose}>
              Cancel
            </Button>
            <Button variant="accent" isPending={save.isPending} disabled={Boolean(reason)} reason={reason} onClick={send}>
              {marking ? "Mark as a service account" : "Save"}
            </Button>
          </>
        )}
      </ModalFooter>
    </Modal>
  );
}

/**
 * Choosing a person: search, pick, and the pick becomes a chip with a cross.
 * Another service account is never offered, nor the account itself — the
 * backend refuses both, and a choice that can only fail is not a choice.
 */
function OwnerPicker({
  id,
  value,
  exclude,
  onChange,
}: {
  id: string;
  value: Owner | null;
  exclude: string;
  onChange: (owner: Owner | null) => void;
}) {
  const [q, setQ] = useState("");
  const people = useUsers(q.trim());
  const matches = (people.data ?? [])
    .filter(({ user }) => user.id !== exclude && !user.service_account && user.kind !== "machine")
    .slice(0, 6);

  if (value) {
    return (
      <span className="inline-flex max-w-full items-center gap-2 rounded-pill border border-line-strong py-1 pl-3.5 pr-1">
        <span className="min-w-0 text-[14.5px] [overflow-wrap:anywhere]">
          <span className="font-semibold">{value.name || <UserName id={value.id} />}</span>
          {value.title && (
            <>
              <span aria-hidden className="text-faint"> · </span>
              <span className="text-faint">{value.title}</span>
            </>
          )}
        </span>
        <Button variant="ghost" size="sm" aria-label="Clear owner" onClick={() => onChange(null)}>
          <span aria-hidden>×</span>
        </Button>
      </span>
    );
  }

  return (
    <div className="flex flex-col gap-1.5">
      <Input
        id={id}
        value={q}
        placeholder="Search by name or email"
        aria-label="Search people by name or email"
        onChange={(e) => setQ(e.target.value)}
      />
      {q.trim() && matches.length > 0 && (
        <div role="radiogroup" aria-label={`People matching ${q.trim()}`} className="flex flex-col gap-0.5">
          {matches.map(({ user }) => (
            <ChoiceRow
              key={user.id}
              single
              name={`${id}-people`}
              selected={false}
              onToggle={() => onChange({ id: user.id, name: user.name || user.email, title: personLine(user) })}
              title={user.name || user.email}
              description={personLine(user) || user.email}
            />
          ))}
        </div>
      )}
      {q.trim() && !people.isLoading && matches.length === 0 && <FieldHint>Nobody matches {q.trim()}.</FieldHint>}
    </div>
  );
}

/** Mark as a person: removes the mark of a human-type account. A machine account is Zitadel's to say, and the backend refuses it. */
export function UnmarkDialog({ account, onClose }: { account: { id: string; name: string }; onClose: () => void }) {
  const unmark = useUnmarkServiceAccount(account.id);
  const [failure, setFailure] = useState<string | null>(null);
  const [done, setDone] = useState(false);
  const titleId = useId();

  async function send() {
    setFailure(null);
    try {
      await unmark.mutateAsync();
      setDone(true);
    } catch (e) {
      setFailure(sentence(describeFailure(e)));
    }
  }

  return (
    <Modal open onClose={onClose} busy={unmark.isPending} size="md" labelledBy={titleId}>
      <ModalHeader
        title={`Mark ${account.name} as a person`}
        titleId={titleId}
        lede="It comes back to People, the people counts and the welcome bundle. Nothing about its access changes."
      />
      <div className="flex flex-col gap-4 px-6">
        {done ? (
          <ActionOutcome
            word="Done"
            outcome={{
              kind: "applied",
              message: `${account.name} is a person again.`,
              detail: "It's back in People, and out of Service accounts.",
            }}
          />
        ) : (
          failure && (
            <Notice tone="danger" title="That didn't save.">
              {failure} Nothing was changed.
            </Notice>
          )
        )}
      </div>
      <ModalFooter note={done ? undefined : "This finishes now."}>
        {done ? (
          <Button onClick={onClose}>Close</Button>
        ) : (
          <>
            <Button disabled={unmark.isPending} onClick={onClose}>
              Cancel
            </Button>
            <Button variant="accent" isPending={unmark.isPending} onClick={send}>
              Mark as a person
            </Button>
          </>
        )}
      </ModalFooter>
    </Modal>
  );
}
