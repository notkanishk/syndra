"use client";

import { useState } from "react";

import { ActionOutcome } from "@/components/ui/ActionOutcome";
import { Button } from "@/components/ui/Button";
import { Card, CardHeader } from "@/components/ui/Card";
import { FieldHint, FieldLabel, Input } from "@/components/ui/Input";
import { Modal, ModalFooter, ModalHeader } from "@/components/ui/Modal";
import { Relative } from "@/components/ui/Time";
import { outcomeFromError, type ActionOutcome as Outcome } from "@/lib/outcome";
import {
  useClearShadowCredential,
  useSetShadowCredential,
  useShadowCredentialAudit,
  useShadowCredentialStatus,
} from "@/lib/queries/useShadowCredential";

/**
 * Member · a workshop password for the machines that can't ask you to log in.
 *
 * Two things are said out loud here and neither is optional.
 *
 * The first is what this password is NOT. A member already signs in with their institutional
 * account, and being shown a second password field with no explanation invites exactly one
 * reading — that their login has changed. It has not, and the lede says so before anything else.
 *
 * The second is that nothing reads it yet. The door and machine bridge is unbuilt (see
 * System › Hardware sync, which says the same thing in the operator's register), so a password
 * set today is stored and used by nothing until that lands. Leaving that out would be worse than
 * leaving out the whole card: somebody would set one, try a door, and conclude the product is
 * broken. Saying it costs one sentence and means an early adopter is simply ready.
 *
 * The card never claims to know the password. It cannot: the plaintext is discarded after
 * hashing, and the hash is served only to the sync service on a route the browser cannot reach.
 * "Set" and "when you last changed it" is the whole of what the owner can be told, and that is
 * the correct amount.
 */
export function ShadowCredential({ userId }: { userId: string }) {
  const status = useShadowCredentialStatus(userId);
  const [editing, setEditing] = useState(false);
  const [clearing, setClearing] = useState(false);

  // Nothing while the status is in flight — a card that flashes "Not set" and corrects itself is
  // worse than a card that arrives a moment late.
  if (status.isLoading) return null;

  // A failed read USED to render nothing here, and that silence hid a real fault: the console
  // proxy did not permit these routes for members, so the card simply vanished for every member
  // and looked like a design decision. Say it instead. A member cannot act on the message, but
  // somebody they report it to can, and an absent card reports nothing to anybody.
  if (status.error) {
    return (
      <Card>
        <CardHeader title="Workshop password" note="Unavailable" tone="warn" />
        <p className="px-5 py-4 text-[14px] leading-[1.55] text-muted">
          This section couldn&rsquo;t load. Your access is unaffected — the list above is what
          matters, and it is complete. Mention it to a lab manager if it stays this way.
        </p>
      </Card>
    );
  }

  const has = status.data?.has_credential ?? false;
  const changedAt = status.data?.rotated_at ?? status.data?.created_at ?? null;

  return (
    <Card>
      <CardHeader
        title="Workshop password"
        note={has ? "Set" : "Not set"}
        tone={has ? "accent" : "warn"}
      />

      <div className="flex flex-wrap items-center gap-4 px-5 py-4">
        <div className="min-w-[320px] flex-1">
          <p className="max-w-[64ch] text-[14.5px] leading-[1.6] text-muted">
            Some machines and door controllers can&rsquo;t send you to a sign-in page. This is the
            password they&rsquo;ll ask for. It is <strong className="text-ink">not</strong> your
            university login, and changing it here changes nothing about how you sign in.
          </p>
          <p className="mt-2 max-w-[64ch] text-[13.5px] leading-[1.55] text-faint">
            {has ? (
              <>
                Set{changedAt ? " " : ""}
                {changedAt ? <Relative iso={changedAt} /> : null}. Nobody can read it back —
                not a lab manager, and not this page. If you&rsquo;ve forgotten it, set a new one.
              </>
            ) : (
              "You haven't set one. Nothing needs it from you right now."
            )}
          </p>
          <p className="mt-2 max-w-[64ch] text-[13.5px] leading-[1.55] text-faint">
            No hardware is connected to the makerspace yet, so a password set today is stored and
            waiting rather than in use.
          </p>
        </div>

        <div className="flex shrink-0 gap-2">
          <Button variant={has ? "outline" : "accent"} onClick={() => setEditing(true)}>
            {has ? "Change it" : "Set a password"}
          </Button>
          {has && (
            <Button variant="danger" onClick={() => setClearing(true)}>
              Remove
            </Button>
          )}
        </div>
      </div>

      {editing && (
        <SetPasswordDialog
          userId={userId}
          rotating={has}
          onClose={() => setEditing(false)}
        />
      )}
      {clearing && <ClearPasswordDialog userId={userId} onClose={() => setClearing(false)} />}
    </Card>
  );
}

