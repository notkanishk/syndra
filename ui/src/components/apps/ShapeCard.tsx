"use client";

import Link from "next/link";
import { useEffect, useRef, useState, type ReactNode } from "react";

import { RolePicker } from "@/components/bundles/RolePicker";
import { AnchoredPanel } from "@/components/ui/Anchored";
import { Chip, Mono, StatusDot } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { ArrowLink } from "@/components/ui/ArrowLink";
import { FormPart } from "@/components/ui/FormPart";
import { Input } from "@/components/ui/Input";
import { Modal, ModalFooter, ModalHeader } from "@/components/ui/Modal";
import { RoleWordsTable, type RoleWordRow } from "@/components/ui/RoleWordsTable";
import { Segmented, Select } from "@/components/ui/Select";
import { Ticks } from "@/components/ui/TokenBlock";
import { Unfold } from "@/components/ui/Unfold";
import {
  ADD_MENU,
  addMuted,
  ATTRIBUTE_LABEL,
  breakingLine,
  claimLines,
  compareKeys,
  fromTemplate,
  ID_TOKEN_TITLE,
  KIND_LABEL,
  LANDS_LABEL,
  landsIn,
  NAME_SET_IN,
  namesRoles,
  READS_LABEL,
  roleWords,
  rolesIndex,
  rowProblems,
  sameProfile,
  SENT_AS_WORDS,
  sentAs,
  setRole,
  sharedWordOwner,
  wordOf,
  type AddEntry,
  type ClaimLine,
} from "@/lib/claims";
import { formatShortDate } from "@/lib/format";
import type {
  AppClaimShape,
  Claim,
  ClaimTemplate,
  Lands,
  Preview,
  Profile,
} from "@/lib/queries/useClaimShape";
import { useIsPhone } from "@/lib/useViewport";

import { ShapeChooser } from "./ShapeChooser";
import type { ShapeDraft } from "./shape-draft";

/**
 * "What <app> is told" (A-14 behaviour, laid out by A-16): one list of the
 * claims the app gets, the roles claim first and the rest A to Z, each line in
 * words. The list is the editor: a line opens its own editor in place, one at a
 * time. In Basic only the roles line opens, to its words and Included. In
 * Advanced, Add a claim (a menu of finished claims) and Lands in follow the
 * list; the template line sits at the card's foot. Save unfolds with the first
 * edit. An app with no shape of its own shows what its project's default sends
 * now, and the offer: start from a template, or copy another app's shape.
 */
export function ShapeCard({
  shape,
  draft,
  setDraft,
  token,
  person,
  preview,
  previewRowError,
  unreached,
  templates,
  advanced,
  projectName,
  onSave,
  onOpenTemplate,
  onOpenCopy,
}: {
  shape: AppClaimShape;
  draft: ShapeDraft | null;
  setDraft: (d: ShapeDraft | null) => void;
  /** The token the editors' "Sent as" lines read: the draft's, else what applies now. */
  token: Record<string, unknown> | null;
  person: string;
  preview?: Preview;
  /** The backend refused the draft at this row (400 VALIDATION_FAILED details.row). */
  previewRowError?: { row: number; message: string };
  /** Check 1 fails because the claims don't reach where the app reads (its `reads` key). */
  unreached?: string;
  templates: ClaimTemplate[];
  advanced: boolean;
  projectName: string;
  onSave: () => void;
  onOpenTemplate: (id: string) => void;
  onOpenCopy: () => void;
}) {
  const app = shape.application.name;
  const isNew = shape.source !== "app" && !draft?.committed;
  // What the project's default sends now, read-only, while the offer is open.
  const now = claimLines(shape.profile, shape.project_roles, { app, project: projectName });
  return (
    <Card className="w-full">
      <div className="px-[22px] pb-2 pt-5">
        <h2 className="type-card-title m-0">What {app} is told</h2>
      </div>
      {isNew ? (
        <>
          {!now.some((l) => l.roles) && <NoRoles app={app} first />}
          {now.map((l, i) => (
            <LineHead key={l.rows[0]} line={l} first={i === 0 && l.roles} />
          ))}
          <Unreached unreached={unreached} />
          <div className="row-divider flex flex-wrap items-center gap-x-3 gap-y-1 px-[22px] pb-4 pt-3">
            <span className="text-[13px] leading-[1.5] text-faint">In {landsIn(shape.profile.lands)}.</span>
            <ArrowLink href={`/projects/${shape.application.project_id}#claims-default`} className="text-[13.5px]">
              {projectName}&rsquo;s default
            </ArrowLink>
          </div>
          <NewAppOffer shape={shape} draft={draft} setDraft={setDraft} templates={templates} project={projectName} onOpenTemplate={onOpenTemplate} />
        </>
      ) : (
        <ShapeForm
          shape={shape}
          draft={draft}
          setDraft={setDraft}
          token={token}
          person={person}
          preview={preview}
          previewRowError={previewRowError}
          unreached={unreached}
          templates={templates}
          advanced={advanced}
          project={projectName}
          onSave={onSave}
          onOpenTemplate={onOpenTemplate}
          onOpenCopy={onOpenCopy}
        />
      )}
    </Card>
  );
}

function Unreached({ unreached }: { unreached?: string }) {
  if (!unreached) return null;
  return (
    <p className="m-0 px-[22px] pb-3 text-[13px] leading-[1.5] text-danger-text">
      Doesn&rsquo;t reach {READS_LABEL[unreached] ?? "the app"}. Check 1 in the preview says why.
    </p>
  );
}

/** The first line when nothing carries roles (Sign-in only, or the roles claim removed). It doesn't open. */
function NoRoles({ app, first }: { app: string; first: boolean }) {
  return (
    <div className={`flex items-start gap-x-3 pb-2 pl-[11px] pr-[22px] pt-1.5 ${first ? "" : "row-divider"}`}>
      <span className="size-[var(--tap)] flex-none" />
      <span className="pt-[11px] text-[14.5px] leading-[1.5] text-ink">No roles. {app} gets none from Syndra.</span>
    </div>
  );
}

