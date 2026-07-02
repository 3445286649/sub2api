#!/usr/bin/env bash
set -Eeuo pipefail

IMAGE=""
TARGET_PORT=""
ACTIVE_PORT=""
NGINX_CONF="${NGINX_CONF:-/etc/nginx/conf.d/subapi.loucer.cn.conf}"
NGINX_TEMPLATE="${NGINX_TEMPLATE:-}"
COMPOSE_DIR="${COMPOSE_DIR:-/root/sub2api-deploy}"
ENV_FILE="${ENV_FILE:-}"
NETWORK="${NETWORK:-}"
CONTAINER_PREFIX="${CONTAINER_PREFIX:-sub2api}"
APP_PORT="${APP_PORT:-8080}"
OBSERVE_SECONDS="${OBSERVE_SECONDS:-120}"
SMOKE_HOST="${SMOKE_HOST:-127.0.0.1}"
STOP_OLD_AFTER_OBSERVE="${STOP_OLD_AFTER_OBSERVE:-1}"
DRY_RUN=0

usage() {
  cat <<'EOF'
Usage:
  deploy/bluegreen-deploy.sh --image IMAGE [options]

Options:
  --image IMAGE              New image tag to deploy.
  --target-port PORT         Target host port. Defaults to the opposite side of active 8080/8081.
  --active-port PORT         Override detected active port.
  --nginx-conf PATH          Nginx vhost config. Default: /etc/nginx/conf.d/subapi.loucer.cn.conf
  --nginx-template PATH      Deprecated fallback template; normal mode edits proxy_pass port in existing config.
  --compose-dir PATH         Deployment directory used for backups and .env discovery.
  --env-file PATH            Optional env file passed to docker run. Contents are never printed.
  --network NAME             Docker network. Defaults to active container network if detectable.
  --observe-seconds N        Post-switch observation window. Default: 120.
  --dry-run                  Print plan only; do not mutate docker/nginx.
  -h, --help                 Show this help.

The script switches nginx only after the target container passes smoke checks.
By default it stops the old app container after the observation window, keeping
the stopped container/image as rollback point.
EOF
}

log() { printf '[bluegreen] %s\n' "$*"; }
fail() { printf '[bluegreen][ERROR] %s\n' "$*" >&2; exit 1; }

while [[ $# -gt 0 ]]; do
  case "$1" in
    --image) IMAGE="${2:-}"; shift 2 ;;
    --target-port) TARGET_PORT="${2:-}"; shift 2 ;;
    --active-port) ACTIVE_PORT="${2:-}"; shift 2 ;;
    --nginx-conf) NGINX_CONF="${2:-}"; shift 2 ;;
    --nginx-template) NGINX_TEMPLATE="${2:-}"; shift 2 ;;
    --compose-dir) COMPOSE_DIR="${2:-}"; shift 2 ;;
    --env-file) ENV_FILE="${2:-}"; shift 2 ;;
    --network) NETWORK="${2:-}"; shift 2 ;;
    --observe-seconds) OBSERVE_SECONDS="${2:-}"; shift 2 ;;
    --stop-old-after-observe) STOP_OLD_AFTER_OBSERVE=1; shift ;;
    --keep-old-running) STOP_OLD_AFTER_OBSERVE=0; shift ;;
    --dry-run) DRY_RUN=1; shift ;;
    -h|--help) usage; exit 0 ;;
    *) fail "unknown argument: $1" ;;
  esac
done

[[ -n "$IMAGE" ]] || fail "--image is required"
[[ "$OBSERVE_SECONDS" =~ ^[0-9]+$ ]] || fail "--observe-seconds must be an integer"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if [[ -z "$NGINX_TEMPLATE" ]]; then
  NGINX_TEMPLATE="$SCRIPT_DIR/nginx-sub2api-bluegreen.conf.tmpl"
fi

