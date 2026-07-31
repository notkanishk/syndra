"use client";

import { toast } from "sonner";

import { ChimeToggle } from "@/components/settings/ChimeToggle";
import { Card, CardHeader } from "@/components/ui/Card";
import { PageHeader } from "@/components/ui/PageHeader";
import { Segmented } from "@/components/ui/Select";
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

  const mode = current.data === "auto" ? "auto" : "manual";

  return (
    <div className="flex flex-col gap-[18px]">
      <PageHeader
        title="Automation settings"
        meta="Policy that applies to every future rule and cascade."
      />

      <Card>
        <CardHeader title="New rules apply" />
        <div className="row-divider flex flex-col gap-3 px-5 py-4">
          <Segmented<"auto" | "manual">
            label="Default confirmation mode"
            value={mode}
            onChange={async (next) => {
              try {
                await save.mutateAsync(next);
                toast.success(
                  next === "auto"
                    ? "New rules will apply immediately."
                    : "New rules will queue for review.",
                );
              } catch (error) {
                toast.error(error instanceof Error ? error.message : "That didn't save.");
              }
            }}
            options={[
              { value: "manual", label: "Queue for review" },
              { value: "auto", label: "Apply immediately" },
            ]}
          />
          <p className="max-w-[70ch] text-[14px] leading-[1.55] text-muted">
            {mode === "manual"
              ? "A new rule's writes wait under Pending changes until an operator resumes them. Slower, and nothing reaches the identity provider without somebody looking at it."
              : "A new rule's writes go straight to the identity provider as it fires. Faster, and a mistake in a rule reaches every holder before anybody reviews it."}
          </p>
          <p className="text-[13px] text-faint">
            This is the default for rules created from now on. Existing rules keep the mode they
            were created with.
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
