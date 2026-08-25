"use client";

import { useEffect, useMemo, useState } from "react";
import type { UseQueryResult } from "@tanstack/react-query";

import { ActionOutcome } from "@/components/ui/ActionOutcome";
import { Mono } from "@/components/ui/Badge";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { FieldLabel, Input } from "@/components/ui/Input";
import { outcomeFromError, type ActionOutcome as Outcome } from "@/lib/outcome";
import { Segmented, Select } from "@/components/ui/Select";
import { RowSkeleton, ErrorState } from "@/components/states";
import {
  useClaimVocabulary,
  useDeleteAppClaimOverride,
  useSaveAppClaimOverride,
  useSaveProjectClaimProfile,
  type ClaimFormat,
  type ClaimProfile,
  type ClaimProfileInput,
  type ClaimShape,
} from "@/lib/queries/useClaimShape";

/**
 * Token format — the real thing, not a display of one.
 *
 * What is edited here is the claim profile the data plane applies on every
 * token issue: the claim key carrying the roles, the shape of that value, any
 * profile attributes riding along, and any constants. Saving invalidates the
 * data plane's cached shape, so the next token carries the change.
 *
 * Two facts this panel has to state rather than hide:
 *
 *   1. Changing the PROJECT default changes what every app reading that
 *      project receives.
 *   2. Zitadel's function trigger does not say which application a token is
 *      for, so a token carries the project default AND every override on that
 *      project. Each app reads its own key; keys must therefore be unique.
 */
export function TokenFormatEditor({
  projectId,
  applicationId,
  applicationName,
  shape,
  siblingCount,
  onDirtyChange,
}: {
  projectId: string;
  applicationId: string;
  applicationName: string;
  shape: UseQueryResult<ClaimShape>;
  siblingCount: number;
  /** True while the form holds edits the preview beside it has not seen. */
  onDirtyChange?: (dirty: boolean) => void;
}) {
  const override = shape.data?.overrides.find((entry) => entry.application_id === applicationId);
  const [scope, setScope] = useState<"project" | "app">("project");
  // Hooks stay unconditional: the mutation is created here, not inside the
  // branch that renders the project form.
  const saveProjectDefault = useSaveProjectClaimProfile(projectId);

  // An app that already has an override opens on its own tab: that is the
  // thing the operator came to look at.
  useEffect(() => {
    if (override) setScope("app");
  }, [override]);

  if (shape.isLoading) {
    return (
      <Card>
        <RowSkeleton rows={4} avatar={false} label="Loading token format" />
      </Card>
    );
  }
  if (shape.error || !shape.data) {
    return (
      <ErrorState
        title="Couldn't load the token format."
        error={shape.error}
        onRetry={() => shape.refetch()}
      />
    );
  }

  return (
    <Card className="h-full">
      <div className="px-[22px] pb-4 pt-5">
        <h2 className="type-card-title">Token format</h2>
        <p className="mt-1 text-[14px] leading-[1.5] text-faint">
          What this app receives, and in what shape. Saving changes real tokens — the next one
          issued carries it.
        </p>
      </div>

      <div className="flex flex-wrap items-center gap-2.5 px-[22px] pb-4">
        <Segmented<"project" | "app">
          label="What to edit"
          size="sm"
          value={scope}
          onChange={setScope}
          options={[
            { value: "project", label: "Project default" },
            { value: "app", label: `${applicationName} only` },
          ]}
        />
      </div>

      {scope === "project" ? (
        <ProfileForm
          key="project"
          profile={shape.data.default}
          scopeNote={
            siblingCount > 0
              ? `Changing this changes what all ${siblingCount + 1} apps reading ${shape.data.project_name} receive.`
              : `Changing this changes what every app reading ${shape.data.project_name} receives.`
          }
          onSave={saveProjectDefault.mutateAsync}
          onDirtyChange={onDirtyChange}
        />
      ) : (
        <AppOverrideForm
          key="app"
          onDirtyChange={onDirtyChange}
          projectId={projectId}
          applicationId={applicationId}
          applicationName={applicationName}
          override={override}
          fallback={shape.data.default}
        />
      )}

      <TokenKeyInventory shape={shape.data} applicationId={applicationId} />
    </Card>
  );
}

