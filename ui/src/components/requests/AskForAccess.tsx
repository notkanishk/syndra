"use client";

import { useMemo, useState } from "react";

import { RolePicker, splitRoleId } from "@/components/bundles/RolePicker";
import { roleId, type PickerPlace } from "@/components/bundles/RolePickerOne";
import { useAskState, type AskState } from "@/components/requests/Holding";
import { ActionOutcome } from "@/components/ui/ActionOutcome";
import { StatusDot, type StatusTone } from "@/components/ui/Badge";
import { Button, PILL } from "@/components/ui/Button";
import { FieldHint, FieldLabel, Input } from "@/components/ui/Input";
import { Modal, ModalFooter, ModalHeader, mountedWhileOpen } from "@/components/ui/Modal";
import { Segmented } from "@/components/ui/Select";
import { ApiError } from "@/lib/api-client";
import { formatShortDate, humanizeKey, lengthWords } from "@/lib/format";
import { outcomeFromError, type ActionOutcome as ActionResult } from "@/lib/outcome";
import { useCatalogProjects } from "@/lib/queries/useCatalogUsers";
import { useCreateRequest } from "@/lib/queries/useRequests";
import { useTerms } from "@/lib/queries/useTerms";
import { daysUntilTermEnd, endOfDay, keepOptions, nextTermEnd } from "@/lib/term";

/** What a member asks for, in the units a member thinks in. */
type HowLong = "7" | "30" | "term";
type KeepHow = "month" | "term";

const ASKED_DETAIL =
  "Makerspace staff decide. The answer appears on this page, and nothing else is needed from you.";

/**
 * Ask for access (A-04 §1, §10; A-06 §3). One role, chosen in the role picker;
 * the sheet then becomes its own result with Close. A role held until a date is
 * asked for as "keep it": how much longer, and the same Why.
 *
 * `selfDecides` is an operator asking for themselves from My access: it says,
 * before sending, that they will decide it.
 */
