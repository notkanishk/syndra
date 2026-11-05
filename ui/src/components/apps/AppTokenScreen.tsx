"use client";

import { useQuery } from "@tanstack/react-query";
import { useEffect, useRef, useState } from "react";

import { AppSignInStrip } from "@/components/apps/AppSignInStrip";
import { ProjectName } from "@/components/names";
import { SignInLookup } from "@/components/people/SignInLookup";
import { ErrorState, RowSkeleton, SkeletonAfterDelay } from "@/components/states";
import { ReadGate } from "@/components/states/ReadGate";
import { ArrowLink } from "@/components/ui/ArrowLink";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { CopyableValue } from "@/components/ui/CopyableValue";
import { BELOW_HEADER, PageHeader } from "@/components/ui/PageHeader";
import { ApiError, request } from "@/lib/api-client";
import { rowProblems, type SayContext } from "@/lib/claims";
import { keepDraft, keepPerson, readKeptDraft, readKeptPerson } from "@/lib/claims-memory";
import { describeFailure } from "@/lib/outcome";
import { useCrumb } from "@/lib/page-crumb";
import { useSeededApp } from "@/lib/queries/seed";
import { useCatalogUsers } from "@/lib/queries/useCatalogUsers";
import {
  useAppClaimShape,
  useClaimPreview,
  useClaimTemplates,
  useDraftFrom,
  useSaveAppClaimShape,
  type Profile,
} from "@/lib/queries/useClaimShape";
import { useNameResolver } from "@/lib/queries/useNameResolver";
import { useAppSignIns } from "@/lib/queries/useSignIns";
import { appSignInMeta } from "@/lib/sign-in-line";
import { useIsAdvanced, useUiView } from "@/lib/ui-view";

import type { CheckContext } from "./claim-words";
import { ClaimHistory } from "./ClaimHistory";
import { ClaimPreview } from "./ClaimPreview";
import { DebugMode } from "./DebugMode";
import { ToolRow } from "./ToolRow";
import { appTemplateId, ShapeCard } from "./ShapeCard";
import { draftFrom, type ShapeDraft } from "./shape-draft";
import { CopySheet, SaveSummary, TemplateSheet } from "./ShapeSheets";

/**
 * An app, and what it's told (A-14, laid out by A-15). One job: make sure this
 * app is told the right thing about people. The page opens on the answer: what
 * it gets, in a few lines, beside whether it works, in one. Every edit is a
 * draft the preview runs at once, through the same shaper the token path uses,
 * and nothing changes until Save. Under them, one quiet card of tools.
 */
export function AppTokenScreen({ applicationId }: { applicationId: string }) {
  const shape = useAppClaimShape(applicationId);
  // The name arrives with you (A-19 P1): the header paints from the Apps list.
  const seed = useSeededApp(applicationId);
  useCrumb(seed?.name);
  // The header is the row's own: on the first frame, outside the skeleton delay.
  if (shape.isLoading && seed) {
    return (
      <div className="flex flex-col gap-4">
        <PageHeader
          title={seed.name}
          meta={
            <span className="flex flex-wrap items-center gap-x-2 gap-y-1 text-[14px] text-faint">
              {kindOf(seed.consumer)} · roles from{" "}
              <ArrowLink href={`/projects/${seed.project_id}`}>
                <ProjectName id={seed.project_id} />
              </ArrowLink>
            </span>
          }
        />
        <SkeletonAfterDelay>
          <Card>
            <RowSkeleton rows={4} avatar={false} label="Loading app" />
          </Card>
        </SkeletonAfterDelay>
      </div>
    );
  }
  return (
    <ReadGate
      loading={shape.isLoading}
      skeleton={
        // The header's seat, so the card doesn't jump down when the name lands.
        <div className="flex flex-col gap-4">
          <div aria-hidden className="h-[58px]" style={BELOW_HEADER} />
          <Card>
            <RowSkeleton rows={4} avatar={false} label="Loading app" />
          </Card>
        </div>
      }
    >
      {shape.error ? (
        <ErrorState
          title={shape.error instanceof ApiError && shape.error.status === 404 ? "That app doesn't exist." : "Couldn't load this app."}
          error={shape.error instanceof ApiError && shape.error.status === 404 ? new Error("It may have been removed in Zitadel.") : shape.error}
          onRetry={() => shape.refetch()}
        />
      ) : shape.data ? (
        <AppClaims key={applicationId} applicationId={applicationId} />
      ) : null}
    </ReadGate>
  );
}

