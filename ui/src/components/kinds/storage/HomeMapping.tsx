"use client";

import { Segmented } from "@/components/ui/Select";
import { ArrowLink } from "@/components/ui/ArrowLink";
import { FieldHint, Input } from "@/components/ui/Input";

/**
 * What a role gives on a storage system, in the mapping dialog (A-03 §2):
 * whether it gives a home folder and that folder's limit. The dialog owns the
 * choice (it composes the plan from it); this draws the controls. Storage
 * words, so they live in the storage kind.
 */
export function HomeChoice({
  target,
  name,
  homeRoot,
  hadHome,
  home,
  onHome,
  limit,
  onLimit,
  gb,
  onGb,
}: {
  target: string;
  name: string;
  /** Where home folders go, as the system labels it; empty until it is chosen. */
  homeRoot: string;
  /** The role already gave a home folder before this change. */
  hadHome: boolean;
  home: boolean;
  onHome: (v: boolean) => void;
  limit: "size" | "none";
  onLimit: (v: "size" | "none") => void;
  gb: string;
  onGb: (v: string) => void;
}) {
  return (
    <div className="grid gap-1.5">
      <span className="type-label text-label" id="m-home">
        Home folder
      </span>
      {!homeRoot && !hadHome ? (
        // One control that can't act now: a sentence and the step.
        <span className="text-[14px] leading-[1.55] text-ink [text-wrap:pretty]">
          No mapping can give one until you choose where home folders go on {name}. This one works without
          it; give it a home folder later by changing it.{" "}
          <ArrowLink href={`/system/targets/${target}`}>Choose where home folders go</ArrowLink>
        </span>
      ) : (
        <>
          <span aria-labelledby="m-home">
            <Segmented
              label="Home folder"
              value={home ? "gives" : "none"}
              onChange={(v) => onHome(v === "gives")}
              options={[
                { value: "none", label: "No home folder" },
                { value: "gives", label: "Gives a home folder" },
              ]}
            />
          </span>
          <FieldHint>
            {home
              ? `Each person who holds the role gets a folder of their own in ${homeRoot}, named after their account and private to them. Syndra never moves or deletes one, even when the role is revoked.`
              : `Home folders go in ${homeRoot}, set on ${name}'s page for every role.`}
          </FieldHint>
          {home && (
            <div className="mt-1 grid gap-1.5">
              <span className="type-label text-label">Home folder limit</span>
              <span className="flex flex-wrap items-center gap-3">
                <Segmented
                  label="Home folder limit"
                  value={limit}
                  onChange={onLimit}
                  options={[
                    { value: "size", label: "Up to a size" },
                    { value: "none", label: "No limit" },
                  ]}
                />
                {limit === "size" && (
                  <span className="flex items-center gap-2">
                    <span className="w-[110px]">
                      <Input
                        aria-label="Limit in GB"
                        type="number"
                        min={1}
                        inputMode="decimal"
                        value={gb}
                        onChange={(e) => onGb(e.target.value)}
                      />
                    </span>
                    <span className="text-[14px] text-muted">GB</span>
                  </span>
                )}
              </span>
              <FieldHint>
                Someone holding two roles that each carry a home folder gets the larger limit. One person can be given
                their own, on {name}&apos;s page under Accounts.
              </FieldHint>
            </div>
          )}
        </>
      )}
    </div>
  );
}

/** "Makes 3 home folders in main › homes; 1 person already has one there." */
export function HomesMade({ homes, homeRoot }: { homes: { makes: number; already: number }; homeRoot: string }) {
  return (
    <>
      Makes {homes.makes} home folder{homes.makes === 1 ? "" : "s"} in <span className="type-mono">{homeRoot}</span>
      {homes.already > 0 ? `; ${homes.already} ${homes.already === 1 ? "person already has" : "people already have"} one there.` : "."}{" "}
    </>
  );
}
