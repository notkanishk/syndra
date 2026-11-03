"use client";

import { useState, type ReactNode } from "react";

import { ActionOutcome } from "@/components/ui/ActionOutcome";
import { StatusDot } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Checkbox } from "@/components/ui/Choice";
import { FieldLabel, Textarea } from "@/components/ui/Input";
import { Modal, ModalFooter, ModalHeader } from "@/components/ui/Modal";
import { Ticks } from "@/components/ui/TokenBlock";
import { changeLine, consequence, TEMPLATE_SHEET, understandLine, type SayContext } from "@/lib/claims";
import { outcomeFromError, type ActionOutcome as Outcome } from "@/lib/outcome";
import type { AppClaimShape, Change, ClaimTemplate, Preview, SaveRequest, SaveResult } from "@/lib/queries/useClaimShape";

import { ShapeChooser } from "./ShapeChooser";
import type { ShapeDraft } from "./shape-draft";

const GLYPH: Record<Change["kind"], string> = {
  add: "+",
  remove: "−",
  rename: "~",
  retype: "~",
  move: "~",
  lands: "~",
  include_new: "~",
};

function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="flex flex-col gap-2">
      <span className="type-label">{title}</span>
      {children}
    </section>
  );
}

function Dot({ text, warn = false }: { text: string; warn?: boolean }) {
  return (
    <div className="flex items-start gap-2.5 text-[14px] leading-[1.55]">
      <span aria-hidden className="flex w-1.5 flex-none pt-[9px]">
        <StatusDot tone={warn ? "warn" : "neutral"} />
      </span>
      <span className="[text-wrap:pretty]">
        <Ticks text={text} />
      </span>
    </div>
  );
}

function Glyph({ change, ctx }: { change: Change; ctx: SayContext }) {
  return (
    <div className="flex items-start gap-2.5 text-[14.5px] leading-[1.5]">
      <span aria-hidden className="w-[1.5ch] flex-none font-mono text-[13px] font-bold">
        {GLYPH[change.kind]}
      </span>
      <span className="[text-wrap:pretty]">
        <Ticks text={changeLine(change, ctx)} />
      </span>
    </div>
  );
}

/**
 * The template sheet (A-14 §2): what it sends, where it lands, what the app
 * does with it, what no claim can do, and the exact settings. On an app that
 * started from it, what changed since, and Start again (a draft).
 */
export function TemplateSheet({
  template,
  shape,
  project,
  onClose,
  onStart,
}: {
  template: ClaimTemplate;
  shape: AppClaimShape;
  project: string;
  onClose: () => void;
  onStart: () => void;
}) {
  const words = TEMPLATE_SHEET[template.id];
  const app = shape.application.name;
  const mine = shape.template?.id === template.id ? shape.template : null;
  const ctx: SayContext = { app, roles: shape.project_roles };
  const verb = shape.source === "app" ? "Start again from the template" : `Start from ${words?.short ?? template.name}`;
  return (
    <Modal open onClose={onClose} size="md">
      <ModalHeader title={`The ${template.name} template`} lede={words?.lede} />
      <div className="flex flex-col gap-[18px] px-6 pb-2">
        {words?.sections.map((s) => (
          <Section key={s.title} title={s.title}>
            {s.lines.map((l) => (typeof l === "string" ? <Dot key={l} text={l} /> : <Dot key={l.warn} text={l.warn} warn />))}
          </Section>
        ))}
        {mine && (
          <Section title="Changed since you started from it">
            {mine.changes.length === 0 ? (
              <p className="text-[14px] text-muted">Nothing yet. Your role choices and words are yours, so they don&rsquo;t count as changes.</p>
            ) : (
              mine.changes.map((c, i) => <Glyph key={i} change={c} ctx={ctx} />)
            )}
          </Section>
        )}
      </div>
      <ModalFooter
        note={
          shape.source === "app"
            ? mine && mine.changes.length === 0
              ? "A template is copied in. Syndra never changes your shape when a template changes."
              : "Starting again makes a draft of the template. Nothing changes until you save."
            : `Makes a draft. ${app} keeps ${project}'s default until you save.`
        }
      >
        <Button onClick={onClose}>Close</Button>
        {!(mine && mine.changes.length === 0) && (
          <Button variant={shape.source === "app" ? "outline" : "accent"} onClick={onStart}>
            {verb}
          </Button>
        )}
      </ModalFooter>
    </Modal>
  );
}

/** Copy another app's shape, later (A-14 §8, frame copy-later): the same chooser in a sheet. */
export function CopySheet({
  shape,
  templates,
  draft,
  setDraft,
  project,
  onClose,
}: {
  shape: AppClaimShape;
  templates: ClaimTemplate[];
  draft: ShapeDraft | null;
  setDraft: (d: ShapeDraft | null) => void;
  project: string;
  /** `copied`: the draft was committed to the card; otherwise it was cancelled. */
  onClose: (copied: boolean) => void;
}) {
  const app = shape.application.name;
  return (
    <Modal open onClose={() => onClose(false)} size="md">
      <ModalHeader title="Copy another app's shape" lede={`Makes a draft of another app's shape on ${app}. Nothing changes until you save.`} />
      <ShapeChooser
        mode="copy"
        shape={shape}
        templates={templates}
        draft={draft}
        setDraft={setDraft}
        project={project}
        footer={(verb, note, start) => (
          <ModalFooter note={note}>
            <Button onClick={() => onClose(false)}>Cancel</Button>
            <Button
              variant="accent"
              disabled={!start}
              onClick={() => {
                start?.();
                onClose(true);
              }}
            >
              {verb}
            </Button>
          </ModalFooter>
        )}
      />
    </Modal>
  );
}

