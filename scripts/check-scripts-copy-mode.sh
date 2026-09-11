#!/usr/bin/env bash

# Verifies that every Dockerfile copying the dist scripts into the image sets an explicit
# owner and mode, so that the files an image runs as root cannot inherit the build context's
# permissions. Without "--chmod" the copy keeps the source modes, which are world-writable in
# many working copies and on Windows hosts.
#
# Usage: scripts/check-scripts-copy-mode.sh [dockerfile ...]
#
# Without arguments, all Dockerfiles in this working copy are checked, including the ones in
# optional private subdirectories when they are present.

set -euo pipefail

cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.."

EXPECTED_OWNER="--chown=root:root"
EXPECTED_MODE="--chmod=755"

if [[ $# -gt 0 ]]; then
  DOCKERFILES=("$@")
else
  mapfile -t DOCKERFILES < <(find . -name Dockerfile -not -path "./.git/*" | sort)
fi

FOUND=0
FAILED=0

for dockerfile in "${DOCKERFILES[@]}"; do
  [[ -f ${dockerfile} ]] || continue

  while IFS=: read -r line_no line; do
    FOUND=$((FOUND + 1))

    for expected in "${EXPECTED_OWNER}" "${EXPECTED_MODE}"; do
      if [[ ${line} != *"${expected}"* ]]; then
        echo "${dockerfile}:${line_no}: copies the dist scripts without ${expected}" 1>&2
        FAILED=$((FAILED + 1))
      fi
    done
  done < <(grep -n "COPY.*scripts/dist/ */scripts/" "${dockerfile}" || true)
done

if [[ ${FAILED} -gt 0 ]]; then
  echo "Found ${FAILED} script copy instruction(s) without an explicit owner and mode." 1>&2
  exit 1
fi

echo "Script copy modes checked (${FOUND} instruction(s) in ${#DOCKERFILES[@]} Dockerfile(s))."
