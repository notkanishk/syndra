"use client";


import { useState } from "react";

import { ChimeToggle } from "@/components/settings/ChimeToggle";
import { ActionOutcome } from "@/components/ui/ActionOutcome";
import { Card, CardHeader } from "@/components/ui/Card";
import { PageHeader } from "@/components/ui/PageHeader";
import { Segmented } from "@/components/ui/Select";
import { outcomeFromError, type ActionOutcome as Outcome } from "@/lib/outcome";
import {
  useGlobalConfirmationDefault,
  useSetGlobalConfirmationDefault,
} from "@/lib/queries/useConfirmationMode";

/**
 * S2b · Automation › Settings.
 *
 * The global default confirmation mode is system-wide policy affecting every
 * future cascade, so it gets a stable home inside Automation. It deliberately
 * does NOT live in a sidebar footer, an account menu or a preferences drawer:
 * those read as personal settings, and somebody will flip an org-wide policy
 * thinking they changed something about themselves.
 */
export default function AutomationSettingsPage() {
  const current = useGlobalConfirmationDefault();
  const save = useSetGlobalConfirmationDefault();
  const [outcome, setOutcome] = useState<Outcome | null>(null);

  const mode = current.data === "auto" ? "auto" : "manual";

  return (
    <div className="flex flex-col gap-[18px]">
      <PageHeader
        title="Automation settings"
        meta="Policy that applies to every future rule and cascade."
      />

      <Card>
        <CardHeader
          title="Default confirmation mode"
          note="What a newly created rule does when it fires. Existing rules keep their own setting."
        />

        {/*
          Each option is described by its COST rather than its name. "Manual"
          and "auto" tell an operator nothing about what they are trading; "no
          chance to catch a bad rule before it lands" does.
        */}
        <div className="row-divider flex flex-col gap-3 px-5 py-4">
          <Segmented<"auto" | "manual">
            label="Default confirmation mode"
            value={mode}
            onChange={async (next) => {
              try {
                await save.mutateAsync(next);
                setOutcome({
                  kind: "applied",
                  message:
                    next === "auto"
                      ? "New rules will apply immediately"
                      : "New rules will queue for review",
                  // The sentence a settings page owes and rarely says: this
                  // changed the default, not the rules that already exist.
                  detail: "Existing rules keep the mode they were created with.",
                });
              } catch (error) {
                setOutcome(outcomeFromError(error));
              }
            }}
            options={[
              { value: "manual", label: "Queue for review" },
              { value: "auto", label: "Apply immediately" },
            ]}
          />

          {/* Under the control that changed it. A setting that reports itself
              somewhere else is a setting an operator cannot tell they changed. */}
          {outcome && <ActionOutcome outcome={outcome} />}

          <div className="grid gap-3 tablet:grid-cols-2">
            <div
              className={`rounded-inner border px-4 py-3.5 ${
                mode === "manual" ? "border-accent-line bg-accent-soft" : "border-line"
              }`}
            >
              <div className="text-[14.5px] font-semibold">Queue for review</div>
              <p className="mt-1 text-[13.5px] leading-[1.55] text-muted">
                Writes wait in Pending changes until someone confirms them. Slower, and nothing
                reaches a machine without a person seeing it first.
              </p>
            </div>
            <div
              className={`rounded-inner border px-4 py-3.5 ${
                mode === "auto" ? "border-accent-line bg-accent-soft" : "border-line"
              }`}
            >
              <div className="text-[14.5px] font-semibold">Apply immediately</div>
              <p className="mt-1 text-[13.5px] leading-[1.55] text-muted">
                Cascades reach the identity provider the moment a rule fires. Fewer clicks, and no
                chance to catch a bad rule before it lands.
              </p>
            </div>
          </div>
        </div>

        {/*
          The placement rule, stated on the screen rather than only in the spec.
          Somebody who finds this page in a preferences drawer one day should
          be able to tell it was moved by mistake.
        */}
        <div className="warn-note m-5 flex items-start gap-3 px-4 py-3.5">
          <span
            aria-hidden
            className="mt-px flex h-5 w-5 flex-none items-center justify-center rounded-pill bg-warn-soft text-[12px] font-bold text-warn-text"
          >
            !
          </span>
          <p className="max-w-[86ch] text-[13.5px] leading-[1.55] text-muted">
            This is org-wide policy, which is why it has a page inside Automation rather than a row
            in a preferences drawer. A setting that changes what happens to everybody must not sit
            where personal preferences live — someone will flip it for the whole makerspace
            thinking they changed something about themselves.
          </p>
        </div>
      </Card>

      <Card>
        <CardHeader title="Alerts" />
        <div className="row-divider flex flex-wrap items-center gap-4 px-5 py-4">
          <div className="min-w-[280px] flex-1">
            <div className="text-[14.5px] font-semibold">Sound on new unexplained access</div>
            <p className="mt-1 text-[13.5px] text-muted">
              Plays once when the unexplained-access count rises while this tab is open. Per
              browser, not per organisation.
            </p>
          </div>
          <ChimeToggle />
        </div>
      </Card>
    </div>
  );
}
