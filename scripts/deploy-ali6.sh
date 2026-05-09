#!/usr/bin/env sh
set -eu

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
REMOTE_HOST="${REMOTE_HOST:-ali6}"
export REMOTE_HOST

exec "${SCRIPT_DIR}/deploy-remote.sh" "$@"