function AskForAccessOpen({
  onClose,
  initial,
  userId,
  selfDecides = false,
}: {
  open: boolean;
  onClose: () => void;
  /** The place, and the role, a link named. */
  initial?: { projectId?: string; roleKey?: string };
  /** Who is asking: what they have and have asked for is read for them. */
  userId: string;
  selfDecides?: boolean;
}) {
  // `/catalog`, the one list of what exists that a member may read.
  const catalog = useCatalogProjects();
  const terms = useTerms();
  const create = useCreateRequest();
  const askState = useAskState(userId);
  const [result, setResult] = useState<{ word: string; outcome: ActionResult; final: boolean } | null>(null);

  // The role a link named, chosen only if it can be asked for. Until the person
  // touches the picker, what the link named is what they see.
  const arrived = initial?.projectId && initial?.roleKey ? roleId(initial.projectId, initial.roleKey) : null;
  const [picked, setPicked] = useState<string | null | undefined>(undefined);
  const stateOfId = (id: string): AskState => {
    const [place, role] = splitRoleId(id);
    return askState(place, role);
  };
  const arrivedState: AskState | null = arrived ? stateOfId(arrived) : null;
  const arrivedPickable = arrivedState?.kind === "none" || arrivedState?.kind === "until";
  const value = picked !== undefined ? picked : arrived && arrivedPickable ? arrived : null;
  const blockedArrival = picked === undefined && arrived && !arrivedPickable ? arrived : null;

  const [why, setWhy] = useState("");
  const [howLong, setHowLong] = useState<HowLong>("term");
  const [keepHow, setKeepHow] = useState<KeepHow>("month");

  const places: PickerPlace[] = catalog.data.map((project) => ({
    id: project.id,
    name: project.name,
    roles: project.roles.map((role) => ({
      key: role.key,
      name: role.label || humanizeKey(role.key),
      description: role.description || undefined,
      note: noteOf(askState(project.id, role.key)),
    })),
  }));

  const nameOf = (id: string | null) => {
    if (!id) return { place: "", role: "" };
    const [placeId, roleKey] = splitRoleId(id);
    const project = catalog.data.find((p) => p.id === placeId);
    return {
      place: project?.name ?? "",
      role: project?.roles.find((r) => r.key === roleKey)?.label || humanizeKey(roleKey),
    };
  };
  const chosen = value ? stateOfId(value) : ({ kind: "none" } as AskState);
  const { place: placeName, role: roleName } = nameOf(value);
  const extend = chosen.kind === "until";
  const current = chosen.kind === "until" ? chosen.until : null;

  const ordinaryDays = howLong === "term" ? daysUntilTermEnd(new Date(), terms.data) : Number(howLong);
  // "Until the end of term" is a date, not a count of days: it is sent as the
  // date so a late approval still ends with the term.
  const ordinaryUntil =
    howLong === "term"
      ? endOfDay(nextTermEnd(new Date(), terms.data)).toISOString()
      : new Date(Date.now() + ordinaryDays * 86_400_000).toISOString();

  const keep = useMemo(
    () => (current ? keepOptions(current, terms.data ?? []) : null),
    [current, terms.data],
  );
  const termOff = keep && "reason" in keep.term ? keep.term.reason : undefined;
  const keepsToTerm = keepHow === "term" && !termOff;
  const keepChoice = keep ? (keepsToTerm ? (keep.term as { days: number; until: string }) : keep.month) : null;

  const final = result?.final ?? false;

  async function send() {
    if (!value) return;
    const [projectId, roleKey] = splitRoleId(value);
    try {
      await create.mutateAsync({
        project_id: projectId,
        role_key: roleKey,
        justification: why,
        duration_days: extend && keepChoice ? keepChoice.days : ordinaryDays,
        // Days alone for a week, a month or another month; the date for a term.
        ...(extend ? (keepsToTerm && keepChoice ? { until: keepChoice.until } : {}) : howLong === "term" ? { until: ordinaryUntil } : {}),
        kind: extend ? "extend" : "access",
      });
      setResult({
        word: "Asked",
        final: true,
        outcome: {
          kind: "no_change",
          message:
            extend && keepChoice
              ? `To keep ${placeName} · ${roleName} until ${formatShortDate(keepChoice.until)}`
              : `${roleName} on ${placeName}, ${
                  howLong === "term" ? `until ${formatShortDate(ordinaryUntil)}` : `for ${lengthWords(ordinaryDays)}`
                }`,
          detail: extend
            ? `Until makerspace staff decide, it still ends ${formatShortDate(current)}. The answer appears on Requests.`
            : ASKED_DETAIL,
        },
      });
    } catch (error) {
      if (error instanceof ApiError && error.code === "ALREADY_ASKED") {
        // Nothing is wrong: it is already in hand, from somewhere else.
        setResult({
          word: "Not sent",
          final: true,
          outcome: {
            kind: "no_change",
            message: extend
              ? "You've already asked to keep this. It's waiting, and the answer appears on this page"
              : `You already asked for ${roleName} on ${placeName}, from another tab. That request is waiting`,
            detail: "Nothing was changed. Its answer appears on this page.",
          },
        });
      } else {
        setResult({ word: "Not sent", final: false, outcome: outcomeFromError(error) });
      }
    }
  }

  const footerNote = final
    ? undefined
    : blockedArrival
      ? blockedFooter(arrivedState)
      : !value
        ? "Choose a role to ask for."
        : extend
          ? `${selfDecides ? "You'll decide this yourself, unless another operator does first. Until then" : "Makerspace staff decide. Until they do"}, it still ends ${formatShortDate(current)}.`
          : selfDecides
            ? "You'll decide this yourself, unless another operator does first."
            : "Makerspace staff decide.";

  return (
    <Modal open onClose={onClose} busy={create.isPending} size="sm" labelledBy="request-title">
      <ModalHeader
        title="Ask for access"
        titleId="request-title"
        lede="Say what you need and why. Makerspace staff decide, and you'll see the answer here."
      />
      <div className="flex flex-col gap-3.5 px-6">
        {!final && (
          <>
            <div>
              <FieldLabel id="ask-role-label">What do you need?</FieldLabel>
              <RolePicker
                mode="one"
                audience="member"
                places={places}
                value={value}
                onChange={setPicked}
                cameFrom={initial?.projectId}
                outlined={blockedArrival ?? undefined}
                disabled={create.isPending}
                labelledBy="ask-role-label"
              />
              {blockedArrival && arrivedState ? (
                <ArrivalLine state={arrivedState} role={nameOf(blockedArrival)} />
              ) : (
                <FieldHint>
                  Can&rsquo;t find it? Pick the closest one and describe what you actually need in Why.
                  Makerspace staff read that before deciding.
                </FieldHint>
              )}
            </div>

            {!blockedArrival && extend && keep && keepChoice && current ? (
              <>
                <p className="text-[14px] leading-[1.55] text-ink">
                  You have this until {formatShortDate(current)}. Ask to keep it longer, and makerspace
                  staff decide.
                </p>
                <div>
                  <FieldLabel>How much longer?</FieldLabel>
                  <div className="mb-2">
                    <Segmented<KeepHow>
                      label="How much longer?"
                      size="sm"
                      value={keepHow}
                      onChange={setKeepHow}
                      options={[
                        { value: "month", label: "Another month" },
                        {
                          value: "term",
                          label: "To the end of next term",
                          disabled: Boolean(termOff),
                          title: termOff,
                        },
                      ]}
                    />
                  </div>
                  <FieldHint>
                    If it&rsquo;s approved, you keep it to {formatShortDate(keepChoice.until)} instead of{" "}
                    {formatShortDate(current)}.
                  </FieldHint>
                </div>
              </>
            ) : !blockedArrival ? (
              <div>
                {/*
                  Three options, no free-form number: a member thinks in weeks and
                  terms, not in days, and the resolved date underneath turns the
                  choice back into something concrete.
                */}
                <FieldLabel>How long do you need it?</FieldLabel>
                <div className="mb-2 flex flex-wrap gap-2">
                  {(
                    [
                      ["7", "A week"],
                      ["30", "A month"],
                      ["term", "Until the end of term"],
                    ] as Array<[HowLong, string]>
                  ).map(([v, label]) => (
                    <button
                      key={v}
                      type="button"
                      aria-pressed={howLong === v}
                      onClick={() => setHowLong(v)}
                      className={`rounded-pill font-semibold motion-tint ${PILL.md} ${
                        howLong === v ? "bg-accent-dense text-accent-ink" : "bg-tint-2 text-ink"
                      }`}
                    >
                      {label}
                    </button>
                  ))}
                </div>
                <FieldHint>
                  If it&rsquo;s approved, you have it until {formatShortDate(ordinaryUntil)}, and then it
                  ends. You can ask again.
                </FieldHint>
              </div>
            ) : null}

            {!blockedArrival && (
              <div>
                <FieldLabel htmlFor="req-why">Why?</FieldLabel>
                <Input
                  id="req-why"
                  value={why}
                  onChange={(event) => setWhy(event.target.value)}
                  placeholder="Running the Trotec for my capstone build"
                />
              </div>
            )}
          </>
        )}
        {/* The sheet becomes its own result. Once asked, the form goes: left in
            place, it sent the same request again on every click. */}
        {result && <ActionOutcome outcome={result.outcome} word={result.word} />}
      </div>

      <ModalFooter note={footerNote}>
        {final ? (
          <Button variant="outline" onClick={onClose}>
            Close
          </Button>
        ) : (
          <>
            <Button
              variant="accent"
              disabled={!value || Boolean(blockedArrival)}
              /* Why it's off is the footer note's sentence; said once. */
              isPending={create.isPending}
              onClick={send}
            >
              {extend ? "Ask to keep it" : "Send request"}
            </Button>
            <Button onClick={onClose}>Cancel</Button>
          </>
        )}
      </ModalFooter>
    </Modal>
  );
}

