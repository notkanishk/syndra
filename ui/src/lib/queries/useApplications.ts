"use client";

import { useQuery } from "@tanstack/react-query";

import { request } from "@/lib/api-client";
import { sortByName } from "@/lib/sort";
import type { ObservationBasis } from "@/lib/queries/useRoles";

export interface ApplicationView {
  application: {
    id: string;
    name: string;
    project_id: string;
    description: string;
    consumer: string;
    claim_name: string;
    format_type: string;
  };
  consumed_roles: string[];
  assigned_user_count: number;
  /**
   * What Zitadel shows holding a role this app reads, right now. Absent, not
   * zero, when the org has never been read.
   */
  observed_user_count?: number;
  /**
   * How many of `assigned_user_count`'s people the observation store also
   * shows holding a role here. Absent — not zero — while nothing has been
   * observed; passing `undefined` where a real overlap exists makes
   * `holdersLine` report every holder as both unexplained and undelivered.
   */
  confirmed_user_count?: number;
  observation?: ObservationBasis;
  /**
   * What this app is told, in a line (A-14 §7): its own shape's top-level
   * keys, where they land, and where the shape comes from. `reaches` is check
   * 1's verdict from Zitadel's last answer; absent when never read. Absent
   * altogether when the shapes couldn't be read.
   */
  claims?: {
    source: "app" | "project_default" | "builtin";
    keys: string[];
    lands: "both" | "access_token" | "userinfo";
    template_id?: string;
    template_changed?: boolean;
    copied_from_application_id?: string;
    reaches?: boolean;
  };
}

const KEYS = {
  list: ["applications"] as const,
  one: (id: string) => ["applications", id] as const,
};

/** List all applications. */
export function useApplications() {
  return useQuery({
    queryKey: KEYS.list,
    queryFn: async (): Promise<ApplicationView[]> => {
      const data = await request<unknown>("/applications");
      return Array.isArray(data) ? (data as ApplicationView[]) : [];
    },
    select: byAppName,
  });
}

const byAppName = (rows: ApplicationView[]) => sortByName(rows, (r) => r.application.name, (r) => r.application.id);

export const applicationsQueryKeys = KEYS;
