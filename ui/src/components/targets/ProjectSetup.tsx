"use client";

import Link from "next/link";
import { useState } from "react";

import { StatusDot } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { Select } from "@/components/ui/Select";
import { describeFailure, sentence } from "@/lib/outcome";
import { targetLabel } from "@/lib/nav";
import { useMappingHistory, useMappings } from "@/lib/queries/useMappings";
import { useProjects } from "@/lib/queries/useProjects";
import { useSetTargetProject, useTargets } from "@/lib/queries/useTargets";

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
            <Button type="submit" variant="accent" disabled={!chosen || set.isPending}>
              {set.isPending ? "Setting…" : "Use this project"}
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
        <Link
          href={`/system/targets/${target}/mappings`}
          className="inline-flex min-h-[44px] items-center self-start whitespace-nowrap text-[15px] font-semibold text-accent-text desktop:min-h-0"
        >
          Map its roles to {name} groups →
        </Link>
      </div>
    </section>
  );
}

/**
 * The project and the mappings, once set: the only link to the mappings
 * route.
 */
export function SetupStrip({ target, project }: { target: string; project: { id: string; name: string } }) {
  const name = targetLabel(target);
  const mappings = useMappings(target);
  const history = useMappingHistory(target);
  const roles = new Set((mappings.data ?? []).map((m) => m.role_key)).size;
  const version = history.data?.current_version;

  return (
    <Card>
      <div className="flex flex-wrap items-center gap-x-7 gap-y-2 px-5 py-3.5">
        <span className="flex flex-none flex-col gap-0.5">
          <span className="type-label text-label">Zitadel project</span>
          <Link href={`/projects/${project.id}`} className="text-[15px] font-semibold text-ink hover:text-accent-text hover:underline">
            {project.name || "Unnamed project"}
          </Link>
        </span>
        <span className="flex flex-none flex-col gap-0.5">
          <span className="type-label text-label">Role mappings</span>
          <span className="whitespace-nowrap text-[15px] text-ink">
            {mappings.isLoading
              ? "…"
              : roles === 0
                ? "None yet"
                : `${roles} role${roles === 1 ? "" : "s"} to ${name} groups${version ? ` · version ${version}` : ""}`}
          </span>
        </span>
        <Link
          href={`/system/targets/${target}/mappings`}
          className="ml-auto inline-flex min-h-[44px] items-center whitespace-nowrap text-[14.5px] font-semibold text-accent-text desktop:min-h-0"
        >
          {roles === 0 ? "Add the first mapping →" : "Mappings →"}
        </Link>
      </div>
    </Card>
  );
}
