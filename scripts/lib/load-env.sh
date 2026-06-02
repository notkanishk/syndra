#!/usr/bin/env bash
# scripts/lib/load-env.sh
#
# Loads KEY=VALUE pairs from an env file into the current shell environment.
# Preserves any KEY already set in the environment (does not overwrite).
# Strips matching surrounding single or double quotes. No-op when the file
# does not exist.
#
# Parsing is deliberately narrow: KEY=VALUE lines with optional leading
# whitespace, optional "…"/'…' quotes stripped, `#` comments and blank
# lines ignored. `${VAR}` inside a value is kept literal — we don't
# re-implement shell expansion here.
#
# The caller sets `_ENV_FILE` to the env-file path before sourcing, e.g.:
#
#   REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
#   _ENV_FILE="${REPO_ROOT}/.env"
#   # shellcheck source=../../scripts/lib/load-env.sh
#   source "${REPO_ROOT}/scripts/lib/load-env.sh"
#
# If `_ENV_FILE` is unset or the file is absent, the helper is a no-op — safe
# under `set -u`. Intentional: the helper does not invent its own path
# resolution — every caller has its own SCRIPT_DIR convention, and the helper
# stays a pure transformation (env file → process environment).

if [[ -f "${_ENV_FILE:-}" ]]; then
  while IFS= read -r _raw || [[ -n "$_raw" ]]; do
    [[ "$_raw" =~ ^[[:space:]]*($|#) ]] && continue
    [[ "$_raw" =~ ^[[:space:]]*([A-Za-z_][A-Za-z0-9_]*)=(.*)$ ]] || continue
    _k="${BASH_REMATCH[1]}"
    _v="${BASH_REMATCH[2]}"
    if [[ "$_v" =~ ^\"(.*)\"$ ]] || [[ "$_v" =~ ^\'(.*)\'$ ]]; then
      _v="${BASH_REMATCH[1]}"
    fi
    [[ -z "${!_k+x}" ]] && export "$_k=$_v"
  done < "$_ENV_FILE"
  unset _raw _k _v
fi
