#!/usr/bin/env bash

# Installs the sudoers drop-in that lets the entrypoint run the init script as root.
# Pass --develop in development images, where all targets and scripts may be run with sudo.
# Run this as a file in the scripts directory, as the rules name the init script beside it.

PATH="/usr/local/sbin:/usr/sbin:/sbin:/usr/local/bin:/usr/bin:/bin:/scripts:$PATH"

# Abort if not executed as root.
if [[ $(id -u) != "0" ]]; then
  echo "Usage: run ${0##*/} as root" 1>&2
  exit 1
fi

set -e

# Resolve the scripts directory from this file, so the rules and the environment file refer
# to the scripts beside it wherever they are installed.
SCRIPTS_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" 2>/dev/null && pwd -P)

if [[ ! -f ${SCRIPTS_DIR}/entrypoint-init.sh ]]; then
  echo "Error: entrypoint-init.sh not found in '${SCRIPTS_DIR}'." 1>&2
  exit 1
fi

# The rules below grant root on a path, so that path must not name something another
# account can replace: require root ownership and no write permission for group or others.
require_root_owned() {
  local target=$1 owner mode

  owner=$(stat -c '%u' "${target}")
  mode=$(stat -c '%a' "${target}")

  if [[ ${owner} != "0" ]]; then
    echo "Error: '${target}' must be owned by root." 1>&2
    return 1
  fi

  if (( 0${mode} & 0022 )); then
    echo "Error: '${target}' must not be writable by group or others." 1>&2
    return 1
  fi
}

# sudoers reads the command as a literal, and visudo accepts a path it would then split on
# whitespace or expand as a wildcard, so a name it cannot represent is rejected here instead.
if [[ ${SCRIPTS_DIR} =~ [^A-Za-z0-9/_.-] ]]; then
  echo "Error: '${SCRIPTS_DIR}' contains a character that sudoers cannot name literally." 1>&2
  exit 1
fi

require_root_owned "${SCRIPTS_DIR}"
require_root_owned "${SCRIPTS_DIR}/entrypoint-init.sh"

INIT_SCRIPT="${SCRIPTS_DIR}/entrypoint-init.sh"
SUDOERS_FILE="/etc/sudoers.d/init"

# Records the environment the image was built for, so that the init script does not have to
# take it from the caller. Written here because this is where the two image families differ.
DOCKER_ENV_FILE="${SCRIPTS_DIR}/.docker-env"
DOCKER_ENV_NAME="prod"

# Variables the init script and the targets it runs read from the environment. Only these are
# passed on, so that the caller cannot supply the ones that change how a command interprets
# its input, such as the variables GNU make accepts options and additional makefiles through.
# The proxy variables are included because the init targets download packages and models,
# and nothing else supplies them.
# DOCKER_ENV stays only as a fallback; the init script prefers the file above, so the
# variable applies only where that file is absent.
INIT_ENV="DOCKER_ENV TF_VERSION ONNX_GPU ONNX_VERSION \
http_proxy https_proxy ftp_proxy all_proxy no_proxy HTTP_PROXY HTTPS_PROXY FTP_PROXY ALL_PROXY NO_PROXY \
PHOTOPRISM_*"

if [[ $1 == "--develop" ]]; then
  # Development images allow every target and script to be run with sudo.
  SUDOERS_FILE="/etc/sudoers.d/all"
  DOCKER_ENV_NAME="develop"
  printf '%s\n' \
    "Defaults env_keep += \"${INIT_ENV}\"" \
    'ALL ALL=(ALL) NOPASSWD:SETENV: ALL' \
    > "${SUDOERS_FILE}"
else
  printf '%s\n' \
    "Cmnd_Alias PHOTOPRISM_INIT_CMND = ${INIT_SCRIPT} \"\"" \
    "Defaults!PHOTOPRISM_INIT_CMND env_keep += \"${INIT_ENV}\"" \
    'ALL ALL=(root) NOPASSWD: PHOTOPRISM_INIT_CMND' \
    > "${SUDOERS_FILE}"
fi

chmod 0440 "${SUDOERS_FILE}"

# Reject a malformed file at build time, as sudo would otherwise refuse to run anything.
visudo -c -f "${SUDOERS_FILE}"

printf '%s\n' "${DOCKER_ENV_NAME}" > "${DOCKER_ENV_FILE}"
chown root:root "${DOCKER_ENV_FILE}"
chmod 0444 "${DOCKER_ENV_FILE}"

echo "✅ Installed ${SUDOERS_FILE} and ${DOCKER_ENV_FILE} (${DOCKER_ENV_NAME})."