function AppOverrideForm({
  projectId,
  applicationId,
  applicationName,
  override,
  fallback,
  onDirtyChange,
}: {
  projectId: string;
  applicationId: string;
  applicationName: string;
  override?: ClaimProfile;
  fallback: ClaimProfile;
  onDirtyChange?: (dirty: boolean) => void;
}) {
  const save = useSaveAppClaimOverride(projectId);
  const drop = useDeleteAppClaimOverride(projectId);
  const [overrideOutcome, setOverrideOutcome] = useState<Outcome | null>(null);

  if (!override) {
    return (
      <div className="px-[22px] pb-5">
        <div className="rounded-block border border-line bg-tint-1 px-4 py-4">
          <div className="text-[14.5px] font-semibold">
            {applicationName} uses the project default.
          </div>
          <p className="mt-1.5 max-w-[52ch] text-[13.5px] leading-[1.55] text-muted">
            Give it its own claim key when it needs a different name or shape from its siblings.
            The token will then carry both keys — this one, and the project default the other apps
            read.
          </p>
          <Button
            className="mt-3"
            variant="accentSoft"
            isPending={save.isPending}
            onClick={async () => {
              try {
                await save.mutateAsync({
                  applicationId,
                  claim_name: `${slug(applicationName)}.roles`,
                  format_type: fallback.format_type,
                  attribute_claims: {},
                  static_claims: {},
                });
                setOverrideOutcome({
                  kind: "applied",
                  message: `${applicationName} now has its own claim`,
                  detail: "Its siblings keep the project default.",
                });
              } catch (error) {
                setOverrideOutcome(outcomeFromError(error));
              }
            }}
          >
            Give {applicationName} its own claim
          </Button>

          {overrideOutcome && <ActionOutcome outcome={overrideOutcome} className="mt-3" />}
        </div>
      </div>
    );
  }

  return (
    <ProfileForm
      profile={override}
      scopeNote={`Only ${applicationName} reads this key. Its siblings keep the project default.`}
      onSave={(body) => save.mutateAsync({ applicationId, ...body })}
      onDirtyChange={onDirtyChange}
      onDelete={async () => {
        await drop.mutateAsync(applicationId);
        setOverrideOutcome({
          kind: "applied",
          message: `${applicationName} is back on the project default`,
        });
      }}
    />
  );
}

/**
 * The editor proper: roles claim + format, then a row per extra claim. Each
 * extra claim is a key plus a source — a profile attribute, or a constant.
 */
