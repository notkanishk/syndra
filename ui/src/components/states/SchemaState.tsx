"use client";

import { ErrorState, RowSkeleton, SkeletonAfterDelay } from "@/components/states";
import { Notice } from "@/components/ui/Notice";
import type { EntitlementField } from "@/lib/fields";
import type { SchemaRead } from "@/lib/queries/useFieldSchema";

type State = { schema: EntitlementField[] | undefined } & SchemaRead;

/**
 * What stands in the place of a screen that needs a system's fields until it
 * has them. Three different facts, never worded as one: still being read (a
 * skeleton, nothing else), the read failed (the standard failed-read state,
 * with Try again), and the read succeeded but the add-on has no accepted
 * description of itself ("not read yet", the only one that says so).
 */
export function SchemaNotReady({ state, name }: { state: State; name: string }) {
  if (state.loading) {
    return (
      <SkeletonAfterDelay>
        <RowSkeleton rows={2} avatar={false} label={`Reading ${name}'s fields`} />
      </SkeletonAfterDelay>
    );
  }
  if (state.error) {
    return <ErrorState title={`${name[0].toUpperCase()}${name.slice(1)}'s fields could not be read`} error={state.error} onRetry={state.refetch} />;
  }
  return (
    <Notice
      tone="neutral"
      title={`Syndra has not read ${name}'s fields yet`}
      nothingToDo={`This fills in once ${name} answers and Syndra accepts what it says about itself.`}
    >
      Without that, Syndra cannot tell which of its settings a role may give.
    </Notice>
  );
}

/**
 * Why a control that needs the fields is off, in the same three facts, or
 * undefined when they are known. The words go beside the disabled control.
 */
export function schemaReason(state: State, name: string): string | undefined {
  if (state.schema) return undefined;
  if (state.loading) return `Syndra is still reading ${name}'s fields. This works once it has.`;
  if (state.error) return `Syndra could not read ${name}'s fields. Reload the page to try again.`;
  return `Syndra has not read ${name}'s fields yet. This works once ${name} answers and Syndra accepts what it says about itself.`;
}
