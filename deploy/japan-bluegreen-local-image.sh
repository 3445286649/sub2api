#!/usr/bin/env bash
set -Eeuo pipefail

IMAGE=""
ACTIVE_PORT=""
TARGET_PORT=""
NGINX_CONF="${NGINX_CONF:-/etc/nginx/conf.d/subapi.loucer.cn.conf}"
COMPOSE_DIR="${COMPOSE_DIR:-/root/sub2api-deploy}"
LOG_DIR="${LOG_DIR:-/root/sub2api-deploy/logs}"
DATA_DIR="${DATA_DIR:-/root/sub2api-deploy/data}"
APP_DATA_TARGET="${APP_DATA_TARGET:-/app/data}"
CONTAINER_PREFIX="${CONTAINER_PREFIX:-sub2api}"
APP_PORT="${APP_PORT:-8080}"
SMOKE_HOST="${SMOKE_HOST:-127.0.0.1}"
OBSERVE_SECONDS="${OBSERVE_SECONDS:-60}"
NETWORK="${NETWORK:-}"
STOP_OLD_AFTER_OBSERVE="${STOP_OLD_AFTER_OBSERVE:-1}"
DRY_RUN=0
PUBLIC_HEALTH_URLS=()
BACKUP_DIR=""
ACTIVE_CONTAINER=""
TARGET_CONTAINER=""
SWITCHED=0
TEMP_ENV_FILE=""

usage() {
  cat <<'EOF'
Usage:
  deploy/japan-bluegreen-local-image.sh --image IMAGE [options]

Deploy a locally loaded Sub2API image on the Japan 8080 <-> 8081 blue/green
slots. This script never runs docker pull. Load the image on the server before
running it.

Options:
  --image IMAGE              Local Docker image tag to deploy. Required.
  --active-port PORT         Override detected active nginx port.
  --target-port PORT         Override target host port. Defaults to opposite 8080/8081.
  --nginx-conf PATH          Nginx vhost config. Default: /etc/nginx/conf.d/subapi.loucer.cn.conf
  --compose-dir PATH         Deployment directory used for backups. Default: /root/sub2api-deploy
  --log-dir PATH             Log directory. Default: /root/sub2api-deploy/logs
  --data-dir PATH            Host data directory mounted into the app. Default: /root/sub2api-deploy/data
  --app-data-target PATH     Container data mount target. Default: /app/data
  --network NAME             Docker network. Defaults to active container network.
  --observe-seconds N        Post-switch observation window. Default: 60.
  --public-health URL        Extra public health URL to check after switch. Can repeat.
  --keep-old-running         Do not stop the old container after successful observation.
  --dry-run                  Print detected plan only; do not mutate docker/nginx.
  -h, --help                 Show this help.

Safety notes:
  - The target port must be idle before deployment.
  - The script backs up nginx config and container inspect before changes.
  - If post-switch smoke fails, nginx is restored to the original config.
  - The old container is stopped only after successful observation and is kept
    as a rollback point.
EOF
}

log() { printf '[jp-bluegreen] %s\n' "$*"; }
fail() { printf '[jp-bluegreen][ERROR] %s\n' "$*" >&2; exit 1; }

while [[ $# -gt 0 ]]; do
  case "$1" in
    --image) IMAGE="${2:-}"; shift 2 ;;
    --active-port) ACTIVE_PORT="${2:-}"; shift 2 ;;
    --target-port) TARGET_PORT="${2:-}"; shift 2 ;;
    --nginx-conf) NGINX_CONF="${2:-}"; shift 2 ;;
    --compose-dir) COMPOSE_DIR="${2:-}"; shift 2 ;;
    --log-dir) LOG_DIR="${2:-}"; shift 2 ;;
    --data-dir) DATA_DIR="${2:-}"; shift 2 ;;
    --app-data-target) APP_DATA_TARGET="${2:-}"; shift 2 ;;
    --network) NETWORK="${2:-}"; shift 2 ;;
    --observe-seconds) OBSERVE_SECONDS="${2:-}"; shift 2 ;;
    --public-health) PUBLIC_HEALTH_URLS+=("${2:-}"); shift 2 ;;
    --keep-old-running) STOP_OLD_AFTER_OBSERVE=0; shift ;;
    --dry-run) DRY_RUN=1; shift ;;
    -h|--help) usage; exit 0 ;;
    *) fail "unknown argument: $1" ;;
  esac
