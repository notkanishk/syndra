"use client";

import { useEffect, useRef, useState } from "react";

import { ErrorState, RowSkeleton } from "@/components/states";
import { Avatar } from "@/components/ui/Avatar";
import { Button } from "@/components/ui/Button";
import { Card } from "@/components/ui/Card";
import { selectContents, useClipboardAvailable } from "@/lib/clipboard";
import { Select } from "@/components/ui/Select";
import { useCatalogUsers } from "@/lib/queries/useCatalogUsers";
import { useTokenSimulator } from "@/lib/queries/useApplications";

/**
 * Preview token — the dry run.
 *
 * It reads like a receipt: mono, copyable, unambiguous, with `//` comments
 * explaining the interesting silences. An empty array is shown AS an empty
 * array with a line saying why, because that silence is usually the answer
 * somebody is looking for.
 *
 * This is not a mock. The payload comes from the same profile resolution and
 * the same shaper the Zitadel Actions v2 path uses; only the signature is
 * missing.
 */
export function TokenPreview({
  applicationId,
  applicationName,
  projectId,
}: {
  applicationId: string;
  applicationName: string;
  projectId: string;
}) {
  const users = useCatalogUsers();
  const [userId, setUserId] = useState("");
  const [showRaw, setShowRaw] = useState(false);
  const [copied, setCopied] = useState(false);
  const [revealed, setRevealed] = useState(false);
  const payloadRef = useRef<HTMLPreElement>(null);
  const canCopy = useClipboardAvailable();

  useEffect(() => {
    if (!userId && (users.data?.length ?? 0) > 0) setUserId(users.data![0].id);
  }, [users.data, userId]);

  const simulation = useTokenSimulator(applicationId, userId);
  const claims = simulation.data?.custom_claims ?? {};
  const owned = new Set(simulation.data?.owned_claims ?? []);
  const owners = new Map(
    (simulation.data?.claim_owners ?? []).map((entry) => [entry.key, entry.owner_label]),
  );
  const keys = Object.keys(claims).sort();

  const payload = JSON.stringify(claims, null, 2);
  const emptyKeys = keys.filter((key) => isEmptyValue(claims[key]));

  return (
    <Card className="flex h-full flex-col">
      <div className="px-[22px] pb-3.5 pt-5">
        <h2 className="type-card-title">Preview token</h2>
        <p className="mt-1 text-[14px] leading-[1.5] text-faint">
          Exactly what {applicationName} would receive right now.
        </p>
      </div>

      <div className="flex flex-wrap items-center gap-2.5 px-[22px] pb-3.5">
        <span className="text-[13.5px] text-faint">for</span>
        <span className="flex items-center gap-2 rounded-pill border border-line-strong py-1 pl-1.5 pr-2">
          <Avatar name={users.data?.find((u) => u.id === userId)?.name} size="row" />
          <Select
            aria-label="Preview for"
            value={userId}
            onChange={(event) => setUserId(event.target.value)}
            className="w-[190px] border-0 px-1 py-0 text-[14px]"
          >
            {(users.data ?? []).map((user) => (
              <option key={user.id} value={user.id}>
                {user.name}
              </option>
            ))}
          </Select>
        </span>
        <Button
          variant="accent"
          size="sm"
          isPending={simulation.isFetching}
          onClick={() => simulation.refetch()}
        >
          Run
        </Button>
      </div>

      <div className="flex-1 px-[22px] pb-5">
        {simulation.isLoading ? (
          <RowSkeleton rows={3} avatar={false} label="Running the simulation" />
        ) : simulation.error ? (
          <ErrorState
            title="Couldn't run the preview."
            error={simulation.error}
            onRetry={() => simulation.refetch()}
          />
        ) : (
          <div className="rounded-inner border border-line bg-surface-0 px-[18px] py-4 font-mono text-[13px] leading-[1.85]">
            <div className="text-ink/35">{"// effective_role_keys → claims"}</div>
            {keys.length === 0 && (
              <div className="text-ink/70">
                {"{}"} <span className="text-ink/35">{"// this project grants them nothing"}</span>
              </div>
            )}
            {keys.map((key, index) => (
              <div key={key} className={owned.has(key) ? "" : "opacity-60"}>
                <span className="text-accent-text">&quot;{key}&quot;</span>
                <span className="text-ink/70">: </span>
                <span className="text-ink/95">{renderValue(claims[key])}</span>
                {index < keys.length - 1 && <span className="text-ink/70">,</span>}
                {!owned.has(key) && (
                  <span className="text-ink/35">{`  // ${owners.get(key) ?? "another app"} reads this`}</span>
                )}
              </div>
            ))}

            {/* The interesting silences, explained. */}
            {emptyKeys.length > 0 && (
              <div className="mt-3 text-ink/35">
                {emptyKeys.map((key) => (
                  <div key={key}>
                    {`// ${key} is empty — they hold no role in ${projectId}.`}
                  </div>
                ))}
              </div>
            )}
            {keys.length > 0 && owned.size < keys.length && (
              <div className="text-ink/35">
                {`// Keys this app doesn't read are carried because a token is issued`}
                <br />
                {`// per project, not per app. Each app reads its own key.`}
              </div>
            )}
          </div>
        )}

        {showRaw && simulation.data && (
          <div className="mt-3 rounded-inner border border-line bg-surface-0 px-[18px] py-3.5">
            <div className="mb-1.5 type-label">Raw roles</div>
            <div className="flex flex-wrap gap-1.5">
              {(simulation.data.raw_roles ?? []).length === 0 ? (
                <span className="text-[13.5px] text-faint">None in this project.</span>
              ) : (
                simulation.data.raw_roles.map((role) => (
                  <span
                    key={role}
                    className="rounded-pill bg-tint-2 px-2.5 py-1 font-mono text-[12.5px]"
                  >
                    {role}
                  </span>
                ))
              )}
            </div>
          </div>
        )}

        {revealed && (
          <pre
            ref={payloadRef}
            className="type-mono mt-3.5 max-h-[40vh] overflow-y-auto whitespace-pre-wrap break-all rounded-inner bg-surface-0 px-3.5 py-3 text-ink"
          >
            {payload}
          </pre>
        )}

        <div className="mt-3.5 flex gap-2.5">
          {/* The button reports itself, the way every copy affordance in the
              product now does — and says `Select` where the browser cannot
              copy rather than failing silently on the tap. */}
          <Button
            size="sm"
            onClick={async () => {
              if (canCopy) {
                await navigator.clipboard.writeText(payload);
              } else {
                // Nothing to select unless it is on screen. Revealing it is
                // the honest fallback and it is what this value is anyway:
                // evidence, which a token debug screen should be willing to
                // show rather than only hand to a clipboard.
                setRevealed(true);
                requestAnimationFrame(() => selectContents(payloadRef.current));
              }
              setCopied(true);
              setTimeout(() => setCopied(false), 900);
            }}
          >
            {copied ? (canCopy ? "Copied" : "Selected") : canCopy ? "Copy payload" : "Select payload"}
          </Button>
          <Button size="sm" onClick={() => setShowRaw((value) => !value)}>
            {showRaw ? "Hide raw roles" : "Show raw roles"}
          </Button>
        </div>
      </div>
    </Card>
  );
}

function renderValue(value: unknown): string {
  if (Array.isArray(value)) {
    return `[${value.map((entry) => JSON.stringify(entry)).join(", ")}]`;
  }
  return JSON.stringify(value ?? null);
}

function isEmptyValue(value: unknown): boolean {
  if (Array.isArray(value)) return value.length === 0;
  return value === "" || value === null || value === undefined;
}