detect_active_port() {
  if [[ -n "$ACTIVE_PORT" ]]; then
    printf '%s' "$ACTIVE_PORT"
    return
  fi
  [[ -f "$NGINX_CONF" ]] || fail "nginx config not found: $NGINX_CONF"
  local port
  local ports
  ports="$(grep -E '^[[:space:]]*proxy_pass[[:space:]]+http://127\.0\.0\.1:(8080|8081)' "$NGINX_CONF" | sed -E 's/.*127\.0\.0\.1:(8080|8081).*/\1/' | sort -u)"
  [[ "$(printf '%s\n' "$ports" | sed '/^$/d' | wc -l)" -le 1 ]] || fail "multiple active upstream ports found in $NGINX_CONF; please resolve manually"
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

detect_network() {
  if [[ -n "$NETWORK" ]]; then
    printf '%s' "$NETWORK"
    return
  fi
  local active_name="$1"
  docker inspect "$active_name" --format '{{range $name, $_ := .NetworkSettings.Networks}}{{println $name}}{{end}}' 2>/dev/null | head -n 1 || true
}

container_on_port() {
  local port="$1"
  docker ps --format '{{.Names}}\t{{.Ports}}' | awk -v p="127.0.0.1:${port}->" '$0 ~ p {print $1; exit}'
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
  log "smoke admin route on ${SMOKE_HOST}:${port}"
  curl -fsS --max-time 10 "http://${SMOKE_HOST}:${port}/admin" >/dev/null || curl -fsSI --max-time 10 "http://${SMOKE_HOST}:${port}/admin" >/dev/null || return 1
  log "smoke invalid login endpoint on ${SMOKE_HOST}:${port}"
  local code
  code="$(curl -sS -o /dev/null -w '%{http_code}' --max-time 10 -H 'Content-Type: application/json' -d '{"username":"__smoke__","password":"__smoke__"}' "http://${SMOKE_HOST}:${port}/api/v1/auth/login" || true)"
  [[ "$code" =~ ^(400|401|403|422)$ ]] || return 1
}

switch_nginx_port() {
  local port="$1"
  [[ -f "$NGINX_CONF" ]] || fail "nginx config not found: $NGINX_CONF"
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
	if [[ -n "${TARGET_PORT_OCCUPIER:-}" && "$TARGET_PORT_OCCUPIER" != "$TARGET_CONTAINER" ]]; then
		docker inspect "$TARGET_PORT_OCCUPIER" >"$backup_dir/${TARGET_PORT_OCCUPIER}.inspect.json" 2>/dev/null || true
	fi
	log "backup saved to $backup_dir"
}

rollback_nginx() {
  local backup_conf="$1"
  log "rolling nginx back to port $ACTIVE_PORT"
  cp "$backup_conf" "$NGINX_CONF"
  nginx -t
  nginx -s reload
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
if [[ -z "$ENV_FILE" && -f "$COMPOSE_DIR/.env" ]]; then
	ENV_FILE="$COMPOSE_DIR/.env"
fi
NETWORK="$(detect_network "$ACTIVE_CONTAINER")"
BACKUP_DIR="$COMPOSE_DIR/backups/bluegreen-$(date +%Y%m%d-%H%M%S)"

log "image: $IMAGE"
log "active port: $ACTIVE_PORT ($ACTIVE_CONTAINER)"
log "target port: $TARGET_PORT ($TARGET_CONTAINER)"
if [[ -n "$TARGET_PORT_OCCUPIER" ]]; then
	log "target port currently occupied by: $TARGET_PORT_OCCUPIER (will be replaced after backup)"
fi
log "nginx conf: $NGINX_CONF"
log "nginx template: $NGINX_TEMPLATE"
log "compose dir: $COMPOSE_DIR"
[[ -n "$ENV_FILE" ]] && log "env file: $ENV_FILE (contents hidden)"
[[ -n "$NETWORK" ]] && log "docker network: $NETWORK"

if [[ "$DRY_RUN" == "1" ]]; then
  log "dry-run enabled; no changes will be made"
  exit 0
fi

backup_state "$BACKUP_DIR"

log "pulling image"
docker pull "$IMAGE"

log "starting target container $TARGET_CONTAINER"
docker rm -f "$TARGET_CONTAINER" >/dev/null 2>&1 || true
if [[ -n "$TARGET_PORT_OCCUPIER" && "$TARGET_PORT_OCCUPIER" != "$TARGET_CONTAINER" ]]; then
	log "removing stale target-port container $TARGET_PORT_OCCUPIER"
	docker rm -f "$TARGET_PORT_OCCUPIER" >/dev/null
fi
docker_args=(run -d --name "$TARGET_CONTAINER" --restart unless-stopped -p "127.0.0.1:${TARGET_PORT}:${APP_PORT}")
if [[ -n "$ENV_FILE" ]]; then
  docker_args+=(--env-file "$ENV_FILE")
fi
if [[ -n "$NETWORK" ]]; then
  docker_args+=(--network "$NETWORK")
fi
if docker inspect "$ACTIVE_CONTAINER" >/dev/null 2>&1; then
  while IFS=$'\t' read -r mount_type source target rw; do
    [[ -n "$mount_type" && -n "$target" ]] || continue
    mount_arg="type=${mount_type},target=${target}"
    if [[ "$mount_type" == "volume" ]]; then
      mount_arg="${mount_arg},source=${source}"
    elif [[ "$mount_type" == "bind" ]]; then
      mount_arg="${mount_arg},source=${source}"
    else
      continue
    fi
    if [[ "$rw" != "true" ]]; then
      mount_arg="${mount_arg},readonly"
    fi
    docker_args+=(--mount "$mount_arg")
  done < <(docker inspect "$ACTIVE_CONTAINER" --format '{{range .Mounts}}{{println .Type "\t" .Source "\t" .Destination "\t" .RW}}{{end}}')
fi
docker_args+=("$IMAGE")
docker "${docker_args[@]}" >/dev/null

log "waiting for target container"
for _ in $(seq 1 30); do
  if smoke_target "$TARGET_PORT"; then
    break
  fi
  sleep 2
done
smoke_target "$TARGET_PORT"

log "switching nginx proxy_pass to target port $TARGET_PORT"
switch_nginx_port "$TARGET_PORT"
nginx -t
nginx -s reload

log "observing switched traffic for ${OBSERVE_SECONDS}s"
deadline=$((SECONDS + OBSERVE_SECONDS))
while (( SECONDS < deadline )); do
  if ! smoke_target "$TARGET_PORT"; then
    rollback_nginx "$BACKUP_DIR/nginx.conf"
    fail "post-switch smoke failed; nginx rolled back to $ACTIVE_PORT"
  fi
  sleep 10
done

if [[ "$STOP_OLD_AFTER_OBSERVE" == "1" && -n "$ACTIVE_CONTAINER" ]]; then
  log "stopping old container $ACTIVE_CONTAINER after successful observation"
  docker stop "$ACTIVE_CONTAINER" >/dev/null 2>&1 || true
  log "switch complete; old container $ACTIVE_CONTAINER is stopped and kept for rollback"
else
  log "switch complete; old container $ACTIVE_CONTAINER is still running"
fi