done

[[ -n "$IMAGE" ]] || fail "--image is required"
[[ "$OBSERVE_SECONDS" =~ ^[0-9]+$ ]] || fail "--observe-seconds must be an integer"

mkdir -p "$LOG_DIR"
LOG_FILE="$LOG_DIR/japan-bluegreen-$(date +%Y%m%d-%H%M%S).log"
exec > >(tee -a "$LOG_FILE") 2>&1

cleanup() {
  if [[ -n "${TEMP_ENV_FILE:-}" && -f "$TEMP_ENV_FILE" ]]; then
    rm -f "$TEMP_ENV_FILE"
  fi
}

rollback_nginx() {
  local backup_conf="$1"
  [[ -f "$backup_conf" ]] || return 1
  log "rolling nginx back from backup: $backup_conf"
  cp "$backup_conf" "$NGINX_CONF"
  nginx -t
  nginx -s reload
}

on_exit() {
  local rc=$?
  if [[ "$rc" -ne 0 && "${SWITCHED:-0}" == "1" && -n "${BACKUP_DIR:-}" ]]; then
    rollback_nginx "$BACKUP_DIR/nginx.conf" || true
  fi
  cleanup
  exit "$rc"
}
trap on_exit EXIT

detect_active_port() {
  if [[ -n "$ACTIVE_PORT" ]]; then
    printf '%s' "$ACTIVE_PORT"
    return
  fi
  [[ -f "$NGINX_CONF" ]] || fail "nginx config not found: $NGINX_CONF"
  local ports
  ports="$(grep -E '^[[:space:]]*proxy_pass[[:space:]]+http://127\.0\.0\.1:(8080|8081)' "$NGINX_CONF" | sed -E 's/.*127\.0\.0\.1:(8080|8081).*/\1/' | sort -u)"
  [[ "$(printf '%s\n' "$ports" | sed '/^$/d' | wc -l)" -le 1 ]] || fail "multiple active upstream ports found in $NGINX_CONF"
  local port
  port="$(printf '%s\n' "$ports" | sed '/^$/d' | head -n 1)"
  [[ -n "$port" ]] || fail "could not detect active 8080/8081 port from $NGINX_CONF"
  printf '%s' "$port"
}

opposite_port() {
  case "$1" in
    8080) printf '8081' ;;
    8081) printf '8080' ;;
    *) fail "active port must be 8080 or 8081 when --target-port is omitted" ;;
  esac
}

container_name_for_port() {
  if [[ "$1" == "8081" ]]; then
    printf '%s-next' "$CONTAINER_PREFIX"
  else
    printf '%s' "$CONTAINER_PREFIX"
  fi
}

container_on_port() {
  local port="$1"
  docker ps --format '{{.Names}}\t{{.Ports}}' | awk -v p=":${port}->" '$0 ~ p {print $1; exit}'
}

detect_network() {
  if [[ -n "$NETWORK" ]]; then
    printf '%s' "$NETWORK"
    return
  fi
  docker inspect "$ACTIVE_CONTAINER" --format '{{range $name, $_ := .NetworkSettings.Networks}}{{println $name}}{{end}}' 2>/dev/null | head -n 1 || true
}

curl_ok() {
  local url="$1"
  curl -fsS --max-time 10 "$url" >/dev/null
}

