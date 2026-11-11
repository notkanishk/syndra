"use client";

import { useState } from "react";

import { ActionOutcome } from "@/components/ui/ActionOutcome";
import { Button } from "@/components/ui/Button";
import { FieldHint, FieldLabel } from "@/components/ui/Input";
import { Modal, ModalFooter, ModalHeader } from "@/components/ui/Modal";
import { Notice } from "@/components/ui/Notice";
import { Select } from "@/components/ui/Select";
import { SettingRow } from "@/components/targets/SettingRow";
import { describeFailure, sentence } from "@/lib/outcome";
import { useSystemNames } from "@/lib/queries/useFieldSchema";
import { HOME_ROOT, useSetTargetSetting, useTargetHomes, useTargetSettings, type TargetSetting } from "@/lib/queries/useTargets";
import { sortByName } from "@/lib/sort";

/**
 * Where home folders go, on the system's setup card (A-03 §1): the row, and
 * the dialog it opens. Storage words, so they live in the storage kind; the
 * card renders this and knows nothing of what it says. Nothing is drawn until
 * the system's settings have been read and it offers this one.
 */
export function HomeLocationRow({ target, lastKnown }: { target: string; lastKnown: boolean }) {
  const nameOf = useSystemNames();
  const name = nameOf(target);
  const settings = useTargetSettings(target);
  const [choosing, setChoosing] = useState(false);
  const home = settings.data?.settings?.find((x) => x.name === HOME_ROOT);
  const verb = home?.value ? "Change folder" : "Choose folder";
  if (!settings.data?.read || !home) return null;

  return (
    <>
      <SettingRow
        label="Home folders"
        action={
          <Button variant="outline" size="sm" disabled={lastKnown} onClick={() => setChoosing(true)}>
            {verb}
          </Button>
        }
      >
        <span className="flex flex-col gap-0.5">
          {home.value ? (
            <span className="text-[15px] text-ink">
              In <span className="type-mono text-[14px]">{home.labels?.[home.value] ?? home.value}</span>
            </span>
          ) : (
            <>
              <span className="text-[15px] text-muted">Not chosen yet</span>
              <span className="text-[13px] text-faint">
                Mappings work without it; none can give a home folder until it is chosen.
              </span>
            </>
          )}
          {/* The value's second line, as the board draws it: one reason, no tooltip. */}
          {lastKnown && (
            <span className="text-[13px] text-faint">
              {verb} waits too: it lists the folders on {name}.
            </span>
          )}
        </span>
      </SettingRow>
      {choosing && <HomeLocationDialog target={target} setting={home} onClose={() => setChoosing(false)} />}
    </>
  );
}

/**
 * Choosing, or changing, where home folders go (A-03 §1). One folder for every
 * role. Changing it moves nothing: homes already made stay, with their files.
 */