/**
 * One claim's line: the 28px round chevron in its 44px seat (the grouped
 * list's), the key in mono in a 132px seat, its words and one faint line. When
 * it opens, the whole line is the control (a button with aria-expanded, as
 * CardRow's onToggle); when it doesn't, the seat stays so the keys align. On a
 * phone the key sits above its words.
 */
function LineHead({
  line,
  first,
  open,
  onToggle,
}: {
  line: ClaimLine;
  first: boolean;
  open?: boolean;
  onToggle?: () => void;
}) {
  const body = (
    <>
      <span className="flex size-[var(--tap)] flex-none items-center justify-center">
        {onToggle && (
          <span
            aria-hidden
            className={`flex size-7 items-center justify-center rounded-pill border border-line-strong text-[13px] text-muted motion-turn ${open ? "rotate-90" : ""}`}
          >
            ›
          </span>
        )}
      </span>
      <span className="flex min-w-0 flex-1 flex-wrap items-baseline gap-x-4 gap-y-0.5 pt-[11px]">
        <span className="min-w-0 flex-[0_0_132px] [overflow-wrap:anywhere]">
          {line.key ? <Mono className="text-[14px]">{line.key}</Mono> : <span className="text-[14px] text-faint">unnamed</span>}
        </span>
        <span className="flex min-w-0 flex-[1_1_220px] flex-col gap-0.5">
          <span className="text-[14.5px] leading-[1.5] text-ink [text-wrap:pretty]">
            <Ticks text={line.words} />
          </span>
          {line.sub && (
            <span className="text-[13px] leading-[1.5] text-faint [text-wrap:pretty]">
              <Ticks text={line.sub} />
            </span>
          )}
          {line.warn && <span className="text-[13px] leading-[1.5] text-warn-text [text-wrap:pretty]">{line.warn}</span>}
        </span>
      </span>
    </>
  );
  const cls = "flex w-full items-start gap-x-3 pb-2 pl-[11px] pr-[22px] pt-1.5 text-left";
  return onToggle ? (
    <button type="button" aria-expanded={open} onClick={onToggle} className={`${cls} hover-tint ${first ? "" : "row-divider"}`}>
      {body}
    </button>
  ) : (
    <div className={`${cls} ${first ? "" : "row-divider"}`}>{body}</div>
  );
}

function NewAppOffer({
  shape,
  draft,
  setDraft,
  templates,
  project,
  onOpenTemplate,
}: {
  shape: AppClaimShape;
  draft: ShapeDraft | null;
  setDraft: (d: ShapeDraft | null) => void;
  templates: ClaimTemplate[];
  project: string;
  onOpenTemplate: (id: string) => void;
}) {
  const app = shape.application.name;
  const [mode, setMode] = useState<"template" | "copy">("template");
  if (/truenas/i.test(app)) {
    return (
      <div className="row-divider flex flex-col gap-2 px-[22px] py-4 text-[14px] text-muted">
        TrueNAS&rsquo;s web interface can&rsquo;t sign in with Zitadel. Access to its shares goes through the TrueNAS add-on.
        <ArrowLink href="/system/targets">TrueNAS add-on</ArrowLink>
      </div>
    );
  }
  return (
    <>
      <div className="row-divider flex flex-wrap items-center gap-x-2.5 gap-y-2 px-[22px] pb-1.5 pt-4">
        <span className="type-label">Shape it for {app}</span>
        <span className="flex-1" />
        <Segmented<"template" | "copy">
          label="Shape it from"
          size="sm"
          value={mode}
          onChange={(m) => {
            setMode(m);
            setDraft(null);
          }}
          options={[
            { value: "template", label: "Start from a template" },
            { value: "copy", label: "Copy another app's shape" },
          ]}
        />
      </div>
      <ShapeChooser
        key={mode}
        mode={mode}
        shape={shape}
        templates={templates}
        draft={draft}
        setDraft={setDraft}
        project={project}
        onOpenTemplate={onOpenTemplate}
        footer={(verb, note, start) => (
          <div className="row-divider flex flex-wrap items-center gap-x-3.5 gap-y-2.5 px-[22px] pb-5 pt-4">
            <span className="flex-none whitespace-nowrap">
              <Button variant="accent" disabled={!start} onClick={start}>
                {verb}
              </Button>
            </span>
            <span className="min-w-0 flex-[1_1_220px] text-[13px] leading-[1.5] text-faint">{note}</span>
          </div>
        )}
      />
    </>
  );
}

// ---------------------------------------------------------------------------

/** Where the role picker was opened from: an if-role row's Change roles, or the menu's "Yes if they hold a role". */
type Picking = { row: number } | { inside?: string };