function SetPasswordDialog({
  userId,
  rotating,
  onClose,
}: {
  userId: string;
  rotating: boolean;
  onClose: () => void;
}) {
  const save = useSetShadowCredential(userId);
  const [outcome, setOutcome] = useState<Outcome | null>(null);
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  /**
   * The server's own words, verbatim. It composes the failing requirements into one sentence
   * ("must be at least 12 characters; must contain at least one symbol"), and re-implementing
   * those rules here to pre-empt it would create a second, quietly divergent opinion about what
   * counts as strong enough. One authority, and it is the one that decides.
   */
  const [rejection, setRejection] = useState<string | null>(null);

  const mismatch = confirm.length > 0 && password !== confirm;
  const ready = password.length > 0 && password === confirm;

  return (
    <Modal open onClose={onClose} busy={save.isPending} size="sm" labelledBy="shadow-password">
      <ModalHeader
        title={rotating ? "Change your workshop password" : "Set a workshop password"}
        titleId="shadow-password"
        lede="For machines that can't send you to a sign-in page. Not your university login."
      />

      <div className="flex flex-col gap-3.5 px-6">
        <div>
          <FieldLabel htmlFor="shadow-new">New password</FieldLabel>
          <Input
            id="shadow-new"
            type="password"
            autoComplete="new-password"
            value={password}
            onChange={(event) => {
              setRejection(null);
              setPassword(event.target.value);
            }}
          />
          <FieldHint>
            At least 12 characters, with an uppercase letter, a lowercase letter, a digit and a
            symbol.
          </FieldHint>
        </div>

        <div>
          <FieldLabel htmlFor="shadow-confirm">Type it again</FieldLabel>
          <Input
            id="shadow-confirm"
            type="password"
            autoComplete="new-password"
            value={confirm}
            onChange={(event) => setConfirm(event.target.value)}
          />
          {/* Nothing can read the password back to check it against, so a typo would only
              surface at a machine that will not open. */}
          {mismatch && <FieldHint>Those two don&rsquo;t match yet.</FieldHint>}
        </div>

        {rejection && (
          <div className="danger-note px-4 py-3.5 text-[14px] leading-[1.55]">
            <div className="type-label mb-1 text-danger-text">Not accepted</div>
            <p className="text-muted">{rejection}</p>
          </div>
        )}
      </div>

      {outcome && <ActionOutcome outcome={outcome} className="mx-6 mb-1" />}

      <ModalFooter note="Nobody, including a lab manager, can read this back to you afterwards.">
        <Button
          variant="accent"
          disabled={!ready}
          isPending={save.isPending}
          reason={!ready ? "Type the same password in both fields." : undefined}
          onClick={async () => {
            try {
              await save.mutateAsync(password);
              // When it will work, not that it works. The credential reaches
              // the machine through the same queue everything else does, and a
              // member told "set" who then cannot log in has been lied to by
              // one word.
              setOutcome({
                kind: "queued",
                message: rotating ? "Workshop password changed" : "Workshop password set",
                detail: "It will work on the workshop machines within a few minutes.",
              });
            } catch (error) {
              setRejection(
                error instanceof Error ? error.message : "That password wasn't accepted.",
              );
            }
          }}
        >
          {rotating ? "Change it" : "Set it"}
        </Button>
        <Button disabled={save.isPending} onClick={onClose}>
          Cancel
        </Button>
      </ModalFooter>
    </Modal>
  );
}

function ClearPasswordDialog({ userId, onClose }: { userId: string; onClose: () => void }) {
  const clear = useClearShadowCredential(userId);
  const [outcome, setOutcome] = useState<Outcome | null>(null);
  const audit = useShadowCredentialAudit(userId);
  const lastSet = (audit.data ?? []).find(
    (entry) => entry.action === "set" || entry.action === "rotated",
  );

  return (
    <Modal open onClose={onClose} busy={clear.isPending} size="sm" labelledBy="shadow-clear">
      <ModalHeader
        title="Remove your workshop password?"
        titleId="shadow-clear"
        lede={
          lastSet ? (
            <>
              You last set it <Relative iso={lastSet.created_at} />.
            </>
          ) : undefined
        }
      />
      <div className="px-6">
        <div className="danger-note px-4 py-3.5 text-[14px] leading-[1.55] text-muted">
          Machines that ask for it will stop letting you in. Your access itself is unchanged —
          this is only the password those machines check. You can set a new one at any time.
        </div>
      </div>
      {outcome && <ActionOutcome outcome={outcome} className="mx-6 mb-1" />}

      <ModalFooter>
        <Button
          variant="dangerConfirm"
          isPending={clear.isPending}
          onClick={async () => {
            try {
              await clear.mutateAsync();
              setOutcome({
                kind: "queued",
                message: "Workshop password removed",
                detail: "The machines stop accepting it within a few minutes.",
              });
            } catch (error) {
              setOutcome(outcomeFromError(error));
            }
          }}
        >
          Remove it
        </Button>
        <Button disabled={clear.isPending} onClick={onClose}>
          Keep it
        </Button>
      </ModalFooter>
    </Modal>
  );
}
