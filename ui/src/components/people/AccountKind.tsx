"use client";

import { useState } from "react";

import { ServiceAccountDialog } from "@/components/system/ServiceAccountDialogs";
import { ArrowLink } from "@/components/ui/ArrowLink";
import { Button } from "@/components/ui/Button";
import { Card, CardHeader, LabelRow } from "@/components/ui/Card";
import { serviceAccountHref } from "@/lib/queries/useServiceAccounts";
import type { UserProfile } from "@/lib/types";

/**
 * The Kind row of "This account" on a person's page (A-07 §28, Advanced, an
 * operator's). The mark lives on the account's own page once it is made, so a
 * service account here only points there.
 *
 * `service_account` is the effective answer (machine, or marked); `kind` is
 * all an older read carries.
 */
export function AccountKind({ user }: { user: Pick<UserProfile, "id" | "name" | "kind" | "service_account"> }) {
  const [marking, setMarking] = useState(false);
  const service = user.service_account ?? user.kind === "machine";

  return (
    <Card>
      <CardHeader title="This account" />
      <LabelRow
        first
        top
        seat="wide"
        label="Kind"
        action={
          service ? (
            <ArrowLink href={serviceAccountHref(user.id)}>Open its service account page</ArrowLink>
          ) : (
            <Button size="sm" onClick={() => setMarking(true)}>
              Mark as a service account
            </Button>
          )
        }
      >
        <span className="font-semibold">{service ? "A service account" : "A person"}</span>
        <span className="text-small text-faint">
          {service ? "Listed under System, not People" : "Signs in with a password in Zitadel"}
        </span>
      </LabelRow>
      {marking && <ServiceAccountDialog mode="mark" account={{ id: user.id, name: user.name }} onClose={() => setMarking(false)} />}
    </Card>
  );
}
