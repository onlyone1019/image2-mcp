#!/usr/bin/env bash
set -euo pipefail

repo_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

if [[ -f "${repo_dir}/.env.local" ]]; then
  set -a
  # shellcheck disable=SC1091
  source "${repo_dir}/.env.local"
  set +a
elif [[ -f "${repo_dir}/.env" ]]; then
  set -a
  # shellcheck disable=SC1091
  source "${repo_dir}/.env"
  set +a
fi

exec "${repo_dir}/dist/image2-mcp"
