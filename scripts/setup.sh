#!/usr/bin/env bash
set -euo pipefail

repo_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
base_url="${OPENAI_IMAGE_BASE_URL:-https://api.schyler.top}"
configure_codex=0
interactive=0
run_tests=1
run_smoke=0
prefer_prebuilt=0
force_config=0

usage() {
  cat <<'EOF'
Usage: ./install.sh [options]

Install and optionally configure the Image2 MCP server for Codex.

Options:
  --interactive          Prompt for base URL and API key, then write .env.local.
  --configure-codex      Append an image2 MCP server block to ~/.codex/config.toml.
  --force-config         Replace existing [mcp_servers.image2] config block.
  --base-url URL         Set OPENAI_IMAGE_BASE_URL in the Codex MCP config.
  --prebuilt             Download a GitHub Release binary even when Go is installed.
  --skip-tests           Build without running go test ./...
  --smoke                Run a real image-generation smoke test after build.
  -h, --help             Show this help.

Environment:
  OPENAI_IMAGE_API_KEY   Required by Codex at runtime, and required for --smoke.
  OPENAI_IMAGE_BASE_URL  Optional default base URL; defaults to https://api.schyler.top.
  IMAGE2_MCP_REPO        Optional GitHub repo slug, for example owner/image2-mcp.
EOF
}

github_repo_slug() {
  if [[ -n "${IMAGE2_MCP_REPO:-}" ]]; then
    printf '%s\n' "${IMAGE2_MCP_REPO}"
    return 0
  fi
  if ! git remote get-url origin >/dev/null 2>&1; then
    return 1
  fi
  local remote
  remote="$(git remote get-url origin)"
  case "$remote" in
    git@github.com:*.git)
      remote="${remote#git@github.com:}"
      remote="${remote%.git}"
      ;;
    https://github.com/*.git)
      remote="${remote#https://github.com/}"
      remote="${remote%.git}"
      ;;
    https://github.com/*)
      remote="${remote#https://github.com/}"
      ;;
    *)
      return 1
      ;;
  esac
  printf '%s\n' "$remote"
}

platform_name() {
  case "$(uname -s)" in
    Darwin) printf 'darwin' ;;
    Linux) printf 'linux' ;;
    *) return 1 ;;
  esac
}

arch_name() {
  case "$(uname -m)" in
    arm64|aarch64) printf 'arm64' ;;
    x86_64|amd64) printf 'amd64' ;;
    *) return 1 ;;
  esac
}

download_prebuilt() {
  local repo os arch asset url tmp
  repo="$(github_repo_slug)" || {
    echo "error: cannot infer GitHub repo. Set IMAGE2_MCP_REPO=owner/image2-mcp or install Go." >&2
    return 1
  }
  os="$(platform_name)" || {
    echo "error: unsupported OS for prebuilt download: $(uname -s)" >&2
    return 1
  }
  arch="$(arch_name)" || {
    echo "error: unsupported architecture for prebuilt download: $(uname -m)" >&2
    return 1
  }
  asset="image2-mcp_${os}_${arch}.tar.gz"
  url="https://github.com/${repo}/releases/latest/download/${asset}"
  tmp="$(mktemp -d)"
  echo "==> Downloading prebuilt binary: ${url}"
  if command -v curl >/dev/null 2>&1; then
    curl -fL "$url" -o "${tmp}/${asset}"
  elif command -v wget >/dev/null 2>&1; then
    wget -O "${tmp}/${asset}" "$url"
  else
    echo "error: curl or wget is required to download prebuilt binary" >&2
    rm -rf "$tmp"
    return 1
  fi
  mkdir -p dist
  tar -xzf "${tmp}/${asset}" -C dist
  chmod +x dist/image2-mcp
  rm -rf "$tmp"
}

remove_image2_config_block() {
  local config_file="$1"
  awk '
    BEGIN { skip = 0 }
    /^\[mcp_servers\.image2\]$/ { skip = 1; next }
    /^\[/ {
      if (skip == 1) {
        skip = 0
      }
    }
    skip == 0 { print }
  ' "$config_file" > "${config_file}.tmp"
  mv "${config_file}.tmp" "$config_file"
}

while [[ $# -gt 0 ]]; do
  case "$1" in
  --configure-codex)
      configure_codex=1
      shift
      ;;
    --force-config)
      force_config=1
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
    --prebuilt)
      prefer_prebuilt=1
      shift
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

if [[ -f "${repo_dir}/.env.local" ]]; then
  set -a
  # shellcheck disable=SC1091
  source "${repo_dir}/.env.local"
  set +a
fi
if [[ -f "${repo_dir}/.env" ]]; then
  set -a
  # shellcheck disable=SC1091
  source "${repo_dir}/.env"
  set +a
fi
if [[ -n "${OPENAI_IMAGE_BASE_URL:-}" ]]; then
  base_url="${OPENAI_IMAGE_BASE_URL}"
fi

go_available=0
if command -v go >/dev/null 2>&1; then
  go_available=1
fi

if [[ "$prefer_prebuilt" -eq 1 || "$go_available" -eq 0 ]]; then
  if [[ "$run_tests" -eq 1 && "$go_available" -eq 0 ]]; then
    echo "==> Go is not installed; skipping source tests and using prebuilt binary"
  fi
  download_prebuilt
else
  if [[ "$run_tests" -eq 1 ]]; then
    echo "==> Running tests"
    go test ./...
  fi
  echo "==> Building dist/image2-mcp"
  go build -o ./dist/image2-mcp ./cmd/image2-mcp
fi

if [[ "$run_smoke" -eq 1 ]]; then
  if [[ "$go_available" -eq 0 ]]; then
    echo "error: --smoke currently requires Go because it runs go test" >&2
    exit 1
  fi
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

  if grep -q '^\[mcp_servers\.image2\]' "$config_file" && [[ "$force_config" -eq 0 ]]; then
    echo "==> Codex MCP config already contains [mcp_servers.image2]; leaving it unchanged: $config_file"
  else
    if grep -q '^\[mcp_servers\.image2\]' "$config_file"; then
      echo "==> Replacing existing image2 MCP config in $config_file"
      remove_image2_config_block "$config_file"
    else
      echo "==> Adding image2 MCP config to $config_file"
    fi
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