function ProfileForm({
  profile,
  scopeNote,
  onSave,
  onDelete,
  onDirtyChange,
}: {
  profile: ClaimProfile;
  scopeNote: string;
  onSave: (body: ClaimProfileInput) => Promise<unknown>;
  onDelete?: () => Promise<void>;
  /**
   * Reported upwards so the preview beside this form can admit it is showing
   * the saved shape rather than the one being typed. Only one ProfileForm is
   * mounted at a time — project scope and app override are exclusive — so a
   * single boolean is the whole story.
   */
  onDirtyChange?: (dirty: boolean) => void;
}) {
  const vocabulary = useClaimVocabulary();
  const [claimName, setClaimName] = useState(profile.claim_name);
  const [format, setFormat] = useState<ClaimFormat>(profile.format_type);
  const [extras, setExtras] = useState<ExtraClaim[]>(() => toExtras(profile));
  const [saving, setSaving] = useState(false);
  const [outcome, setOutcome] = useState<Outcome | null>(null);

  const dirty =
    claimName !== profile.claim_name ||
    format !== profile.format_type ||
    JSON.stringify(toExtras(profile)) !== JSON.stringify(extras);

  useEffect(() => {
    onDirtyChange?.(dirty);
    // Unmounting mid-edit — switching scope, leaving the screen — must clear
    // the claim, or the preview keeps apologising for edits that are gone.
    return () => onDirtyChange?.(false);
  }, [dirty, onDirtyChange]);

  async function save() {
    setSaving(true);
    try {
      const { attributes, statics } = fromExtras(extras);
      await onSave({
        claim_name: claimName.trim(),
        format_type: format,
        attribute_claims: attributes,
        static_claims: statics,
      });
      // Never "applied": a claim edit changes the shape of the NEXT token, and
      // every token already issued keeps the shape it was minted with.
      setOutcome({
        kind: "applied",
        message: "Token format saved",
        detail: "The next token this app issues carries it. Tokens already out keep their shape.",
      });
    } catch (error) {
      // The backend rejects duplicate keys and malformed names; surfacing its
      // sentence verbatim is more useful than a generic failure.
      setOutcome(outcomeFromError(error));
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="flex flex-col gap-4 px-[22px] pb-5">
      <p className="text-[13.5px] leading-[1.5] text-faint">{scopeNote}</p>

      <div className="rounded-block bg-tint-1 px-4 py-3.5">
        <FieldLabel htmlFor="claim-name">Roles claim</FieldLabel>
        <div className="flex flex-wrap items-center gap-2.5">
          <Input
            id="claim-name"
            value={claimName}
            spellCheck={false}
            onChange={(event) => setClaimName(event.target.value)}
            className="min-w-[240px] flex-1 font-mono text-[13.5px]"
            placeholder="syndra.laser.roles"
          />
          <Segmented<ClaimFormat>
            label="Claim format"
            size="sm"
            value={format}
            onChange={setFormat}
            options={(vocabulary.data?.formats ?? ["array", "csv", "space_delimited"]).map(
              (value) => ({ value, label: value === "space_delimited" ? "spaced" : value }),
            )}
          />
        </div>
        <p className="mt-2 text-[12.5px] text-faint">
          {format === "array"
            ? 'Sent as ["trained","maintainer"].'
            : format === "csv"
              ? 'Sent as "trained,maintainer".'
              : 'Sent as "trained maintainer".'}
        </p>
      </div>

      <div>
        <div className="mb-2 flex items-center gap-2">
          <span className="type-label">Also send</span>
          <span className="flex-1" />
          <button
            type="button"
            onClick={() =>
              setExtras((prev) => [...prev, { key: "", kind: "attribute", value: "email" }])
            }
            className="text-[13px] font-semibold text-accent-text"
          >
            Add a claim +
          </button>
        </div>

        {extras.length === 0 ? (
          <p className="text-[13.5px] text-faint">
            Just the roles. Add a claim when an app needs the person&rsquo;s email or team without
            asking for another scope.
          </p>
        ) : (
          <div className="flex flex-col gap-2">
            {extras.map((extra, index) => (
              <div key={index} className="flex flex-wrap items-center gap-2">
                <Input
                  value={extra.key}
                  spellCheck={false}
                  aria-label="Claim key"
                  placeholder="syndra.laser.email"
                  onChange={(event) =>
                    setExtras((prev) => replace(prev, index, { ...extra, key: event.target.value }))
                  }
                  className="min-w-[200px] flex-1 font-mono text-[13.5px]"
                />
                <Select
                  aria-label="Claim source"
                  value={extra.kind === "static" ? "__static" : extra.value}
                  onChange={(event) =>
                    setExtras((prev) =>
                      replace(
                        prev,
                        index,
                        event.target.value === "__static"
                          ? { ...extra, kind: "static", value: "" }
                          : { ...extra, kind: "attribute", value: event.target.value },
                      ),
                    )
                  }
                  className="w-[170px]"
                >
                  {(vocabulary.data?.attributes ?? []).map((attribute) => (
                    <option key={attribute} value={attribute}>
                      {attribute}
                    </option>
                  ))}
                  <option value="__static">a fixed value…</option>
                </Select>
                {extra.kind === "static" && (
                  <Input
                    value={extra.value}
                    aria-label="Fixed value"
                    placeholder="makerspace"
                    onChange={(event) =>
                      setExtras((prev) =>
                        replace(prev, index, { ...extra, value: event.target.value }),
                      )
                    }
                    className="w-[150px]"
                  />
                )}
                <Button
                  size="sm"
                  aria-label={`Remove ${extra.key || "claim"}`}
                  onClick={() => setExtras((prev) => prev.filter((_, i) => i !== index))}
                >
                  Remove
                </Button>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Under the control that saved it, and above nothing: an editor that
          reports elsewhere leaves the operator unsure whether the shape on
          screen is the shape that was written. */}
      {outcome && <ActionOutcome outcome={outcome} />}

      <div className="flex flex-wrap items-center gap-2.5">
        <Button variant="accent" disabled={!dirty || !claimName.trim()} isPending={saving} onClick={save}>
          Save token format
        </Button>
        {dirty && (
          <Button
            onClick={() => {
              setClaimName(profile.claim_name);
              setFormat(profile.format_type);
              setExtras(toExtras(profile));
            }}
          >
            Discard
          </Button>
        )}
        {onDelete && (
          <>
            <span className="flex-1" />
            <Button variant="danger" size="sm" onClick={onDelete}>
              Use the project default instead
            </Button>
          </>
        )}
      </div>
    </div>
  );
}

/**
 * Every key the token carries and who put it there.
 *
 * A sibling app's key showing up in your token is not a bug, and an operator
 * should learn that here rather than by decoding a production token and
 * filing one.
 */
function TokenKeyInventory({
  shape,
  applicationId,
}: {
  shape: ClaimShape;
  applicationId: string;
}) {
  const mine = useMemo(
    () =>
      new Set(
        shape.emitted_keys
          .filter((entry) =>
            shape.overrides.some((o) => o.application_id === applicationId)
              ? entry.application_id === applicationId
              : !entry.application_id,
          )
          .map((entry) => entry.key),
      ),
    [shape, applicationId],
  );

  const conflicts = shape.conflicts ?? [];

  return (
    <div className="row-divider px-[22px] py-4">
      <div className="mb-2.5 type-label">A token for this project carries</div>
      <div className="flex flex-col gap-1.5">
        {shape.emitted_keys.map((entry) => (
          <div key={`${entry.key}-${entry.owner_label}`} className="flex items-center gap-2.5">
            <span
              aria-hidden
              className={`h-1.5 w-1.5 flex-none rounded-pill ${
                mine.has(entry.key) ? "bg-accent" : "bg-ink/25"
              }`}
            />
            <Mono className={mine.has(entry.key) ? "text-ink" : "text-muted"}>{entry.key}</Mono>
            <span className="text-[12.5px] text-faint">
              {entry.kind === "roles"
                ? "roles"
                : entry.kind === "attribute"
                  ? entry.source
                  : "fixed value"}
            </span>
            <span className="flex-1" />
            <span className="text-[12.5px] text-faint">
              {mine.has(entry.key) ? "read by this app" : entry.owner_label}
            </span>
          </div>
        ))}
      </div>

      {conflicts.length > 0 && (
        <div className="warn-note mt-3 px-4 py-3 text-[13.5px] text-warn-text">
          {conflicts.map((conflict) => (
            <div key={conflict.claim_key}>
              <Mono>{conflict.claim_key}</Mono> is claimed twice — {conflict.owner} and{" "}
              {conflict.other}. A token holds one value per name, so one of them is reading the
              other&rsquo;s roles.
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

interface ExtraClaim {
  key: string;
  kind: "attribute" | "static";
  value: string;
}

function toExtras(profile: ClaimProfile): ExtraClaim[] {
  const attributes = Object.entries(profile.attribute_claims ?? {}).map(([key, value]) => ({
    key,
    kind: "attribute" as const,
    value,
  }));
  const statics = Object.entries(profile.static_claims ?? {}).map(([key, value]) => ({
    key,
    kind: "static" as const,
    value: String(value ?? ""),
  }));
  return [...attributes, ...statics].sort((a, b) => a.key.localeCompare(b.key));
}

function fromExtras(extras: ExtraClaim[]) {
  const attributes: Record<string, string> = {};
  const statics: Record<string, unknown> = {};
  for (const extra of extras) {
    const key = extra.key.trim();
    if (!key) continue;
    if (extra.kind === "attribute") attributes[key] = extra.value;
    else statics[key] = extra.value;
  }
  return { attributes, statics };
}

function replace(list: ExtraClaim[], index: number, next: ExtraClaim): ExtraClaim[] {
  return list.map((entry, i) => (i === index ? next : entry));
}

function slug(name: string): string {
  return name.toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-|-$/g, "") || "app";
}
