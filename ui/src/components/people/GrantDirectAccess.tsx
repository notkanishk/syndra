"use client";

import { useMemo, useState } from "react";
import { toast } from "sonner";

import { Button } from "@/components/ui/Button";
import { FieldHint, FieldLabel, Input } from "@/components/ui/Input";
import { Modal, ModalFooter, ModalHeader } from "@/components/ui/Modal";
import { Select } from "@/components/ui/Select";
import { useGlobalRoleCatalog } from "@/lib/queries/useRoles";
import { useProjects } from "@/lib/queries/useProjects";
import { useCreateGrant } from "@/lib/queries/useUsers";
import { daysUntil, formatLongDate, humanizeKey, roleLabel } from "@/lib/format";
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

export function GrantDirectAccess({
  userId,
  userName,
  open,
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
  // Named as a pair, because that is what was written. The dialog holds the
  // project in a select the operator is about to close; the toast outlives it.
  const selectedLabel = selectedRole
    ? roleLabel(selectedRole.project_name, selectedRole.role_key, selectedRole.display_name)
    : roleKey;

  const resolved = resolveExpiry(preset, customDate);

  if (!open) return null;

  const ready = Boolean(projectId && roleKey) && (preset !== "date" || Boolean(customDate));

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
            {projectRoles.map((role) => (
              <option key={role.role_key} value={role.role_key}>
                {role.display_name || humanizeKey(role.role_key)} · {role.role_key}
              </option>
            ))}
          </Select>
          {selectedRole && (
            // Plain language, not a spec: what this role unlocks and how many
            // people already carry it.
            <FieldHint>
              {selectedRole.assigned_user_count}{" "}
              {selectedRole.assigned_user_count === 1 ? "person holds" : "people hold"} it
              {selectedRole.description ? ` · ${selectedRole.description}` : ""}
            </FieldHint>
          )}
        </div>

        <div>
          <FieldLabel>Expires</FieldLabel>
          <div className="mb-2.5 flex flex-wrap gap-2">
            {(
              [
                ["30", "30 days"],
                ["term", "End of term"],
                ["date", "Pick a date"],
                ["never", "Never"],
              ] as Array<[Preset, string]>
            ).map(([value, label]) => (
              <button
                key={value}
                type="button"
                aria-pressed={preset === value}
                onClick={() => setPreset(value)}
                className={`rounded-pill px-3.5 py-[7px] text-[13px] font-semibold transition-colors ${
                  preset === value ? "bg-accent text-accent-ink" : "bg-tint-2 text-ink"
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

      <div className="accent-note mx-6 mt-4 px-4 py-3.5 text-[14px] leading-[1.55] text-ink/[.78]">
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

      <ModalFooter>
        <Button
          variant="accent"
          disabled={!ready}
          isPending={grant.isPending}
          onClick={async () => {
            try {
              await grant.mutateAsync({
                project_id: projectId,
                role_key: roleKey,
                reason,
                duration_days: resolved.days,
              });
              toast.success(`${userName} now holds ${selectedLabel}.`);
              onClose();
            } catch (error) {
              toast.error(error instanceof Error ? error.message : "The grant didn't go through.");
            }
          }}
        >
          Grant access
        </Button>
        <Button onClick={onClose}>Cancel</Button>
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
