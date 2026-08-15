#!/usr/bin/env bash
# Build versioned prdpr binaries and a SHA-256 checksums file.
# Usage: scripts/build-release.sh <version> [output-dir]
# Version is the release number without a leading v (for example 0.1.0).
set -euo pipefail

if [[ $# -lt 1 || $# -gt 2 ]]; then
  echo "usage: $0 <version> [output-dir]" >&2
  exit 2
fi

version="$1"
if [[ ! "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+([.-][0-9A-Za-z.-]+)?$ ]]; then
  echo "invalid version ${version}; expected MAJOR.MINOR.PATCH" >&2
  exit 2
fi

root="$(cd "$(dirname "$0")/.." && pwd)"
out="${2:-${root}/dist/release}"
module_version="github.com/lanternfold/prd-pr/internal/cli.Version"
ldflags="-s -w -buildid= -X ${module_version}=${version}"

rm -rf "$out"
mkdir -p "$out"

export CGO_ENABLED=0

targets=(
  darwin/arm64
  darwin/amd64
  linux/arm64
  linux/amd64
)

artifacts=()
for pair in "${targets[@]}"; do
  goos="${pair%/*}"
  goarch="${pair#*/}"
  name="prdpr_${version}_${goos}_${goarch}"
  echo "building ${name}"
  GOOS="$goos" GOARCH="$goarch" go build -trimpath -ldflags "$ldflags" -o "${out}/${name}" "${root}/cmd/prdpr"
  artifacts+=("${name}")
done

checksums="${out}/prdpr_${version}_checksums.txt"
(
  cd "$out"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "${artifacts[@]}"
  else
    shasum -a 256 "${artifacts[@]}"
  fi
) >"$checksums"

echo "wrote artifacts in ${out}"
ls -1 "$out"
