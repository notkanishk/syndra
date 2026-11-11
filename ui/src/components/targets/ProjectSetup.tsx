"use client";

import Link from "next/link";
import { useState } from "react";

import { ActionOutcome } from "@/components/ui/ActionOutcome";
import { ArrowLink } from "@/components/ui/ArrowLink";
import { Button } from "@/components/ui/Button";
import { Card, CardHeader, CardHeaderLink } from "@/components/ui/Card";
import { FieldHint, FieldLabel } from "@/components/ui/Input";
import { Modal, ModalFooter, ModalHeader } from "@/components/ui/Modal";
import { Notice } from "@/components/ui/Notice";
import { Select } from "@/components/ui/Select";
import { describeFailure, sentence } from "@/lib/outcome";
import { mappedTo } from "@/lib/fields";
import { useFieldSchema } from "@/lib/queries/useFieldSchema";
import { targetLabel } from "@/lib/nav";
import { useMappingHistory, useMappings } from "@/lib/queries/useMappings";
import { useNameResolver } from "@/lib/queries/useNameResolver";
import { useProjects } from "@/lib/queries/useProjects";
import {
  HOME_ROOT,
  useSetTargetProject,
  useSetTargetSetting,
  useTargetHomes,
  useTargetSettings,
  useTargets,
  type TargetSetting,
} from "@/lib/queries/useTargets";
import { projectLabel } from "@/components/names/Unresolved";
import { sortByName } from "@/lib/sort";

/**
 * A system's one Zitadel project (3a, frames `noproject` and `projectset`).
 *
 * Its roles decide who gets an account there, so it comes before anything
 * that gives access. A project serves one system: one already in use is
 * listed, disabled and named, rather than hidden.
 */

export function ProjectStep({ target, onSet }: { target: string; onSet: (name: string) => void }) {
  const name = targetLabel(target);
  const schema = useFieldSchema(target);
  const projects = useProjects();
  const roster = useTargets();
  const set = useSetTargetProject(target);
  const [chosen, setChosen] = useState("");

  const usedBy = new Map<string, string>();
  for (const t of roster.data ?? []) {
    if (t.target !== target && t.project) usedBy.set(t.project.id, targetLabel(t.target));
  }
  const rows = sortByName(projects.data ?? [], (r) => r.project.name, (r) => r.project.id);

  return (
    <section id="project" aria-labelledby="project-title" className="scroll-mt-4">
      <Card>
        <CardHeader
          title={<span id="project-title">Choose {name}&apos;s Zitadel project</span>}
          note="Step 1 of 2"
        />
        <div className="flex flex-col gap-4 px-5 pb-5">
          <p className="max-w-[72ch] text-[14.5px] leading-[1.55] text-ink [text-wrap:pretty]">
            Its roles decide who gets an account on {name}, so nothing below can give anyone access until it
            is set. Next, you map those roles to {mappedTo(name, schema)}.
          </p>
          <form
            className="flex flex-wrap items-center gap-x-3 gap-y-2.5"
            onSubmit={(e) => {
              e.preventDefault();
              const project = rows.find((r) => r.project.id === chosen)?.project;
              if (project) set.mutate(project.id, { onSuccess: () => onSet(project.name) });
            }}
          >
            <div className="min-w-[220px] flex-[0_1_360px]">
              <Select aria-label="Zitadel project" value={chosen} onChange={(e) => setChosen(e.target.value)}>
                <option value="">{projects.isLoading ? "Reading Zitadel's projects…" : "Choose a project…"}</option>
                {rows.map(({ project }) => {
                  const user = usedBy.get(project.id);
                  const roles = project.roles.length;
                  return (
                    <option key={project.id} value={project.id} disabled={Boolean(user)}>
                      {project.name} ·{" "}
                      {user ? `used by ${user}` : roles === 0 ? "no roles yet" : `${roles} role${roles === 1 ? "" : "s"}`}
                    </option>
                  );
                })}
              </Select>
            </div>
            <span className="flex-none whitespace-nowrap">
              <Button type="submit" variant="accent" size="sm" disabled={!chosen} isPending={set.isPending}>
                Use this project
              </Button>
            </span>
          </form>
          <span className="text-[13px] leading-[1.5] text-muted [text-wrap:pretty]">
            A project serves one system, so one already in use can&apos;t be picked. Changing it later means
            removing its mappings first.
          </span>
          {set.error && <p className="text-[13.5px] text-danger-text">{sentence(describeFailure(set.error))}</p>}
        </div>
      </Card>
    </section>
  );
}

