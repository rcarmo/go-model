#!/usr/bin/env bash
set -euo pipefail

CANONICAL_PROJECT="go-model"
ORIGINAL_TMPDIR="${TMPDIR:-}"

project_name_valid() {
  case "$1" in ''|*[!A-Za-z0-9._-]*|.*|-*) return 1;; esac
}

project_path_usable() {
  local path="$1" parent ancestor
  case "$path" in /*) ;; *) return 1;; esac
  case "/${path#/}/" in */../*|*/./*) return 1;; esac
  [[ ! -L "$path" ]] || return 1
  ancestor="${path%/*}"
  while [[ -n "$ancestor" && "$ancestor" != / ]]; do
    [[ ! -L "$ancestor" || "$ancestor" == /workspace ]] || return 1
    ancestor="${ancestor%/*}"
  done
  if [[ -e "$path" ]]; then
    [[ -d "$path" && -w "$path" && -x "$path" ]] || return 1
  fi
  parent="$path"
  while [[ ! -e "$parent" && ! -L "$parent" ]]; do parent="${parent%/*}"; [[ -n "$parent" ]] || parent=/; done
  [[ -d "$parent" && -w "$parent" && -x "$parent" ]] || return 1
}

project_tmp_candidate() {
  local base="$1" project="$2"
  [[ -n "$base" ]] || return 1
  printf '%s/%s\n' "${base%/}" "$project"
}

project_tmp_validate_root() {
  local project="$1" root="${2%/}"
  [[ "${root##*/}" == "$project" ]] && project_path_usable "$root" || {
    echo 'PROJECT_TMP_ROOT must be a usable absolute project-named directory, not a symlink' >&2
    return 1
  }
}

project_tmp_resolve() {
  local project="$1" system_tmp="${TMPDIR:-/tmp}" candidate base runner_temp original_tmp explicit_base explicit_root
  project_name_valid "$project" || { echo 'Invalid canonical project name' >&2; return 1; }

  explicit_base="${PROJECT_TMP_BASE:-}"
  explicit_root="${PROJECT_TMP_ROOT:-}"
  if [[ -n "$explicit_base" ]]; then
    case "$explicit_base" in /*) ;; *) echo 'PROJECT_TMP_BASE must be absolute' >&2; return 1;; esac
    candidate="$(project_tmp_candidate "$explicit_base" "$project")"
    project_tmp_validate_root "$project" "$candidate" || return 1
    if [[ -n "$explicit_root" ]]; then
      explicit_root="${explicit_root%/}"
      [[ "$explicit_root" == "$candidate" ]] || { echo 'PROJECT_TMP_BASE and PROJECT_TMP_ROOT must resolve to the same project root' >&2; return 1; }
    fi
    printf '%s\n' "$candidate"; return
  fi

  if [[ -n "$explicit_root" ]]; then
    explicit_root="${explicit_root%/}"
    project_tmp_validate_root "$project" "$explicit_root" || return 1
    printf '%s\n' "$explicit_root"; return
  fi

  runner_temp="${RUNNER_TEMP:-}"
  original_tmp="$ORIGINAL_TMPDIR"
  if [[ -n "${CI:-}" || -n "${GITHUB_ACTIONS:-}" || -n "$runner_temp" ]]; then
    for base in "$runner_temp" "$original_tmp" "$system_tmp"; do
      candidate="$(project_tmp_candidate "$base" "$project" 2>/dev/null || true)"
      [[ -n "$candidate" ]] || continue
      if project_path_usable "$candidate"; then printf '%s\n' "$candidate"; return; fi
    done
  else
    for base in /workspace/tmp "$system_tmp"; do
      candidate="$(project_tmp_candidate "$base" "$project" 2>/dev/null || true)"
      [[ -n "$candidate" ]] || continue
      if project_path_usable "$candidate"; then printf '%s\n' "$candidate"; return; fi
    done
  fi

  echo 'No writable project-owned temporary root available' >&2
  return 1
}

project_tmp_init() {
  local root="$1" path
  for path in "$root" "$root/cache" "$root/build" "$root/tests" "$root/logs" "$root/runs"; do
    project_path_usable "$path" || { echo "Unsafe scratch path: $path" >&2; return 1; }
  done
  mkdir -p "$root/cache" "$root/build" "$root/tests" "$root/logs" "$root/runs"
}

gomodel_project_tmp_root() {
  project_tmp_resolve "$CANONICAL_PROJECT"
}

gomodel_project_tmp_init() {
  local root
  root="$(gomodel_project_tmp_root)"
  project_tmp_init "$root"
  printf '%s\n' \
    "PROJECT_TMP_ROOT=$root" \
    "PROJECT_CACHE_ROOT=$root/cache" \
    "PROJECT_BUILD_ROOT=$root/build" \
    "PROJECT_TESTS_ROOT=$root/tests" \
    "PROJECT_LOGS_ROOT=$root/logs" \
    "PROJECT_RUNS_ROOT=$root/runs"
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  gomodel_project_tmp_init
fi
