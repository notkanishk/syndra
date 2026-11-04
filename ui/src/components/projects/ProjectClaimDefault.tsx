"use client";

import { useMemo, useState } from "react";

import { ActionOutcome } from "@/components/ui/ActionOutcome";
import { Button } from "@/components/ui/Button";
import { Card, CardHeader } from "@/components/ui/Card";
import { Crossfade } from "@/components/ui/Crossfade";
import { FormPart } from "@/components/ui/FormPart";
import { useLeavingRows } from "@/components/ui/LeavingRows";
import { FieldHint, FieldLabel, Input } from "@/components/ui/Input";
import { outcomeFromError, type ActionOutcome as Outcome } from "@/lib/outcome";
import { Segmented, Select } from "@/components/ui/Select";
import { RowSkeleton, ErrorState } from "@/components/states";
import {
  useClaimVocabulary,
  useProjectClaimShape,
  useSaveProjectClaimProfile,
  type ClaimFormat,
  type ProjectDefaultForm,
} from "@/lib/queries/useClaimShape";
import { compareNames, sortNames } from "@/lib/sort";

/**
 * The project's default shape (A-14 §2, §7): what every app on this project
 * with no shape of its own is told. Its one write place is here, the
 * project's page; each app page reads it, and links back.
 *
 * It stays flat — one roles claim, its format, and extra claims by name —
 * because that is what the default stores (claims-shaping design.md).
 */
export function ProjectClaimDefault({ projectId }: { projectId: string }) {
  const shape = useProjectClaimShape(projectId);
  const save = useSaveProjectClaimProfile(projectId);
  if (shape.isLoading) {
    return (
      <Card>
        <RowSkeleton rows={3} avatar={false} label="Loading the default shape" />
      </Card>
    );
  }
  if (shape.error || !shape.data) {
    return <ErrorState title="Couldn't load the default shape." error={shape.error} onRetry={() => shape.refetch()} />;
  }
  const onDefault = shape.data.applications.filter((a) => a.source !== "app");
  return (
    <Card id="claims-default">
      <CardHeader title="Default shape" className="pb-1" />
      <p className="px-5 pb-4 text-[14px] leading-[1.5] text-faint">
        What an app on {shape.data.project_name} is told when it has no shape of its own.{" "}
        {onDefault.length === 0
          ? "Every app here has its own shape now, so this reaches none of them."
          : `It reaches ${sortNames(onDefault.map((a) => a.name)).join(", ")} today.`}
      </p>
      <ProfileForm
        key={projectId}
        profile={shape.data.default}
        configured={shape.data.configured}
        onSave={save.mutateAsync}
      />
    </Card>
  );
}

/**
 * The editor proper: roles claim + format, then a row per extra claim. Each
 * extra claim is a key plus a source — a profile attribute, or a constant.
 */
