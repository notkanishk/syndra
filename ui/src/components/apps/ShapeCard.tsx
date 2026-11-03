"use client";

import Link from "next/link";
import { useState } from "react";

import { RolePicker } from "@/components/bundles/RolePicker";
import { Chip, Mono, StatusDot } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { ClaimRow } from "@/components/ui/ClaimRow";
import { ArrowLink } from "@/components/ui/ArrowLink";
import { Input } from "@/components/ui/Input";
import { Modal, ModalFooter, ModalHeader } from "@/components/ui/Modal";
import { NotOpenYet } from "@/components/ui/NotOpenYet";
import { RoleWordsTable, type RoleWordRow } from "@/components/ui/RoleWordsTable";
import { Segmented, Select } from "@/components/ui/Select";
import { Ticks } from "@/components/ui/TokenBlock";
import {
  ATTRIBUTE_LABEL,
  BASIC_ATTRIBUTES,
  breakingLine,
  changesSince,
  isAdvancedRow,
  KIND_LABEL,
  LANDS_LABEL,
  pathKey,
  READS_LABEL,
  roleWords,
  rowProblems,
  sameProfile,
  sentAs,
  setRole,
  sharedWordOwner,
  TEMPLATE_CAPTION,
  wordOf,
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

import { ShapeChooser } from "./ShapeChooser";
import type { ShapeDraft } from "./shape-draft";

/**
 * "What <app> is told" (A-14 §1–5): the template line, the roles it sees in
 * its words, one row per claim, where they land, and Save. An app with no
 * shape of its own shows what its project's default sends now, and the offer
 * on top: start from a template, or copy another app's shape.
 */
export function ShapeCard({
  shape,
  draft,
  setDraft,
  token,
  person,
  preview,
  previewRowError,
  checkOneFailsReads,
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
  /** The token the rows' "Sent as" lines read: the draft's, else what applies now. */
  token: Record<string, unknown> | null;
  person: string;
  preview?: Preview;
  /** The backend refused the draft at this row (400 VALIDATION_FAILED details.row). */
  previewRowError?: { row: number; message: string };
  checkOneFailsReads: boolean;
  templates: ClaimTemplate[];
  advanced: boolean;
  projectName: string;
  onSave: () => void;
  onOpenTemplate: (id: string) => void;
  onOpenCopy: () => void;
}) {
  const app = shape.application.name;
  const isNew = shape.source !== "app" && !draft?.committed;
  return (
    <Card className="w-full">
      <div className="px-[22px] pb-3 pt-5">
        <h2 className="type-card-title">What {app} is told</h2>
        {isNew && <NewLede shape={shape} project={projectName} />}
      </div>
      {isNew ? (
        <NewAppOffer shape={shape} draft={draft} setDraft={setDraft} token={token} person={person} templates={templates} project={projectName} onOpenTemplate={onOpenTemplate} />
      ) : (
        <ShapeForm
          shape={shape}
          draft={draft}
          setDraft={setDraft}
          token={token}
          person={person}
          preview={preview}
          previewRowError={previewRowError}
          checkOneFailsReads={checkOneFailsReads}
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

function NewLede({ shape, project }: { shape: AppClaimShape; project: string }) {
  const app = shape.application.name;
  return (
    <p className="mt-1 max-w-[64ch] text-[14px] leading-[1.5] text-faint [text-wrap:pretty]">
      {shape.source === "builtin" ? (
        <Ticks
          text={`${app} uses ${project}'s default shape. Nobody has set one for ${project}, so it's Syndra's built-in: \`roles\`, every role in Syndra's words, as a list, in the access token and userinfo.`}
        />
      ) : (
        `${app} uses ${project}'s default shape, set on ${project}'s page.`
      )}{" "}
      <ArrowLink href={`/projects/${shape.application.project_id}#claims-default`} className="text-[14px]">
        {project}&rsquo;s default
      </ArrowLink>
    </p>
  );
}

function NewAppOffer({
  shape,
  draft,
  setDraft,
  token,
  person,
  templates,
  project,
  onOpenTemplate,
}: {
  shape: AppClaimShape;
  draft: ShapeDraft | null;
  setDraft: (d: ShapeDraft | null) => void;
  token: Record<string, unknown> | null;
  person: string;
  templates: ClaimTemplate[];
  project: string;
  onOpenTemplate: (id: string) => void;
}) {
  const app = shape.application.name;
  const [mode, setMode] = useState<"template" | "copy">("template");
  // The token "Sends now" reads is what applies now, never the chooser's draft.
  const nowToken = draft ? null : token;
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
      <div className="row-divider flex flex-col gap-1 px-[22px] py-3.5">
        <span className="type-label">Sends now</span>
        {shape.profile.claims.map((c) => (
          <div key={pathKey(c.path)} className="flex flex-col gap-1">
            <div className="flex flex-wrap items-baseline gap-x-3 gap-y-1">
              <Mono>{c.path.join(" › ")}</Mono>
              <span className="text-[14px] text-muted">{carriesLabel(c, project)}</span>
            </div>
            {nowToken && (
              <span className="text-[12.5px] leading-[1.5] text-faint [overflow-wrap:anywhere]">
                <Ticks text={sentAs(c, nowToken, person, { app })} />
              </span>
            )}
          </div>
        ))}
      </div>
      <div className="row-divider flex flex-col gap-2.5 px-[22px] pb-1.5 pt-4">
        <span className="type-label">Shape it for {app}</span>
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

function carriesLabel(c: Claim, project: string): string {
  switch (c.kind) {
    case "roles":
      return `Every role in ${project}, in Syndra's words`;
    case "attribute":
      return ATTRIBUTE_LABEL[c.source ?? ""] ?? c.source ?? "";
    default:
      return KIND_LABEL[c.kind];
  }
}

// ---------------------------------------------------------------------------

function ShapeForm({
  shape,
  draft,
  setDraft,
  token,
  person,
  preview,
  previewRowError,
  checkOneFailsReads,
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
  checkOneFailsReads: boolean;
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
  const [picking, setPicking] = useState<number | null>(null);

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

  const dirty = Boolean(draft?.committed) && (draft!.origin !== "edit" || shape.source !== "app" || !sameProfile(draft!.profile, shape.profile));
  const problems = rowProblems(profile, { project, missing: draft?.missing });
  const firstProblem = problems.findIndex(Boolean);
  const breaking = dirty ? (preview?.breaking ?? []) : [];
  const sendsRoles = profile.claims.some((c) => c.kind === "roles" || c.kind === "role_ends");
  const words = roleWords(profile, shape.project_roles);
  const savedWords = saved ? new Map(roleWords(saved, shape.project_roles).map((w) => [w.role.key, w])) : null;

  const rows: RoleWordRow[] = words.map((w, i) => {
    const notes: string[] = [];
    const was = savedWords?.get(w.role.key);
    const word = wordOf({ key: w.role.key, as: w.as });
    if (was?.included && !w.included) notes.push(`Was sent as \`${wordOf({ key: w.role.key, as: was.as })}\`.`);
    else if (was?.included && w.included && wordOf({ key: w.role.key, as: was.as }) !== word) notes.push(`Was \`${wordOf({ key: w.role.key, as: was.as })}\`.`);
    const owner = sharedWordOwner(words, i);
    if (owner) notes.push(`Also ${owner.role.label}'s word.`);
    if (w.added) {
      notes.push(
        w.included
          ? draft?.origin === "copy" && draft.copiedName
            ? `New. Included, as ${draft.copiedName} includes roles added later.`
            : "New. Included, with roles added later."
          : "New. Left out until you include it.",
      );
    }
    return { key: w.role.key, name: w.role.label || w.role.key, word: w.as, included: w.included, notes };
  });
  const included = words.filter((w) => w.included).length;
  const ownWords = words.some((w) => w.as);

  const tplLine = (() => {
    if (draft?.committed && draft.origin === "copy") return null; // drawn with its link below
    if (draft?.committed && draft.origin === "template") return `From the ${template?.name ?? "chosen"} template, as a draft. Not saved yet.`;
    if (shape.version?.copied_from_application_id && !draft) return null;
    if (shape.template) {
      return `From the ${shape.template.name} template. ${changesSince(shape.template.changes.length)}${
        shape.template.updated ? ` The ${shape.template.name} template has changed since ${formatShortDate(shape.version?.saved_at)}.` : ""
      }`;
    }
    return "A shape of its own, from no template.";
  })();
  const caption =
    draft?.origin === "copy" && draft.copiedName
      ? `${draft.copiedName}'s shape, on ${project}'s roles. What doesn't fit ${project} is said where it stops fitting.`
      : TEMPLATE_CAPTION[templateId];

  const reads = template?.reads;
  const landsNote = [
    reads ? `${app} reads ${READS_LABEL[reads]}.` : "",
    !advanced && template ? `Set by the ${template.name} template.` : "",
    draft?.origin === "copy" && draft.copiedName ? `Copied from ${draft.copiedName}.` : "",
  ]
    .filter(Boolean)
    .join(" ");

  const beforeLine = !dirty
    ? { warn: false, text: "Nothing to save yet." }
    : firstProblem >= 0
      ? { warn: false, text: `Fix \`${profile.claims[firstProblem].path.join(" › ") || "the unnamed claim"}\` to save.` }
      : breaking.length > 0
        ? { warn: true, text: breakingLine(breaking, { app, roles: shape.project_roles, reach: preview?.reach, templateId: appTemplateId(shape, draft) }) }
        : { warn: false, text: `Takes effect at each person's next sign-in to ${app}.` };

  return (
    <>
      <div className="flex flex-col gap-1.5 px-[22px] pb-4">
        <div className="flex flex-wrap items-center gap-x-3 gap-y-1">
          <span className="text-[13.5px] leading-[1.5] text-faint">
            {tplLine ? (
              tplLine
            ) : draft?.origin === "copy" ? (
              <>
                Copied from{" "}
                <Link href={`/applications/${draft.copiedFrom?.application_id}`} className="text-muted link-hover">
                  {draft.copiedName}
                </Link>
                , as a draft. Not saved yet.
              </>
            ) : (
              `Copied from another app on ${formatShortDate(shape.version?.saved_at)}.`
            )}
          </span>
          <span className="-ml-2.5 inline-flex flex-wrap">
            {template && draft?.origin !== "copy" && (
              <Button variant="ghost" size="sm" onClick={() => onOpenTemplate(template.id)}>
                See the template
              </Button>
            )}
            <Button variant="ghost" size="sm" onClick={onOpenCopy}>
              Copy another app&rsquo;s shape
            </Button>
          </span>
        </div>
        {caption && (
          <p className="max-w-[64ch] text-[13.5px] leading-[1.55] text-muted [text-wrap:pretty]">
            <Ticks text={caption} />
          </p>
        )}
      </div>

      {/* Roles NetBird sees (§3) */}
      <div className="row-divider flex flex-col gap-1 px-[22px] pb-3 pt-4">
        <div className="flex flex-wrap items-baseline gap-x-2.5 gap-y-1">
          <span className="type-label">Roles {app} sees</span>
          <span className="flex-1" />
          {sendsRoles && (
            <span className="text-[12.5px] text-faint">
              {included} of {words.length} included{ownWords ? "" : " · Syndra's words"}
            </span>
          )}
        </div>
        <p className="text-[13.5px] leading-[1.5] text-faint">
          Which of{" "}
          <Link href={`/projects/${shape.application.project_id}`} className="text-muted link-hover">
            {project}
          </Link>
          &rsquo;s roles {app} sees, and the word it sees for each.
        </p>
      </div>
      {sendsRoles ? (
        <RoleWordsTable
          app={app}
          rows={rows}
          onWord={(key, word) => edit(setRole(profile, key, { as: word.trim() === key ? "" : word }))}
          onIncluded={(key, inc) => edit(setRole(profile, key, { included: inc }))}
        />
      ) : (
        <div className="px-[22px] pb-3">
          <NotOpenYet
            open={false}
            title={`Roles reach ${app} only through a roles claim.`}
            why={templateId === "sign-in-only" ? `The Sign-in only shape sends none, on purpose: ${app} has no groups.` : "This shape sends none. Add a claim that carries them."}
          >
            {shape.project_roles.slice(0, 3).map((r) => (
              <div key={r.key} className="flex items-baseline gap-3">
                <span className="w-[150px] text-[14.5px] font-semibold">{r.label}</span>
                <Mono className="text-faint">{r.key}</Mono>
              </div>
            ))}
          </NotOpenYet>
        </div>
      )}
      {draft?.notCopied.length ? (
        <p className="row-divider px-[22px] py-3 text-[13px] leading-[1.5] text-faint [text-wrap:pretty]">
          <Ticks text={`Not in this project, so not copied: ${draft.notCopied.map((r) => `${r.name} (\`${r.role_key}\`)`).join(", ")}.`} />
        </p>
      ) : null}
      {advanced && sendsRoles && (
        <SettingRow label="Roles added later" note={laterNote(profile.include_new, app, project, templateId)}>
          <Segmented<"out" | "in">
            label="Roles added later"
            size="sm"
            value={profile.include_new ? "in" : "out"}
            onChange={(v) => edit({ ...profile, include_new: v === "in" })}
            options={[
              { value: "out", label: "Left out" },
              { value: "in", label: "Included" },
            ]}
          />
        </SettingRow>
      )}

      {/* Claims (§4) */}
      <div className="row-divider px-[22px] pb-1 pt-4">
        <span className="type-label">Claims</span>
      </div>
      {profile.claims.length === 0 && (
        <p className="px-[22px] pb-2 text-[13.5px] text-muted">Sends nothing of its own. Zitadel&rsquo;s standard claims do the work.</p>
      )}
      {profile.claims.map((c, i) => (
        <ClaimEditor
          key={i}
          claim={c}
          index={i}
          app={app}
          advanced={advanced}
          token={token}
          person={person}
          sameObject={profile.claims.slice(0, i).some((o) => o.path.length > 1 && o.path[0] === c.path[0])}
          leftOut={preview?.left_out.includes(i) ?? false}
          error={problems[i]?.message ?? (previewRowError?.row === i ? previewRowError.message : undefined)}
          roles={shape.project_roles}
          onChange={(next) => editClaim(i, next)}
          onRemove={() => edit({ ...profile, claims: profile.claims.filter((_, j) => j !== i) })}
          onPickRoles={() => setPicking(i)}
        />
      ))}
      <div className="px-3 pb-3">
        <Button
          variant="ghost"
          size="sm"
          onClick={() => edit({ ...profile, claims: [...profile.claims, { path: [""], kind: "attribute", source: "email" }] })}
        >
          Add a claim
        </Button>
      </div>

      {/* Lands in (§5) */}
      <SettingRow
        label="Lands in"
        value={LANDS_LABEL[profile.lands]}
        note={landsNote}
        bad={checkOneFailsReads && reads ? `Doesn't reach ${READS_LABEL[reads]}. Check 1 in the preview says why.` : undefined}
      >
        {advanced && (
          <span className="flex flex-col items-end gap-1">
            <Segmented<Lands>
              label="Lands in"
              size="sm"
              value={profile.lands}
              onChange={(lands) => edit({ ...profile, lands })}
              options={[
                { value: "both", label: "Both" },
                { value: "access_token", label: "Access token" },
                { value: "userinfo", label: "ID token and userinfo" },
              ]}
            />
            <span className="max-w-[340px] text-right text-[12.5px] leading-[1.45] text-faint">
              ID token and userinfo is one choice: Syndra answers userinfo, and the ID token carries it only while User Info inside ID Token is on.
            </span>
          </span>
        )}
      </SettingRow>

      {/* Save, with its before-line (A-11 §4) */}
      <div className="row-divider flex flex-wrap items-center gap-x-3.5 gap-y-2.5 px-[22px] pb-5 pt-4">
        <span className="flex flex-none gap-2 whitespace-nowrap">
          <Button variant="accent" disabled={!dirty || firstProblem >= 0} onClick={onSave}>
            Save
          </Button>
          {draft?.committed && (
            <Button variant="ghost" onClick={() => setDraft(null)}>
              Discard the draft
            </Button>
          )}
        </span>
        <span className={`flex min-w-0 flex-[1_1_220px] items-start gap-2 text-[13px] leading-[1.5] ${beforeLine.warn ? "text-warn-text" : "text-faint"}`}>
          {beforeLine.warn && <StatusDot tone="warn" className="mt-[7px]" />}
          <span>
            <Ticks text={beforeLine.text} />
          </span>
        </span>
      </div>

      {picking !== null && (
        <PickRoles
          projectId={shape.application.project_id}
          claim={profile.claims[picking]}
          onClose={() => setPicking(null)}
          onDone={(roles) => {
            editClaim(picking, { ...profile.claims[picking], roles });
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

function SettingRow({
  label,
  value,
  note,
  bad,
  children,
}: {
  label: string;
  value?: string;
  note?: string;
  bad?: string;
  children?: React.ReactNode;
}) {
  return (
    <div className="row-divider flex flex-wrap items-start gap-x-[18px] gap-y-2 px-[22px] py-3.5">
      <span className="w-[170px] flex-none pt-0.5 text-[14px] font-semibold">{label}</span>
      <span className="flex min-w-0 flex-[1_1_160px] flex-col gap-0.5">
        {value && <span className="text-[14.5px]">{value}</span>}
        {note && <span className={`${value ? "text-[13px] text-faint" : "text-[13.5px] text-muted"} leading-[1.5] [text-wrap:pretty]`}>{note}</span>}
        {bad && <span className="text-[13px] leading-[1.5] text-danger-text">{bad}</span>}
      </span>
      {children && <span className="ml-auto flex justify-end">{children}</span>}
    </div>
  );
}

// ---------------------------------------------------------------------------
// One claim row (§4).

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

type FixedType = "text" | "number" | "bool" | "list";
const typeOf = (v: unknown): FixedType =>
  typeof v === "number" ? "number" : typeof v === "boolean" ? "bool" : Array.isArray(v) ? "list" : "text";

function ClaimEditor({
  claim: c,
  index,
  app,
  advanced,
  token,
  person,
  sameObject,
  leftOut,
  error,
  roles,
  onChange,
  onRemove,
  onPickRoles,
}: {
  claim: Claim;
  index: number;
  app: string;
  advanced: boolean;
  token: Record<string, unknown> | null;
  person: string;
  sameObject: boolean;
  leftOut: boolean;
  error?: string;
  roles: AppClaimShape["project_roles"];
  onChange: (c: Claim) => void;
  onRemove: () => void;
  onPickRoles: () => void;
}) {
  const leaf = c.path[c.path.length - 1] ?? "";
  const setLeaf = (name: string) => onChange({ ...c, path: [...c.path.slice(0, -1), name] });
  const nested = c.path.length > 1;
  // "More claims" waits behind Advanced, unless this row already is one.
  const more = advanced || isAdvancedRow(c);
  const text = c.kind === "roles" && (c.format ?? "array") !== "array";
  const name = leaf || "this claim";

  let second: React.ReactNode = null;
  if (c.kind === "static") {
    const t = typeOf(c.value);
    second = (
      <>
        <Segmented<FixedType>
          label="Type"
          size="sm"
          value={t}
          onChange={(next) =>
            onChange({ ...c, value: next === "number" ? 0 : next === "bool" ? true : next === "list" ? [] : "" })
          }
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
            onChange={(v) => onChange({ ...c, value: v === "yes" })}
            options={[
              { value: "yes", label: "Yes" },
              { value: "no", label: "No" },
            ]}
          />
        ) : (
          <Input
            aria-label="Value for everyone"
            type={t === "number" ? "number" : "text"}
            value={t === "list" ? ((c.value as unknown[]) ?? []).join(", ") : String(c.value ?? "")}
            placeholder={t === "list" ? "makers, staff" : "makerspace"}
            onChange={(e) => {
              const raw = e.target.value;
              onChange({
                ...c,
                value:
                  t === "number"
                    ? Number(raw)
                    : t === "list"
                      ? raw.split(",").map((s) => s.trim()).filter(Boolean)
                      : raw,
              });
            }}
            className="w-auto min-w-[160px] flex-1 py-2 text-[14px]"
          />
        )}
      </>
    );
  } else if (c.kind === "if_role") {
    second = (
      <>
        If they hold
        {(c.roles ?? []).map((k) => (
          <Chip key={k} className="py-1 text-[13px]">
            {roles.find((r) => r.key === k)?.label ?? k}
          </Chip>
        ))}
        <Button variant="ghost" size="sm" onClick={onPickRoles}>
          Change roles
        </Button>
        <span>
          Otherwise <Mono>false</Mono>.
        </span>
      </>
    );
  } else if (c.kind === "roles" && (advanced || text)) {
    second = (
      <>
        Sent as
        <Segmented<"array" | "csv" | "space_delimited">
          label="Sent as"
          size="sm"
          value={c.format ?? "array"}
          onChange={(format) => onChange({ ...c, format })}
          options={[
            { value: "array", label: "A list" },
            { value: "csv", label: "Text, with commas" },
            { value: "space_delimited", label: "Text, with spaces" },
          ]}
        />
      </>
    );
  }

  return (
    <ClaimRow
      inside={
        nested ? (
          <Input
            aria-label="Inside"
            value={c.path.slice(0, -1).join(" › ")}
            readOnly={c.path.length > 2 || !advanced}
            spellCheck={false}
            onChange={(e) => onChange({ ...c, path: [e.target.value, leaf] })}
            className="py-2 font-mono text-[13.5px]"
          />
        ) : undefined
      }
      name={
        <Input
          aria-label="Claim name"
          value={leaf}
          spellCheck={false}
          placeholder="claim_name"
          onChange={(e) => setLeaf(e.target.value)}
          className="py-2 font-mono text-[13.5px]"
        />
      }
      carries={
        <Select aria-label="Carries" value={carriesOf(c)} onChange={(e) => onChange(withCarries(c, e.target.value))} className="py-2 text-[14px]">
          <optgroup label="From the person">
            <option value="roles">Roles {app} sees</option>
            {(more ? Object.keys(ATTRIBUTE_LABEL) : BASIC_ATTRIBUTES).map((a) => (
              <option key={a} value={`attr:${a}`}>
                {ATTRIBUTE_LABEL[a]}
              </option>
            ))}
          </optgroup>
          <optgroup label="The same for everyone">
            <option value="static">A fixed value</option>
          </optgroup>
          {more && (
            <optgroup label="More claims">
              <option value="if_role">{KIND_LABEL.if_role}</option>
              <option value="bundles">{KIND_LABEL.bundles}</option>
              <option value="role_ends">{KIND_LABEL.role_ends}</option>
            </optgroup>
          )}
        </Select>
      }
      onRemove={onRemove}
      removeLabel={`Remove ${name}`}
      second={second}
      sentAs={
        leftOut
          ? "Left out of the draft until it has a role."
          : token && leaf
            ? sentAs(c, token, person, { app, sameObject })
            : undefined
      }
      warn={
        text
          ? `No app Syndra has a template for reads roles as text. NetBird would make one group called \`netbird-staff,netbird-admins\`. Use this only if ${app}'s own code splits it.`
          : undefined
      }
      error={error}
      footer={
        advanced && !nested && index >= 0 ? (
          <Button variant="ghost" size="sm" onClick={() => onChange({ ...c, path: ["", ...c.path] })}>
            Put it inside another claim
          </Button>
        ) : undefined
      }
    />
  );
}

/** The role picker in single-project mode: the roles an if-role row checks. */
function PickRoles({
  projectId,
  claim,
  onClose,
  onDone,
}: {
  projectId: string;
  claim: Claim;
  onClose: () => void;
  onDone: (roles: string[]) => void;
}) {
  const [chosen, setChosen] = useState(() => new Set((claim.roles ?? []).map((k) => `${projectId}:${k}`)));
  return (
    <Modal open onClose={onClose} size="md">
      <ModalHeader title="Which roles make it true" lede={`\`${claim.path.join(" › ")}\` is true for anyone who holds any of these, and false otherwise.`.replaceAll("`", "")} />
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
        <Button variant="accent" onClick={() => onDone(Array.from(chosen).map((id) => id.slice(projectId.length + 1)))}>
          {chosen.size === 1 ? "Use this role" : `Use these ${chosen.size} roles`}
        </Button>
      </ModalFooter>
    </Modal>
  );
}
