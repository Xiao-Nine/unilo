#!/usr/bin/env sh
set -eu

SCRIPT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
PROJECT_ROOT="$(CDPATH= cd -- "${SCRIPT_DIR}/.." && pwd)"

REMOTE_HOST="${REMOTE_HOST:-wang}"
REMOTE_DIR="${REMOTE_DIR:-~/unilo/data}"
COMPOSE_FILE="${COMPOSE_FILE:-docker-compose.production.yaml}"
EXECUTE=0

if [ "${1:-}" = "--execute" ]; then
  EXECUTE=1
elif [ "${1:-}" != "" ]; then
  echo "Usage: $0 [--execute]"
  exit 1
fi

echo "Deploy source: ${PROJECT_ROOT}"
echo "Deploy target: ${REMOTE_HOST}:${REMOTE_DIR}"
echo "Compose file: ${COMPOSE_FILE}"
echo "Warning: this uploads the whole project directory, including dotfiles such as .env if present."

if [ "$EXECUTE" -ne 1 ]; then
  echo "Dry run only. Re-run with --execute to scp the project directory and deploy on ${REMOTE_HOST}."
  exit 0
fi

confirm_phrase="deploy ${REMOTE_HOST}"
printf 'Type "%s" to continue: ' "$confirm_phrase"
read -r answer
if [ "$answer" != "$confirm_phrase" ]; then
  echo "Aborted."
  exit 1
fi

ssh "${REMOTE_HOST}" "mkdir -p ${REMOTE_DIR}"
scp -r "${PROJECT_ROOT}/." "${REMOTE_HOST}:${REMOTE_DIR}/"
ssh "${REMOTE_HOST}" "cd ${REMOTE_DIR} && test -f .env"
ssh "${REMOTE_HOST}" "cd ${REMOTE_DIR} && docker compose -f ${COMPOSE_FILE} config >/dev/null"
ssh "${REMOTE_HOST}" "cd ${REMOTE_DIR} && docker compose -f ${COMPOSE_FILE} up -d --build"
ssh "${REMOTE_HOST}" "cd ${REMOTE_DIR} && docker compose -f ${COMPOSE_FILE} ps"
ssh "${REMOTE_HOST}" "cd ${REMOTE_DIR} && port=\$(sed -n 's/^BACKEND_HOST_PORT=//p' .env | tail -n 1) && curl -fsS http://127.0.0.1:\${port:-8000}/healthz"
