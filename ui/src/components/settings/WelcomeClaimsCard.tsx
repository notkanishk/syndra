"use client";

import { useState } from "react";

import { ListStates, RowSkeleton } from "@/components/states";
import { Card, CardHeader } from "@/components/ui/Card";
import { Notice } from "@/components/ui/Notice";
import { Segmented } from "@/components/ui/Select";
import { Unfold } from "@/components/ui/Unfold";
import { ApiError } from "@/lib/api-client";
import { describeFailure, sentence } from "@/lib/outcome";
import { useSaveWelcomeClaims, useWelcomeClaims } from "@/lib/queries/useWelcomeClaims";

import { SetByLine } from "./SetByLine";

const LEDE_ID = "welcome-claims-lede";

/**
 * Welcome roles at a first sign-in (owner, 3 Nov 2026; claims-delivery-integrity
 * SC4 option b). A brand-new member's first token is issued before onboarding
 * gives them the welcome bundle, so it lists no roles. On, Syndra answers that
 * one token with the roles the welcome bundle is about to give. Off by
 * default; this card is its one write place, and every change is recorded.
 */
export function WelcomeClaimsCard() {
  const setting = useWelcomeClaims();
  const save = useSaveWelcomeClaims();
  const [failure, setFailure] = useState<{ text: string; reference?: string } | null>(null);
  const on = save.isPending ? Boolean(save.variables) : Boolean(setting.data?.on);
  const setBy = setting.data?.set_by;
  const setAt = setting.data?.set_at;

  async function change(next: boolean) {
    setFailure(null);
    try {
      await save.mutateAsync(next);
    } catch (e) {
      const ref = e instanceof ApiError ? e.details?.request_id : undefined;
      setFailure({ text: sentence(describeFailure(e)), reference: typeof ref === "string" ? ref : undefined });
    }
  }

  return (
    <Card>
      <CardHeader title="First sign-in" />
      <p
        id={LEDE_ID}
        className="row-divider max-w-[86ch] px-5 py-4 text-[14.5px] leading-[1.55] text-muted [text-wrap:pretty]"
      >
        For a new member&apos;s first sign-in only, apps are told the roles the welcome bundle is about to give, so
        they see the right things straight away. Roles that wait for you are never sent early.
      </p>
      <ListStates
        isLoading={setting.isLoading}
        error={setting.data ? null : setting.error}
        isEmpty={false}
        onRetry={() => setting.refetch()}
        errorTitle="Couldn't load this setting."
        skeleton={<RowSkeleton rows={1} avatar={false} label="Loading the first sign-in setting" />}
        empty={null}
      >
        <div data-welcome-row className="row-divider flex flex-wrap items-center gap-x-[18px] gap-y-2 px-5 py-3.5">
          <Segmented
            size="sm"
            label="Welcome roles early"
            describedBy={LEDE_ID}
            options={[
              { value: "off", label: "Off" },
              { value: "on", label: "On" },
            ]}
            value={on ? "on" : "off"}
            onChange={(next) => {
              if ((next === "on") !== on) void change(next === "on");
            }}
          />
        </div>
        <Unfold open={Boolean(failure)} gap={16}>
          {failure && (
            <div className="px-5 pt-3">
              <Notice tone="danger" title="The setting didn't save." reference={failure.reference}>
                {failure.text} Nothing was changed.
              </Notice>
            </div>
          )}
        </Unfold>
        <SetByLine by={setBy} at={setAt} never="Never set. Off until you turn it on." />
      </ListStates>
    </Card>
  );
}