smoke_target() {
  local port="$1"
  log "smoke /health on ${SMOKE_HOST}:${port}"
  curl_ok "http://${SMOKE_HOST}:${port}/health" || return 1

  log "smoke / on ${SMOKE_HOST}:${port}"
  curl -fsSI --max-time 10 "http://${SMOKE_HOST}:${port}/" >/dev/null || return 1

  log "smoke /admin on ${SMOKE_HOST}:${port}"
  curl -fsSI --max-time 10 "http://${SMOKE_HOST}:${port}/admin" >/dev/null || curl -fsS --max-time 10 "http://${SMOKE_HOST}:${port}/admin" >/dev/null || return 1

  log "smoke invalid login endpoint on ${SMOKE_HOST}:${port}"
  local code
  code="$(curl -sS -o /dev/null -w '%{http_code}' --max-time 10 -H 'Content-Type: application/json' -d '{"username":"__smoke__","password":"__smoke__"}' "http://${SMOKE_HOST}:${port}/api/v1/auth/login" || true)"
  [[ "$code" =~ ^(400|401|403|422)$ ]] || return 1
}

smoke_public_urls() {
  local url
  for url in "${PUBLIC_HEALTH_URLS[@]}"; do
    [[ -n "$url" ]] || continue
    log "smoke public health: $url"
    curl_ok "$url" || return 1
  done
}

switch_nginx_port() {
  local port="$1"
  perl -0pi -e "s#(proxy_pass\\s+http://127\\.0\\.0\\.1:)(8080|8081)#\${1}${port}#g" "$NGINX_CONF"
}

backup_state() {
  local backup_dir="$1"
  mkdir -m 700 -p "$backup_dir"
  cp "$NGINX_CONF" "$backup_dir/nginx.conf"
  if [[ -d "$COMPOSE_DIR" ]]; then
    find "$COMPOSE_DIR" -maxdepth 1 \( -name 'docker-compose*.yml' -o -name 'compose*.yml' -o -name '.env' \) -exec cp -p {} "$backup_dir/" \;
    find "$backup_dir" -maxdepth 1 -name '.env' -exec chmod 600 {} \;
  fi
  docker inspect "$ACTIVE_CONTAINER" >"$backup_dir/${ACTIVE_CONTAINER}.inspect.json" 2>/dev/null || true
  docker inspect "$TARGET_CONTAINER" >"$backup_dir/${TARGET_CONTAINER}.inspect.json" 2>/dev/null || true
  log "backup saved to $backup_dir"
}

ACTIVE_PORT="$(detect_active_port)"
if [[ -z "$TARGET_PORT" ]]; then
  TARGET_PORT="$(opposite_port "$ACTIVE_PORT")"
fi
[[ "$TARGET_PORT" =~ ^[0-9]+$ ]] || fail "--target-port must be numeric"
[[ "$ACTIVE_PORT" != "$TARGET_PORT" ]] || fail "target port equals active port"

ACTIVE_CONTAINER="$(container_on_port "$ACTIVE_PORT")"
if [[ -z "$ACTIVE_CONTAINER" ]]; then
  ACTIVE_CONTAINER="$(container_name_for_port "$ACTIVE_PORT")"
fi
TARGET_CONTAINER="$(container_name_for_port "$TARGET_PORT")"
TARGET_PORT_OCCUPIER="$(container_on_port "$TARGET_PORT")"
NETWORK="$(detect_network)"
BACKUP_DIR="$COMPOSE_DIR/backups/japan-bluegreen-$(date +%Y%m%d-%H%M%S)"

log "log file: $LOG_FILE"
log "image: $IMAGE"
log "active port: $ACTIVE_PORT ($ACTIVE_CONTAINER)"
log "target port: $TARGET_PORT ($TARGET_CONTAINER)"
log "nginx conf: $NGINX_CONF"
log "compose dir: $COMPOSE_DIR"
log "data mount: $DATA_DIR -> $APP_DATA_TARGET"
[[ -n "$NETWORK" ]] && log "docker network: $NETWORK"

