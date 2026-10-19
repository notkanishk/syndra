"use client";

import { useState } from "react";

import { RolePicker, splitRoleId } from "@/components/bundles/RolePicker";
import { type PickerPlace, type RoleNote } from "@/components/bundles/RolePickerOne";
import { nothingSentYet } from "@/components/access/AccessSource";
import { ActionOutcome } from "@/components/ui/ActionOutcome";
import { Button, PILL } from "@/components/ui/Button";
import { FieldLabel, Input } from "@/components/ui/Input";
import { Modal, ModalFooter, ModalHeader, mountedWhileOpen } from "@/components/ui/Modal";
import { waitReasonLine } from "@/lib/wait-reason";
import { useGlobalRoleCatalog } from "@/lib/queries/useRoles";
import { useProjects } from "@/lib/queries/useProjects";
import { useCreateGrant, useUserAccess, useUserGrants } from "@/lib/queries/useUsers";
import { outcomeFromError, type ActionOutcome as Outcome } from "@/lib/outcome";
import { daysUntil, formatShortDate, humanizeKey } from "@/lib/format";
import { useTerms, type Term } from "@/lib/queries/useTerms";
import { daysUntilTermEnd, nextTermEnd } from "@/lib/term";

/**
 * E5 · Grant direct access. Project → role → expiry.
 *
 * Expiry gets presets before the picker, because "end of term" is what people
 * actually mean and a date field alone makes them count weeks. The consequence
 * of the date is stated in the panel: the sweep removes this automatically,
 * and two weeks earlier it appears under Expiring access where the only action
 * is to extend.
 *
 * Operator-only. Members get 403 by design, so the affordance is never
 * rendered for them.
 */

type Preset = "30" | "term" | "date" | "never";