export function HomeLocationDialog({
  target,
  setting,
  onClose,
}: {
  target: string;
  setting: TargetSetting;
  onClose: () => void;
}) {
  const nameOf = useSystemNames();
  const name = nameOf(target);
  const settings = useTargetSettings(target);
  const homes = useTargetHomes(target);
  const save = useSetTargetSetting(target);
  const current = setting.value;
  const [chosen, setChosen] = useState(current || setting.values[0] || "");
  const label = (v: string) => setting.labels?.[v] ?? v;
  const share = (label(setting.values[0] ?? current).split(" › ")[0] || "the share").trim();
  const none = setting.values_read && setting.values.length === 0;
  const changing = Boolean(current) && chosen !== "" && chosen !== current;
  // The dialog ends on its result, with Close; it never closes onto the row.
  const [done, setDone] = useState<{ word: string; message: string } | null>(null);
  const already = homes.data?.read
    ? homes.data.accounts.filter((a) => a.home?.startsWith(`${current}/`)).length
    : undefined;

  return (
    <Modal open onClose={onClose} size="md" labelledBy="home-title" busy={save.isPending}>
      <ModalHeader
        title={current ? "Change where home folders go" : "Choose where home folders go"}
        titleId="home-title"
        lede={`Everyone whose role gives a home folder gets one of their own in it, named after their account and private to them. It's one folder for every role on ${name}.`}
      />
      {done ? (
        <div className="px-6">
          <ActionOutcome word={done.word} outcome={{ kind: "applied", message: done.message }} />
        </div>
      ) : (
      <div className="flex flex-col gap-4 px-6">
        <div className="flex flex-col gap-1.5">
          <FieldLabel htmlFor="home-folder">Folder in {share}</FieldLabel>
          {none || !setting.values_read ? (
            <Select id="home-folder" disabled>
              <option>{none ? "No folder to offer" : `${name} couldn't be read`}</option>
            </Select>
          ) : (
            <Select id="home-folder" value={chosen} onChange={(e) => setChosen(e.target.value)}>
              {sortByName(setting.values, label, (v) => v).map((v) => (
                <option key={v} value={v}>
                  {label(v)}
                </option>
              ))}
            </Select>
          )}
          <FieldHint>
            {none ? `Read from ${name} just now.` : `The folders at the top of the ${share} share, read from ${name} just now.`}
          </FieldHint>
        </div>
        {none && (
          <Notice
            tone="neutral"
            title={`${share} has no folder to make homes in.`}
            action={{ label: "Read again", onClick: () => void settings.refetch() }}
            next={`Make a folder on ${name}, then press Read again.`}
          >
            Homes go inside a folder at the top of the {share} share, and {share} has none yet. Make one on {name},
            then read again. Mappings work without it; none can give a home folder until one is chosen.
          </Notice>
        )}
        {changing && (
          <div className="flex flex-col rounded-inner border border-line-strong">
            <div className="flex gap-3.5 px-4 py-3">
              <span className="w-[72px] flex-none text-[12.5px] font-semibold text-label">Changes</span>
              <span className="text-[14px] leading-[1.5]">
                New home folders are made in <span className="type-mono text-[13px]">{label(chosen)}</span>.
              </span>
            </div>
            <div className="row-divider flex gap-3.5 px-4 py-3">
              <span className="w-[72px] flex-none text-[12.5px] font-semibold text-label">Stays</span>
              <span className="text-[14px] leading-[1.5] text-muted [text-wrap:pretty]">
                The {already !== undefined ? `${already} ` : ""}home folder{already === 1 ? "" : "s"} already in{" "}
                <span className="type-mono text-[13px]">{label(current)}</span> stay there, with their files, and
                their people keep connecting to them where they are. Syndra never moves or deletes a home folder.
              </span>
            </div>
          </div>
        )}
        {save.error && <p className="text-[13.5px] text-danger-text">{sentence(describeFailure(save.error))}</p>}
      </div>
      )}
      <ModalFooter
        cancel={done ? undefined : { onClick: onClose }}
        note={
          done
            ? undefined
            : none
              ? `Nothing to choose until ${share} has a folder.`
              : current
                ? "Takes effect now. Nothing is moved, and nobody's access changes."
                : "Takes effect now. Nothing is made until a mapping gives home folders."
        }
      >
        {done ? (
          <Button variant="outline" onClick={onClose}>
            Close
          </Button>
        ) : (
          <Button
            variant="accent"
            disabled={none || !chosen || chosen === current || save.isPending}
            onClick={() =>
              save.mutate(
                { name: HOME_ROOT, value: chosen },
                {
                  onSuccess: () =>
                    setDone({
                      word: current ? "Changed" : "Chosen",
                      message: `New home folders go in ${label(chosen)}. Nothing was moved.`,
                    }),
                },
              )
            }
          >
            {current ? `Make new homes in ${label(chosen)}` : `Use ${label(chosen)}`}
          </Button>
        )}
      </ModalFooter>
    </Modal>
  );
}