/** What the row says on its right: only what a member needs before choosing. */
function noteOf(state: AskState) {
  switch (state.kind) {
    case "for_good":
      return { kind: "held" } as const;
    case "until":
      return { kind: "until", until: state.until } as const;
    case "asked":
      return { kind: "asked", at: state.at } as const;
    case "onway":
      return { kind: "onway", at: state.at } as const;
    default:
      return { kind: "none" } as const;
  }
}

const ARRIVAL_TONE: Record<string, StatusTone> = { for_good: "healthy", asked: "neutral", onway: "neutral" };

/**
 * A link that named a role that can't be asked for: a dot and one sentence in
 * place of the hint (A-04 §10). How long and Why go, and Send is off.
 */
function ArrivalLine({ state, role }: { state: AskState; role: { place: string; role: string } }) {
  const both = `${role.place} · ${role.role}`;
  const text =
    state.kind === "for_good"
      ? `You already have ${both}, with no end date, so there's nothing to ask for.`
      : state.kind === "asked"
        ? `You've already asked for ${both}. It's waiting, and the answer appears on this page.`
        : state.kind === "onway"
          ? `Makerspace staff gave you ${role.role}${state.at ? ` on ${formatShortDate(state.at)}` : ""}. It starts working once they send it, so there's nothing to ask for.`
          : "";
  if (!text) return null;
  return (
    <p className="mt-2.5 flex items-start gap-2.5 text-[14px] leading-[1.5] text-ink [text-wrap:pretty]">
      <StatusDot tone={ARRIVAL_TONE[state.kind] ?? "neutral"} className="mt-[7px]" />
      <span>{text}</span>
    </p>
  );
}

function blockedFooter(state: AskState | null): string {
  switch (state?.kind) {
    case "asked":
      return "Nothing more to do: the answer appears on this page.";
    case "onway":
      return "Nothing more to do: it starts working once makerspace staff send it.";
    default:
      return "Choose another role to ask for.";
  }
}

export const AskForAccess = mountedWhileOpen(AskForAccessOpen);
