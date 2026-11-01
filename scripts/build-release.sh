#!/usr/bin/env bash
# Cross-compiles kanbancli for Linux and Windows (amd64, arm64) into ./dist and writes checksums.txt.
set -euo pipefail

cd "$(dirname "$0")/.."
version="${1:-$(git describe --tags --always 2>/dev/null || echo dev)}"
version="${version#v}"
rm -rf dist
mkdir -p dist

# shellcheck source=scripts/go-licenses-env.sh
source scripts/go-licenses-env.sh

license_stage="dist/.license-staging"
rm -rf "${license_stage}"
mkdir -p "${license_stage}"

kanbancli_go_licenses report "${KANBANCLI_LICENSE_PACKAGE}" >"${license_stage}/report.csv"
kanbancli_go_licenses save "${KANBANCLI_LICENSE_PACKAGE}" --save_path="${license_stage}/licenses"

{
  echo "Third-party licenses for kanbancli ${version}"
  echo "Generated with github.com/google/go-licenses ${GO_LICENSES_VERSION}"
  echo
  echo "=== Dependency report (CSV) ==="
  cat "${license_stage}/report.csv"
  echo
  echo "=== License texts ==="
  while IFS= read -r -d '' f; do
    echo
    echo "--- ${f#"${license_stage}/licenses/"} ---"
    cat "$f"
  done < <(find "${license_stage}/licenses" -type f \( -name 'LICENSE*' -o -name 'COPYING*' \) -print0 | sort -z)
  echo
  echo "=== PLANKA (documentation reference) ==="
  echo "No PLANKA source code or documentation is bundled in this CLI."
  echo "docs/api-scope.md lists endpoints observed from public HTTP behaviour only."
} >"${license_stage}/THIRD_PARTY_NOTICES"

mkdir -p "${license_stage}/apache-notices"
declare -A _kanbancli_apache_mod_seen=()
while IFS=, read -r pkg_path _ lic; do
  [ -n "$pkg_path" ] || continue
  [ "$lic" = "Apache-2.0" ] || continue
  mod_path=""
  mod_dir=""
  read -r mod_path mod_dir <<<"$(go list -f '{{if .Module}}{{.Module.Path}} {{.Module.Dir}}{{end}}' "${pkg_path}")"
  [ -n "${mod_dir}" ] || continue
  [ -n "${_kanbancli_apache_mod_seen[${mod_path}]+x}" ] && continue
  _kanbancli_apache_mod_seen["${mod_path}"]=1
  while IFS= read -r -d '' notice; do
    safe="$(echo "${mod_path}_$(basename "$notice")" | tr '/@.' '___')"
    cp "$notice" "${license_stage}/apache-notices/${safe}"
  done < <(find "${mod_dir}" -maxdepth 1 -type f \( -iname 'NOTICE' -o -iname 'NOTICE.txt' \) -print0)
done < <(tail -n +1 "${license_stage}/report.csv")

targets="linux/amd64 linux/arm64 windows/amd64 windows/arm64"
for t in $targets; do
  os="${t%/*}"
  arch="${t#*/}"
  name="kanbancli_${os}_${arch}"
  stage="dist/${name}"
  mkdir -p "$stage"
  bin="kanbancli"
  [ "$os" = "windows" ] && bin="kanbancli.exe"
  CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" go build -trimpath \
    -ldflags "-s -w -X github.com/swtormy-agents/kanbancli/internal/cli.Version=${version}" \
    -o "${stage}/${bin}" ./cmd/kanbancli
  cp LICENSE NOTICE "${license_stage}/THIRD_PARTY_NOTICES" "$stage/"
  if [ -d "${license_stage}/licenses" ]; then
    cp -r "${license_stage}/licenses" "${stage}/third_party_licenses"
  fi
  if [ -d "${license_stage}/apache-notices" ] && [ -n "$(ls -A "${license_stage}/apache-notices" 2>/dev/null || true)" ]; then
    cp -r "${license_stage}/apache-notices" "${stage}/apache-notices"
  fi
  cp README.md "$stage/"
  cp -r docs "$stage/docs"
  cp -r skills "$stage/skills"
  if [ "$os" = "windows" ]; then
    if command -v zip >/dev/null; then
      (cd dist && zip -qr "${name}.zip" "$name")
    else
      (cd dist && python3 -m zipfile -c "${name}.zip" "$name")
    fi
  else
    tar -C dist -czf "dist/${name}.tar.gz" "$name"
  fi
  rm -rf "$stage"
done

rm -rf "${license_stage}"

cd dist
if command -v sha256sum >/dev/null; then
  sha256sum -- *.tar.gz *.zip > checksums.txt 2>/dev/null || true
else
  shasum -a 256 -- *.tar.gz *.zip > checksums.txt 2>/dev/null || true
fi
ls -la
