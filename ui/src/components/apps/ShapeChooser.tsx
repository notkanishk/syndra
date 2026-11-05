"use client";

import { Fragment, useEffect, useRef, useState, type ReactNode } from "react";

import { ProjectName } from "@/components/names";
import { StatusDot } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { ChoiceRow } from "@/components/ui/ChoiceRow";
import { Ticks } from "@/components/ui/TokenBlock";
import { Unfold } from "@/lib/motion";
import { TEMPLATE_FOR, TEMPLATE_META, TEMPLATE_SHEET } from "@/lib/claims";
import { outcomeFromError } from "@/lib/outcome";
import { useApplications, type ApplicationView } from "@/lib/queries/useApplications";
import { useDraftFrom, type AppClaimShape, type ClaimTemplate } from "@/lib/queries/useClaimShape";

import { draftFrom, type ShapeDraft } from "./shape-draft";
import { sortByName } from "@/lib/sort";

/**
 * The one chooser for both offers (A-14 §2, §8): a template, or another
 * app's shape. Choosing a row makes a draft at once, so the preview runs it
 * against what the app gets now; the footer's button commits it to the card.
 */
export function ShapeChooser({
  mode,
  shape,
  templates,
  draft,
  setDraft,
  project,
  onOpenTemplate,
  footer,
}: {
  mode: "template" | "copy";
  shape: AppClaimShape;
  templates: ClaimTemplate[];
  draft: ShapeDraft | null;
  setDraft: (d: ShapeDraft | null) => void;
  project: string;
  onOpenTemplate?: (id: string) => void;
  /** The button and its before-line; `start` is absent until a row's draft is in. */
  footer: (verb: string, note: string, start?: () => void) => ReactNode;
}) {
  const app = shape.application.name;
  const make = useDraftFrom(shape.application.id);
  const apps = useApplications();
  const [chosen, setChosen] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const choose = async (id: string, name?: string) => {
    setChosen(id);
    setError(null);
    try {
      const d = await make.mutateAsync(mode === "template" ? { template_id: id } : { application_id: id });
      setDraft(draftFrom(d, mode, name));
    } catch (e) {
      setDraft(null);
      setError(outcomeFromError(e).message);
    }
  };

  // The template the app's name suggests is preselected, so its draft previews at once.
  const preselected = useRef(false);
  useEffect(() => {
    if (preselected.current || mode !== "template" || !shape.suggested_template || chosen) return;
    preselected.current = true;
    void choose(shape.suggested_template);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [mode, shape.suggested_template]);

  const ready = draft && !draft.committed && chosen ? () => setDraft({ ...draft, committed: true }) : undefined;

  if (mode === "template") {
    const tpl = templates.find((t) => t.id === chosen);
    const short = tpl ? (TEMPLATE_SHEET[tpl.id]?.short ?? tpl.name) : "a template";
    return (
      <>
        <div className="flex flex-col gap-1.5 px-3.5 pb-3 pt-1">
          {templates.map((t) => {
            const words = TEMPLATE_SHEET[t.id];
            const suggested = shape.suggested_template === t.id || TEMPLATE_FOR[t.id]?.test(app);
            return (
              <div key={t.id} className="flex flex-col">
                <ChoiceRow
                  single
                  name="shape-from"
                  selected={chosen === t.id}
                  onToggle={() => void choose(t.id)}
                  title={t.name}
                  meta={<Ticks text={TEMPLATE_META[t.id] ?? ""} />}
                  description={words?.description ?? t.description}
                  note={suggested ? `Suggested: this app is called ${app}` : undefined}
                />
                <Unfold open={chosen === t.id}>
                  <div className="flex flex-col gap-[7px] pb-2 pl-11 pr-3.5 pt-2.5">
                    {words?.chooser.map((line) => <DotLine key={line} text={line} />)}
                    {onOpenTemplate && (
                      <span className="inline-flex">
                        <Button size="sm" onClick={() => onOpenTemplate(t.id)}>
                          See the whole template
                        </Button>
                      </span>
                    )}
                  </div>
                </Unfold>
              </div>
            );
          })}
          {error && <p role="alert" className="px-3.5 text-[13px] text-danger-text">{error}</p>}
        </div>
        {footer(
          `Start from ${short}`,
          shape.source === "app"
            ? "Makes a draft. Nothing changes until you save."
            : `Makes a draft. ${app} keeps ${project}'s default until you save.`,
          ready,
        )}
      </>
    );
  }

  // Copy: every app with a shape of its own, this project's first.
  const others = (apps.data ?? []).filter((a) => a.claims?.source === "app" && a.application.id !== shape.application.id);
  const here = others.filter((a) => a.application.project_id === shape.application.project_id);
  const elsewhere = others.filter((a) => a.application.project_id !== shape.application.project_id);
  const picked = others.find((a) => a.application.id === chosen);
  const name = picked?.application.name ?? "";
  const groups: Array<[ReactNode, ApplicationView[]]> = [
    [<>In {project}</>, here],
    ["In other projects", elsewhere],
  ];
  return (
    <>
      <div className="flex flex-col gap-1.5 px-3.5 pb-3 pt-1">
        {others.length === 0 && (
          <p className="px-2 py-2 text-[13.5px] text-faint">No other app has a shape of its own yet.</p>
        )}
        {groups.map(([label, list], gi) =>
          list.length === 0 ? null : (
            <Fragment key={gi}>
              <span className="type-label px-2 pb-0.5 pt-2.5">{label}</span>
              {sortByName(list, (a) => a.application.name, (a) => a.application.id).map((a) => (
                  <div key={a.application.id} className="flex flex-col">
                    <ChoiceRow
                      single
                      name="copy-from"
                      selected={chosen === a.application.id}
                      onToggle={() => void choose(a.application.id, a.application.name)}
                      title={a.application.name}
                      meta={
                        <>
                          <Ticks text={keysLine(a)} /> · <ProjectName id={a.application.project_id} />
                        </>
                      }
                      note={shapeNote(a, apps.data ?? [], templates)}
                    />
                    <Unfold open={chosen === a.application.id && Boolean(draft && !draft.committed)}>
                      {draft && <CopyLines draft={draft} from={a.application.name} project={project} sameProject={a.application.project_id === shape.application.project_id} roles={shape.project_roles} />}
                    </Unfold>
                  </div>
                ))}
            </Fragment>
          ),
        )}
        {error && <p role="alert" className="px-3.5 text-[13px] text-danger-text">{error}</p>}
      </div>
      {footer(
        picked ? `Copy ${name}'s shape` : "Copy this shape",
        picked
          ? `Makes a draft on ${app}. Nothing changes until you save. ${name} keeps its own; the two aren't linked.`
          : `Makes a draft on ${app}. Nothing changes until you save.`,
        ready,
      )}
    </>
  );
}

function DotLine({ text, faint = false }: { text: string; faint?: boolean }) {
  if (faint) {
    return (
      <span className="pl-4 text-[13px] leading-[1.5] text-faint [text-wrap:pretty]">
        <Ticks text={text} />
      </span>
    );
  }
  return (
    <div className="flex items-start gap-2.5">
      <span aria-hidden className="flex w-1.5 flex-none pt-2">
        <StatusDot tone="neutral" />
      </span>
      <span className="text-[13.5px] leading-[1.5] text-ink [text-wrap:pretty]">
        <Ticks text={text} />
      </span>
    </div>
  );
}

function CopyLines({
  draft,
  from,
  project,
  sameProject,
  roles,
}: {
  draft: ShapeDraft;
  from: string;
  project: string;
  sameProject: boolean;
  roles: AppClaimShape["project_roles"];
}) {
  const p = draft.profile;
  const n = p.claims.length;
  const lands = p.lands === "both" ? "both" : p.lands === "access_token" ? "access token" : "ID token and userinfo";
  const mapped = (p.roles ?? []).map((r) => roles.find((x) => x.key === r.key)?.label ?? r.key);
  const lines = [
    n === 1
      ? `Copies 1 claim, \`${p.claims[0].path.join(".")}\`, where it lands (${lands}) and roles added later (${p.include_new ? "included" : "left out"}).`
      : `Copies ${n} claims, where they land (${lands}) and roles added later (${p.include_new ? "included" : "left out"}).`,
    sameProject
      ? `Role words for all ${roles.length} roles: both apps are on ${project}.`
      : mapped.length
        ? `Role words for ${mapped.length === 1 ? mapped[0] : `${mapped.slice(0, -1).join(", ")} and ${mapped[mapped.length - 1]}`}, which ${project} has too.`
        : "",
    ...Object.entries(draft.missing).map(
      ([key, role]) => `\`${key.split("\u0000").join(" › ")}\` depends on ${role}, which ${project} doesn't have. It copies with no role; choose one before you save.`,
    ),
  ].filter(Boolean);
  return (
    <div className="flex flex-col gap-[7px] pb-2 pl-11 pr-3.5 pt-2.5">
      {lines.map((l) => (
        <DotLine key={l} text={l} />
      ))}
      <DotLine
        faint
        text={
          draft.notCopied.length
            ? `Not in this project, so not copied: ${draft.notCopied.map((r) => `${r.name} (\`${r.role_key}\`)`).join(", ")}.`
            : `${from} keeps its own; the two aren't linked.`
        }
      />
    </div>
  );
}

/** "`roles` and 7 more" for a copy row's meta. */
function keysLine(a: ApplicationView): string {
  const keys = a.claims?.keys ?? [];
  if (keys.length === 0) return "nothing of its own";
  return `\`${keys[0]}\`${keys.length > 1 ? ` and ${keys.length - 1} more` : ""}`;
}

/** The note seat: "NetBird template", "Custom, changed", "Copied from NetBird". */
export function shapeNote(a: ApplicationView, all: ApplicationView[], templates: ClaimTemplate[]): string {
  const c = a.claims;
  if (!c) return "";
  if (c.copied_from_application_id) {
    const from = all.find((x) => x.application.id === c.copied_from_application_id)?.application.name;
    return from ? `Copied from ${from}` : "Copied from another app";
  }
  const t = templates.find((x) => x.id === c.template_id);
  if (!t) return c.source === "app" ? "Its own shape" : "";
  const bare = t.id === "custom" || t.id === "sign-in-only";
  return c.template_changed ? `${TEMPLATE_SHEET[t.id]?.short ?? t.name}, changed` : bare ? t.name : `${t.name} template`;
}
