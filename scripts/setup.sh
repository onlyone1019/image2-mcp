#!/usr/bin/env bash
set -euo pipefail

repo_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
base_url="${OPENAI_IMAGE_BASE_URL:-https://api.schyler.top}"
configure_codex=0
interactive=0
run_tests=1
run_smoke=0

usage() {
  cat <<'EOF'
Usage: ./install.sh [options]

Build and optionally configure the Image2 MCP server for Codex.

Options:
  --interactive          Prompt for base URL and API key, then write .env.local.
  --configure-codex      Append an image2 MCP server block to ~/.codex/config.toml.
  --base-url URL         Set OPENAI_IMAGE_BASE_URL in the Codex MCP config.
  --skip-tests           Build without running go test ./...
  --smoke                Run a real image-generation smoke test after build.
  -h, --help             Show this help.

Environment:
  OPENAI_IMAGE_API_KEY   Required by Codex at runtime, and required for --smoke.
  OPENAI_IMAGE_BASE_URL  Optional default base URL; defaults to https://api.schyler.top.
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
  --configure-codex)
      configure_codex=1
      shift
      ;;
    --interactive)
      interactive=1
      shift
      ;;
    --base-url)
      if [[ $# -lt 2 ]]; then
        echo "error: --base-url requires a value" >&2
        exit 2
      fi
      base_url="$2"
      shift 2
      ;;
    --skip-tests)
      run_tests=0
      shift
      ;;
    --smoke)
      run_smoke=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "error: unknown option: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

cd "$repo_dir"

if ! command -v go >/dev/null 2>&1; then
  echo "error: go is required but was not found in PATH" >&2
  exit 1
fi

mkdir -p dist

if [[ "$interactive" -eq 1 ]]; then
  echo "==> Interactive configuration"
  read -r -p "OPENAI_IMAGE_BASE_URL [${base_url}]: " input_base_url
  if [[ -n "${input_base_url}" ]]; then
    base_url="${input_base_url}"
  fi

  current_key="${OPENAI_IMAGE_API_KEY:-}"
  if [[ -n "$current_key" ]]; then
    read -r -p "OPENAI_IMAGE_API_KEY is already set in this shell. Save it to .env.local? [y/N]: " save_current_key
    if [[ "$save_current_key" =~ ^[Yy]$ ]]; then
      input_api_key="$current_key"
    else
      input_api_key=""
    fi
  else
    printf "OPENAI_IMAGE_API_KEY: "
    if [[ -t 0 ]]; then
      stty -echo
      read -r input_api_key
      stty echo
    else
      read -r input_api_key
    fi
    printf "\n"
  fi

  {
    printf 'OPENAI_IMAGE_BASE_URL=%q\n' "$base_url"
    if [[ -n "${input_api_key:-}" ]]; then
      printf 'OPENAI_IMAGE_API_KEY=%q\n' "$input_api_key"
    fi
  } > .env.local
  chmod 600 .env.local
  echo "==> Wrote local environment file: ${repo_dir}/.env.local"
fi

if [[ "$run_tests" -eq 1 ]]; then
  echo "==> Running tests"
  go test ./...
fi

echo "==> Building dist/image2-mcp"
go build -o ./dist/image2-mcp ./cmd/image2-mcp

if [[ "$run_smoke" -eq 1 ]]; then
  if [[ -z "${OPENAI_IMAGE_API_KEY:-}" ]]; then
    echo "error: --smoke requires OPENAI_IMAGE_API_KEY" >&2
    exit 1
  fi
  echo "==> Running real image-generation smoke test"
  RUN_IMAGE2_SMOKE=1 OPENAI_IMAGE_BASE_URL="$base_url" go test ./internal/image2 -run TestRealGenerateImage2Smoke -count=1 -v
fi

if [[ "$configure_codex" -eq 1 ]]; then
  config_dir="${HOME}/.codex"
  config_file="${config_dir}/config.toml"
  mkdir -p "$config_dir"
  touch "$config_file"

  if grep -q '^\[mcp_servers\.image2\]' "$config_file"; then
    echo "==> Codex MCP config already contains [mcp_servers.image2]; leaving it unchanged: $config_file"
  else
    echo "==> Adding image2 MCP config to $config_file"
    cat >> "$config_file" <<EOF

[mcp_servers.image2]
command = "${repo_dir}/scripts/run-image2-mcp.sh"
EOF
  fi
fi

echo
echo "Image2 MCP is ready."
echo "Binary: ${repo_dir}/dist/image2-mcp"
echo "Runner: ${repo_dir}/scripts/run-image2-mcp.sh"
echo "Base URL: ${base_url}"
if [[ ! -f "${repo_dir}/.env.local" && -z "${OPENAI_IMAGE_API_KEY:-}" ]]; then
  echo "Note: set OPENAI_IMAGE_API_KEY or run ./install.sh --interactive before Codex uses the MCP server."
fi
