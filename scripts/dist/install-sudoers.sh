#!/usr/bin/env bash

# Installs the sudoers drop-in that lets the entrypoint run the init script as root.
# Pass --develop in development images, where all targets and scripts may be run with sudo.
# The rules name the init script where it actually is, so this must be run from its directory.

PATH="/usr/local/sbin:/usr/sbin:/sbin:/usr/local/bin:/usr/bin:/bin:/scripts:$PATH"

# Abort if not executed as root.
if [[ $(id -u) != "0" ]]; then
  echo "Usage: run ${0##*/} as root" 1>&2
  exit 1
fi

set -e

# Resolve the scripts directory from this file rather than assuming one, as the scripts are
# also shipped in installation packages that may place them elsewhere.
SCRIPTS_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" 2>/dev/null && pwd -P)

if [[ ! -f ${SCRIPTS_DIR}/entrypoint-init.sh ]]; then
  echo "Error: entrypoint-init.sh not found in '${SCRIPTS_DIR}'." 1>&2
  exit 1
fi

INIT_SCRIPT="${SCRIPTS_DIR}/entrypoint-init.sh"
SUDOERS_FILE="/etc/sudoers.d/init"

# Records the environment the image was built for, so that the init script does not have to
# take it from the caller. Written here because this is where the two image families differ.
DOCKER_ENV_FILE="${SCRIPTS_DIR}/.docker-env"
DOCKER_ENV_NAME="prod"

# Variables the init script and the targets it runs read from the environment. Only these are
# passed on, so that the caller cannot supply the ones that change how a command interprets
# its input, such as the variables GNU make accepts options and additional makefiles through.
# The proxy variables are included because the init targets download packages and models, and
# nothing else provides them. Package integrity rests on the apt signature check rather than on
# the transport, as the distribution sources are plain HTTP.
# DOCKER_ENV stays only as a fallback for an image built before the file above existed; the
# init script prefers the file, so a caller cannot select the other environment's settings.
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
