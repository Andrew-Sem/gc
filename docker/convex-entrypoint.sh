#!/usr/bin/env bash

set -Eeuo pipefail

node --input-type=module -e '
  import { parseRuntimeEnv } from "/opt/garbage-collector/runtime-env.mjs";
  try {
    parseRuntimeEnv(process.env);
  } catch (error) {
    process.stderr.write(`${error instanceof Error ? error.message : "Invalid runtime environment"}\n`);
    process.exit(1);
  }
'

backend_pid=""
stop_backend() {
  if [[ -n "$backend_pid" ]] && kill -0 "$backend_pid" 2>/dev/null; then
    kill -INT "$backend_pid" 2>/dev/null || true
    wait "$backend_pid" || true
  fi
}
trap stop_backend EXIT INT TERM

/convex/run_backend.sh &
backend_pid=$!

for _ in $(seq 1 60); do
  if ! kill -0 "$backend_pid" 2>/dev/null; then
    wait "$backend_pid"
    exit 1
  fi
  if curl --fail --silent http://127.0.0.1:3210/version >/dev/null; then
    break
  fi
  sleep 1
done

if ! curl --fail --silent http://127.0.0.1:3210/version >/dev/null; then
  echo "Convex backend did not become ready within 60 seconds" >&2
  exit 1
fi

export CONVEX_SELF_HOSTED_URL=http://127.0.0.1:3210
export CONVEX_SELF_HOSTED_ADMIN_KEY
CONVEX_SELF_HOSTED_ADMIN_KEY=$(/convex/generate_admin_key.sh)

convex_cli=/app/packages/convex/node_modules/.bin/convex
cd /app/packages/convex
for name in \
  BETTER_AUTH_SECRET \
  BETTER_AUTH_ALLOWED_ORIGIN \
  SECRET_MASTER_KEY \
  WORKER_TOKEN \
  CODEX_HOME_ROOT \
  PUBLIC_WEB_URL \
  GITHUB_APP_ID \
  GITHUB_APP_CLIENT_ID \
  GITHUB_APP_CLIENT_SECRET \
  GITHUB_APP_PRIVATE_KEY \
  GITHUB_APP_SLUG \
  GITHUB_APP_WEBHOOK_SECRET; do
  printf '%s' "${!name}" | "$convex_cli" env set "$name" >/dev/null
done

"$convex_cli" dev --once --typecheck disable --codegen disable --tail-logs disable

wait "$backend_pid"