/** The step's result, in its place, for this visit only. */
export function ProjectSet({ target, project }: { target: string; project: string }) {
  const name = targetLabel(target);
  const schema = useFieldSchema(target);
  return (
    <div className="flex flex-col gap-3">
      <ActionOutcome
        word="Chosen"
        outcome={{
          kind: "applied",
          message: `${name} uses ${project}.`,
          detail: `Nobody's access changed: no role reaches ${name} yet. Map ${project}'s roles to ${mappedTo(name, schema)} to give people accounts there.`,
        }}
      />
      <ArrowLink href={`/system/targets/${target}/mappings`} className="self-start whitespace-nowrap">
        Map its roles to {mappedTo(name, schema)}
      </ArrowLink>
    </div>
  );
}

/**
 * The system's settings, once the project is set (A-03 §1): one row per
 * setting, the label in a 170px seat, the value, then its way on in the 200px
 * right seat. The only link to the mappings, and the one place the home
 * location is chosen.
 */
export function SetupStrip({
  target,
  project,
  lastKnown = false,
}: {
  target: string;
  project: { id: string; name: string };
  /** The system isn't answering: choosing a folder lists its folders, so it waits (A-13 §1.5). */
  lastKnown?: boolean;
}) {
  const name = targetLabel(target);
  const schema = useFieldSchema(target);
  const mappings = useMappings(target);
  const history = useMappingHistory(target);
  const settings = useTargetSettings(target);
  const resolver = useNameResolver();
  const [choosing, setChoosing] = useState(false);
  const roles = new Set((mappings.data ?? []).map((m) => m.role_key)).size;
  const version = history.data?.current_version;
  const home = settings.data?.settings?.find((x) => x.name === HOME_ROOT);
  const verb = home?.value ? "Change folder" : "Choose folder";

  return (
    <Card>
      <SettingRow label="Zitadel project">
        <Link href={`/projects/${project.id}`} className="text-[15px] font-semibold text-ink link-hover">
          {projectLabel(project.name || resolver.resolveProject(project.id).value?.name)}
        </Link>
      </SettingRow>
      <SettingRow
        label="Role mappings"
        action={
          <CardHeaderLink href={`/system/targets/${target}/mappings`}>
            {roles === 0 ? "Add the first mapping" : "Mappings"}
          </CardHeaderLink>
        }
      >
        <span className="text-[15px] text-ink">
          {mappings.isLoading
            ? "…"
            : roles === 0
              ? "None yet"
              : `${roles} role${roles === 1 ? "" : "s"} to ${mappedTo(name, schema)}${version ? ` · version ${version}` : ""}`}
        </span>
      </SettingRow>
      {settings.data?.read && home && (
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
      )}
      {choosing && home && <HomeLocationDialog target={target} setting={home} onClose={() => setChoosing(false)} />}
    </Card>
  );
}

function SettingRow({
  label,
  action,
  children,
}: {
  label: string;
  action?: React.ReactNode;
  children: React.ReactNode;
}) {
  return (
    <div className="row-divider flex flex-wrap items-center gap-x-5 gap-y-1.5 px-5 py-3 first:border-t-0">
      <span className="type-label w-[170px] flex-none text-label">{label}</span>
      <span className="min-w-0 flex-[1_1_240px]">{children}</span>
      {/* Wraps right-aligned when the row is narrow, never under the label. */}
      <span className="ml-auto flex w-[200px] flex-none justify-end">{action}</span>
    </div>
  );
}

/**
 * Choosing, or changing, where home folders go (A-03 §1). One folder for every
 * role. Changing it moves nothing: homes already made stay, with their files.
 */
function HomeLocationDialog({
  target,
  setting,
  onClose,
}: {
  target: string;
  setting: TargetSetting;
  onClose: () => void;
}) {
  const name = targetLabel(target);
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
