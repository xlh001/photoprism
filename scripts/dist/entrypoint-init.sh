#!/usr/bin/env bash

# INITIALIZES CONTAINER PACKAGES AND PERMISSIONS
export PATH="/usr/local/sbin:/usr/sbin:/sbin:/usr/local/bin:/usr/bin:/bin:/scripts"

# Abort if not executed as root.
if [[ $(id -u) != "0" ]]; then
  echo "Usage: run ${0##*/} as root" 1>&2
  exit 1
fi

# Remove the variables through which GNU make accepts options and additional makefiles,
# so that only the Makefile in INIT_SCRIPTS can provide the init targets.
unset MAKEFLAGS GNUMAKEFLAGS MAKEFILES MFLAGS

# The apt targets expect a frontend that never prompts, as there is no terminal to prompt on.
export DEBIAN_FRONTEND="noninteractive"

# Resolve the scripts directory from this file, so that the lock and the environment file
# stay beside it wherever the scripts are installed.
INIT_SCRIPTS=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" 2>/dev/null && pwd -P)
INIT_LOCK="${INIT_SCRIPTS}/.init-lock"
DOCKER_ENV_FILE="${INIT_SCRIPTS}/.docker-env"

# Prefer the environment recorded when the image was built, since it is a property of the
# image rather than something to be chosen per run. Without that file, use the restricted
# settings rather than the caller's value; a source checkout can opt in by creating it.
if [[ -r ${DOCKER_ENV_FILE} ]]; then
  read -r DOCKER_ENV < "${DOCKER_ENV_FILE}"
else
  DOCKER_ENV="prod"
fi

export DOCKER_ENV

# regular expressions
re='^[0-9]+$'

# init_in_dir runs a command inside the given directory, which must be one the image
# created itself. It enters the directory first and lets the command act on ".", so the
# update applies to the directory the process is in rather than to a path that is
# resolved a second time and may no longer lead to the same place.
init_in_dir() {
  local init_dir=$1

  shift

  if [[ ! -d ${init_dir} ]] || [[ -L ${init_dir} ]]; then
    echo "init: skipping ${init_dir}" 1>&2
    return 0
  fi

  (cd -P "${init_dir}" && [[ $(pwd -P) == "${init_dir}" ]] && "$@")
}

# init_target runs a single init target from the Makefile in INIT_SCRIPTS.
# Names are limited to plain target names, as make would otherwise read them
# as an option or as a variable assignment instead.
init_target() {
  if [[ ! $1 =~ ^[a-zA-Z0-9][a-zA-Z0-9._-]*$ ]]; then
    echo "init: invalid target $1" 1>&2
    return 1
  fi

  make --no-print-directory -C "$INIT_SCRIPTS" -- "$1"
}

# detect environment
case $DOCKER_ENV in
  prod)
    export PATH="/usr/local/sbin:/usr/sbin:/sbin:/usr/local/bin:/usr/bin:/bin:/scripts:/opt/photoprism/bin";
    CHOWN_DIRS=("/photoprism/storage")
    CHMOD_DIRS=("/photoprism/storage")
    ;;

  develop)
    export PATH="/usr/local/sbin:/usr/sbin:/sbin:/usr/local/bin:/usr/bin:/bin:/scripts:/usr/local/go/bin:/go/bin:/opt/photoprism/bin";
    CHOWN_DIRS=("/photoprism" "/opt/photoprism" "/go" "/tmp/photoprism")
    CHMOD_DIRS=("/opt/photoprism" "/tmp/photoprism")
    ;;

  *)
    echo "init: unsupported environment $DOCKER_ENV";
    exit
    ;;
esac

if [[ ${PHOTOPRISM_UID} =~ $re ]] && [[ ${PHOTOPRISM_UID} != "0" ]]; then
  # Create user account if it does not exist yet (required by /usr/bin/setpriv).
  getent passwd "${PHOTOPRISM_UID}" > /dev/null
  if [ $? -eq 2 ] ; then
    userdel -r -f "user-${PHOTOPRISM_UID}" >/dev/null 2>&1
    groupdel -f "group-${PHOTOPRISM_UID}" >/dev/null 2>&1
    groupadd -f -g "${PHOTOPRISM_UID}" "group-${PHOTOPRISM_UID}"
    useradd -u "${PHOTOPRISM_UID}" -g "${PHOTOPRISM_UID}" -G photoprism,www-data,video,davfs2,renderd,render,ssl-cert,videodriver -s /bin/bash -m -d "/home/user-${PHOTOPRISM_UID}" "user-${PHOTOPRISM_UID}" 2>/dev/null
    echo "init: account with the user id ${PHOTOPRISM_UID} has been created"
  else
    echo "init: account with the user id ${PHOTOPRISM_UID} already exists"
  fi

  if [[ ${PHOTOPRISM_GID} =~ $re ]] && [[ ${PHOTOPRISM_GID} != "0" ]]; then
    CHOWN="${PHOTOPRISM_UID}:${PHOTOPRISM_GID}"
  else
    CHOWN="${PHOTOPRISM_UID}"
  fi

  if [[ -z ${PHOTOPRISM_DISABLE_CHOWN} ]] || [[ ${PHOTOPRISM_DISABLE_CHOWN} == "false" ]]; then
    echo "init: updating filesystem permissions"
    echo "PHOTOPRISM_DISABLE_CHOWN=\"true\" disables permission updates"
    for INIT_DIR in "${CHOWN_DIRS[@]}"; do
      init_in_dir "${INIT_DIR}" chown --preserve-root --silent -R "${CHOWN}" .
    done

    for INIT_DIR in "${CHMOD_DIRS[@]}"; do
      init_in_dir "${INIT_DIR}" chmod --preserve-root --silent -R u+rwX .
    done
  fi
fi

# do nothing if PHOTOPRISM_INIT was not set
if [[ -z ${PHOTOPRISM_INIT} ]]; then
  if [[ ${PHOTOPRISM_DEFAULT_TLS} = "true" ]]; then
    init_target "https"
  fi
  exit
fi

# execute targets via /usr/bin/make
if [[ ! -e ${INIT_LOCK} ]]; then
  for INIT_TARGET in $PHOTOPRISM_INIT; do
    echo "init: $INIT_TARGET"
    init_target "$INIT_TARGET"
  done

  echo 1 >"${INIT_LOCK}"
fi
