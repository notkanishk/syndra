#!/usr/bin/env bash
# scripts/lib/zitadel-api.sh
#
# Shared helper for authenticated calls against Zitadel's v2 Actions API.
# Sourced by zitadel/actions/register.sh and zitadel/actions/rotate.sh after
# each has established API_BASE (e.g. "https://${ZITADEL_DOMAIN}/v2/actions")
# and TOKEN in scope. This file defines only the function — it reads no env of
# its own beyond those two and the optional ZITADEL_API_TOLERATE_404.
#
# zitadel_api METHOD PATH [JSON_BODY]
# On 2xx, prints the response body on stdout. On 4xx/5xx, prints method + path
# + status + Zitadel's own JSON error on stderr and returns non-zero — so the
# operator sees what Zitadel actually complained about instead of a bare
# "curl: (22)". 401/403 get an inline pointer at the permissions doc so
# operators don't over-grant; see the "Service-Account Permissions" section
# of zitadel/actions/README.md for the least-privilege matrix.
#
# Set ZITADEL_API_TOLERATE_404=1 in the caller's environment to swallow a
# 404 silently (returns 0, prints nothing). Used by --remove paths that
# want idempotent cleanup against partially-applied / never-applied state:
# Zitadel returns HTTP 404 + COMMAND-74aaqj8fv9 ("Execution condition is
# invalid") from PUT /executions when no execution row matches the
# condition, which is exactly the post-state --remove targets.
zitadel_api() {
  local method="$1" path="$2" body="${3:-}"
  local tmp status
  tmp="$(mktemp)"
  # shellcheck disable=SC2064
  trap "rm -f '$tmp'" RETURN
  local -a args=(-sS -o "$tmp" -w '%{http_code}'
    -X "$method" "${API_BASE}${path}"
    -H "Authorization: Bearer ${TOKEN}")
  if [[ -n "$body" ]]; then
    args+=(-H 'Content-Type: application/json' -d "$body")
  fi
  status="$(curl "${args[@]}")"
  if [[ "$status" == "404" && "${ZITADEL_API_TOLERATE_404:-}" == "1" ]]; then
    return 0
  fi
  if [[ "$status" -lt 200 || "$status" -ge 300 ]]; then
    {
      printf 'error: %s %s -> HTTP %s\n' "$method" "$path" "$status"
      if [[ "$status" == "401" || "$status" == "403" ]]; then
        printf '       Actions v2 target management is instance-scoped — the org-level\n'
        printf '       service-user roles (ORG_OWNER / ORG_USER_MANAGER) do not cover it.\n'
        printf '       Minimum permissions: action.target.read, action.target.write,\n'
        printf '       action.execution.write (plus action.target.delete for full removal).\n'
        printf '       Assign via a narrow custom role, a prebuilt action-scoped role, or\n'
        printf '       IAM_OWNER as a fallback. See the "Service-Account Permissions"\n'
        printf '       section of zitadel/actions/README.md for the full matrix.\n'
      fi
      printf 'response body:\n'
      cat "$tmp"
      printf '\n'
    } >&2
    return 1
  fi
  cat "$tmp"
}
