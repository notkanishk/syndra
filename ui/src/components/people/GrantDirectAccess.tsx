"use client";

import { useMemo, useState } from "react";

import { ActionOutcome } from "@/components/ui/ActionOutcome";
import { Button, PILL } from "@/components/ui/Button";
import { FieldHint, FieldLabel, Input } from "@/components/ui/Input";
import { Modal, ModalFooter, ModalHeader, mountedWhileOpen } from "@/components/ui/Modal";
import { Select } from "@/components/ui/Select";
import { WaitsMark } from "@/components/ui/WaitsMark";
import { waitReasonLine } from "@/lib/wait-reason";
import { holdersLine } from "@/lib/holders";
import { useGlobalRoleCatalog } from "@/lib/queries/useRoles";
import { useProjects } from "@/lib/queries/useProjects";
import { useCreateGrant, useUserAccess, useUserGrants } from "@/lib/queries/useUsers";
import { outcomeFromError, type ActionOutcome as Outcome } from "@/lib/outcome";
import { daysUntil, formatLongDate, formatShortDate, humanizeKey } from "@/lib/format";
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

/**
 * How many people hold the role, from what Zitadel shows — never from
 * Syndra's own count, which hides everyone Syndra did not give it to.
 */
function holdersHint(role: {
  assigned_user_count: number;
  confirmed_user_count?: number;
  observed_user_count?: number;
}): string {
  const line = holdersLine(role.assigned_user_count, role.confirmed_user_count, role.observed_user_count);
  const n = Number(line.headline);
  const who = `${line.headline} ${n === 1 ? "person holds" : "people hold"} it`;
  return line.note && line.note !== "confirmed" ? `${who} (${line.note})` : who;
}

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
  const [outcome, setOutcome] = useState<Outcome | null>(null);

  const [projectId, setProjectId] = useState("");
  const [roleKey, setRoleKey] = useState("");
  const [preset, setPreset] = useState<Preset>("30");
  const [customDate, setCustomDate] = useState("");
  const [reason, setReason] = useState("");

  const projectRoles = useMemo(
    () => (roles.data ?? []).filter((role) => role.project_id === projectId),
    [roles.data, projectId],
  );
  const selectedRole = projectRoles.find((role) => role.role_key === roleKey);
  const resolved = resolveExpiry(preset, customDate);

  // What the person holds already, so giving it again is said before the click.
  // A direct grant's own end wins; otherwise what Zitadel shows is held with
  // no end Syndra knows of.
  const heldGrants = useUserGrants(userId);
  const heldAccess = useUserAccess(userId);
  // What they hold of a role in this project: its end, null for none, or
  // undefined when they don't hold it. The picker marks each role with it, the
  // same way Ask for access marks the asker's.
  const holdingOf = (key: string): string | null | undefined => {
    const g = (heldGrants.data ?? []).find((x) => x.project_id === projectId && x.role_key === key);
    if (g) return g.expires_at ?? null;
    const seen = (heldAccess.data?.projects ?? []).find((p) => p.project_id === projectId)?.observed_role_keys?.includes(key);
    return seen ? null : undefined;
  };
  const heldGrant = (heldGrants.data ?? []).find((g) => g.project_id === projectId && g.role_key === roleKey);
  const alreadyHeld = roleKey !== "" && holdingOf(roleKey) !== undefined;
  const heldUntil = heldGrant?.expires_at ?? null;
  const heldNoEnd = alreadyHeld && !heldUntil;
  const who = userName || "They";
  const first = userName.trim().split(/\s+/)[0] || "They";
  const roleName = selectedRole?.display_name || humanizeKey(roleKey);


  const ready = Boolean(projectId && roleKey) && (preset !== "date" || Boolean(customDate)) && !alreadyHeld;

  return (
    <Modal open onClose={onClose} busy={grant.isPending} size="md" labelledBy="grant-title">
      <ModalHeader title="Grant direct access" titleId="grant-title" />
      <div className="-mt-2 px-6 pb-3 text-[13px] text-faint">{userName}</div>

      <div className="flex flex-col gap-3.5 px-6">
        <div>
          <FieldLabel htmlFor="grant-project">Project</FieldLabel>
          <Select
            id="grant-project"
            value={projectId}
            onChange={(event) => {
              setProjectId(event.target.value);
              setRoleKey("");
            }}
          >
            <option value="">Choose a project…</option>
            {(projects.data ?? []).map((entry) => (
              <option key={entry.project.id} value={entry.project.id}>
                {entry.project.name}
              </option>
            ))}
          </Select>
        </div>

        <div>
          <FieldLabel htmlFor="grant-role">Role</FieldLabel>
          <Select
            id="grant-role"
            emphasis={Boolean(roleKey)}
            value={roleKey}
            disabled={!projectId}
            onChange={(event) => setRoleKey(event.target.value)}
          >
            <option value="">{projectId ? "Choose a role…" : "Pick a project first"}</option>
            {projectRoles.map((role) => {
              const held = holdingOf(role.role_key);
              return (
                <option key={role.role_key} value={role.role_key} disabled={held !== undefined}>
                  {role.display_name || humanizeKey(role.role_key)} · {role.role_key}
                  {role.waits_for_send ? " · waits for you" : ""}
                  {held === undefined ? "" : held === null ? " · has this" : ` · has this until ${formatShortDate(held)}`}
                </option>
              );
            })}
          </Select>
          {selectedRole?.waits_for_send && (
            // An <option> can't hold the mark, so the chosen role carries it
            // beside the select, where the operator is looking.
            <WaitsMark withLabel className="mt-1.5" />
          )}
          {selectedRole && (
            // Plain language, not a spec: what this role unlocks and how many
            // people already carry it.
            <FieldHint>
              {holdersHint(selectedRole)}
              {selectedRole.description ? ` · ${selectedRole.description}` : ""}
            </FieldHint>
          )}
          {alreadyHeld && (
            <FieldHint>
              {heldNoEnd
                ? `${who} already has this, with no end.`
                : `${who} has this until ${formatShortDate(heldUntil)}. To keep it longer, extend it from Expiring access.`}
            </FieldHint>
          )}
        </div>

        <div>
          <FieldLabel htmlFor="grant-expiry-date">Expires</FieldLabel>
          <div className="mb-2.5 flex flex-wrap gap-2">
            {(
              [
                ["30", "30 days"],
                ["term", "End of term"],
                ["date", "Pick a date"],
                ["never", "Never"],
              ] as Array<[Preset, string]>
            ).map(([value, label]) => (
              // The same control as the request form's "how long", and now the
              // same box: it had restated the pill by hand at 13px/7px, which
              // is both a drifted surface and a 30px target on a phone.
              <button
                key={value}
                type="button"
                aria-pressed={preset === value}
                onClick={() => setPreset(value)}
                className={`rounded-pill font-semibold motion-tint ${PILL.md} ${
                  preset === value ? "bg-accent-dense text-accent-ink" : "bg-tint-2 text-ink"
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
                  {formatLongDate(resolved.date)}
                  <span className="text-[14px] text-faint">
                    — in {daysUntil(resolved.date)} days
                  </span>
                </>
              ) : (
                <span className="text-muted">No expiry — this access does not lapse</span>
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
      </div>

      <div className="accent-note mx-6 mt-4 px-4 py-3.5 text-[14px] leading-[1.55] text-ink/80">
        {resolved.date ? (
          <>
            On {formatLongDate(resolved.date)} the sweep removes this automatically. They&rsquo;ll
            show up under <strong className="font-semibold text-ink">Expiring access</strong> two
            weeks before, where the only action is to extend.
          </>
        ) : (
          <>
            This access will not lapse on its own. It stays until someone removes it — which is
            what &ldquo;Never&rdquo; means here.
          </>
        )}
      </div>

      {/* The sheet becomes its own result rather than handing it to a corner.
          It stays open on success too: what an operator wants next is to read
          what happened to the person they just acted on, and a dialog that
          closes itself takes that with it. */}
      {outcome && <ActionOutcome outcome={outcome} className="mx-6 mb-1" />}

      <ModalFooter
        note={
          outcome || !selectedRole ? undefined : selectedRole.waits_for_send ? (
            <span className="rounded-inner bg-warn-soft px-2.5 py-1.5 text-warn-text">
              {roleName} always waits for you, so {first} gets it once you send it.
            </span>
          ) : (
            `${first} can use it as soon as you give it.`
          )
        }
      >
        {outcome?.kind === "applied" || outcome?.kind === "queued" ? (
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
                  ? !projectId || !roleKey
                    ? "Choose a project and a role."
                    : alreadyHeld
                      ? "They already have this."
                      : "Pick the date the access should end."
                  : undefined
              }
              isPending={grant.isPending}
              onClick={async () => {
                setOutcome(null);
                try {
                  const result = await grant.mutateAsync({
                    project_id: projectId,
                    role_key: roleKey,
                    reason,
                    duration_days: resolved.days,
                  });
                  // `wait_reason` is the backend's account of why this isn't
                  // in Zitadel yet; without one the grant went straight through.
                  const why = waitReasonLine(result.wait_reason, roleName, new Date().toISOString());
                  if (result.status !== "applied" || result.wait_reason) {
                    setOutcome({
                      kind: "queued",
                      message: "Given in Syndra",
                      detail:
                        result.wait_reason === "role_waits"
                          ? `${why}, so ${first} can't use it yet.`
                          : why
                            ? `${why}, so it goes when you send it.`
                            : "It goes when you send it.",
                    });
                  } else {
                    setOutcome({ kind: "applied", message: `${first} can use ${roleName} now` });
                  }
                } catch (error) {
                  setOutcome(outcomeFromError(error));
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
 * distance. "End of term" is 18 December or 18 May, whichever comes next —
 * the makerspace's own calendar, not a generic 90 days.
 */
function resolveExpiry(preset: Preset, custom: string): { date: string | null; days: number } {
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
  const endOfTerm = nextTermEnd(now);
  return { date: endOfTerm.toISOString(), days: daysUntilTermEnd(now) };
}

export const GrantDirectAccess = mountedWhileOpen(GrantDirectAccessOpen);