function ShapeForm({
  shape,
  draft,
  setDraft,
  token,
  person,
  preview,
  previewRowError,
  unreached,
  templates,
  advanced,
  project,
  onSave,
  onOpenTemplate,
  onOpenCopy,
}: {
  shape: AppClaimShape;
  draft: ShapeDraft | null;
  setDraft: (d: ShapeDraft | null) => void;
  token: Record<string, unknown> | null;
  person: string;
  preview?: Preview;
  previewRowError?: { row: number; message: string };
  unreached?: string;
  templates: ClaimTemplate[];
  advanced: boolean;
  project: string;
  onSave: () => void;
  onOpenTemplate: (id: string) => void;
  onOpenCopy: () => void;
}) {
  const app = shape.application.name;
  const saved = shape.source === "app" ? shape.profile : null;
  const profile = draft?.profile ?? shape.profile;
  const templateId = draft?.templateId ?? shape.version?.template_id ?? "";
  const template = templates.find((t) => t.id === templateId);
  const suggested = templates.find((t) => t.id === shape.suggested_template);
  const [picking, setPicking] = useState<Picking | null>(null);
  // One line open at a time, by its first row; closed on every arrival.
  const [open, setOpen] = useState<number | null>(null);
  // The line a menu pick just added: it arrives as a FormPart (A-16 §6).
  const [fresh, setFresh] = useState<number | null>(null);

  // The first edit makes a draft of what's saved; nothing changes until Save.
  const edit = (next: Profile) =>
    setDraft(
      draft
        ? { ...draft, profile: next }
        : {
            profile: next,
            origin: "edit",
            committed: true,
            templateId: shape.version?.template_id,
            templateVersion: shape.version?.template_version,
            notCopied: [],
            missing: {},
          },
    );
  const editClaim = (i: number, c: Claim) => edit({ ...profile, claims: profile.claims.map((x, j) => (j === i ? c : x)) });
  /** The line id (first row) that holds row `i` in `p`. */
  const lineOf = (p: Profile, i: number) => {
    const c = p.claims[i];
    return c.path.length === 1 ? i : p.claims.findIndex((o) => o.path.length > 1 && o.path[0] === c.path[0]);
  };
  const removeRows = (rows: number[]) => {
    const next = { ...profile, claims: profile.claims.filter((_, j) => !rows.includes(j)) };
    edit(next);
    // The open line keeps open if it is still there (an object losing one key).
    const was = open === null ? undefined : profile.claims[open];
    const at = was && was.path.length > 1 ? next.claims.findIndex((c) => c.path.length > 1 && c.path[0] === was.path[0]) : -1;
    setOpen(at >= 0 ? at : null);
  };
  const append = (c: Claim, opened: boolean) => {
    const next = { ...profile, claims: [...profile.claims, c] };
    edit(next);
    const id = lineOf(next, next.claims.length - 1);
    setFresh(id);
    if (opened) setOpen(id);
  };

  const dirty = Boolean(draft?.committed) && (draft!.origin !== "edit" || shape.source !== "app" || !sameProfile(draft!.profile, shape.profile));
  const problems = rowProblems(profile, { project, missing: draft?.missing });
  const firstProblem = problems.findIndex(Boolean);
  const breaking = dirty ? (preview?.breaking ?? []) : [];
  const hero = rolesIndex(profile);
  const words = roleWords(profile, shape.project_roles);
  const savedWords = saved ? new Map(roleWords(saved, shape.project_roles).map((w) => [w.role.key, w])) : null;

  // A line holding the row that stops Save opens by itself, once per problem
  // (Advanced), so the reason is never folded away; then it stays as you leave it.
  const problemRow = firstProblem >= 0 ? firstProblem : (previewRowError?.row ?? -1);
  const [forced, setForced] = useState(-1);
  if (forced !== problemRow && (problemRow < 0 || (advanced && profile.claims[problemRow]))) {
    setForced(problemRow);
    if (problemRow >= 0) setOpen(lineOf(profile, problemRow));
  }

  // The list's order is a view (A-16 §2): roles first, then A to Z. It is
  // settled while a line is open, so a name being typed doesn't carry its
  // field away; it takes its place when the line closes or the rows change.
  const lines = claimLines(profile, shape.project_roles, { app, project, saved });
  const ids = lines.map((l) => l.rows[0]);
  const sig = `${open}|${[...ids].sort((a, b) => a - b).join(",")}`;
  const [order, setOrder] = useState({ sig, ids });
  if (order.sig !== sig) setOrder({ sig, ids });
  const shown = order.ids.map((id) => lines.find((l) => l.rows[0] === id)).filter((l): l is ClaimLine => Boolean(l));

  const rows: RoleWordRow[] = words.map((w, i) => {
    const notes: string[] = [];
    const was = savedWords?.get(w.role.key);
    const word = wordOf({ key: w.role.key, as: w.as });
    let changed = false;
    if (was?.included && !w.included) {
      notes.push(`Was sent as \`${wordOf({ key: w.role.key, as: was.as })}\`.`);
      changed = true;
    } else if (was?.included && w.included && wordOf({ key: w.role.key, as: was.as }) !== word) {
      notes.push(`Was \`${wordOf({ key: w.role.key, as: was.as })}\`.`);
      changed = true;
    }
    const owner = sharedWordOwner(words, i);
    if (owner) notes.push(`Also ${owner.role.label}'s word.`);
    // "New" only means something once the shape has named roles: a shape that
    // never chose any (the seeded v1 shape) would call every role new.
    if (w.added && namesRoles(saved)) {
      notes.push(
        w.included
          ? draft?.origin === "copy" && draft.copiedName
            ? `New. Included, as ${draft.copiedName} includes roles added later.`
            : "New. Included, with roles added later."
          : "New. Left out until you include it.",
      );
    }
    return { key: w.role.key, name: w.role.label || w.role.key, word: w.as, included: w.included, changed, notes };
  });

  const tplLine = (() => {
    if (draft?.committed && draft.origin === "copy") return null; // drawn with its link below
    if (draft?.committed && draft.origin === "template") return `From the ${template?.name ?? "chosen"} template, as a draft. Not saved yet.`;
    if (shape.version?.copied_from_application_id && !draft) return `Copied from another app on ${formatShortDate(shape.version?.saved_at)}.`;
    if (shape.template) {
      return `${fromTemplate(shape.template.name, shape.template.changes.length)}${
        shape.template.updated ? ` The ${shape.template.name} template has changed since ${formatShortDate(shape.version?.saved_at)}.` : ""
      }`;
    }
    return "A shape of its own, from no template.";
  })();
  // The app's own template, one click away when it's on another (a seeded NetBird on Custom).
  const seeTemplate =
    draft?.origin === "copy"
      ? null
      : suggested && suggested.id !== templateId
        ? { id: suggested.id, label: `See the ${suggested.name} template` }
        : template
          ? { id: template.id, label: "See the template" }
          : null;

  const problemPath = firstProblem >= 0 ? profile.claims[firstProblem].path.join(" › ") || "the unnamed claim" : "";
  const beforeLine = !dirty
    ? { warn: false, text: "" }
    : firstProblem >= 0
      ? { warn: false, text: advanced ? `Fix \`${problemPath}\` to save.` : `Fix \`${problemPath}\` in Advanced to save.` }
      : breaking.length > 0
        ? { warn: true, text: breakingLine(breaking, { app, roles: shape.project_roles, reach: preview?.reach, templateId: appTemplateId(shape, draft) }) }
        : { warn: false, text: `Takes effect at each person's next sign-in to ${app}.` };

  const ctx: EditorContext = {
    app,
    project,
    profile,
    token,
    person,
    roles: shape.project_roles,
    leftOut: preview?.left_out ?? [],
    errorOf: (i) => problems[i]?.message ?? (previewRowError?.row === i ? previewRowError.message : undefined),
    edit,
    editClaim,
    removeRows,
    setOpen,
    lineOf,
    pickRoles: (row) => setPicking({ row }),
    onAdd: (e, inside) => add(e, inside),
  };

  function add(e: AddEntry, inside?: string) {
    const at = (key: string) => (inside === undefined ? [key] : [inside, key]);
    if (e.then === "roles") setPicking({ inside });
    else if (e.then === "value") append({ path: at(""), kind: "static", value: "" }, true);
    else append({ path: at(e.key!), ...e.claim! } as Claim, false);
  }

  const rolesPart = (
    <RolesPart
      ctx={ctx}
      row={hero}
      rows={rows}
      advanced={advanced}
      nameNote={NAME_SET_IN[appTemplateId(shape, draft) ?? ""]}
      onWord={(key, word) => edit(setRole(profile, key, { as: word.trim() === key ? "" : word }))}
      onIncluded={(key, inc) => edit(setRole(profile, key, { included: inc }))}
      onLater={(on) => edit({ ...profile, include_new: on })}
      laterNote={laterNote(profile.include_new, app, project, templateId)}
    />
  );

  return (
    <>
      {!lines.some((l) => l.roles) && <NoRoles app={app} first />}
      {shown.map((l, i) => {
        const id = l.rows[0];
        const opens = advanced || l.roles;
        const isOpen = open === id && opens;
        const body = (
          <div>
            <LineHead
              line={l}
              first={i === 0 && lines.some((x) => x.roles)}
              open={isOpen}
              onToggle={opens ? () => setOpen(isOpen ? null : id) : undefined}
            />
            <Unfold open={isOpen}>
              <div className="flex flex-col pb-1.5">
                {l.object && advanced ? <ObjectEditor ctx={ctx} line={l} /> : null}
                {!l.object && advanced && !l.roles ? <ClaimEditor ctx={ctx} row={l.rows[0]} /> : null}
                {l.roles ? rolesPart : null}
              </div>
            </Unfold>
          </div>
        );
        return (
          <FormPart key={id} appear={fresh === id} leaving={false} held={false} onGone={() => {}}>
            {body}
          </FormPart>
        );
      })}
      <Unreached unreached={unreached} />
      {draft?.notCopied.length ? (
        <p className="row-divider m-0 px-[22px] py-3 text-[13px] leading-[1.5] text-faint [text-wrap:pretty]">
          <Ticks text={`Not in this project, so not copied: ${draft.notCopied.map((r) => `${r.name} (\`${r.role_key}\`)`).join(", ")}.`} />
        </p>
      ) : null}

      {advanced && (
        <>
          <div className="row-divider pb-3.5 pl-5 pr-[22px] pt-2.5 tablet:pl-[67px]">
            <AddClaim label="Add a claim" app={app} profile={profile} onPick={(e) => add(e)} />
          </div>
          <div className="row-divider flex flex-wrap items-start gap-x-[18px] gap-y-2 px-[22px] py-3">
            <span className="w-[130px] flex-none pt-[5px] text-[14px] font-semibold">Lands in</span>
            <span className="min-w-0 flex-[1_1_140px] pt-[5px] text-[14px] text-ink">{LANDS_LABEL[profile.lands]}</span>
            <span className="ml-auto flex justify-end">
              <Segmented<Lands>
                label="Lands in"
                size="sm"
                value={profile.lands}
                onChange={(lands) => edit({ ...profile, lands })}
                options={[
                  { value: "both", label: "Both" },
                  { value: "access_token", label: "Access token" },
                  { value: "userinfo", label: "ID token and userinfo", title: ID_TOKEN_TITLE },
                ]}
              />
            </span>
          </div>
        </>
      )}

      {/* The foot: where it came from, and the two ways to start again (A-16 §1). */}
      <div className="row-divider flex flex-wrap items-center gap-x-3 gap-y-2 px-[22px] pb-4 pt-3">
        <span className="min-w-0 flex-[1_1_220px] text-[13px] leading-[1.5] text-faint [text-wrap:pretty]">
          {advanced ? "" : `In ${landsIn(profile.lands)}. `}
          {tplLine ?? (
            <>
              Copied from{" "}
              <Link href={`/applications/${draft?.copiedFrom?.application_id}`} className="text-muted link-hover">
                {draft?.copiedName}
              </Link>
              , as a draft. Not saved yet.
            </>
          )}
        </span>
        <span className="ml-auto inline-flex flex-none flex-wrap justify-end gap-1.5">
          {seeTemplate && (
            <Button size="sm" onClick={() => onOpenTemplate(seeTemplate.id)}>
              {seeTemplate.label}
            </Button>
          )}
          <Button size="sm" onClick={onOpenCopy}>
            Copy another app&rsquo;s shape
          </Button>
        </span>
      </div>

      {/* Save comes with the first edit (A-11 §4, A-15 §3). */}
      <Unfold open={Boolean(draft?.committed)}>
        <div className="row-divider flex flex-wrap items-center gap-x-3.5 gap-y-2.5 px-[22px] pb-5 pt-4">
          <span className="flex flex-none gap-2 whitespace-nowrap">
            <Button variant="accent" disabled={!dirty || firstProblem >= 0} onClick={onSave}>
              Save
            </Button>
            <Button variant="ghost" onClick={() => setDraft(null)}>
              Discard the draft
            </Button>
          </span>
          <span className={`flex min-w-0 flex-[1_1_240px] items-start gap-2 text-[13px] leading-[1.5] ${beforeLine.warn ? "text-warn-text" : "text-faint"}`}>
            {beforeLine.warn && <StatusDot tone="warn" className="mt-[7px]" />}
            <span>
              <Ticks text={beforeLine.text} />
            </span>
          </span>
        </div>
      </Unfold>

      {picking !== null && (
        <PickRoles
          projectId={shape.application.project_id}
          name={"row" in picking ? profile.claims[picking.row].path.join(" › ") : undefined}
          chosen={"row" in picking ? (profile.claims[picking.row].roles ?? []) : []}
          onClose={() => setPicking(null)}
          onDone={(roles) => {
            if ("row" in picking) editClaim(picking.row, { ...profile.claims[picking.row], roles });
            else {
              const key = `is_${roles[0]}`;
              append({ path: picking.inside === undefined ? [key] : [picking.inside, key], kind: "if_role", roles }, true);
            }
            setPicking(null);
          }}
        />
      )}
    </>
  );
}

/** The template whose breaking rule the app follows (the backend decides; this only names it). */
export function appTemplateId(shape: AppClaimShape, draft: ShapeDraft | null): string | undefined {
  const ids = [draft?.templateId, shape.version?.template_id, shape.suggested_template];
  return ids.find((id) => id && id !== "custom" && id !== "sign-in-only");
}

function laterNote(on: boolean, app: string, project: string, templateId: string): string {
  if (on) return `Included. ${app} sees every role added to ${project} later.`;
  return `Left out until you include them.${templateId === "netbird" ? " Every role NetBird sees becomes a group, so none arrives by surprise." : ""}`;
}

// ---------------------------------------------------------------------------
// The editors (A-16 §2–§3): settings rows, label 130 · control, indented to the
// line's key. On a phone the label sits above its control.

interface EditorContext {
  app: string;
  project: string;
  profile: Profile;
  token: Record<string, unknown> | null;
  person: string;
  roles: AppClaimShape["project_roles"];
  leftOut: number[];
  errorOf: (i: number) => string | undefined;
  edit: (p: Profile) => void;
  editClaim: (i: number, c: Claim) => void;
  removeRows: (rows: number[]) => void;
  setOpen: (id: number | null) => void;
  lineOf: (p: Profile, i: number) => number;
  pickRoles: (row: number) => void;
  onAdd: (e: AddEntry, inside?: string) => void;
}

const INDENT = "pl-5 pr-[22px] tablet:pl-[67px]";

function EditorRow({ label, note, seat, children }: { label: string; note?: string; seat?: ReactNode; children?: ReactNode }) {
  return (
    <div className={`row-divider flex flex-wrap items-start gap-x-4 gap-y-1.5 py-[9px] ${INDENT}`}>
      <span className="w-full flex-none pt-1.5 text-[13.5px] font-semibold tablet:w-[130px]">{label}</span>
      <span className="flex min-w-0 flex-[1_1_180px] flex-col gap-1">
        {children}
        {note && (
          <span className="text-[12.5px] leading-[1.5] text-faint [text-wrap:pretty]">
            <Ticks text={note} />
          </span>
        )}
      </span>
      {seat && <span className="ml-auto flex justify-end">{seat}</span>}
    </div>
  );
}

function Plain({ text }: { text: string }) {
  return (
    <span className="pt-1.5 text-[13.5px] leading-[1.5] text-muted [text-wrap:pretty]">
      <Ticks text={text} />
    </span>
  );
}

function EditorFoot({ children }: { children: ReactNode }) {
  return <div className={`flex flex-wrap items-center gap-2 pb-1.5 pt-2 ${INDENT}`}>{children}</div>;
}

function Literal({ text }: { text?: string }) {
  if (!text) return null;
  return (
    <p className={`row-divider m-0 pb-1 pt-2.5 text-[12.5px] leading-[1.55] text-faint [overflow-wrap:anywhere] [text-wrap:pretty] ${INDENT}`}>
      <Ticks text={text} />
    </p>
  );
}

function RowError({ text }: { text?: string }) {
  if (!text) return null;
  return (
    <p role="alert" className={`m-0 pb-1 pt-1.5 text-[13px] leading-[1.5] text-danger-text [text-wrap:pretty] ${INDENT}`}>
      <Ticks text={text} />
    </p>
  );
}

const TEXT_WARN = (app: string) =>
  `No app Syndra has a template for reads roles as text. NetBird would make one group called \`netbird-staff,netbird-admins\`. Use this only if ${app}'s own code splits it.`;

/** The roles line's editor: Claim name, Sent as (Advanced), the dense roles table, Roles added later (Advanced), Remove. */
function RolesPart({
  ctx,
  row,
  rows,
  advanced,
  nameNote,
  onWord,
  onIncluded,
  onLater,
  laterNote,
}: {
  ctx: EditorContext;
  row: number;
  rows: RoleWordRow[];
  advanced: boolean;
  nameNote?: string;
  onWord: (key: string, word: string) => void;
  onIncluded: (key: string, included: boolean) => void;
  onLater: (on: boolean) => void;
  laterNote: string;
}) {
  const c = ctx.profile.claims[row];
  const top = c.path.length === 1;
  const format = c.format ?? "array";
  return (
    <>
      {advanced && top && (
        <EditorRow label="Claim name" note={nameNote}>
          <span className="block max-w-[240px]">
            <ClaimName ctx={ctx} row={row} />
          </span>
        </EditorRow>
      )}
      {advanced && (
        <EditorRow
          label="Sent as"
          seat={
            <Segmented<"array" | "csv" | "space_delimited">
              label="Sent as"
              size="sm"
              value={format}
              onChange={(f) => ctx.editClaim(row, { ...c, format: f })}
              options={[
                { value: "array", label: "A list" },
                { value: "csv", label: "Text, with commas" },
                { value: "space_delimited", label: "Text, with spaces" },
              ]}
            />
          }
        >
          <Plain text={SENT_AS_WORDS[format]} />
          {format !== "array" && <span className="text-[13px] leading-[1.5] text-warn-text [text-wrap:pretty]"><Ticks text={TEXT_WARN(ctx.app)} /></span>}
        </EditorRow>
      )}
      <div className="row-divider">
        <RoleWordsTable dense app={ctx.app} rows={rows} onWord={onWord} onIncluded={onIncluded} />
      </div>
      {advanced && (
        <EditorRow
          label="Roles added later"
          seat={
            <Segmented<"out" | "in">
              label="Roles added later"
              size="sm"
              value={ctx.profile.include_new ? "in" : "out"}
              onChange={(v) => onLater(v === "in")}
              options={[
                { value: "out", label: "Left out" },
                { value: "in", label: "Included" },
              ]}
            />
          }
        >
          <Plain text={laterNote} />
        </EditorRow>
      )}
      {advanced && top && (
        <EditorFoot>
          <span className="ml-auto whitespace-nowrap">
            <Button variant="danger" size="sm" aria-label={`Remove the ${c.path[0]} claim`} onClick={() => ctx.removeRows([row])}>
              Remove {c.path[0]}
            </Button>
          </span>
        </EditorFoot>
      )}
    </>
  );
}

/** A claim's own name (the leaf of its path), mono, dense. A new fixed value arrives with it focused. */
function ClaimName({ ctx, row, label = "Claim name" }: { ctx: EditorContext; row: number; label?: string }) {
  const c = ctx.profile.claims[row];
  const leaf = c.path[c.path.length - 1] ?? "";
  const ref = useRef<HTMLInputElement>(null);
  const [focusOnMount] = useState(leaf === "");
  useEffect(() => {
    if (focusOnMount) ref.current?.focus({ preventScroll: true });
  }, [focusOnMount]);
  return (
    <Input
      ref={ref}
      dense
      aria-label={label}
      value={leaf}
      spellCheck={false}
      placeholder="claim_name"
      onChange={(e) => ctx.editClaim(row, { ...c, path: [...c.path.slice(0, -1), e.target.value] })}
      className="font-mono"
    />
  );
}

type Carries = string; // "roles" | "attr:<source>" | "static" | "if_role" | "bundles" | "role_ends"

const carriesOf = (c: Claim): Carries => (c.kind === "attribute" ? `attr:${c.source}` : c.kind);

function withCarries(c: Claim, v: Carries): Claim {
  const path = c.path;
  if (v.startsWith("attr:")) return { path, kind: "attribute", source: v.slice(5) };
  switch (v) {
    case "roles":
      return { path, kind: "roles", format: "array" };
    case "static":
      return { path, kind: "static", value: "" };
    case "if_role":
      return { path, kind: "if_role", roles: [] };
    default:
      return { path, kind: v as Claim["kind"] };
  }
}

/** What a claim carries: the menu's groups, plus the attributes behind them (fully technical, A-16 §3). */
function CarriesSelect({ ctx, row }: { ctx: EditorContext; row: number }) {
  const c = ctx.profile.claims[row];
  return (
    <Select dense aria-label="Carries" value={carriesOf(c)} onChange={(e) => ctx.editClaim(row, withCarries(c, e.target.value))}>
      <optgroup label="From the person">
        <option value="bundles">{KIND_LABEL.bundles}</option>
        <option value="role_ends">{KIND_LABEL.role_ends}</option>
        {Object.keys(ATTRIBUTE_LABEL).map((a) => (
          <option key={a} value={`attr:${a}`}>
            {ATTRIBUTE_LABEL[a]}
          </option>
        ))}
        <option value="roles">Roles {ctx.app} sees</option>
        <option value="if_role">{KIND_LABEL.if_role}</option>
      </optgroup>
      <optgroup label="The same for everyone">
        <option value="static">A fixed value</option>
      </optgroup>
    </Select>
  );
}

type FixedType = "text" | "number" | "bool" | "list";
const typeOf = (v: unknown): FixedType =>
  typeof v === "number" ? "number" : typeof v === "boolean" ? "bool" : Array.isArray(v) ? "list" : "text";

/** The kind's own row, only where one exists: the roles an if-role checks, a fixed value, a second roles claim's format. */
function KindRow({ ctx, row }: { ctx: EditorContext; row: number }) {
  const c = ctx.profile.claims[row];
  const set = (next: Claim) => ctx.editClaim(row, next);
  if (c.kind === "if_role") {
    return (
      <EditorRow label="If they hold">
        <span className="flex flex-wrap items-center gap-2">
          {(c.roles ?? []).map((k) => (
            <Chip key={k} className="py-1 text-[13px]">
              {ctx.roles.find((r) => r.key === k)?.label ?? k}
            </Chip>
          ))}
          <Button size="sm" onClick={() => ctx.pickRoles(row)}>
            Change roles
          </Button>
        </span>
      </EditorRow>
    );
  }
  if (c.kind === "static") {
    const t = typeOf(c.value);
    return (
      <EditorRow label="Value">
        <span className="flex flex-wrap items-center gap-2">
          <Segmented<FixedType>
            label="Type"
            size="sm"
            value={t}
            onChange={(next) => set({ ...c, value: next === "number" ? 0 : next === "bool" ? true : next === "list" ? [] : "" })}
            options={[
              { value: "text", label: "Text" },
              { value: "number", label: "Number" },
              { value: "bool", label: "Yes/no" },
              { value: "list", label: "List" },
            ]}
          />
          {t === "bool" ? (
            <Segmented<"yes" | "no">
              label="Value for everyone"
              size="sm"
              value={c.value ? "yes" : "no"}
              onChange={(v) => set({ ...c, value: v === "yes" })}
              options={[
                { value: "yes", label: "Yes" },
                { value: "no", label: "No" },
              ]}
            />
          ) : (
            <span className="min-w-[160px] flex-1">
              <Input
                dense
                aria-label="Value for everyone"
                type={t === "number" ? "number" : "text"}
                value={t === "list" ? ((c.value as unknown[]) ?? []).join(", ") : String(c.value ?? "")}
                placeholder={t === "list" ? "makers, staff" : "makerspace"}
                onChange={(e) => {
                  const raw = e.target.value;
                  set({
                    ...c,
                    value: t === "number" ? Number(raw) : t === "list" ? raw.split(",").map((s) => s.trim()).filter(Boolean) : raw,
                  });
                }}
              />
            </span>
          )}
        </span>
      </EditorRow>
    );
  }
  if (c.kind === "roles") {
    const format = c.format ?? "array";
    return (
      <EditorRow
        label="Sent as"
        seat={
          <Segmented<"array" | "csv" | "space_delimited">
            label="Sent as"
            size="sm"
            value={format}
            onChange={(f) => set({ ...c, format: f })}
            options={[
              { value: "array", label: "A list" },
              { value: "csv", label: "Text, with commas" },
              { value: "space_delimited", label: "Text, with spaces" },
            ]}
          />
        }
      >
        <Plain text={SENT_AS_WORDS[format]} />
      </EditorRow>
    );
  }
  return null;
}

/** "Sent as …" for one row, or why it's left out of the draft. */
function sentLine(ctx: EditorContext, row: number): string | undefined {
  const c = ctx.profile.claims[row];
  if (ctx.leftOut.includes(row)) return "Left out of the draft until it has a role.";
  return ctx.token && c.path.every(Boolean) ? sentAs(c, ctx.token, ctx.person, { app: ctx.app }) : undefined;
}

/** A top-level claim's editor: Claim name · Carries · its kind's row · Inside · the literal · Remove. */
function ClaimEditor({ ctx, row }: { ctx: EditorContext; row: number }) {
  const c = ctx.profile.claims[row];
  const leaf = c.path[0];
  const objects = Array.from(new Set(ctx.profile.claims.filter((o) => o.path.length > 1).map((o) => o.path[0]))).sort(compareKeys);
  return (
    <>
      <EditorRow label="Claim name">
        <span className="block max-w-[240px]">
          <ClaimName ctx={ctx} row={row} />
        </span>
      </EditorRow>
      <EditorRow label="Carries">
        <span className="block max-w-[280px]">
          <CarriesSelect ctx={ctx} row={row} />
        </span>
      </EditorRow>
      <KindRow ctx={ctx} row={row} />
      {/* Nesting lives here (A-16 §3): a dot stays a dot, rows sharing a parent merge. */}
      <EditorRow label="Inside">
        <span className="block max-w-[280px]">
          <Select
            dense
            aria-label="Inside"
            value=""
            onChange={(e) => {
              const path = e.target.value === "\u0000new" ? ["", leaf] : [e.target.value, leaf];
              const next = { ...ctx.profile, claims: ctx.profile.claims.map((x, j) => (j === row ? { ...c, path } : x)) };
              ctx.editClaim(row, { ...c, path });
              ctx.setOpen(ctx.lineOf(next, row));
            }}
          >
            <option value="">Nothing: top level</option>
            {objects.map((o) => (
              <option key={o} value={o}>
                {o || "unnamed"}
              </option>
            ))}
            <option value={"\u0000new"}>A new object</option>
          </Select>
        </span>
      </EditorRow>
      <Literal text={sentLine(ctx, row)} />
      <RowError text={ctx.errorOf(row)} />
      <EditorFoot>
        <span className="ml-auto whitespace-nowrap">
          <Button variant="danger" size="sm" aria-label={`Remove ${leaf || "this claim"}`} onClick={() => ctx.removeRows([row])}>
            Remove
          </Button>
        </span>
      </EditorFoot>
    </>
  );
}

/** An object's editor: its name, one compact row per key inside it (A to Z), the literal, Add a claim inside, Remove. */
function ObjectEditor({ ctx, line }: { ctx: EditorContext; line: ClaimLine }) {
  const inside = [...line.rows].sort((a, b) => compareKeys(ctx.profile.claims[a].path.slice(1).join("."), ctx.profile.claims[b].path.slice(1).join(".")));
  const key = line.key;
  const rename = (name: string) => {
    // One edit for every row inside: the object's name is their shared first segment.
    ctx.edit({ ...ctx.profile, claims: ctx.profile.claims.map((c, i) => (line.rows.includes(i) ? { ...c, path: [name, ...c.path.slice(1)] } : c)) });
  };
  const [focusOnMount] = useState(key === "");
  const ref = useRef<HTMLInputElement>(null);
  useEffect(() => {
    if (focusOnMount) ref.current?.focus({ preventScroll: true });
  }, [focusOnMount]);
  const n = line.rows.length;
  return (
    <>
      <EditorRow label="Claim name">
        <span className="block max-w-[240px]">
          <Input ref={ref} dense aria-label="Object name" value={key} spellCheck={false} placeholder="custom_claims" onChange={(e) => rename(e.target.value)} className="font-mono" />
        </span>
      </EditorRow>
      {inside.map((i) => {
        const c = ctx.profile.claims[i];
        const leaf = c.path[c.path.length - 1];
        return (
          <div key={i}>
            <div className={`row-divider flex flex-wrap items-center gap-x-2 gap-y-1.5 py-[7px] ${INDENT}`}>
              {c.path.length > 2 && <span className="text-[12.5px] text-faint">{c.path.slice(1, -1).join(" › ")} ›</span>}
              <span className="min-w-[120px] flex-[0_1_150px]">
                <ClaimName ctx={ctx} row={i} />
              </span>
              <span className="min-w-[150px] flex-[1_1_170px]">
                <CarriesSelect ctx={ctx} row={i} />
              </span>
              <span className="flex-none whitespace-nowrap">
                <Button
                  variant="danger"
                  size="sm"
                  aria-label={c.kind === "roles" ? `Remove ${leaf} from ${key}` : `Remove ${leaf || "this claim"}`}
                  onClick={() => ctx.removeRows([i])}
                >
                  Remove
                </Button>
              </span>
            </div>
            {c.kind !== "roles" && <KindRow ctx={ctx} row={i} />}
            <RowError text={ctx.errorOf(i)} />
          </div>
        );
      })}
      <Literal text={sentLine(ctx, line.rows[0])} />
      <EditorFoot>
        <AddClaim label="Add a claim inside" app={ctx.app} profile={ctx.profile} inside={key} onPick={(e) => ctx.onAdd(e, key)} />
        <span className="ml-auto whitespace-nowrap">
          <Button
            variant="danger"
            size="sm"
            aria-label={`Remove ${key || "this object"} and the ${n === 1 ? "claim" : `${n} claims`} inside it`}
            onClick={() => ctx.removeRows(line.rows)}
          >
            Remove {key || "the object"}
          </Button>
        </span>
      </EditorFoot>
    </>
  );
}

// ---------------------------------------------------------------------------

/**
 * Add a claim (A-16 §3, option a): outline sm, opening a short menu of
 * finished claims under it (AnchoredPanel, the QuestionSeat listbox's option
 * rows), or a Modal sm sheet below tablet. A taken key is muted, inert and says
 * why. Choosing is the act, so the sheet ends in no button.
 */
function AddClaim({
  label,
  app,
  profile,
  inside,
  onPick,
}: {
  label: string;
  app: string;
  profile: Profile;
  inside?: string;
  onPick: (e: AddEntry) => void;
}) {
  const [open, setOpen] = useState(false);
  const phone = useIsPhone();
  const button = useRef<HTMLSpanElement>(null);
  const panel = useRef<HTMLDivElement>(null);
  const top = inside === undefined;
  const taken = new Set(profile.claims.filter((c) => (top ? c.path.length >= 1 : c.path[0] === inside && c.path.length > 1)).map((c) => (top ? c.path[0] : c.path[1])));
  const hero = rolesIndex(profile);
  const rolesKey = hero >= 0 && profile.claims[hero].path.length === 1 ? profile.claims[hero].path[0] : undefined;

  useEffect(() => {
    if (!open || phone) return;
    const away = (e: MouseEvent) => {
      const t = e.target as Node;
      if (!button.current?.contains(t) && !panel.current?.contains(t)) setOpen(false);
    };
    const esc = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        setOpen(false);
        button.current?.querySelector("button")?.focus();
      }
    };
    document.addEventListener("mousedown", away);
    document.addEventListener("keydown", esc);
    return () => {
      document.removeEventListener("mousedown", away);
      document.removeEventListener("keydown", esc);
    };
  }, [open, phone]);

  const pick = (e: AddEntry) => {
    setOpen(false);
    onPick(e);
  };
  const list = (
    <div role="listbox" aria-label={label} className="flex flex-col gap-px p-2">
      {ADD_MENU.map((g) => (
        <div key={g.group} role="group" aria-label={g.group} className="flex flex-col gap-px">
          <span className="type-label px-3 pb-1 pt-2">{g.group}</span>
          {g.entries.map((e) => {
            const muted = addMuted(e, taken, { inside: !top, rolesKey });
            const right = muted ?? (e.key ? `\`${e.key}\`` : e.then === "roles" ? "Then choose the roles" : "Then name it and set it");
            return (
              <button
                key={e.label}
                type="button"
                role="option"
                aria-selected={false}
                aria-disabled={muted ? true : undefined}
                onClick={muted ? undefined : () => pick(e)}
                className={`motion-tint flex min-h-[var(--tap)] w-full flex-wrap items-baseline gap-x-3 gap-y-0.5 rounded-inner px-3 py-2 text-left desktop:min-h-0 ${
                  muted ? "cursor-default" : "hover:bg-[var(--hover)] focus-visible:bg-[var(--hover)]"
                }`}
              >
                <span className={`flex-1 text-[14px] font-semibold ${muted ? "text-muted" : "text-ink"}`}>{e.label.replace("{app}", app)}</span>
                <span className="flex-none text-[12.5px] text-faint">
                  <Ticks text={right} />
                </span>
              </button>
            );
          })}
        </div>
      ))}
    </div>
  );

  return (
    <span ref={button} className="inline-flex">
      <Button size="sm" aria-expanded={open} aria-haspopup="listbox" onClick={() => setOpen(!open)}>
        {label}
      </Button>
      {phone ? (
        <Modal open={open} onClose={() => setOpen(false)} size="sm">
          <ModalHeader title={label} />
          <div className="px-3 pb-4">{list}</div>
        </Modal>
      ) : (
        <AnchoredPanel
          anchor={button}
          open={open}
          width={390}
          expectHeight={520}
          panelRef={panel}
          className="popover-in overflow-hidden rounded-panel border border-line-strong bg-surface-2 shadow-popover"
        >
          {list}
        </AnchoredPanel>
      )}
    </span>
  );
}

/** The role picker in single-project mode: the roles an if-role claim checks. */
function PickRoles({
  projectId,
  name,
  chosen: initial,
  onClose,
  onDone,
}: {
  projectId: string;
  /** The claim's name, once it has one; a claim from the menu is named after its first role. */
  name?: string;
  chosen: string[];
  onClose: () => void;
  onDone: (roles: string[]) => void;
}) {
  const [chosen, setChosen] = useState(() => new Set(initial.map((k) => `${projectId}:${k}`)));
  return (
    <Modal open onClose={onClose} size="md">
      <ModalHeader
        title="Which roles make it true"
        lede={name ? `${name} is true for anyone who holds any of these, and false otherwise.` : "True for anyone who holds any of these, and false otherwise."}
      />
      <div className="px-6">
        <RolePicker
          onlyProject={projectId}
          selected={chosen}
          onToggle={(id) =>
            setChosen((prev) => {
              const next = new Set(prev);
              if (next.has(id)) next.delete(id);
              else next.add(id);
              return next;
            })
          }
        />
      </div>
      <ModalFooter>
        <Button onClick={onClose}>Cancel</Button>
        <Button variant="accent" disabled={chosen.size === 0} onClick={() => onDone(Array.from(chosen).map((id) => id.slice(projectId.length + 1)))}>
          {chosen.size <= 1 ? "Use this role" : `Use these ${chosen.size} roles`}
        </Button>
      </ModalFooter>
    </Modal>
  );
}