function useZitadelDomain(): string | undefined {
  const health = useQuery({
    queryKey: ["zitadel", "health"],
    queryFn: () => request<{ domain?: string }>("/zitadel/health", { preserveErrorBody: true }),
    staleTime: Infinity,
  });
  return health.data?.domain;
}

function AppClaims({ applicationId }: { applicationId: string }) {
  const shape = useAppClaimShape(applicationId).data!;
  const templates = useClaimTemplates().data ?? [];
  const users = useCatalogUsers();
  const { selfId } = useUiView();
  const advanced = useIsAdvanced();
  const resolver = useNameResolver();
  const domain = useZitadelDomain();
  const save = useSaveAppClaimShape(applicationId);
  const make = useDraftFrom(applicationId);
  const signIns = useAppSignIns(applicationId);
  const signInMeta = appSignInMeta(signIns.data);
  const app = shape.application.name;
  const projectId = shape.application.project_id;
  const project = resolver.resolveProject(projectId).value?.name ?? "this project";
  useCrumb(app);

  // "Preview for" remembers its last person for this app, this tab (A-19 P10.6).
  const [personId, setPersonId] = useState(() => readKeptPerson(applicationId));
  useEffect(() => {
    if (users.data.length === 0) return;
    // Someone no longer listed is not remembered: back to the usual first choice.
    if (personId && users.data.some((u) => u.id === personId)) return;
    setPersonId(users.data.some((u) => u.id === selfId) ? selfId! : users.data[0].id);
  }, [users.data, selfId, personId]);
  useEffect(() => keepPerson(applicationId, personId), [applicationId, personId]);
  const person = users.data.find((u) => u.id === personId)?.name ?? "this person";

  // A draft survives leaving the page and coming back, until it is saved or discarded (A-19 P10.6).
  const [restored] = useState(() => readKeptDraft(applicationId));
  const [draft, setDraft] = useState<ShapeDraft | null>(restored?.draft ?? null);
  useEffect(() => {
    // The restored draft is already kept, with its own time; only a change after it is written again.
    if (draft !== restored?.draft) keepDraft(applicationId, draft);
  }, [applicationId, draft, restored]);
  const [sheet, setSheet] = useState<{ kind: "template"; id: string } | { kind: "copy" } | { kind: "save" } | null>(null);
  const [lookingUp, setLookingUp] = useState(false);
  const beforeCopy = useRef<ShapeDraft | null>(null);

  // A draft with a row that can't be shaped (unnamed, repeated) keeps the
  // last draft that could on the preview, rather than flashing an error.
  const problems = draft ? rowProblems(draft.profile, { project, missing: draft.missing }) : [];
  const lastGood = useRef<Profile | null>(null);
  const shapeable = draft && !problems.some((p) => p?.blocksPreview);
  if (shapeable) lastGood.current = draft.profile;
  const previewed = draft ? (shapeable ? draft.profile : lastGood.current) : null;
  const preview = useClaimPreview(applicationId, personId ? [personId] : [], previewed, draft?.templateId);

  const one = preview.data?.people[0];
  const shown = one?.draft ?? one?.now;
  const token = (shown?.would_receive ?? shown?.token) ?? null;
  const c1 = shown?.checks[0];
  const reads = String(c1?.facts?.reads ?? "");
  const checkOneFailsReads = c1?.state === "fail" && Boolean(reads) && !(c1.facts.reached as Record<string, boolean> | undefined)?.[reads];
  const rowError =
    preview.error instanceof ApiError && preview.error.code === "VALIDATION_FAILED" && preview.error.details?.row
      ? { row: Number(preview.error.details.row), message: describeFailure(preview.error) }
      : undefined;

  const consoleBase = domain ? `https://${domain}/ui/console/projects/${projectId}` : undefined;
  const ctx: CheckContext = {
    app,
    project,
    person,
    roles: shape.project_roles,
    profile: previewed ?? shape.profile,
    templates,
    links: consoleBase ? { project: consoleBase, app: `${consoleBase}/apps/${applicationId}` } : undefined,
    draft:
      draft && previewed
        ? {
            origin: draft.committed || draft.origin !== "edit" ? draft.origin : "edit",
            name: draft.origin === "copy" ? draft.copiedName : templates.find((t) => t.id === draft.templateId)?.name,
            leftOut: (preview.data?.left_out ?? []).map((i) => previewed.claims[i]?.path.join(" › ")).filter(Boolean),
          }
        : undefined,
  };
  const say: SayContext = { app, roles: shape.project_roles, reach: preview.data?.reach, templateId: appTemplateId(shape, draft) };

  const tpl = sheet?.kind === "template" ? templates.find((t) => t.id === sheet.id) : undefined;

  return (
    <div className="flex flex-col gap-4">
      <PageHeader
        title={app}
        meta={
          <span className="flex flex-wrap items-center gap-x-2 gap-y-1 text-[14px] text-faint">
            {kindOf(shape.application.consumer)} · roles from{" "}
            <ArrowLink href={`/projects/${projectId}`}>
              <ProjectName id={projectId} />
            </ArrowLink>
            {signInMeta && (
              <span className="tint-in">
                <span aria-hidden>·</span> {signInMeta}
              </span>
            )}
          </span>
        }
        actions={
          <Button variant="outline" onClick={() => setLookingUp(true)}>
            Look up sign-ins
          </Button>
        }
      />
      {lookingUp && <SignInLookup appId={applicationId} name={app} onClose={() => setLookingUp(false)} />}

      {/* What the app is told | Preview: two halves; on a phone the shape comes first (#90). */}
      <div className="flex flex-wrap items-start gap-4">
        <div className="flex min-w-0 flex-[1_1_460px]">
          <ShapeCard
            shape={shape}
            draft={draft}
            setDraft={setDraft}
            restoredAt={restored && draft === restored.draft ? restored.at : undefined}
            token={token}
            person={person}
            preview={preview.data}
            previewRowError={rowError}
            unreached={checkOneFailsReads ? reads : undefined}
            templates={templates}
            advanced={advanced}
            projectName={project}
            onSave={() => setSheet({ kind: "save" })}
            onOpenTemplate={(id) => setSheet({ kind: "template", id })}
            onOpenCopy={() => {
              beforeCopy.current = draft;
              setSheet({ kind: "copy" });
            }}
          />
        </div>
        {/* The preview stays in view while the shape scrolls, so every edit is seen beside its effect. */}
        <div className="flex min-w-0 flex-[1_1_440px] desktop:sticky desktop:top-5">
          <ClaimPreview
            preview={preview.data}
            isLoading={preview.isLoading}
            error={preview.error}
            onRetry={() => preview.refetch()}
            people={users.data}
            personId={personId}
            onPerson={setPersonId}
            ctx={ctx}
          />
        </div>
      </div>

      {/* Quiet tools (A-15 §6): one card, a settings row each; nothing happens without a named button. */}
      <Card className="[&>.row-divider:first-child]:border-t-0">
        {shape.client_id && (
          <ToolRow label="Client ID">
            <CopyableValue value={shape.client_id} label="Client ID" />
          </ToolRow>
        )}
        {signIns.data?.receiving && signIns.data.last_at && <AppSignInStrip usage={signIns.data} />}
        <DebugMode appId={applicationId} ctx={{ ...ctx, profile: shape.profile, draft: undefined }} />
        <ClaimHistory appId={applicationId} ctx={say} project={project} templates={templates} />
      </Card>

      {tpl && (
        <TemplateSheet
          template={tpl}
          shape={shape}
          project={project}
          onClose={() => setSheet(null)}
          onStart={async () => {
            setSheet(null);
            try {
              const d = await make.mutateAsync({ template_id: tpl.id });
              setDraft({ ...draftFrom(d, "template"), committed: true });
            } catch {
              // The chooser says why when it can't make one; here the sheet just closes.
            }
          }}
        />
      )}
      {sheet?.kind === "copy" && (
        <CopySheet
          shape={shape}
          templates={templates}
          draft={draft}
          setDraft={setDraft}
          project={project}
          onClose={(copied) => {
            // Cancelled: the chooser's draft goes with the sheet, and any draft from before comes back.
            if (!copied) setDraft(beforeCopy.current);
            setSheet(null);
          }}
        />
      )}
      {sheet?.kind === "save" && draft && (
        <SaveSummary
          shape={shape}
          draft={draft}
          preview={preview.data}
          ctx={say}
          save={(body) => save.mutateAsync(body)}
          onClose={(saved) => {
            setSheet(null);
            if (saved) setDraft(null);
          }}
        />
      )}
    </div>
  );
}

/** The protocol, the word an operator debugging a token already has in mind. */
function kindOf(consumer: string): string {
  const raw = (consumer || "").toLowerCase();
  if (raw.includes("saml")) return "SAML";
  if (raw.includes("oidc") || raw.includes("oauth")) return "OIDC";
  if (raw.includes("api")) return "API";
  return consumer || "App";
}