[[ -f "$NGINX_CONF" ]] || fail "nginx config not found: $NGINX_CONF"
[[ -d "$DATA_DIR" ]] || fail "data dir not found: $DATA_DIR"
docker image inspect "$IMAGE" >/dev/null || fail "local image not found: $IMAGE"
docker inspect "$ACTIVE_CONTAINER" >/dev/null || fail "active container not found: $ACTIVE_CONTAINER"
[[ -n "$NETWORK" ]] || fail "docker network could not be detected; pass --network"
if [[ -n "$TARGET_PORT_OCCUPIER" ]]; then
  fail "target port $TARGET_PORT is occupied by running container: $TARGET_PORT_OCCUPIER"
fi

if [[ "$DRY_RUN" == "1" ]]; then
  log "dry-run enabled; no changes will be made"
  exit 0
fi

backup_state "$BACKUP_DIR"

log "starting target container $TARGET_CONTAINER from local image"
docker rm -f "$TARGET_CONTAINER" >/dev/null 2>&1 || true
TEMP_ENV_FILE="$(mktemp /tmp/sub2api-blue-env.XXXXXX)"
chmod 600 "$TEMP_ENV_FILE"
docker inspect "$ACTIVE_CONTAINER" --format '{{range .Config.Env}}{{println .}}{{end}}' >"$TEMP_ENV_FILE"
docker run -d \
  --name "$TARGET_CONTAINER" \
  --restart unless-stopped \
  -p "127.0.0.1:${TARGET_PORT}:${APP_PORT}" \
  --env-file "$TEMP_ENV_FILE" \
  --network "$NETWORK" \
  --mount "type=bind,source=${DATA_DIR},target=${APP_DATA_TARGET}" \
  "$IMAGE" >/dev/null

log "waiting for target container health"
for _ in $(seq 1 30); do
  if smoke_target "$TARGET_PORT"; then
    break
  fi
  sleep 2
done
smoke_target "$TARGET_PORT"

log "checking target logs for startup blockers"
if docker logs --since 5m "$TARGET_CONTAINER" 2>&1 | grep -Eiq 'panic|fatal|migration.*fail|failed to migrate|setup wizard|First run|database.*error'; then
  docker logs --since 5m "$TARGET_CONTAINER" 2>&1 | grep -Ei 'panic|fatal|migration.*fail|failed to migrate|setup wizard|First run|database.*error' || true
  fail "target logs contain startup blocker keywords"
fi

log "switching nginx proxy_pass to target port $TARGET_PORT"
switch_nginx_port "$TARGET_PORT"
nginx -t
nginx -s reload
SWITCHED=1

log "observing switched traffic for ${OBSERVE_SECONDS}s"
deadline=$((SECONDS + OBSERVE_SECONDS))
while (( SECONDS < deadline )); do
  smoke_target "$TARGET_PORT" || fail "post-switch local smoke failed"
  smoke_public_urls || fail "post-switch public smoke failed"
  if docker logs --since 2m "$TARGET_CONTAINER" 2>&1 | grep -Eiq 'panic|fatal|migration.*fail|failed to migrate|database.*error'; then
    docker logs --since 2m "$TARGET_CONTAINER" 2>&1 | grep -Ei 'panic|fatal|migration.*fail|failed to migrate|database.*error' || true
    fail "post-switch logs contain blocker keywords"
  fi
  sleep 10
done

if [[ "$STOP_OLD_AFTER_OBSERVE" == "1" ]]; then
  log "stopping old container $ACTIVE_CONTAINER after successful observation"
  docker stop "$ACTIVE_CONTAINER" >/dev/null 2>&1 || true
  log "switch complete; old container $ACTIVE_CONTAINER is stopped and kept for rollback"
else
  log "switch complete; old container $ACTIVE_CONTAINER is still running"
fi
