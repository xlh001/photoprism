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

# A copy instruction, ignoring comments, and allowing tabs and either instruction keyword.
COPY_PATTERN='^[[:space:]]*(COPY|ADD)[[:space:]].*scripts/dist'

# Accept any spelling of root and of mode 755, rather than one literal form of each.
OWNER_PATTERN='--chown=(root|0):(root|0)([[:space:]]|$)'
MODE_PATTERN='--chmod=0?755([[:space:]]|$)'

if [[ $# -gt 0 ]]; then
  DOCKERFILES=("$@")

  for dockerfile in "${DOCKERFILES[@]}"; do
    if [[ ! -f ${dockerfile} ]]; then
      echo "${dockerfile}: no such file, relative paths resolve from the repository root" 1>&2
      exit 1
    fi
  done
else
  mapfile -t DOCKERFILES < <(
    find . -name Dockerfile -not -path "./.git/*" -not -path "*/node_modules/*" \
      -not -path "./.local/*" -not -path "./storage/*" -not -path "./.gocache/*" | sort
  )

  if [[ ${#DOCKERFILES[@]} -eq 0 ]]; then
    echo "No Dockerfile found, so nothing was checked." 1>&2
    exit 1
  fi
fi

FOUND=0
FAILED=0

for dockerfile in "${DOCKERFILES[@]}"; do
  matched=0

  while IFS=: read -r line_no line; do
    matched=$((matched + 1))
    FOUND=$((FOUND + 1))

    if [[ ! ${line} =~ ${OWNER_PATTERN} ]]; then
      echo "${dockerfile}:${line_no}: copies the dist scripts without --chown=root:root" 1>&2
      FAILED=$((FAILED + 1))
    fi

    if [[ ! ${line} =~ ${MODE_PATTERN} ]]; then
      echo "${dockerfile}:${line_no}: copies the dist scripts without --chmod=755" 1>&2
      FAILED=$((FAILED + 1))
    fi
  done < <(grep -nEi "${COPY_PATTERN}" "${dockerfile}" || true)

  # A file that names the scripts without a recognized instruction has been reshaped into a
  # form this check no longer reads, which would otherwise disable it silently.
  if [[ ${matched} -eq 0 ]] && grep -qE '^[[:space:]]*[^#]*scripts/dist' "${dockerfile}"; then
    echo "${dockerfile}: names the dist scripts in an instruction this check cannot read" 1>&2
    FAILED=$((FAILED + 1))
  fi
done

if [[ ${FAILED} -gt 0 ]]; then
  echo "Found ${FAILED} problem(s) with how the dist scripts are copied." 1>&2
  exit 1
fi

echo "Script copy modes checked (${FOUND} instruction(s) in ${#DOCKERFILES[@]} Dockerfile(s))."