/**
 * The save summary (A-11 §4, A-14 §7): what changes, what the app will do,
 * and, for a change the app will act on, an I-understand tick before Save.
 * The tick is what sends `acknowledge_breaking`; the backend refuses without it.
 */
export function SaveSummary({
  shape,
  draft,
  preview,
  ctx,
  save,
  onClose,
}: {
  shape: AppClaimShape;
  draft: ShapeDraft;
  preview?: Preview;
  ctx: SayContext;
  save: (body: SaveRequest) => Promise<SaveResult>;
  onClose: (saved: boolean) => void;
}) {
  const app = shape.application.name;
  const changes = preview?.changes ?? [];
  const breaking = preview?.breaking ?? [];
  const [understood, setUnderstood] = useState(false);
  const [note, setNote] = useState("");
  const [busy, setBusy] = useState(false);
  const [saved, setSaved] = useState<SaveResult | null>(null);
  const [outcome, setOutcome] = useState<Outcome | null>(null);
  const next = (shape.version?.version ?? 0) + 1;
  const reach = preview?.reach;
  const moved = breaking.reduce((n, c) => n + (c.role ? (reach?.by_role[c.role] ?? 0) : 0), 0);

  const submit = async () => {
    setBusy(true);
    setOutcome(null);
    try {
      const res = await save({
        profile: draft.profile,
        template_id: draft.templateId,
        template_version: draft.templateVersion,
        copied_from: draft.copiedFrom,
        note: note.trim() || undefined,
        acknowledge_breaking: breaking.length > 0 && understood,
      });
      setSaved(res);
    } catch (e) {
      setOutcome(outcomeFromError(e));
    } finally {
      setBusy(false);
    }
  };

  return (
    <Modal open onClose={() => onClose(Boolean(saved))} busy={busy} size="md">
      <ModalHeader title={`Save what ${app} is told`} lede={saved ? undefined : `${app}'s claims, kept in History as version ${next}.`} />
      {saved ? (
        <div role="status" className="lift-in flex flex-col gap-2 px-6 pb-2">
          <span className="flex items-center gap-2.5">
            <StatusDot tone="healthy" bloom />
            <span className="font-display text-[22px] font-semibold">Saved</span>
          </span>
          <p className="text-[15px] leading-[1.55]">
            {app}&rsquo;s claims are now version {saved.version.version}.
          </p>
          <p className="text-[13px] leading-[1.5] text-faint">
            <Ticks
              text={
                saved.breaking.length && ctx.templateId === "netbird"
                  ? `${moved ? `${moved} people` : "People"} move as they sign in. Point NetBird's policies at the new group once it appears there; a group made by hand isn't joined.`
                  : `It reaches ${app} at each person's next sign-in.`
              }
            />
          </p>
        </div>
      ) : (
        <div className="flex flex-col gap-[18px] px-6 pb-2">
          <Section title="What changes">
            {changes.length === 0 ? (
              <p className="text-[14px] text-muted">The shape is the same as what {app} gets now; this saves it as {app}&rsquo;s own.</p>
            ) : (
              changes.map((c, i) => <Glyph key={i} change={c} ctx={ctx} />)
            )}
          </Section>
          {breaking.length > 0 && (
            <Section title={`What ${app} will do`}>
              {breaking.map((c, i) => (
                <Dot key={i} text={consequence(c, ctx)} warn />
              ))}
            </Section>
          )}
          {breaking.length > 0 && (
            <label className="flex items-start gap-2.5 text-[14.5px] leading-[1.5]">
              <Checkbox className="mt-1" checked={understood} onChange={(e) => setUnderstood(e.target.checked)} />
              <span>
                <Ticks text={understandLine(breaking, ctx)} />
              </span>
            </label>
          )}
          <div>
            <FieldLabel htmlFor="shape-note">What changed, for History</FieldLabel>
            <Textarea id="shape-note" value={note} onChange={(e) => setNote(e.target.value)} placeholder="Why this change" />
          </div>
          {outcome && <ActionOutcome outcome={outcome} />}
        </div>
      )}
      <ModalFooter
        note={
          saved
            ? undefined
            : reach
              ? `Reaches ${app} and ${reach.people} ${reach.people === 1 ? "person" : "people"} at their next sign-in. Anyone signed in now keeps the old claims until then.`
              : `Reaches ${app} at each person's next sign-in. Anyone signed in now keeps the old claims until then.`
        }
      >
        {saved ? (
          <Button onClick={() => onClose(true)}>Close</Button>
        ) : (
          <>
            <Button disabled={busy} onClick={() => onClose(false)}>
              Cancel
            </Button>
            <Button variant="accent" isPending={busy} disabled={breaking.length > 0 && !understood} onClick={submit}>
              Save
            </Button>
          </>
        )}
      </ModalFooter>
    </Modal>
  );
}