function GrantDirectAccessOpen({
  userId,
  userName,
  onClose,
}: {
  userId: string;
  userName: string;
  open: boolean;
  onClose: () => void;
}) {
  const projects = useProjects();
  const roles = useGlobalRoleCatalog();
  const grant = useCreateGrant(userId);
  const terms = useTerms();

  const [value, setValue] = useState<string | null>(null);
  const [projectId, roleKey] = value ? splitRoleId(value) : ["", ""];
  const [preset, setPreset] = useState<Preset>("30");
  const [customDate, setCustomDate] = useState("");
  const [reason, setReason] = useState("");
  const [result, setResult] = useState<{ word: string; outcome: Outcome } | null>(null);

  const selectedRole = (roles.data ?? []).find((role) => role.project_id === projectId && role.role_key === roleKey);
  const resolved = resolveExpiry(preset, customDate, terms.data);

  // What the person holds already, so giving it again is said before the click,
  // in the row's note seat. A direct grant's own end wins; otherwise what
  // Zitadel shows is held with no end Syndra knows of; a role given and not
  // yet sent is on its way.
  const heldGrants = useUserGrants(userId);
  const heldAccess = useUserAccess(userId);
  const noteOf = (project: string, key: string): RoleNote => {
    const g = (heldGrants.data ?? []).find((x) => x.project_id === project && x.role_key === key);
    if (g?.expires_at) return { kind: "until", until: g.expires_at };
    const entry = (heldAccess.data?.projects ?? []).find((p) => p.project_id === project);
    const queued = [...(entry?.source_roles ?? []), ...(entry?.derived_roles ?? [])].find((r) => r.role_key === key);
    if (queued && nothingSentYet(queued.reasons)) return { kind: "onway" };
    if (g || entry?.observed_role_keys?.includes(key)) return { kind: "held" };
    return { kind: "none" };
  };
  const places: PickerPlace[] = (projects.data ?? []).map((entry) => ({
    id: entry.project.id,
    name: entry.project.name,
    roles: (roles.data ?? [])
      .filter((role) => role.project_id === entry.project.id)
      .map((role) => ({
        key: role.role_key,
        name: role.display_name || humanizeKey(role.role_key),
        description: role.description || undefined,
        // Observed in Zitadel, or nothing: a count that was never read isn't zero.
        people: role.observed_user_count,
        waits: role.waits_for_send,
        note: noteOf(entry.project.id, role.role_key),
      })),
  }));
  const first = userName.trim().split(/\s+/)[0] || "They";
  const roleName = selectedRole?.display_name || humanizeKey(roleKey);

  const ready = Boolean(value) && (preset !== "date" || Boolean(customDate));
  // Given, or waiting for you to send: the sheet is its result and nothing sends twice.
  const done = result?.outcome.kind === "applied" || result?.outcome.kind === "queued";

  return (
    <Modal open onClose={onClose} busy={grant.isPending} size="md" labelledBy="grant-title">
      <ModalHeader title="Grant direct access" titleId="grant-title" />
      <div className="-mt-2 px-6 pb-3 text-[13px] text-faint">{userName}</div>

      <div className="flex flex-col gap-3.5 px-6">
        {!done && (
          <>
            <div>
              <FieldLabel id="grant-role-label">Role</FieldLabel>
              <RolePicker
                mode="one"
                audience="operator"
                places={places}
                value={value}
                onChange={setValue}
                disabled={grant.isPending}
                labelledBy="grant-role-label"
              />
            </div>

        <div>
          <FieldLabel htmlFor="grant-expiry-date">Ends</FieldLabel>
          <div className="mb-2.5 flex flex-wrap gap-2">
            {(
              [
                ["30", "30 days"],
                ["term", "End of term"],
                ["date", "Pick a date"],
                ["never", "Never"],
              ] as Array<[Preset, string]>
            ).map(([choice, label]) => (
              // The same control as the request form's "how long", and now the
              // same box: it had restated the pill by hand at 13px/7px, which
              // is both a drifted surface and a 30px target on a phone.
              <button
                key={choice}
                type="button"
                aria-pressed={preset === choice}
                onClick={() => setPreset(choice)}
                className={`rounded-pill font-semibold motion-tint ${PILL.md} ${
                  preset === choice ? "bg-accent-dense text-accent-ink" : "bg-tint-2 text-ink"
                }`}
              >
                {label}
              </button>
            ))}
          </div>

          {preset === "date" ? (
            // Native date input: the platform's picker is keyboard-accessible,
            // localised and familiar. A custom one would be none of those.
            <Input
              id="grant-expiry-date"
              type="date"
              value={customDate}
              onChange={(event) => setCustomDate(event.target.value)}
            />
          ) : (
            <div className="flex items-center gap-2.5 rounded-inner border border-line-strong px-[15px] py-3 text-[15px]">
              {resolved.date ? (
                <>
                  {formatShortDate(resolved.date)}
                  <span className="text-[14px] text-faint">
                    — in {daysUntil(resolved.date)} days
                  </span>
                </>
              ) : (
                <span className="text-muted">No end date — this access does not end on its own</span>
              )}
            </div>
          )}
        </div>

        <div>
          <FieldLabel htmlFor="grant-reason">Reason</FieldLabel>
          <Input
            id="grant-reason"
            value={reason}
            onChange={(event) => setReason(event.target.value)}
            placeholder="Capstone build, through end of term"
          />
        </div>
          </>
        )}
      </div>

      {!done && (
      <div className="accent-note mx-6 mt-4 px-4 py-3.5 text-[14px] leading-[1.55] text-ink/80">
        {resolved.date ? (
          <>
            It ends on {formatShortDate(resolved.date)}, and the sweep removes it automatically. They&rsquo;ll
            show up under <strong className="font-semibold text-ink">Expiring access</strong> two
            weeks before, where the only action is to extend.
          </>
        ) : (
          <>
            This access will not end on its own. It stays until someone removes it — which is
            what &ldquo;Never&rdquo; means here.
          </>
        )}
      </div>
      )}

      {/* The sheet becomes its own result rather than handing it to a corner,
          in the act's own word. It stays open on success too: what an operator
          wants next is to read what happened to the person they just acted on,
          and a dialog that closes itself takes that with it. */}
      {result && <ActionOutcome outcome={result.outcome} word={result.word} className="mx-6 mb-1" />}

      <ModalFooter
        note={
          result || !selectedRole ? undefined : selectedRole.waits_for_send ? (
            <span className="rounded-inner bg-warn-soft px-2.5 py-1.5 text-warn-text">
              {roleName} always waits for you, so {first} gets it once you send it.
            </span>
          ) : (
            `${first} can use it as soon as you give it.`
          )
        }
      >
        {done ? (
          <Button variant="outline" onClick={onClose}>
            Close
          </Button>
        ) : (
          <>
            <Button
              variant="accent"
              disabled={!ready}
              // The date case is the invisible one: pick "until a date", leave
              // the field empty, and the only acting button dies with nothing
              // said.
              reason={
                !ready
                  ? !value
                    ? "Choose a role to give."
                    : "Pick the date the access should end."
                  : undefined
              }
              isPending={grant.isPending}
              onClick={async () => {
                setResult(null);
                try {
                  const made = await grant.mutateAsync({
                    project_id: projectId,
                    role_key: roleKey,
                    reason,
                    duration_days: resolved.days,
                  });
                  // `wait_reason` is the backend's account of why this isn't
                  // in Zitadel yet; without one the grant went straight through.
                  const why = waitReasonLine(made.wait_reason, roleName, new Date().toISOString());
                  if (made.status !== "applied" || made.wait_reason) {
                    setResult({
                      word: "Given",
                      outcome: {
                        kind: "queued",
                        message: `${roleName} is given to ${first}`,
                        detail:
                          made.wait_reason === "role_waits"
                            ? `${why}, so ${first} can't use it yet.`
                            : why
                              ? `${why}, so it goes when you send it.`
                              : "It goes when you send it.",
                      },
                    });
                  } else {
                    setResult({
                      word: "Given",
                      outcome: { kind: "applied", message: `${first} can use ${roleName} on their own now` },
                    });
                  }
                } catch (error) {
                  setResult({ word: "Not given", outcome: outcomeFromError(error) });
                }
              }}
            >
              Grant access
            </Button>
            <Button onClick={onClose}>Cancel</Button>
          </>
        )}
      </ModalFooter>
    </Modal>
  );
}

/**
 * Presets resolve to a concrete date so the panel can show it with its
 * distance. "End of term" is the end of the term holding today (or the next to
 * start), from the term setting; 18 May / 18 December only when it can't be read.
 */
function resolveExpiry(preset: Preset, custom: string, terms?: Term[]): { date: string | null; days: number } {
  const now = new Date();
  if (preset === "never") return { date: null, days: 0 };
  if (preset === "30") {
    const date = new Date(now.getTime() + 30 * 86_400_000);
    return { date: date.toISOString(), days: 30 };
  }
  if (preset === "date") {
    if (!custom) return { date: null, days: 0 };
    const days = daysUntil(custom) ?? 0;
    return { date: new Date(custom).toISOString(), days: Math.max(1, days) };
  }
  const endOfTerm = nextTermEnd(now, terms);
  return { date: endOfTerm.toISOString(), days: daysUntilTermEnd(now, terms) };
}

export const GrantDirectAccess = mountedWhileOpen(GrantDirectAccessOpen);
