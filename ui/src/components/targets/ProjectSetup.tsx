"use client";

import Link from "next/link";
import { useState } from "react";

import { StatusDot } from "@/components/ui/Badge";
import { ArrowLink } from "@/components/ui/ArrowLink";
import { Button } from "@/components/ui/Button";
import { Card, CardHeaderLink } from "@/components/ui/Card";
import { FieldHint, FieldLabel } from "@/components/ui/Input";
import { Modal, ModalFooter, ModalHeader } from "@/components/ui/Modal";
import { Select } from "@/components/ui/Select";
import { describeFailure, sentence } from "@/lib/outcome";
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
import { UNKNOWN_PROJECT } from "@/components/names/Unresolved";

/**
 * A system's one Zitadel project (3a, frames `noproject` and `projectset`).
 *
 * Its roles decide who gets an account there, so it comes before anything
 * that gives access. A project serves one system: one already in use is
 * listed, disabled and named, rather than hidden.
 */

function StepBar({ step }: { step: 1 | 2 }) {
  return (
    <span className="inline-flex items-center gap-2.5 text-[12.5px] font-semibold text-accent-text">
      Step {step} of 2
      <span aria-hidden className="flex w-[72px] gap-1">
        <span className="h-1 flex-1 rounded-pill bg-accent" />
        <span className={`h-1 flex-1 rounded-pill ${step === 2 ? "bg-accent" : "bg-accent-line"}`} />
      </span>
    </span>
  );
}

export function ProjectStep({ target, onSet }: { target: string; onSet: (name: string) => void }) {
  const name = targetLabel(target);
  const projects = useProjects();
  const roster = useTargets();
  const set = useSetTargetProject(target);
  const [chosen, setChosen] = useState("");

  const usedBy = new Map<string, string>();
  for (const t of roster.data ?? []) {
    if (t.target !== target && t.project) usedBy.set(t.project.id, targetLabel(t.target));
  }
  const rows = projects.data ?? [];

  return (
    <section
      id="project"
      aria-labelledby="project-title"
      className="flex scroll-mt-4 flex-col gap-3.5 rounded-panel border border-accent-line bg-accent-soft px-6 pb-6 pt-[22px]"
    >
      <div className="flex flex-wrap items-center justify-between gap-x-5 gap-y-2">
        <h2 id="project-title" className="flex items-center gap-2.5 font-display text-[19px] font-semibold text-accent-text">
          <StatusDot tone="accent" />
          Choose {name}&apos;s Zitadel project
        </h2>
        <StepBar step={1} />
      </div>
      <div className="flex flex-col gap-4 tablet:pl-[18px]">
        <p className="max-w-[72ch] text-[14.5px] leading-[1.55] text-ink [text-wrap:pretty]">
          Its roles decide who gets an account on {name}, so nothing below can give anyone access until it
          is set. Next, you map those roles to {name} groups.
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
            <Button type="submit" variant="accent" disabled={!chosen} isPending={set.isPending}>
              Use this project
            </Button>
          </span>
        </form>
        <span className="-mt-1.5 text-[13px] leading-[1.5] text-muted [text-wrap:pretty]">
          A project serves one system, so one already in use can&apos;t be picked. Changing it later means
          removing its mappings first.
        </span>
        {set.error && <p className="text-[13.5px] text-danger-text">{sentence(describeFailure(set.error))}</p>}
      </div>
    </section>
  );
}

/** The step's result, in its place, for this visit only. */
export function ProjectSet({ target, project }: { target: string; project: string }) {
  const name = targetLabel(target);
  return (
    <section
      role="status"
      className="flex flex-col gap-3.5 rounded-panel border border-accent-line bg-accent-soft px-6 pb-6 pt-[22px]"
    >
      <div className="flex flex-wrap items-center justify-between gap-x-5 gap-y-2">
        <h2 className="flex items-center gap-2.5 font-display text-[19px] font-semibold text-ink">
          <StatusDot tone="healthy" />
          {name} uses {project}.
        </h2>
        <StepBar step={2} />
      </div>
      <div className="flex flex-col gap-4 tablet:pl-[18px]">
        <p className="max-w-[72ch] text-[14.5px] leading-[1.55] text-ink [text-wrap:pretty]">
          Nobody&apos;s access changed: no role reaches {name} yet. Map {project}&apos;s roles to {name} groups to
          give people accounts there.
        </p>
        <ArrowLink href={`/system/targets/${target}/mappings`} className="self-start whitespace-nowrap">
          Map its roles to {name} groups
        </ArrowLink>
      </div>
    </section>
  );
}

