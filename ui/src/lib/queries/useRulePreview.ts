import { useQuery } from "@tanstack/react-query";

import { request } from "@/lib/api-client";

/** One role an automatic rule would also give: services.RuleRow. */
export interface RuleRow {
  project_id: string;
  role_key: string;
  rule_id: string;
  /** "Also gives Bench in Workshop, by the automatic rule from Trained." */
  sentence: string;
}

/**
 * What the automatic rules would also give `userId` with these roles, asked
 * before the operator confirms (coherence audit C1, C2): giving a role,
 * approving a request, adopting drift. The backend answers from the same
 * closure the write follows (services.RulesFollowing); the UI never walks the
 * rules itself. Under ["users"], so any access change refreshes it.
 */
export function useRulePreview(userId: string, projectId: string, roleKeys: string[], enabled = true) {
  const keys = roleKeys.filter(Boolean);
  return useQuery({
    queryKey: ["users", userId, "rule-preview", projectId, ...keys],
    queryFn: async (): Promise<RuleRow[]> => {
      const query = new URLSearchParams({ project_id: projectId });
      for (const key of keys) query.append("role_key", key);
      const data = await request<{ rows?: RuleRow[] }>(`/users/${encodeURIComponent(userId)}/rule-preview?${query}`);
      return data?.rows ?? [];
    },
    enabled: enabled && Boolean(userId && projectId && keys.length > 0),
  });
}
