#!/usr/bin/env bash
# Assemble a self-contained nyt-cli skill bundle: the skill source plus a matching
# binary, zipped into dist/ for attachment to the release. Invoked by GoReleaser's
# before-hook (see .goreleaser.yaml) with the release version as $1.
#
# Usage: scripts/build-skill-bundle.sh <version> [goos] [goarch]
set -euo pipefail

VERSION="${1:-dev}"
GOOS_BUNDLE="${2:-darwin}"
GOARCH_BUNDLE="${3:-arm64}"

PKG="gitea.jonn.me/jons-org/nyt_cli"
SKILL_DIR=".claude/skills/nyt-cli"
# Stage outside dist/: GoReleaser's --clean wipes dist/ and then refuses to run if
# the before-hook has repopulated it. release.extra_files globs this path instead.
OUT="build/nyt-cli-skill-${GOOS_BUNDLE}-${GOARCH_BUNDLE}.zip"

mkdir -p build "${SKILL_DIR}/bin"

echo "building skill binary ${GOOS_BUNDLE}/${GOARCH_BUNDLE} (version ${VERSION})"
GOOS="${GOOS_BUNDLE}" GOARCH="${GOARCH_BUNDLE}" CGO_ENABLED=0 \
	go build -ldflags "-s -w -X ${PKG}/cmd.Version=${VERSION}" \
	-o "${SKILL_DIR}/bin/nyt" .

echo "zipping skill bundle -> ${OUT}"
rm -f "${OUT}"
# Zip with the nyt-cli/ prefix preserved so it extracts as a ready-to-use skill dir.
( cd "$(dirname "${SKILL_DIR}")" && zip -q -r -X "${OLDPWD}/${OUT}" "$(basename "${SKILL_DIR}")" )

echo "done: ${OUT}"