function ProfileForm({
  profile,
  configured,
  onSave,
}: {
  profile: ProjectDefaultForm;
  /** False while the project uses Syndra's built-in default. */
  configured: boolean;
  onSave: (body: ProjectDefaultForm) => Promise<unknown>;
}) {
  const vocabulary = useClaimVocabulary();
  const [claimName, setClaimName] = useState(profile.claim_name);
  const [format, setFormat] = useState<ClaimFormat>(profile.format_type);
  // What the profile holds, with ids that last as long as the profile does: Discard puts these
  // rows back, and a row that was never touched must be the same row (the same key) after it.
  const fromProfile = useMemo(() => toExtras(profile), [profile]);
  const [extras, setExtras] = useState<ExtraClaim[]>(fromProfile);
  // A claim added while the reader is here opens to its height; one removed fades
  // first and its gap settles after (Motion v2, form parts).
  const [added, setAdded] = useState<number[]>([]);
  // Discard is a change in place (A-08 §5): untouched rows hold still, a claim added since
  // fades and its gap settles, one removed since opens its gap and tints in. Counting the
  // discards is what tells the list the reader asked, so the gaps do not wait on anything.
  const [discards, setDiscards] = useState(0);
  const drawn = useLeavingRows(extras, (extra) => String(extra.id), true, { narrowedBy: String(discards) });
  // Which seats had their words put back by a Discard ("name", or a claim's id): each one
  // crossfades, and a seat that was not edited does not.
  const [reverted, setReverted] = useState<Record<string, number>>({});
  const [saving, setSaving] = useState(false);
  const [outcome, setOutcome] = useState<Outcome | null>(null);
  // What was last written, so "unsaved" can be judged against it rather than
  // against `profile` — which stays stale until the parent's refetch lands.
  // Without this, `dirty` stayed true the instant a save succeeded (the local
  // fields now differ from the still-stale `profile`), the Save button stayed
  // live, and the success message rendered next to a control that would
  // resubmit the identical write.
  const [savedSnapshot, setSavedSnapshot] = useState<{
    claimName: string;
    format: ClaimFormat;
    extras: ExtraClaim[];
  } | null>(null);

  const baseline = savedSnapshot ?? {
    claimName: profile.claim_name,
    format: profile.format_type,
    extras: fromProfile,
  };
  const dirty =
    claimName !== baseline.claimName ||
    format !== baseline.format ||
    JSON.stringify(baseline.extras.map(withoutId)) !== JSON.stringify(extras.map(withoutId));

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
        message: "Default shape saved",
        detail: "It reaches each app on the default at the next sign-in. Anyone already signed in keeps the old claims until then.",
      });
      setSavedSnapshot({ claimName: claimName.trim(), format, extras });
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
      <div>
        <FieldLabel htmlFor="claim-name">Roles claim</FieldLabel>
        <FieldHint>
          The claim (one field inside the token) the app reads roles from. It must match the name
          the app&rsquo;s developer configured.
        </FieldHint>
        {!configured && (
          // Nobody may have set this — "roles" here can be the built-in
          // default as easily as a deliberate choice, and printing it as
          // plain fact is how a list elsewhere ended up disagreeing with
          // this screen about the same app's real claim name.
          <p className="mt-1 text-[13px] text-faint">
            Nobody has set this yet. Syndra sends the built-in default below.
          </p>
        )}
        <div className="flex flex-wrap items-center gap-2.5">
          <Crossfade id={String(reverted.name ?? 0)} className="min-w-[240px] flex-1">
            <Input
              id="claim-name"
              value={claimName}
              spellCheck={false}
              onChange={(event) => setClaimName(event.target.value)}
              className="font-mono text-[13.5px]"
              placeholder="syndra.laser.roles"
            />
          </Crossfade>
          <Segmented<ClaimFormat>
            label="Sent as"
            size="sm"
            value={format}
            onChange={setFormat}
            options={(vocabulary.data?.formats ?? ["array", "csv", "space_delimited"]).map(
              (value) => ({ value, label: value === "space_delimited" ? "Space-separated" : value }),
            )}
          />
        </div>
      </div>

      <div>
        <div className="mb-2 flex items-center gap-2">
          <span className="type-label">Also send</span>
          <span className="flex-1" />
          <Button
            size="sm"
            onClick={() => {
              const id = nextExtraId++;
              setAdded((prev) => [...prev, id]);
              setExtras((prev) => [...prev, { id, key: "", kind: "attribute", value: "email" }]);
            }}
          >
            Add a claim
          </Button>
        </div>

        {drawn.rows.length === 0 ? (
          <p className="text-[13.5px] text-faint">
            Only the roles are sent. Add a claim when the app also needs the person&rsquo;s email
            or team.
          </p>
        ) : (
          <div className="-mb-2 flex flex-col">
            {drawn.rows.map((extra) => (
              <FormPart key={extra.id} appear={added.includes(extra.id)} {...drawn.props(String(extra.id))}>
              <Crossfade id={String(reverted[String(extra.id)] ?? 0)}>
              <div className="flex flex-wrap items-center gap-2 pb-2">
                <Input
                  value={extra.key}
                  spellCheck={false}
                  aria-label="Claim name"
                  placeholder="syndra.laser.email"
                  onChange={(event) =>
                    setExtras((prev) => replace(prev, extra.id, { ...extra, key: event.target.value }))
                  }
                  className="min-w-[200px] flex-1 font-mono text-[13.5px]"
                />
                <Select
                  aria-label="Where the value comes from"
                  value={extra.kind === "static" ? "__static" : extra.value}
                  onChange={(event) =>
                    setExtras((prev) =>
                      replace(
                        prev,
                        extra.id,
                        event.target.value === "__static"
                          ? { ...extra, kind: "static", value: "" }
                          : { ...extra, kind: "attribute", value: event.target.value },
                      ),
                    )
                  }
                  className="w-[170px]"
                >
                  {sortNames(vocabulary.data?.attributes ?? []).map((attribute) => (
                    <option key={attribute} value={attribute}>
                      {attribute}
                    </option>
                  ))}
                  <option value="__static">the same value for everyone…</option>
                </Select>
                {extra.kind === "static" && (
                  <Input
                    value={extra.value}
                    aria-label="Fixed value"
                    placeholder="makerspace"
                    onChange={(event) =>
                      setExtras((prev) =>
                        replace(prev, extra.id, { ...extra, value: event.target.value }),
                      )
                    }
                    className="w-[150px]"
                  />
                )}
                <Button
                  size="sm"
                  aria-label={`Remove ${extra.key || "claim"}`}
                  onClick={() => setExtras((prev) => prev.filter((entry) => entry.id !== extra.id))}
                >
                  Remove
                </Button>
              </div>
              </Crossfade>
              </FormPart>
            ))}
          </div>
        )}
      </div>

      {/* Under the control that saved it, and above nothing: an editor that
          reports elsewhere leaves the operator unsure whether the shape on
          screen is the shape that was written. */}
      {outcome && <ActionOutcome outcome={outcome} word="Saved" />}

      <div className="flex flex-wrap items-center gap-2.5">
        <Button variant="accent" disabled={!dirty || !claimName.trim()} isPending={saving} onClick={save}>
          Save the default
        </Button>
        {dirty && (
          <Button
            variant="ghost"
            onClick={() => {
              // Back to `baseline`, not `profile` directly — after a save that
              // has not refetched yet, `profile` is still the pre-save value.
              setReverted((now) => {
                const next = { ...now };
                if (claimName !== baseline.claimName) next.name = (now.name ?? 0) + 1;
                for (const was of baseline.extras) {
                  const edited = extras.find((extra) => extra.id === was.id);
                  if (edited && JSON.stringify(withoutId(edited)) !== JSON.stringify(withoutId(was))) {
                    next[String(was.id)] = (now[String(was.id)] ?? 0) + 1;
                  }
                }
                return next;
              });
              setDiscards((count) => count + 1);
              setClaimName(baseline.claimName);
              setFormat(baseline.format);
              setExtras(baseline.extras);
            }}
          >
            Discard changes
          </Button>
        )}
      </div>
    </div>
  );
}

interface ExtraClaim {
  /** Which row this is while it is edited, added or removed; never sent. */
  id: number;
  key: string;
  kind: "attribute" | "static";
  value: string;
}

let nextExtraId = 0;

const withoutId = ({ key, kind, value }: ExtraClaim) => ({ key, kind, value });

function toExtras(profile: ProjectDefaultForm): ExtraClaim[] {
  const attributes = Object.entries(profile.attribute_claims ?? {}).map(([key, value]) => ({
    id: nextExtraId++,
    key,
    kind: "attribute" as const,
    value: String(value),
  }));
  const statics = Object.entries(profile.static_claims ?? {}).map(([key, value]) => ({
    id: nextExtraId++,
    key,
    kind: "static" as const,
    value: String(value ?? ""),
  }));
  return [...attributes, ...statics].sort((a, b) => compareNames(a.key, b.key));
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

function replace(list: ExtraClaim[], id: number, next: ExtraClaim): ExtraClaim[] {
  return list.map((entry) => (entry.id === id ? next : entry));
}
