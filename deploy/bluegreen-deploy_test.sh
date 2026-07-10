#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SCRIPT="$ROOT_DIR/deploy/bluegreen-deploy.sh"
PASS_COUNT=0
FAIL_COUNT=0

fail_test() {
  printf 'not ok - %s: %s\n' "$1" "$2" >&2
  FAIL_COUNT=$((FAIL_COUNT + 1))
}

pass_test() {
  printf 'ok - %s\n' "$1"
  PASS_COUNT=$((PASS_COUNT + 1))
}

new_fixture() {
  FIXTURE_DIR="$(mktemp -d)"
  export FIXTURE_DIR
  export FAKE_CALLS="$FIXTURE_DIR/calls.log"
  export FAKE_LONG_TX_COUNT=0
  export FAKE_MIGRATION_COUNT=1
  export FAKE_BATCH_ENABLED_COUNT=0
  export FAKE_CURL_FAIL=0
  export FAKE_PG_RESTORE_FAIL=0
  export FAKE_FREE_BYTES=10737418240
  export FAKE_CAPTURED_ACTIVE_ENV="$FIXTURE_DIR/captured-active.env"
  export FAKE_ACTIVE_ENV_PATH="$FIXTURE_DIR/active-env-path.txt"
  mkdir -p "$FIXTURE_DIR/bin" "$FIXTURE_DIR/compose" "$FIXTURE_DIR/data"
  : >"$FAKE_CALLS"
  printf 'DATABASE_URL=hidden\n' >"$FIXTURE_DIR/compose/.env"
  cat >"$FIXTURE_DIR/nginx.conf" <<'EOF'
server {
  location / {
    proxy_pass http://127.0.0.1:8080;
  }
}
EOF

  cat >"$FIXTURE_DIR/bin/docker" <<'EOF'
#!/usr/bin/env bash
set -u
printf 'docker %s\n' "$*" >>"$FAKE_CALLS"

case "${1:-}" in
  ps)
    printf 'sub2api\t127.0.0.1:8080->8080/tcp\n'
    ;;
  image|pull|run|rm|stop)
    if [[ "${1:-}" == run ]]; then
      last_env_file=''
      previous=''
      for arg in "$@"; do
        if [[ "$previous" == '--env-file' ]]; then
          last_env_file="$arg"
        fi
        previous="$arg"
      done
      if [[ -n "$last_env_file" ]]; then
        cp "$last_env_file" "$FAKE_CAPTURED_ACTIVE_ENV"
        printf '%s\n' "$last_env_file" >"$FAKE_ACTIVE_ENV_PATH"
      fi
      printf 'fake-container-id\n'
    fi
    ;;
  inspect)
    case "$*" in
      *NetworkSettings.Networks*) printf 'sub2api-test-network\n' ;;
      *.Config.Env*)
        printf 'DATABASE_URL=active-database\n'
        printf 'REDIS_URL=active-redis\n'
        ;;
      *.Mounts*) : ;;
      *) : ;;
    esac
    ;;
  logs)
    :
    ;;
  exec)
    case "$*" in
      *pg_dump*) printf 'FAKE_CUSTOM_DUMP\n' ;;
      *pg_restore*)
        [[ "${FAKE_PG_RESTORE_FAIL:-0}" == 0 ]] || exit 1
        printf 'fake archive listing\n'
        ;;
      *pg_database_size*) printf '1024\n' ;;
      *pg_stat_activity*) printf '%s\n' "${FAKE_LONG_TX_COUNT:-0}" ;;
      *schema_migrations*) printf '%s\n' "${FAKE_MIGRATION_COUNT:-1}" ;;
      *allow_batch_image_generation*) printf '%s\n' "${FAKE_BATCH_ENABLED_COUNT:-0}" ;;
      *) : ;;
    esac
    ;;
  *)
    :
    ;;
esac
EOF

  cat >"$FIXTURE_DIR/bin/curl" <<'EOF'
#!/usr/bin/env bash
set -u
printf 'curl %s\n' "$*" >>"$FAKE_CALLS"
[[ "${FAKE_CURL_FAIL:-0}" == 0 ]] || exit 1
case "$*" in
  *%\{http_code\}*) printf '401' ;;
  *) : ;;
esac
EOF

  cat >"$FIXTURE_DIR/bin/nginx" <<'EOF'
#!/usr/bin/env bash
set -u
printf 'nginx %s\n' "$*" >>"$FAKE_CALLS"
EOF

  cat >"$FIXTURE_DIR/bin/df" <<'EOF'
#!/usr/bin/env bash
set -u
printf 'Filesystem 1-blocks Used Available Use%% Mounted on\n'
printf 'fake 1 1 %s 1%% /tmp\n' "${FAKE_FREE_BYTES:-10737418240}"
EOF

  cat >"$FIXTURE_DIR/bin/sleep" <<'EOF'
#!/usr/bin/env bash
exit 0
EOF

  chmod +x "$FIXTURE_DIR/bin/docker" "$FIXTURE_DIR/bin/curl" "$FIXTURE_DIR/bin/nginx" "$FIXTURE_DIR/bin/df" "$FIXTURE_DIR/bin/sleep"
}

cleanup_fixture() {
  rm -rf "$FIXTURE_DIR"
}

run_deploy() {
  PATH="$FIXTURE_DIR/bin:$PATH" "$SCRIPT" \
    --image fake:image \
    --nginx-conf "$FIXTURE_DIR/nginx.conf" \
    --compose-dir "$FIXTURE_DIR/compose" \
    --env-file "$FIXTURE_DIR/compose/.env" \
    --startup-wait-seconds 0 \
    --observe-seconds 0 \
    "$@"
}