/**
 * The system's settings, once the project is set (A-03 §1): one row per
 * setting, the label in a 170px seat, the value, then its way on in the 200px
 * right seat. The only link to the mappings, and the one place the home
 * location is chosen.
 */
export function SetupStrip({ target, project }: { target: string; project: { id: string; name: string } }) {
  const name = targetLabel(target);
  const mappings = useMappings(target);
  const history = useMappingHistory(target);
  const settings = useTargetSettings(target);
  const resolver = useNameResolver();
  const [choosing, setChoosing] = useState(false);
  const roles = new Set((mappings.data ?? []).map((m) => m.role_key)).size;
  const version = history.data?.current_version;
  const home = settings.data?.settings?.find((x) => x.name === HOME_ROOT);

  return (
    <Card>
      <SettingRow label="Zitadel project">
        <Link href={`/projects/${project.id}`} className="text-[15px] font-semibold text-ink hover:text-accent-text hover:underline">
          {project.name || resolver.resolveProject(project.id).value?.name || UNKNOWN_PROJECT}
        </Link>
      </SettingRow>
      <SettingRow
        label="Role mappings"
        action={
          <CardHeaderLink href={`/system/targets/${target}/mappings`}>
            {roles === 0 ? "Add the first mapping →" : "Mappings →"}
          </CardHeaderLink>
        }
      >
        <span className="text-[15px] text-ink">
          {mappings.isLoading
            ? "…"
            : roles === 0
              ? "None yet"
              : `${roles} role${roles === 1 ? "" : "s"} to ${name} groups${version ? ` · version ${version}` : ""}`}
        </span>
      </SettingRow>
      {settings.data?.read && home && (
        <SettingRow
          label="Home folders"
          action={
            <Button variant="outline" size="sm" onClick={() => setChoosing(true)}>
              {home.value ? "Change folder" : "Choose folder"}
            </Button>
          }
        >
          {home.value ? (
            <span className="text-[15px] text-ink">
              In <span className="type-mono text-[14px]">{home.labels?.[home.value] ?? home.value}</span>
            </span>
          ) : (
            <span className="flex flex-col gap-0.5">
              <span className="text-[15px] text-muted">Not chosen yet</span>
              <span className="text-[13px] text-faint">
                Mappings work without it; none can give a home folder until it is chosen.
              </span>
            </span>
          )}
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
      <div className="flex flex-col gap-4 px-6">
        <div className="flex flex-col gap-1.5">
          <FieldLabel htmlFor="home-folder">Folder in {share}</FieldLabel>
          {none || !setting.values_read ? (
            <Select id="home-folder" disabled>
              <option>{none ? "No folder to offer" : `${name} couldn't be read`}</option>
            </Select>
          ) : (
            <Select id="home-folder" value={chosen} onChange={(e) => setChosen(e.target.value)}>
              {setting.values.map((v) => (
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
          <div role="status" className="panel flex flex-col gap-1.5 px-4 py-3.5 text-[14px] leading-[1.55]">
            <span className="font-semibold">{share} has no folder to make homes in.</span>
            <span className="text-muted [text-wrap:pretty]">
              Homes go inside a folder at the top of the {share} share, and {share} has none yet. Make one on {name},
              then read again. Mappings work without it; none can give a home folder until one is chosen.
            </span>
            <span className="mt-1 flex">
              <Button variant="outline" size="sm" onClick={() => void settings.refetch()}>
                Read again
              </Button>
            </span>
          </div>
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
      <ModalFooter
        note={
          none
            ? `Nothing to choose until ${share} has a folder.`
            : current
              ? "Takes effect now. Nothing is moved, and nobody's access changes."
              : "Takes effect now. Nothing is made until a mapping gives home folders."
        }
      >
        <Button onClick={onClose}>
          Cancel
        </Button>
        <Button
          variant="accent"
          disabled={none || !chosen || chosen === current || save.isPending}
          onClick={() => save.mutate({ name: HOME_ROOT, value: chosen }, { onSuccess: onClose })}
        >
          {current ? `Make new homes in ${label(chosen)}` : `Use ${label(chosen)}`}
        </Button>
      </ModalFooter>
    </Modal>
  );
}