test_prepare_only_creates_verified_backup_without_candidate() {
  local name='prepare-only creates verified backup without candidate'
  new_fixture
  if run_deploy --prepare-only >"$FIXTURE_DIR/output.log" 2>&1 \
    && find "$FIXTURE_DIR/compose/backups" -name database.dump -type f -size +0c | grep -q . \
    && ! grep -q '^docker run ' "$FAKE_CALLS"; then
    pass_test "$name"
  else
    fail_test "$name" "prepare-only did not create a verified backup or started a candidate"
  fi
  cleanup_fixture
}

test_loaded_local_image_prepare_only_skips_pull() {
  local name='loaded local image prepare-only skips pull'
  new_fixture
  if run_deploy --local-image --prepare-only >"$FIXTURE_DIR/output.log" 2>&1 \
    && ! grep -q '^docker pull ' "$FAKE_CALLS" \
    && ! grep -q '^docker run ' "$FAKE_CALLS"; then
    pass_test "$name"
  else
    fail_test "$name" "local image preparation pulled or started a candidate"
  fi
  cleanup_fixture
}

test_insufficient_disk_blocks_backup() {
  local name='insufficient disk blocks backup'
  new_fixture
  export FAKE_FREE_BYTES=1024
  if ! run_deploy --prepare-only >"$FIXTURE_DIR/output.log" 2>&1 \
    && grep -qi 'insufficient free space' "$FIXTURE_DIR/output.log" \
    && ! grep -q 'pg_dump' "$FAKE_CALLS" \
    && ! grep -q '^docker run ' "$FAKE_CALLS"; then
    pass_test "$name"
  else
    fail_test "$name" "pg_dump or candidate started without backup headroom"
  fi
  cleanup_fixture
}

test_backup_validation_failure_blocks_candidate() {
  local name='backup validation failure blocks candidate'
  new_fixture
  export FAKE_PG_RESTORE_FAIL=1
  if ! run_deploy >"$FIXTURE_DIR/output.log" 2>&1 \
    && grep -q 'pg_restore' "$FAKE_CALLS" \
    && grep -qi 'database backup validation failed' "$FIXTURE_DIR/output.log" \
    && ! grep -q '^docker run ' "$FAKE_CALLS"; then
    pass_test "$name"
  else
    fail_test "$name" "candidate started after backup validation failure"
  fi
  cleanup_fixture
}

test_long_transaction_blocks_backup_and_candidate() {
  local name='long transaction blocks backup and candidate'
  new_fixture
  export FAKE_LONG_TX_COUNT=1
  if ! run_deploy --prepare-only >"$FIXTURE_DIR/output.log" 2>&1 \
    && grep -q 'pg_stat_activity' "$FAKE_CALLS" \
    && grep -qi 'long-running transaction' "$FIXTURE_DIR/output.log" \
    && ! grep -q 'pg_dump' "$FAKE_CALLS" \
    && ! grep -q '^docker run ' "$FAKE_CALLS"; then
    pass_test "$name"
  else
    fail_test "$name" "backup or candidate started while a long transaction existed"
  fi
  cleanup_fixture
}

test_failed_candidate_is_removed() {
  local name='failed candidate is removed'
  new_fixture
  export FAKE_CURL_FAIL=1
  if ! run_deploy >"$FIXTURE_DIR/output.log" 2>&1 \
    && [[ "$(grep -c '^docker rm -f sub2api-next' "$FAKE_CALLS" || true)" -ge 2 ]] \
    && ! grep -q '^nginx -s reload' "$FAKE_CALLS"; then
    pass_test "$name"
  else
    fail_test "$name" "failed candidate was not cleaned up before switching nginx"
  fi
  cleanup_fixture
}

test_missing_expected_migration_blocks_switch() {
  local name='missing expected migration blocks switch'
  new_fixture
  export FAKE_MIGRATION_COUNT=0
  if ! run_deploy >"$FIXTURE_DIR/output.log" 2>&1 \
    && [[ "$(grep -c '^docker rm -f sub2api-next' "$FAKE_CALLS" || true)" -ge 2 ]] \
    && ! grep -q '^nginx -s reload' "$FAKE_CALLS"; then
    pass_test "$name"
  else
    fail_test "$name" "nginx switched without the expected migration"
  fi
  cleanup_fixture
}

test_candidate_inherits_active_container_environment_without_leaving_temp_file() {
  local name='candidate inherits active container environment without leaving temp file'
  new_fixture
  if run_deploy >"$FIXTURE_DIR/output.log" 2>&1 \
    && grep -qx 'DATABASE_URL=active-database' "$FAKE_CAPTURED_ACTIVE_ENV" \
    && grep -qx 'REDIS_URL=active-redis' "$FAKE_CAPTURED_ACTIVE_ENV" \
    && [[ -s "$FAKE_ACTIVE_ENV_PATH" ]] \
    && [[ ! -e "$(cat "$FAKE_ACTIVE_ENV_PATH")" ]]; then
    pass_test "$name"
  else
    fail_test "$name" "candidate did not inherit the active environment or the temporary file remained"
  fi
  cleanup_fixture
}

test_prepare_only_creates_verified_backup_without_candidate
test_loaded_local_image_prepare_only_skips_pull
test_insufficient_disk_blocks_backup
test_backup_validation_failure_blocks_candidate
test_long_transaction_blocks_backup_and_candidate
test_failed_candidate_is_removed
test_missing_expected_migration_blocks_switch
test_candidate_inherits_active_container_environment_without_leaving_temp_file

printf 'tests: %s passed, %s failed\n' "$PASS_COUNT" "$FAIL_COUNT"
[[ "$FAIL_COUNT" -eq 0 ]]
